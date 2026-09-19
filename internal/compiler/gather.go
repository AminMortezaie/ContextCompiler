package compiler

import (
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// maxBruteVector skips on-the-fly cosine when the allowed set is huge.
// Day-0 / scaled fixtures filter noise first, so N stays tiny.
const maxBruteVector = 2000

// minVectorCosine drops weak hash-bow hits so small N does not union the
// entire allowed set into "vector" candidates.
const minVectorCosine = 0.12

func gatherCandidates(allowed []state.Entity, contract TaskContract, opts Options) []candidate {
	byID := make(map[string]candidate, 32)

	add := func(e state.Entity, src string, hops int, lex, vec float64) {
		if e.ID == "" {
			return
		}
		if c, ok := byID[e.ID]; ok {
			if c.source == "" {
				c.source = src
			}
			if hops < c.hops {
				c.hops = hops
			}
			if lex > c.lex {
				c.lex = lex
			}
			if vec > c.vec {
				c.vec = vec
			}
			byID[e.ID] = c
			return
		}
		byID[e.ID] = candidate{e: e, source: src, hops: hops, lex: lex, vec: vec}
	}

	ents := skipNoise(allowed)
	entByID := make(map[string]state.Entity, len(ents))
	for _, e := range ents {
		entByID[e.ID] = e
	}

	// 1. Lexical: keyword + project-hint hits (and ID hint matches).
	for _, e := range ents {
		lex := lexicalScore(e, contract)
		if lex > 0 {
			add(e, SourceLexical, 0, lex, 0)
		}
	}

	// 2. Vector: hash (or provided) cosine vs the question, top-k.
	if contract.Question != "" && len(ents) > 0 && len(ents) <= maxBruteVector {
		emb := opts.Embedder
		if emb == nil {
			emb = embed.NewHash()
		}
		ctx := opts.Context
		qvec, err := embed.EmbedDocument(ctx, emb, contract.Question, "")
		if err == nil {
			type vh struct {
				e     state.Entity
				score float64
			}
			hits := make([]vh, 0, len(ents))
			for _, e := range ents {
				vec, err := embed.EmbedDocument(ctx, emb, e.Title, e.Text)
				if err != nil {
					continue
				}
				s := embed.Cosine(qvec, vec)
				if s >= minVectorCosine {
					hits = append(hits, vh{e: e, score: s})
				}
			}
			// selection sort top-k (N is small)
			for i := 0; i < len(hits); i++ {
				best := i
				for j := i + 1; j < len(hits); j++ {
					if hits[j].score > hits[best].score {
						best = j
					}
				}
				hits[i], hits[best] = hits[best], hits[i]
			}
			k := opts.TopK
			if k > len(hits) {
				k = len(hits)
			}
			for i := 0; i < k; i++ {
				add(hits[i].e, SourceVector, 0, 0, hits[i].score)
			}
		}
	}

	// 3. Graph seeds: GraphStore.Search (keywords + seed IDs from hints).
	if opts.Graph != nil {
		q := memory.Query{
			Text:     contract.Question,
			Kinds:    contract.RequiredKinds,
			Keywords: contract.Keywords,
			Seeds:    seedIDs(contract),
			Limit:    opts.TopK,
		}
		nb, err := opts.Graph.Search(opts.Context, opts.Handle, q)
		if err == nil {
			for _, n := range nb.Nodes {
				if e, ok := entByID[n.Entity.ID]; ok {
					add(e, SourceGraph, 0, 0, 0)
				}
			}
		}
	}

	if len(byID) == 0 {
		// Safety: never return an empty set when org-state exists.
		for _, e := range ents {
			add(e, SourceLexical, 0, 0, 0)
		}
	}

	out := make([]candidate, 0, len(byID))
	for _, c := range byID {
		out = append(out, c)
	}
	return out
}

func skipNoise(entities []state.Entity) []state.Entity {
	out := make([]state.Entity, 0, len(entities))
	for _, e := range entities {
		if e.Meta != nil && e.Meta["noise"] == "true" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func lexicalScore(e state.Entity, c TaskContract) float64 {
	titleLower := strings.ToLower(e.Title)
	idLower := strings.ToLower(e.ID)
	score := 0.0
	for _, kw := range c.Keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(titleLower, kw) || containsFold(e.Text, kw) {
			score += 3
		}
	}
	for _, ph := range c.ProjectHints {
		if ph == "" {
			continue
		}
		if strings.Contains(titleLower, ph) || containsFold(e.Text, ph) ||
			strings.Contains(idLower, strings.ReplaceAll(ph, " ", "-")) {
			score += 4
		}
	}
	return score
}

func seedIDs(c TaskContract) []string {
	var ids []string
	for _, ph := range c.ProjectHints {
		ph = strings.TrimSpace(ph)
		if ph == "" {
			continue
		}
		ids = append(ids, ph, strings.ReplaceAll(ph, " ", "-"))
	}
	return ids
}
