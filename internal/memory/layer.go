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

// InMemory implements Layer with named handles.
type InMemory struct {
	mu   sync.RWMutex
	data map[string][]state.Entity
}

// NewInMemory returns an empty in-memory layer.
func NewInMemory() *InMemory {
	return &InMemory{data: make(map[string][]state.Entity)}
}

// WithFixtures registers built-in fixture handles for local dev and tests.
func (m *InMemory) WithFixtures() *InMemory {
	st, _ := fixture.Day0()
	_ = m.Register("day0", st.All())
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
