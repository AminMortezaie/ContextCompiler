package store

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// Memory is an in-memory VectorStore used when Postgres is unavailable
// and for unit tests. Cosine search over stored float32 vectors.
type Memory struct {
	mu    sync.RWMutex
	ents  map[string]state.Entity
	embs  map[string][]float32
	order []string
}

func NewMemory() *Memory {
	return &Memory{
		ents: make(map[string]state.Entity),
		embs: make(map[string][]float32),
	}
}

func (m *Memory) Name() string { return "memory" }

func (m *Memory) Close() error { return nil }

func (m *Memory) Upsert(_ context.Context, entities []state.Entity, embeddings [][]float32) error {
	if len(entities) != len(embeddings) {
		return fmt.Errorf("upsert: %d entities vs %d embeddings", len(entities), len(embeddings))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range entities {
		if _, ok := m.ents[e.ID]; !ok {
			m.order = append(m.order, e.ID)
		}
		m.ents[e.ID] = e
		// copy embedding
		vec := make([]float32, len(embeddings[i]))
		copy(vec, embeddings[i])
		m.embs[e.ID] = vec
	}
	return nil
}

func (m *Memory) Search(_ context.Context, queryEmbedding []float32, k int) ([]ScoredEntity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	type hit struct {
		id    string
		score float64
	}
	var hits []hit
	// Walk insertion order instead of the map so equal-score results are
	// reproducible for a deterministic fixture and seed.
	for _, id := range m.order {
		vec, ok := m.embs[id]
		if !ok {
			continue
		}
		hits = append(hits, hit{id: id, score: embed.Cosine(queryEmbedding, vec)})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if k > len(hits) {
		k = len(hits)
	}
	out := make([]ScoredEntity, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, ScoredEntity{Entity: m.ents[hits[i].id], Score: hits[i].score})
	}
	return out, nil
}

func (m *Memory) Count(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.ents), nil
}

// IndexStore embeds all entities from a state.Store into this Memory store.
func IndexStore(ctx context.Context, vs VectorStore, st *state.Store, p embed.Provider) error {
	ents := st.All()
	const batch = 64
	for i := 0; i < len(ents); i += batch {
		j := i + batch
		if j > len(ents) {
			j = len(ents)
		}
		embs := make([][]float32, j-i)
		for k, e := range ents[i:j] {
			vec, err := embed.EmbedDocument(ctx, p, e.Title, e.Text)
			if err != nil {
				return err
			}
			embs[k] = vec
		}
		if err := vs.Upsert(ctx, ents[i:j], embs); err != nil {
			return err
		}
	}
	return nil
}
