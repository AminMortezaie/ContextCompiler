// Package embed provides text embedding providers for RAG (arm B).
//
// Default: deterministic bag-of-words hash into a fixed 384-dimensional unit
// vector (no network, reproducible).
package embed

import "context"

// Dim is the fixed embedding dimensionality used by the hash provider and pgvector schema.
const Dim = 384

// Provider turns text into a fixed-length float32 embedding.
type Provider interface {
	Name() string
	Dim() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// EmbedOne is a convenience wrapper for a single string.
func EmbedOne(ctx context.Context, p Provider, text string) ([]float32, error) {
	out, err := p.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return make([]float32, p.Dim()), nil
	}
	return out[0], nil
}

// NewFromEnv returns the hash embedder used for bakeoffs (offline, reproducible).
func NewFromEnv() Provider {
	return NewHash()
}
