package compiler

// Ablation toggles compiler pipeline stages for science-track ablation runs.
// Zero value is the full pipeline (production / Phase 2 default).
type Ablation struct {
	NoTaskContract    bool // ignore derived keywords, kinds, and project hints
	NoRanking         bool // flat scores; preserve input entity order
	NoBudgetFit       bool // pack in rank order (no score/token density reorder)
	NoAudit           bool // omit include/exclude audit rows (budget + selection unchanged)
	NoRefExpansion    bool // skip ref-graph score boost after initial rank
}

// Label returns a short bench arm suffix, or "" for the full compiler.
func (a Ablation) Label() string {
	switch {
	case a == (Ablation{}):
		return ""
	case a.NoTaskContract && !a.NoRanking && !a.NoBudgetFit && !a.NoAudit && !a.NoRefExpansion:
		return "no-contract"
	case !a.NoTaskContract && a.NoRanking && !a.NoBudgetFit && !a.NoAudit && !a.NoRefExpansion:
		return "no-ranking"
	case !a.NoTaskContract && !a.NoRanking && a.NoBudgetFit && !a.NoAudit && !a.NoRefExpansion:
		return "no-budget-fit"
	case !a.NoTaskContract && !a.NoRanking && !a.NoBudgetFit && a.NoAudit && !a.NoRefExpansion:
		return "no-audit"
	case !a.NoTaskContract && !a.NoRanking && !a.NoBudgetFit && !a.NoAudit && a.NoRefExpansion:
		return "no-ref-expansion"
	default:
		return "custom-ablation"
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
