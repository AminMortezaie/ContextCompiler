package arms

import (
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

// PackEntities concatenates entity pack texts in order until tokenBudget is reached.
// Returns packed string and the IDs that fit.
func PackEntities(entities []state.Entity, tokenBudget int) (packed string, selected []string) {
	var b strings.Builder
	used := 0
	for _, e := range entities {
		chunk := e.PackText() + "\n\n"
		cost := tokens.Estimate(chunk)
		if used+cost > tokenBudget && used > 0 {
			break
		}
		// Always include at least one entity even if it exceeds budget slightly.
		b.WriteString(chunk)
		selected = append(selected, e.ID)
		used += cost
		if used >= tokenBudget {
			break
		}
	}
	return b.String(), selected
}

// ExcludedFrom returns IDs in all that are not in selected.
func ExcludedFrom(all []string, selected []string) []string {
	set := make(map[string]struct{}, len(selected))
	for _, id := range selected {
		set[id] = struct{}{}
	}
	var out []string
	for _, id := range all {
		if _, ok := set[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}
