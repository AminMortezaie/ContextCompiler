package compiler

import (
	"github.com/aminmortezaie/contextcompiler/internal/memory"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// expandResult is the hop-limited neighborhood attached to gathered seeds.
type expandResult struct {
	candidates []candidate
	edges      []memory.Edge
	episodes   []memory.Episode
	hopOf      map[string]int
}

func expandCandidates(gathered []candidate, allowed []state.Entity, opts Options) expandResult {
	out := expandResult{
		candidates: append([]candidate(nil), gathered...),
		hopOf:      make(map[string]int, len(gathered)),
	}
	for _, c := range gathered {
		out.hopOf[c.e.ID] = 0
	}
	if opts.Ablation.NoRefExpansion || opts.Graph == nil || opts.Hops <= 0 {
		return out
	}

	entByID := make(map[string]state.Entity, len(allowed))
	for _, e := range allowed {
		if e.Meta != nil && e.Meta["noise"] == "true" {
			continue
		}
		entByID[e.ID] = e
	}

	seeds := make([]string, 0, len(gathered))
	have := make(map[string]bool, len(gathered))
	for _, c := range gathered {
		seeds = append(seeds, c.e.ID)
		have[c.e.ID] = true
	}

	nb, err := opts.Graph.Neighbors(opts.Context, opts.Handle, seeds, opts.Hops)
	if err != nil {
		return out
	}
	out.edges = nb.Edges
	out.episodes = nb.Episodes

	// Hop distance: BFS over the returned undirected neighborhood from seeds.
	out.hopOf = hopDistances(seeds, nb.Edges, opts.Hops)
	for _, n := range nb.Nodes {
		e, ok := entByID[n.Entity.ID]
		if !ok || have[e.ID] {
			continue
		}
		h := out.hopOf[e.ID]
		if h == 0 && !seedSet(seeds)[e.ID] {
			h = 1
			out.hopOf[e.ID] = h
		}
		out.candidates = append(out.candidates, candidate{e: e, source: SourceExpand, hops: h})
		have[e.ID] = true
	}
	return out
}

func seedSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func hopDistances(seeds []string, edges []memory.Edge, maxHops int) map[string]int {
	dist := make(map[string]int, len(seeds)*4)
	adj := make(map[string][]string)
	for _, e := range edges {
		adj[e.FromID] = append(adj[e.FromID], e.ToID)
		adj[e.ToID] = append(adj[e.ToID], e.FromID)
	}
	type qn struct {
		id string
		d  int
	}
	q := make([]qn, 0, len(seeds))
	for _, id := range seeds {
		dist[id] = 0
		q = append(q, qn{id, 0})
	}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if cur.d >= maxHops {
			continue
		}
		for _, nb := range adj[cur.id] {
			if _, seen := dist[nb]; seen {
				continue
			}
			dist[nb] = cur.d + 1
			q = append(q, qn{nb, cur.d + 1})
		}
	}
	return dist
}

// incidentEdges returns edges that touch any of the given IDs.
func incidentEdges(edges []memory.Edge, ids []string) []memory.Edge {
	keep := make(map[string]bool, len(ids))
	for _, id := range ids {
		keep[id] = true
	}
	var out []memory.Edge
	for _, e := range edges {
		if keep[e.FromID] || keep[e.ToID] {
			out = append(out, e)
		}
	}
	return out
}
