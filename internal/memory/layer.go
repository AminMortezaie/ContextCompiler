package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/aminmortezaie/contextcompiler/internal/fixture"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// Layer is a pluggable org-state backend (Zep-like handle in v0; inline entities bypass it).
type Layer interface {
	Load(ctx context.Context, handle string) ([]state.Entity, error)
}

// InMemory implements Layer and GraphStore with named handles.
type InMemory struct {
	mu     sync.RWMutex
	data   map[string][]state.Entity
	graphs map[string]*graphIndex
}

// NewInMemory returns an empty in-memory layer.
func NewInMemory() *InMemory {
	return &InMemory{
		data:   make(map[string][]state.Entity),
		graphs: make(map[string]*graphIndex),
	}
}

// WithFixtures registers built-in fixture handles plus the Day-0 typed graph.
func (m *InMemory) WithFixtures() *InMemory {
	st, _ := fixture.Day0()
	_ = m.Register("day0", st.All())
	_ = m.RegisterGraph("day0", EdgesFromSpecs(fixture.Day0Edges()), EpisodesFromSpecs(fixture.Day0Episodes()))
	return m
}

func (m *InMemory) Register(handle string, entities []state.Entity) error {
	if handle == "" {
		return fmt.Errorf("memory: empty handle")
	}
	cp := make([]state.Entity, len(entities))
	copy(cp, entities)
	m.mu.Lock()
	m.data[handle] = cp
	m.mu.Unlock()
	return nil
}

func (m *InMemory) Load(ctx context.Context, handle string) ([]state.Entity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	ents, ok := m.data[handle]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("memory: unknown handle %q", handle)
	}
	out := make([]state.Entity, len(ents))
	copy(out, ents)
	return out, nil
}

// RegisterGraph attaches typed edges (and optional episodes) to a handle.
// Entities must already be Register'd. Missing endpoint IDs are ignored at walk time.
func (m *InMemory) RegisterGraph(handle string, edges []Edge, episodes ...[]Episode) error {
	if handle == "" {
		return fmt.Errorf("memory: empty handle")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ents, ok := m.data[handle]
	if !ok {
		return fmt.Errorf("memory: unknown handle %q", handle)
	}
	var eps []Episode
	if len(episodes) > 0 {
		eps = episodes[0]
	}
	related := RelatedEdgesFromRefIDs(ents, edges)
	all := append(append([]Edge{}, edges...), related...)
	m.graphs[handle] = newGraphIndex(ents, all, eps)
	return nil
}

func (m *InMemory) Search(ctx context.Context, handle string, q Query) (Neighborhood, error) {
	if err := ctx.Err(); err != nil {
		return Neighborhood{}, err
	}
	idx, err := m.index(handle)
	if err != nil {
		return Neighborhood{}, err
	}
	return idx.search(q), nil
}

func (m *InMemory) Neighbors(ctx context.Context, handle string, ids []string, hops int) (Neighborhood, error) {
	if err := ctx.Err(); err != nil {
		return Neighborhood{}, err
	}
	idx, err := m.index(handle)
	if err != nil {
		return Neighborhood{}, err
	}
	return idx.neighbors(ids, hops), nil
}

func (m *InMemory) index(handle string) (*graphIndex, error) {
	m.mu.RLock()
	idx, ok := m.graphs[handle]
	ents, haveEnts := m.data[handle]
	m.mu.RUnlock()
	if ok {
		return idx, nil
	}
	if !haveEnts {
		return nil, fmt.Errorf("memory: unknown handle %q", handle)
	}
	// No graph registered: synthesize RelRelated from RefIDs so compile still expands.
	return newGraphIndex(ents, RelatedEdgesFromRefIDs(ents, nil), nil), nil
}
