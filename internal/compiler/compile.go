package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/permissions"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// BudgetUsage reports token budget consumption for the compiled context.
type BudgetUsage struct {
	TokenBudget int `json:"token_budget"`
	TokensUsed  int `json:"tokens_used"`
}

// Result is the output of a compile pass (no LLM).
type Result struct {
	Contract      TaskContract `json:"contract"`
	Context       string       `json:"compiled_context"`
	RetrievedIDs  []string     `json:"retrieved_ids,omitempty"` // ranked candidates before budget-fit
	SelectedIDs   []string     `json:"selected_ids"`
	ExcludedIDs   []string     `json:"excluded_ids"`
	Audit         []AuditEntry `json:"audit"`
	Budget        BudgetUsage  `json:"budget_usage"`
}

// Options configures compile, budget, permission hooks, and ablation toggles.
type Options struct {
	TokenBudget int
	Permissions permissions.Policy
	Ablation    Ablation
}

// Compile ranks and packs entities for a task contract with audit and budget enforcement.
func Compile(entities []state.Entity, contract TaskContract, opts Options) Result {
	contract = contract.Normalize()
	if opts.TokenBudget <= 0 {
		opts.TokenBudget = 2000
	}

	contract = applyAblationContract(contract, opts.Ablation)

	allowed, permDenials := permissions.Filter(entities, opts.Permissions)
	ranked, below, noiseExcluded := rankByContract(allowed, contract, opts.Ablation)

	retrieved := make([]string, 0, len(ranked))
	for _, r := range ranked {
		retrieved = append(retrieved, r.e.ID)
	}

	packed, selected, fitAudit := budgetFit(ranked, opts.TokenBudget, opts.Ablation.NoBudgetFit)
	excluded := compilerExcludedIDs(allowed, selected)

	var audit []AuditEntry
	if !opts.Ablation.NoAudit {
		audit = buildAudit(selected, below, fitAudit, noiseExcluded)
		for _, d := range permDenials {
			audit = append([]AuditEntry{{ID: d.ID, Action: "exclude", Reason: d.Reason}}, audit...)
		}
	}

	tokensUsed := tokens.Estimate(packed)
	if tokensUsed <= 0 && packed != "" {
		tokensUsed = 1
	}

	return Result{
		Contract:     contract,
		Context:      packed,
		RetrievedIDs: retrieved,
		SelectedIDs:  selected,
		ExcludedIDs:  excluded,
		Audit:        audit,
		Budget: BudgetUsage{
			TokenBudget: opts.TokenBudget,
			TokensUsed:  tokensUsed,
		},
	}
}

type scoredEnt struct {
	e      state.Entity
	score  float64
	reason string
}

func rankByContract(entities []state.Entity, c TaskContract, ab Ablation) ([]scoredEnt, map[string]string, int) {
	if ab.NoRanking {
		var ranked []scoredEnt
		for _, e := range entities {
			if e.Meta != nil && e.Meta["noise"] == "true" {
				continue
			}
			ranked = append(ranked, scoredEnt{e: e, score: 1.0, reason: "ablation-no-ranking"})
		}
		return ranked, map[string]string{}, 0
	}
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
			noiseExcluded++
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
			if strings.Contains(titleLower, kw) || containsFold(e.Text, kw) {
				kwHits++
				score += 3.0
			}
		}
		if kwHits > 0 {
			reasons = append(reasons, fmt.Sprintf("keyword-hits=%d", kwHits))
		}

		for _, ph := range c.ProjectHints {
			if strings.Contains(titleLower, ph) || containsFold(e.Text, ph) ||
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

	if !ab.NoRefExpansion {
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
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].e.ID < ranked[j].e.ID
		}
		return ranked[i].score > ranked[j].score
	})
	return ranked, below, noiseExcluded
}

func containsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func budgetFit(ranked []scoredEnt, tokenBudget int, rankOrderOnly bool) (packed string, selected []string, fitAudit []AuditEntry) {
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
	if !rankOrderOnly {
		sort.Slice(cands, func(i, j int) bool {
			if cands[i].density == cands[j].density {
				return cands[i].score > cands[j].score
			}
			return cands[i].density > cands[j].density
		})
	}

	var b strings.Builder
	used := 0
	for _, c := range cands {
		if used+c.tok > tokenBudget && used > 0 {
			reason := fmt.Sprintf("budget-fit drop density=%.4f", c.density)
			if rankOrderOnly {
				reason = "rank-order budget drop"
			}
			fitAudit = append(fitAudit, AuditEntry{
				ID: c.e.ID, Action: "exclude", Reason: reason, Score: c.score,
			})
			continue
		}
		b.WriteString(c.e.PackText() + "\n\n")
		selected = append(selected, c.e.ID)
		used += c.tok
		includeReason := fmt.Sprintf("budget-fit keep density=%.4f score=%.2f", c.density, c.score)
		if rankOrderOnly {
			includeReason = fmt.Sprintf("rank-order keep score=%.2f", c.score)
		}
		fitAudit = append(fitAudit, AuditEntry{
			ID: c.e.ID, Action: "include", Reason: includeReason, Score: c.score,
		})
		if used >= tokenBudget {
			continue
		}
	}
	return b.String(), selected, fitAudit
}

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
