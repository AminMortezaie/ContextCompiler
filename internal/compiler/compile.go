package compiler

import (
	"context"
	"fmt"

	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/permissions"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// Compile ranks and packs entities for a task contract with audit and budget enforcement.
// Pipeline: hybrid gather (lexical ∪ vector ∪ graph) → typed expand → one ranker
// → budget pack (nodes + edges + snippets) → deterministic sufficiency.
// No LLM. No hardcoded hub IDs.
func Compile(entities []state.Entity, contract TaskContract, opts Options) Result {
	contract = contract.Normalize()
	opts = normalizeOptions(opts)

	contract = applyAblationContract(contract, opts.Ablation)

	if opts.Graph == nil {
		opts.Graph = synthesizeGraph(entities)
		opts.Handle = memory.InlineHandle
	}

	allowed, permDenials := permissions.Filter(entities, opts.Permissions)

	gathered := gatherCandidates(allowed, contract, opts)
	expanded := expandCandidates(gathered, allowed, opts)
	ranked, below := rankCandidates(expanded.candidates, contract, expanded.edges, expanded.hopOf, opts.Ablation)

	retrieved := make([]string, 0, len(ranked))
	for _, r := range ranked {
		retrieved = append(retrieved, r.e.ID)
	}

	packed := budgetPack(ranked, expanded.edges, expanded.episodes, opts.TokenBudget, opts.Ablation.NoBudgetFit)
	excluded := compilerExcludedIDs(allowed, packed.selected)

	var audit []AuditEntry
	if !opts.Ablation.NoAudit {
		audit = buildAudit(packed.selected, below, packed.audit, noiseCount(allowed), expanded.candidates, allowed)
		for _, d := range permDenials {
			audit = append([]AuditEntry{{
				ID: d.ID, Action: "exclude", Reason: d.Reason, Source: SourcePermission,
			}}, audit...)
		}
	}

	tokensUsed := tokens.Estimate(packed.text)
	if tokensUsed <= 0 && packed.text != "" {
		tokensUsed = 1
	}

	suff := checkSufficiency(packed.selected, packed.edges, packed.text, allowed, contract)
	if !opts.Ablation.NoAudit {
		for _, miss := range suff.Missing {
			audit = append(audit, AuditEntry{
				ID: "sufficiency:" + miss, Action: "exclude", Reason: "sufficiency miss: " + miss,
			})
		}
	}

	return Result{
		Contract:      contract,
		Context:       packed.text,
		RetrievedIDs:  retrieved,
		SelectedIDs:   packed.selected,
		ExcludedIDs:   excluded,
		Audit:         audit,
		Sufficiency:   suff,
		SelectedEdges: packed.edges,
		Budget: BudgetUsage{
			TokenBudget: opts.TokenBudget,
			TokensUsed:  tokensUsed,
		},
	}
}

func normalizeOptions(opts Options) Options {
	if opts.TokenBudget <= 0 {
		opts.TokenBudget = 2000
	}
	if opts.TopK <= 0 {
		opts.TopK = 32
	}
	if opts.Hops <= 0 {
		opts.Hops = 1
	}
	if opts.Hops > memory.MaxHops {
		opts.Hops = memory.MaxHops
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	return opts
}

func synthesizeGraph(entities []state.Entity) memory.GraphStore {
	return memory.GraphFromEntities(entities, nil, nil)
}

func noiseCount(entities []state.Entity) int {
	n := 0
	for _, e := range entities {
		if e.Meta != nil && e.Meta["noise"] == "true" {
			n++
		}
	}
	return n
}

func containsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if len(s[i:]) >= len(substr) && equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, i int, substr string) bool {
	if i+len(substr) > len(s) {
		return false
	}
	a, b := s[i:i+len(substr)], substr
	for j := 0; j < len(a); j++ {
		ca, cb := a[j], b[j]
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

func buildAudit(selectedIDs []string, below map[string]string, fit []AuditEntry, noiseExcluded int, cands []candidate, allowed []state.Entity) []AuditEntry {
	sel := make(map[string]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		sel[id] = true
	}
	srcByID := make(map[string]string, len(cands))
	for _, c := range cands {
		srcByID[c.e.ID] = c.source
	}
	fitByID := make(map[string]AuditEntry, len(fit))
	for _, a := range fit {
		fitByID[a.ID] = a
	}

	out := make([]AuditEntry, 0, len(selectedIDs)+len(below)+len(fit)+4)
	for _, id := range selectedIDs {
		a, ok := fitByID[id]
		if !ok {
			a = AuditEntry{ID: id, Action: "include", Reason: "budget-fit retained", Source: srcByID[id]}
		} else {
			a.Action = "include"
			if a.Source == "" {
				a.Source = srcByID[id]
			}
		}
		out = append(out, a)
	}
	for id, reason := range below {
		if sel[id] {
			continue
		}
		out = append(out, AuditEntry{ID: id, Action: "exclude", Reason: reason, Source: srcByID[id]})
	}
	for _, a := range fit {
		if a.Action == "exclude" {
			out = append(out, a)
			continue
		}
		// Edge / snippet includes are not entity selected_ids; keep them in the audit.
		if a.Action == "include" && !sel[a.ID] && (a.EdgeID != "" || a.EpisodeID != "") {
			out = append(out, a)
		}
	}
	seen := make(map[string]bool, len(out))
	for _, a := range out {
		seen[a.ID] = true
	}
	for _, e := range allowed {
		if e.Meta != nil && e.Meta["noise"] == "true" {
			continue
		}
		if sel[e.ID] || seen[e.ID] {
			continue
		}
		src := srcByID[e.ID]
		reason := "not a hybrid candidate"
		if src != "" {
			reason = "ranked but not packed"
		}
		out = append(out, AuditEntry{ID: e.ID, Action: "exclude", Reason: reason, Source: src})
		seen[e.ID] = true
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
