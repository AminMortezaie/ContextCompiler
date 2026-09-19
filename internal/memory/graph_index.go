package memory

import (
	"strings"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// graphIndex is the shared in-memory adjacency used by InMemory and StaticGraph.
type graphIndex struct {
	nodes    map[string]state.Entity
	order    []string
	edges    []Edge
	out      map[string][]int // from → edge indexes
	in       map[string][]int // to → edge indexes
	episodes map[string]Episode
}

func newGraphIndex(entities []state.Entity, edges []Edge, episodes []Episode) *graphIndex {
	idx := &graphIndex{
		nodes:    make(map[string]state.Entity, len(entities)),
		order:    make([]string, 0, len(entities)),
		edges:    append([]Edge(nil), edges...),
		out:      make(map[string][]int),
		in:       make(map[string][]int),
		episodes: make(map[string]Episode, len(episodes)),
	}
	for _, e := range entities {
		if _, ok := idx.nodes[e.ID]; !ok {
			idx.order = append(idx.order, e.ID)
		}
		idx.nodes[e.ID] = e
	}
	for i, e := range idx.edges {
		if e.ID == "" {
			idx.edges[i].ID = edgeID(e.Rel, e.FromID, e.ToID)
			e = idx.edges[i]
		}
		idx.out[e.FromID] = append(idx.out[e.FromID], i)
		idx.in[e.ToID] = append(idx.in[e.ToID], i)
	}
	for _, ep := range episodes {
		idx.episodes[ep.ID] = ep
	}
	return idx
}

func edgeID(rel, from, to string) string {
	return "e-" + rel + "-" + from + "-" + to
}

func (g *graphIndex) search(q Query) Neighborhood {
	limit := q.Limit
	if limit <= 0 {
		limit = 32
	}
	kindSet := make(map[string]bool, len(q.Kinds))
	for _, k := range q.Kinds {
		kindSet[strings.ToLower(k)] = true
	}
	seedSet := make(map[string]bool, len(q.Seeds))
	for _, id := range q.Seeds {
		if id != "" {
			seedSet[id] = true
		}
	}
	type hit struct {
		id    string
		score int
	}
	var hits []hit
	for _, id := range g.order {
		e := g.nodes[id]
		if len(kindSet) > 0 && !kindSet[string(e.Kind)] {
			continue
		}
		score := 0
		if seedSet[e.ID] {
			score += 10
		}
		blob := strings.ToLower(e.Title + " " + e.Text + " " + e.ID)
		for _, kw := range q.Keywords {
			if kw != "" && strings.Contains(blob, strings.ToLower(kw)) {
				score += 3
			}
		}
		if q.Text != "" {
			for _, f := range strings.Fields(strings.ToLower(q.Text)) {
				if len(f) < 3 {
					continue
				}
				if strings.Contains(blob, f) {
					score++
				}
			}
		}
		if score > 0 {
			hits = append(hits, hit{id: id, score: score})
		}
	}
	// Insertion-order stable: higher score first, then id.
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].score > hits[i].score || (hits[j].score == hits[i].score && hits[j].id < hits[i].id) {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	keep := make(map[string]bool, len(hits))
	nb := Neighborhood{}
	for _, h := range hits {
		keep[h.id] = true
		nb.Nodes = append(nb.Nodes, Node{Entity: g.nodes[h.id]})
	}
	nb.Edges, nb.Episodes = g.edgesAmong(keep)
	return nb
}

func (g *graphIndex) neighbors(ids []string, hops int) Neighborhood {
	if hops < 0 {
		hops = 0
	}
	if hops > MaxHops {
		hops = MaxHops
	}
	keep := make(map[string]bool, len(ids)*4)
	frontier := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := g.nodes[id]; !ok {
			continue
		}
		if !keep[id] {
			keep[id] = true
			frontier = append(frontier, id)
		}
	}
	edgeKeep := make(map[int]bool)
	for d := 0; d < hops; d++ {
		var next []string
		for _, id := range frontier {
			for _, ei := range g.out[id] {
				edgeKeep[ei] = true
				to := g.edges[ei].ToID
				if !keep[to] {
					if _, ok := g.nodes[to]; ok {
						keep[to] = true
						next = append(next, to)
					}
				}
			}
			for _, ei := range g.in[id] {
				edgeKeep[ei] = true
				from := g.edges[ei].FromID
				if !keep[from] {
					if _, ok := g.nodes[from]; ok {
						keep[from] = true
						next = append(next, from)
					}
				}
			}
		}
		frontier = next
	}
	nb := Neighborhood{}
	for _, id := range g.order {
		if keep[id] {
			nb.Nodes = append(nb.Nodes, Node{Entity: g.nodes[id]})
		}
	}
	epKeep := make(map[string]bool)
	for i, e := range g.edges {
		if !edgeKeep[i] {
			continue
		}
		nb.Edges = append(nb.Edges, e)
		for _, epid := range e.EpisodeIDs {
			epKeep[epid] = true
		}
	}
	for id, ep := range g.episodes {
		if epKeep[id] {
			nb.Episodes = append(nb.Episodes, ep)
		}
	}
	return nb
}

func (g *graphIndex) edgesAmong(keep map[string]bool) ([]Edge, []Episode) {
	var edges []Edge
	epKeep := make(map[string]bool)
	for _, e := range g.edges {
		if keep[e.FromID] && keep[e.ToID] {
			edges = append(edges, e)
			for _, id := range e.EpisodeIDs {
				epKeep[id] = true
			}
		}
	}
	var eps []Episode
	for id, ep := range g.episodes {
		if epKeep[id] {
			eps = append(eps, ep)
		}
	}
	return edges, eps
}

// EdgesFromSpecs converts fixture edge specs into GraphStore edges.
func EdgesFromSpecs(specs []fixture.EdgeSpec) []Edge {
	out := make([]Edge, 0, len(specs))
	for _, s := range specs {
		out = append(out, Edge{
			ID:         edgeID(s.Rel, s.From, s.To),
			FromID:     s.From,
			ToID:       s.To,
			Rel:        s.Rel,
			Fact:       s.Fact,
			EpisodeIDs: append([]string(nil), s.EpisodeIDs...),
		})
	}
	return out
}

// EpisodesFromSpecs converts fixture episode specs.
func EpisodesFromSpecs(specs []fixture.EpisodeSpec) []Episode {
	out := make([]Episode, 0, len(specs))
	for _, s := range specs {
		out = append(out, Episode{ID: s.ID, Text: s.Text, Source: s.Source})
	}
	return out
}

// RelatedEdgesFromRefIDs synthesizes untyped RelRelated edges from Entity.RefIDs
// for pairs that are not already present as typed edges.
func RelatedEdgesFromRefIDs(entities []state.Entity, typed []Edge) []Edge {
	have := make(map[string]bool, len(typed))
	for _, e := range typed {
		have[e.FromID+"\x00"+e.ToID] = true
		have[e.ToID+"\x00"+e.FromID] = true
	}
	var out []Edge
	for _, e := range entities {
		for _, ref := range e.RefIDs {
			if ref == "" || ref == e.ID {
				continue
			}
			key := e.ID + "\x00" + ref
			if have[key] {
				continue
			}
			have[key] = true
			out = append(out, Edge{
				ID:     edgeID(RelRelated, e.ID, ref),
				FromID: e.ID,
				ToID:   ref,
				Rel:    RelRelated,
				Fact:   e.ID + " references " + ref,
			})
		}
	}
	return out
}
