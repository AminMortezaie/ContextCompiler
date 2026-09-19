package compiler

// Ablation toggles compiler pipeline stages for science-track ablation runs.
// Zero value is the full pipeline (production / Phase 2 default).
type Ablation struct {
	NoTaskContract bool // ignore derived keywords, kinds, and project hints
	NoRanking      bool // flat scores; preserve input entity order
	NoBudgetFit    bool // pack in rank order (no score/token density reorder)
	NoAudit        bool // omit include/exclude audit rows (budget + selection unchanged)
	NoRefExpansion bool // skip ref-graph score boost after initial rank
}

// Label returns a short bench arm suffix, or "" for the full compiler.
// When multiple flags are set, the first in this order wins.
func (a Ablation) Label() string {
	switch {
	case a.NoTaskContract:
		return "no-contract"
	case a.NoRanking:
		return "no-ranking"
	case a.NoBudgetFit:
		return "no-budget-fit"
	case a.NoAudit:
		return "no-audit"
	case a.NoRefExpansion:
		return "no-ref-expansion"
	default:
		return ""
	}
}

func applyAblationContract(c TaskContract, ab Ablation) TaskContract {
	if !ab.NoTaskContract {
		return c
	}
	return TaskContract{
		Question:      c.Question,
		RequiredKinds: append([]string(nil), defaultKinds...),
	}
}
