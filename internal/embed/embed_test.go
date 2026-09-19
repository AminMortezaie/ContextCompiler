package embed

import (
	"context"
	"testing"
)

func TestHashEmbedDeterministic(t *testing.T) {
	p := NewHash()
	a, err := p.Embed(context.Background(), []string{"Project X delay auth freeze"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Embed(context.Background(), []string{"Project X delay auth freeze"})
	if err != nil {
		t.Fatal(err)
	}
	if len(a[0]) != Dim {
		t.Fatalf("dim=%d want %d", len(a[0]), Dim)
	}
	for i := range a[0] {
		if a[0][i] != b[0][i] {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
	// Related texts should be more similar than unrelated.
	q, _ := EmbedOne(context.Background(), p, "Why was Project X delayed?")
	rel, _ := EmbedOne(context.Background(), p, "Project X delayed due to API redesign; Sarah approved delay")
	noise, _ := EmbedOne(context.Background(), p, "Cafeteria menu survey and holiday calendar sync")
	if Cosine(q, rel) <= Cosine(q, noise) {
		t.Fatalf("expected query closer to relevant than noise: rel=%f noise=%f", Cosine(q, rel), Cosine(q, noise))
	}
}
