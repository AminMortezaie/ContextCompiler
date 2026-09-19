package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/tokens"
)

type packResult struct {
	text     string
	selected []string
	edges    []PackedEdge
	audit    []AuditEntry
}

type packCand struct {
	scoredEnt
	tok     int
	density float64
	kind    string // node | edge | snippet
	edge    memory.Edge
	ep      memory.Episode
	text    string
}

func (c packCand) auditID() string {
	switch c.kind {
	case "edge":
		return c.edge.ID
	case "snippet":
		return c.ep.ID
	default:
		return c.e.ID
	}
}

// budgetPack density-packs nodes, then incident edges (fact snippets), then
// episode provenance snippets, under the same token budget.
func budgetPack(ranked []scoredEnt, edges []memory.Edge, episodes []memory.Episode, tokenBudget int, rankOrderOnly bool) packResult {
	cands := make([]packCand, 0, len(ranked)+len(edges)+len(episodes))
	for _, r := range ranked {
		text := r.e.PackText() + "\n\n"
		tok := tokens.Estimate(text)
		if tok <= 0 {
			tok = 1
		}
		cands = append(cands, packCand{scoredEnt: r, tok: tok, density: r.score / float64(tok), kind: "node", text: text})
	}
	for _, e := range edges {
		text := e.PackText() + "\n"
		tok := tokens.Estimate(text)
		if tok <= 0 {
			tok = 1
		}
		cands = append(cands, packCand{
			scoredEnt: scoredEnt{score: 1.5, source: SourceExpand, reason: "typed-edge"},
			tok:       tok,
			density:   1.5 / float64(tok),
			kind:      "edge",
			edge:      e,
			text:      text,
		})
	}
	for _, ep := range episodes {
		text := ep.PackText() + "\n"
		tok := tokens.Estimate(text)
		if tok <= 0 {
			tok = 1
		}
		cands = append(cands, packCand{
			scoredEnt: scoredEnt{score: 1.2, source: SourceExpand, reason: "episode-snippet"},
			tok:       tok,
			density:   1.2 / float64(tok),
			kind:      "snippet",
			ep:        ep,
			text:      text,
		})
	}

	if !rankOrderOnly {
		sort.SliceStable(cands, func(i, j int) bool {
			pri := func(k string) int {
				switch k {
				case "node":
					return 0
				case "edge":
					return 1
				default:
					return 2
				}
			}
			if pri(cands[i].kind) != pri(cands[j].kind) {
				return pri(cands[i].kind) < pri(cands[j].kind)
			}
			if cands[i].density == cands[j].density {
				return cands[i].score > cands[j].score
			}
			return cands[i].density > cands[j].density
		})
	}

	var b strings.Builder
	used := 0
	var selected []string
	var packedEdges []PackedEdge
	var fit []AuditEntry
	selectedSet := make(map[string]bool)

	for _, c := range cands {
		if c.kind == "edge" && used > 0 && !selectedSet[c.edge.FromID] && !selectedSet[c.edge.ToID] {
			continue
		}
		if used+c.tok > tokenBudget && used > 0 {
			reason := fmt.Sprintf("budget-fit drop density=%.4f", c.density)
			if rankOrderOnly {
				reason = "rank-order budget drop"
			}
			id := c.auditID()
			if id == "" {
				continue
			}
			a := AuditEntry{ID: id, Action: "exclude", Reason: reason, Score: c.score, Source: c.source}
			if c.kind == "edge" {
				a.EdgeID = c.edge.ID
			}
			if c.kind == "snippet" {
				a.EpisodeID = c.ep.ID
			}
			fit = append(fit, a)
			continue
		}
		b.WriteString(c.text)
		used += c.tok
		switch c.kind {
		case "node":
			selected = append(selected, c.e.ID)
			selectedSet[c.e.ID] = true
			includeReason := fmt.Sprintf("budget-fit keep density=%.4f score=%.2f", c.density, c.score)
			if rankOrderOnly {
				includeReason = fmt.Sprintf("rank-order keep score=%.2f", c.score)
			}
			fit = append(fit, AuditEntry{
				ID: c.e.ID, Action: "include", Reason: includeReason, Score: c.score, Source: c.source,
			})
		case "edge":
			packedEdges = append(packedEdges, PackedEdge{
				ID: c.edge.ID, FromID: c.edge.FromID, ToID: c.edge.ToID, Rel: c.edge.Rel, Fact: c.edge.Fact,
			})
			fit = append(fit, AuditEntry{
				ID: c.edge.ID, Action: "include", Reason: "budget-fit edge " + c.edge.Rel,
				Score: c.score, Source: SourceExpand, EdgeID: c.edge.ID,
			})
		case "snippet":
			fit = append(fit, AuditEntry{
				ID: c.ep.ID, Action: "include", Reason: "budget-fit snippet",
				Score: c.score, Source: SourceExpand, EpisodeID: c.ep.ID,
			})
		}
	}
	return packResult{text: b.String(), selected: selected, edges: packedEdges, audit: fit}
}
