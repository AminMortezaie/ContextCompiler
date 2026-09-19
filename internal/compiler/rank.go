package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// relWeight scores typed dependency edges. Mentions/related are weaker than
// blocks / decided_by / owns. Unknown rels use the related weight.
var relWeight = map[string]float64{
	memory.RelBlocks:     2.5,
	memory.RelBlockedBy:  2.5,
	memory.RelDecidedBy:  2.2,
	memory.RelDecides:    2.2,
	memory.RelOwns:       2.0,
	memory.RelOwnedBy:    2.0,
	memory.RelAssignedTo: 1.8,
	memory.RelImplements: 1.8,
	memory.RelMemberOf:   1.2,
	memory.RelMentions:   0.8,
	memory.RelRelated:    0.5,
}

func rankCandidates(cands []candidate, contract TaskContract, edges []memory.Edge, hopOf map[string]int, ab Ablation) ([]scoredEnt, map[string]string) {
	if ab.NoRanking {
		var ranked []scoredEnt
		for _, c := range cands {
			ranked = append(ranked, scoredEnt{e: c.e, score: 1.0, reason: "ablation-no-ranking", source: c.source, hops: c.hops})
		}
		return ranked, map[string]string{}
	}

	kindSet := make(map[string]bool)
	for _, k := range contract.RequiredKinds {
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

	// Adjacency for graph features (typed edges only — no fixture-ID special cases).
	type link struct {
		rel string
		id  string
		eid string
	}
	adj := make(map[string][]link)
	for _, e := range edges {
		adj[e.FromID] = append(adj[e.FromID], link{rel: e.Rel, id: e.ToID, eid: e.ID})
		adj[e.ToID] = append(adj[e.ToID], link{rel: e.Rel, id: e.FromID, eid: e.ID})
	}

	seed := make(map[string]bool)
	for _, c := range cands {
		if c.source == SourceLexical || c.source == SourceGraph || c.source == SourceVector {
			if lexicalScore(c.e, contract) > 0 || c.source == SourceGraph {
				seed[c.e.ID] = true
			}
		}
	}

	var ranked []scoredEnt
	below := make(map[string]string)

	type draft struct {
		scoredEnt
		lex float64
	}
	drafts := make([]draft, 0, len(cands))
	scoreByID := make(map[string]float64, len(cands))

	for _, c := range cands {
		e := c.e
		score := kindPrior[e.Kind]
		reasons := []string{"src=" + c.source}

		if !kindSet[string(e.Kind)] {
			score *= 0.15
			reasons = append(reasons, "kind-not-required")
		} else {
			reasons = append(reasons, "kind-prior")
		}

		titleLower := strings.ToLower(e.Title)
		kwHits := 0
		for _, kw := range contract.Keywords {
			if strings.Contains(titleLower, kw) || containsFold(e.Text, kw) {
				kwHits++
				score += 3.0
			}
		}
		if kwHits > 0 {
			reasons = append(reasons, fmt.Sprintf("keyword-hits=%d", kwHits))
		}

		for _, ph := range contract.ProjectHints {
			if strings.Contains(titleLower, ph) || containsFold(e.Text, ph) ||
				strings.Contains(strings.ToLower(e.ID), strings.ReplaceAll(ph, " ", "-")) {
				score += 4.0
				reasons = append(reasons, "project-hint")
			}
		}

		if c.vec > 0 {
			score += 2.0 * c.vec
			reasons = append(reasons, fmt.Sprintf("vector=%.2f", c.vec))
		}

		hops := c.hops
		if h, ok := hopOf[e.ID]; ok {
			hops = h
		}
		if hops > 0 {
			score *= 1.0 / (1.0 + 0.35*float64(hops))
			reasons = append(reasons, fmt.Sprintf("hops=%d", hops))
		}

		d := draft{
			scoredEnt: scoredEnt{e: e, score: score, reason: strings.Join(reasons, ","), source: c.source, hops: hops},
			lex:       c.lex,
		}
		drafts = append(drafts, d)
		scoreByID[e.ID] = score
	}

	// Second pass: typed-neighbor features. No hardcoded entity IDs.
	if !ab.NoRefExpansion {
		for i := range drafts {
			boost := 0.0
			var eids []string
			for _, ln := range adj[drafts[i].e.ID] {
				w := relWeight[ln.rel]
				if w == 0 {
					w = relWeight[memory.RelRelated]
				}
				if seed[ln.id] {
					boost += w
					eids = append(eids, ln.eid)
					continue
				}
				if s, ok := scoreByID[ln.id]; ok && s >= 5 {
					boost += 0.6 * w
					eids = append(eids, ln.eid)
				}
			}
			if boost > 0 {
				drafts[i].score += boost
				drafts[i].reason += fmt.Sprintf(",graph-boost=%.1f", boost)
				drafts[i].edgeIDs = eids
				scoreByID[drafts[i].e.ID] = drafts[i].score
			}
		}
	}

	for _, d := range drafts {
		if d.score < 1.0 {
			below[d.e.ID] = "below-threshold:" + d.reason
			continue
		}
		ranked = append(ranked, d.scoredEnt)
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].e.ID < ranked[j].e.ID
		}
		return ranked[i].score > ranked[j].score
	})
	return ranked, below
}
