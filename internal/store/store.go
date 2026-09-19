// Package store persists entities and embeddings for vector RAG.
// Implementations: Memory (always available) and Postgres+pgvector (when DB up).
package store

import (
	"context"

	"github.com/aminmortezaie/contextcompiler/internal/state"
)

// VectorStore indexes entities with embeddings and supports top-k similarity search.
type VectorStore interface {
	// Upsert stores/replaces entities and their embeddings.
	Upsert(ctx context.Context, entities []state.Entity, embeddings [][]float32) error
	// Search returns up to k entities most similar to queryEmbedding (cosine).
	Search(ctx context.Context, queryEmbedding []float32, k int) ([]ScoredEntity, error)
	// Count returns number of indexed entities.
	Count(ctx context.Context) (int, error)
	Close() error
	Name() string
}

// ScoredEntity is a search hit with cosine similarity score.
type ScoredEntity struct {
	Entity state.Entity
	Score  float64
}
