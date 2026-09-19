package memory

import (
	"context"
	"fmt"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// InlineHandle is the synthetic handle used by StaticGraph.Load.
const InlineHandle = "inline"

// StaticGraph is a GraphStore over a fixed entity+edge snapshot (inline
// compile, tests, Arm C). It does not share state with InMemory handles.
type StaticGraph struct {
	entities []state.Entity
	idx      *graphIndex
}

// NewStaticGraph builds a GraphStore from entities, typed edges, and optional episodes.
func NewStaticGraph(entities []state.Entity, edges []Edge, episodes ...[]Episode) *StaticGraph {
	var eps []Episode
	if len(episodes) > 0 {
		eps = episodes[0]
	}
	cp := make([]state.Entity, len(entities))
	copy(cp, entities)
	return &StaticGraph{entities: cp, idx: newGraphIndex(cp, edges, eps)}
}

// GraphFromEntities builds a GraphStore: typed fixture specs first, then
// RelRelated leftovers from RefIDs. Used when the API has inline entities
// or Arm C compiles a state.Store.
func GraphFromEntities(entities []state.Entity, typed []Edge, episodes []Episode) *StaticGraph {
	related := RelatedEdgesFromRefIDs(entities, typed)
	edges := append(append([]Edge{}, typed...), related...)
	return NewStaticGraph(entities, edges, episodes)
}

// DefaultGraph wires Day-0 / scale fixture typed edges plus leftover RefIDs.
func DefaultGraph(entities []state.Entity) *StaticGraph {
	return GraphFromEntities(entities, EdgesFromSpecs(fixture.GraphSpecs(entities)), EpisodesFromSpecs(fixture.Day0Episodes()))
}

func (g *StaticGraph) Load(ctx context.Context, handle string) ([]state.Entity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if handle != "" && handle != InlineHandle {
		return nil, fmt.Errorf("memory: static graph has no handle %q", handle)
	}
	out := make([]state.Entity, len(g.entities))
	copy(out, g.entities)
	return out, nil
}

func (g *StaticGraph) Search(ctx context.Context, _ string, q Query) (Neighborhood, error) {
	if err := ctx.Err(); err != nil {
		return Neighborhood{}, err
	}
	return g.idx.search(q), nil
}

func (g *StaticGraph) Neighbors(ctx context.Context, _ string, ids []string, hops int) (Neighborhood, error) {
	if err := ctx.Err(); err != nil {
		return Neighborhood{}, err
	}
	return g.idx.neighbors(ids, hops), nil
}
