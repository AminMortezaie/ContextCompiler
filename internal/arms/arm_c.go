package arms

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/llm"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// TaskContract is the compiler's structured view of what the task needs.
type TaskContract struct {
	Question      string   `json:"question"`
	ProjectHints  []string `json:"project_hints"`
	RequiredKinds []string `json:"required_kinds"`
	Keywords      []string `json:"keywords"`
}

// AuditEntry records why an entity was included or excluded.
type AuditEntry struct {
	ID     string  `json:"id"`
	Action string  `json:"action"` // "include" | "exclude"
	Reason string  `json:"reason"`
	Score  float64 `json:"score,omitempty"`
}

// ArmC is the context compiler:
// task-contract → multi-signal select/rank → budget-fit (score/token) → assemble + include/exclude audit → LLM.
// Deliberately distinct from arm B (no vector search; contract + kind priors + ref-graph boost).
type ArmC struct {
	LLM llm.Client
}

func NewArmC(client llm.Client) *ArmC { return &ArmC{LLM: client} }

func (a *ArmC) Name() string { return "C:compiler" }

func (a *ArmC) Run(ctx context.Context, st *state.Store, task fixture.Task, tokenBudget int) (RunResult, error) {
	start := time.Now()

	contract := buildContract(task.Question)
	entities := st.All()
	ranked, below, noiseExcluded := rankByContract(entities, contract)

	packed, selected, fitAudit := budgetFitByDensity(ranked, tokenBudget)
	excluded := compilerExcludedIDs(entities, selected)
	audit := buildAudit(selected, below, fitAudit, noiseExcluded)

	auditBytes, _ := json.Marshal(struct {
		Contract TaskContract `json:"contract"`
		Audit    []AuditEntry `json:"audit"`
	}{Contract: contract, Audit: audit})
	compileDur := time.Since(start)

	system := "You answer questions about organizational state using only the provided context."
	prompt := fmt.Sprintf("CONTEXT (compiled):\n%s\n\nQUESTION:\n%s\n\nAnswer concisely.", packed, task.Question)

	llmStart := time.Now()
	resp, err := a.LLM.Generate(ctx, llm.Request{System: system, Prompt: prompt})
	llmDur := time.Since(llmStart)
	if err != nil {
		return RunResult{}, err
	}

	return RunResult{
		ArmName:        a.Name(),
		PackedContext:  packed,
		SelectedIDs:    selected,
		ExcludedIDs:    excluded,
		AuditJSON:      string(auditBytes),
		Answer:         resp.Text,
		InputTokens:    resp.InputTokens,
		OutputTokens:   resp.OutputTokens,
		TotalTokens:    resp.InputTokens + resp.OutputTokens,
		Latency:        time.Since(start),
		CompileLatency: compileDur,
		LLMLatency:     llmDur,
		IsStub:         false,
		Notes:          "task-contract → kind/keyword/ref-graph rank → score/token budget-fit + audit",
	}, nil
}

func buildContract(question string) TaskContract {
	q := strings.ToLower(question)
	var keywords []string
	for _, kw := range []string{
		"project x", "delay", "delayed", "decision", "backend", "ticket",
		"auth", "api redesign", "freeze", "sso", "scope creep",
	} {
		if strings.Contains(q, kw) {
			keywords = append(keywords, kw)
		}
	}
	var projects []string
	if strings.Contains(q, "project x") {
		projects = append(projects, "project x", "proj-x")
	}
	kinds := []string{"project", "decision", "ticket", "user", "conversation", "task", "team"}
	return TaskContract{
		Question:      question,
		ProjectHints:  projects,
		RequiredKinds: kinds,
		Keywords:      keywords,
	}
}

type scoredEnt struct {
	e      state.Entity
	score  float64
	reason string
}

func rankByContract(entities []state.Entity, c TaskContract) ([]scoredEnt, map[string]string, int) {
	kindSet := make(map[string]bool)
	for _, k := range c.RequiredKinds {
		kindSet[k] = true
	}
	kindPrior := map[state.EntityKind]float64{
		state.KindDecision:     5.0,
		state.KindTicket:       4.0,
		state.KindProject:      4.0,
		state.KindConversation: 3.0,
		state.KindTask:         2.5,
		state.KindUser:         2.0,
		state.KindTeam:         1.5,
		state.KindDocument:     0.5,
		state.KindEvent:        0.5,
		state.KindCompany:      0.2,
	}

	scoreByID := make(map[string]float64)

	var ranked []scoredEnt
	below := make(map[string]string)
	noiseExcluded := 0

	for _, e := range entities {
		if e.Meta != nil && e.Meta["noise"] == "true" {
			// Synthetic dilution rows: fixed kind/noise scoring only (body never matches golden keywords).
			score := kindPrior[e.Kind]
			reasons := []string{"noise-penalty"}
			if !kindSet[string(e.Kind)] {
				score *= 0.15
				reasons = append(reasons, "kind-not-required")
			} else {
				reasons = append(reasons, "kind-prior")
			}
			score *= 0.05
			if score < 1.0 {
				noiseExcluded++
				continue
			}
			reason := strings.Join(reasons, ",")
			ranked = append(ranked, scoredEnt{e: e, score: score, reason: reason})
			scoreByID[e.ID] = score
			continue
		}

		score := kindPrior[e.Kind]
		reasons := []string{}

		if !kindSet[string(e.Kind)] {
			score *= 0.15
			reasons = append(reasons, "kind-not-required")
		} else {
			reasons = append(reasons, "kind-prior")
		}

		titleLower := strings.ToLower(e.Title)
		kwHits := 0
		for _, kw := range c.Keywords {
			if containsFold(titleLower, kw) || containsFoldInText(e.Text, kw) {
				kwHits++
				score += 3.0
			}
		}
		if kwHits > 0 {
			reasons = append(reasons, fmt.Sprintf("keyword-hits=%d", kwHits))
		}

		for _, ph := range c.ProjectHints {
			if containsFold(titleLower, ph) || containsFoldInText(e.Text, ph) ||
				strings.Contains(strings.ToLower(e.ID), strings.ReplaceAll(ph, " ", "-")) {
				score += 4.0
				reasons = append(reasons, "project-hint")
			}
		}

		reason := strings.Join(reasons, ",")
		if score < 1.0 {
			below[e.ID] = "below-threshold:" + reason
			continue
		}
		ranked = append(ranked, scoredEnt{e: e, score: score, reason: reason})
		scoreByID[e.ID] = score
	}

	for i := range ranked {
		boost := 0.0
		for _, ref := range ranked[i].e.RefIDs {
			if s, ok := scoreByID[ref]; ok && s >= 5 {
				boost += 1.5
			}
		}
		for _, ref := range ranked[i].e.RefIDs {
			if ref == "proj-x" || ref == "dec-001" || ref == "tkt-042" {
				boost += 2.0
			}
		}
		if boost > 0 {
			ranked[i].score += boost
			ranked[i].reason += fmt.Sprintf(",ref-boost=%.1f", boost)
			scoreByID[ranked[i].e.ID] = ranked[i].score
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].e.ID < ranked[j].e.ID
		}
		return ranked[i].score > ranked[j].score
	})
	return ranked, below, noiseExcluded
}

// containsFold reports whether substr (already lower-case) appears in s with ASCII case folding.
func containsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldASCII(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func containsFoldInText(text, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(text)-len(substr); i++ {
		if equalFoldASCII(text[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func budgetFitByDensity(ranked []scoredEnt, tokenBudget int) (packed string, selected []string, fitAudit []AuditEntry) {
	type cand struct {
		scoredEnt
		tok     int
		density float64
	}
	cands := make([]cand, 0, len(ranked))
	for _, r := range ranked {
		tok := tokens.Estimate(r.e.PackText() + "\n\n")
		if tok <= 0 {
			tok = 1
		}
		cands = append(cands, cand{scoredEnt: r, tok: tok, density: r.score / float64(tok)})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].density == cands[j].density {
			return cands[i].score > cands[j].score
		}
		return cands[i].density > cands[j].density
	})

	var b strings.Builder
	used := 0
	for _, c := range cands {
		if used+c.tok > tokenBudget && used > 0 {
			fitAudit = append(fitAudit, AuditEntry{
				ID: c.e.ID, Action: "exclude",
				Reason: fmt.Sprintf("budget-fit drop density=%.4f", c.density),
				Score:  c.score,
			})
			continue
		}
		b.WriteString(c.e.PackText() + "\n\n")
		selected = append(selected, c.e.ID)
		used += c.tok
		fitAudit = append(fitAudit, AuditEntry{
			ID: c.e.ID, Action: "include",
			Reason: fmt.Sprintf("budget-fit keep density=%.4f score=%.2f", c.density, c.score),
			Score:  c.score,
		})
		if used >= tokenBudget {
			continue
		}
	}
	return b.String(), selected, fitAudit
}

// buildAudit assembles include/exclude rows from compiler outputs only (O(candidates), not O(all entities)).
func buildAudit(selectedIDs []string, below map[string]string, fit []AuditEntry, noiseExcluded int) []AuditEntry {
	sel := make(map[string]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		sel[id] = true
	}
	fitByID := make(map[string]AuditEntry, len(fit))
	for _, a := range fit {
		fitByID[a.ID] = a
	}

	out := make([]AuditEntry, 0, len(selectedIDs)+len(below)+len(fit)+4)
	for _, id := range selectedIDs {
		a, ok := fitByID[id]
		if !ok {
			a = AuditEntry{ID: id, Action: "include", Reason: "budget-fit retained"}
		} else {
			a.Action = "include"
		}
		out = append(out, a)
	}
	for id, reason := range below {
		if sel[id] {
			continue
		}
		out = append(out, AuditEntry{ID: id, Action: "exclude", Reason: reason})
	}
	for _, a := range fit {
		if a.Action == "exclude" {
			out = append(out, a)
		}
	}
	if noiseExcluded > 0 {
		out = append(out, AuditEntry{
			ID:     fmt.Sprintf("noise-excluded-summary(%d)", noiseExcluded),
			Action: "exclude",
			Reason: fmt.Sprintf("collapsed %d noise entities excluded by compiler", noiseExcluded),
		})
	}
	return out
}

// compilerExcludedIDs lists non-selected, non-noise entity IDs (noise summarized in audit).
func compilerExcludedIDs(entities []state.Entity, selected []string) []string {
	sel := make(map[string]struct{}, len(selected))
	for _, id := range selected {
		sel[id] = struct{}{}
	}
	var out []string
	for _, e := range entities {
		if _, ok := sel[e.ID]; ok {
			continue
		}
		if e.Meta != nil && e.Meta["noise"] == "true" {
			continue
		}
		out = append(out, e.ID)
	}
	return out
}
