package store

import (
	"context"
	"testing"

	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/fixture"
)

func TestMemoryIndexAndSearch(t *testing.T) {
	st, task := fixture.Day0()
	ctx := context.Background()
	vs := NewMemory()
	emb := embed.NewHash()
	if err := IndexStore(ctx, vs, st, emb); err != nil {
		t.Fatal(err)
	}
	n, err := vs.Count(ctx)
	if err != nil || n != len(st.All()) {
		t.Fatalf("count=%d err=%v want %d", n, err, len(st.All()))
	}
	q, _ := embed.EmbedOne(ctx, emb, task.Question)
	hits, err := vs.Search(ctx, q, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
}

func TestPostgresOptional(t *testing.T) {
	ctx := context.Background()
	pg, err := TryOpenPostgres(ctx)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pg.Close()
	st, task := fixture.Day0()
	emb := embed.NewHash()
	if err := IndexStore(ctx, pg, st, emb); err != nil {
		t.Fatal(err)
	}
	q, _ := embed.EmbedOne(ctx, emb, task.Question)
	hits, err := pg.Search(ctx, q, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected pgvector hits")
	}
	t.Logf("top hit: %s score=%.3f", hits[0].Entity.ID, hits[0].Score)
}
