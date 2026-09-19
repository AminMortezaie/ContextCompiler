package compiler

import (
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

var strongRels = map[string]bool{
	memory.RelBlocks:     true,
	memory.RelBlockedBy:  true,
	memory.RelDecidedBy:  true,
	memory.RelDecides:    true,
	memory.RelOwns:       true,
	memory.RelImplements: true,
	memory.RelAssignedTo: true,
}

// checkSufficiency is a deterministic contract-coverage gate. It never calls
// an LLM and does not start a retrieve loop (Phase C / agentic reflect later).
func checkSufficiency(selected []string, selectedEdges []PackedEdge, packed string, entities []state.Entity, contract TaskContract) Sufficiency {
	byID := make(map[string]state.Entity, len(entities))
	for _, e := range entities {
		byID[e.ID] = e
	}
	selectedSet := make(map[string]bool, len(selected))
	kinds := make(map[string]bool)
	for _, id := range selected {
		selectedSet[id] = true
		if e, ok := byID[id]; ok {
			kinds[string(e.Kind)] = true
		}
	}
	packedLower := strings.ToLower(packed)
	q := strings.ToLower(contract.Question)

	var checks []SufficiencyCheck

	for _, k := range focusKinds(contract, q) {
		ok := kinds[k]
		d := "packed"
		if !ok {
			d = "missing kind in selected_ids"
		}
		checks = append(checks, SufficiencyCheck{Name: "required_kind:" + k, OK: ok, Detail: d})
	}

	if len(contract.Keywords) > 0 {
		hits := 0
		var missing []string
		for _, kw := range contract.Keywords {
			if kw == "" {
				continue
			}
			if strings.Contains(packedLower, strings.ToLower(kw)) {
				hits++
			} else {
				missing = append(missing, kw)
			}
		}
		// At least one keyword must appear; list misses for audit.
		ok := hits > 0
		detail := "keyword hits in packed context"
		if !ok {
			detail = "no contract keywords in packed context"
		} else if len(missing) > 0 {
			detail = "partial keyword coverage; miss " + strings.Join(missing, ",")
		}
		checks = append(checks, SufficiencyCheck{Name: "keywords", OK: ok, Detail: detail})
	}

	if len(contract.ProjectHints) > 0 {
		ok := false
		for _, ph := range contract.ProjectHints {
			if ph == "" {
				continue
			}
			if strings.Contains(packedLower, strings.ToLower(ph)) {
				ok = true
				break
			}
			for _, id := range selected {
				if strings.Contains(strings.ToLower(id), strings.ReplaceAll(strings.ToLower(ph), " ", "-")) {
					ok = true
					break
				}
			}
		}
		d := "project hint packed"
		if !ok {
			d = "no project-hint / seed entity packed"
		}
		checks = append(checks, SufficiencyCheck{Name: "project_hint", OK: ok, Detail: d})
	}

	if looksCausal(q) {
		hasKind := kinds["decision"] || kinds["ticket"]
		hasEdge := false
		for _, e := range selectedEdges {
			if strongRels[e.Rel] {
				hasEdge = true
				break
			}
		}
		ok := hasKind && hasEdge
		d := "decision/ticket + typed causal edge"
		if !hasKind {
			d = "causal question but no decision/ticket packed"
		} else if !hasEdge {
			d = "causal question but no blocks/decided_by/owns/implements edge packed"
		}
		checks = append(checks, SufficiencyCheck{Name: "causal_coverage", OK: ok, Detail: d})
	}

	var missing []string
	ok := true
	for _, c := range checks {
		if !c.OK {
			ok = false
			missing = append(missing, c.Name)
		}
	}
	return Sufficiency{Sufficient: ok, Missing: missing, Checks: checks}
}

func focusKinds(c TaskContract, q string) []string {
	// Kitchen-sink default RequiredKinds are not a coverage contract.
	// Derive a focused set from the question; fall back to a small core
	// when the caller passed an explicit short kind list.
	if looksCausal(q) {
		return []string{"decision", "ticket"}
	}
	if strings.Contains(q, "own") || strings.Contains(q, "responsible") {
		return []string{"user", "ticket"}
	}
	if strings.Contains(q, "action") || strings.Contains(q, "should") || strings.Contains(q, "right now") {
		return []string{"ticket", "task"}
	}
	if strings.Contains(q, "approved") || strings.Contains(q, "decision") {
		return []string{"decision", "user"}
	}
	if len(c.RequiredKinds) > 0 && len(c.RequiredKinds) <= 3 {
		return append([]string(nil), c.RequiredKinds...)
	}
	return nil
}

func looksCausal(q string) bool {
	for _, w := range []string{"why", "caused", "cause", "delay", "delayed", "slip"} {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}
