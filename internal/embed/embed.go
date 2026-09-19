// Package embed provides text embedding providers for RAG (arm B).
//
// Default offline provider: deterministic bag-of-words hash into a fixed
// 384-dimensional unit vector (no network, reproducible).
// Optional: OpenAI-compatible embeddings when OPENAI_API_KEY is set and
// provider is selected via NewFromEnv.
package embed

import "context"

// Dim is the fixed embedding dimensionality used by the local hash provider
// and by the pgvector schema. OpenAI text-embedding-3-small is 1536 by default;
// when using the OpenAI provider we still project/pad to Dim for a single schema,
// OR we use a dedicated OpenAI path — see openai.go. For Day-1 the offline path
// is Dim=384; OpenAI path uses native dims only if a separate table is used.
// Day-1 choice: keep schema at 384 and use HashProvider always for pgvector
// reproducibility; OpenAI provider is available for experiments but Hash is default.
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
