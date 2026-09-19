package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aminmortezaie/contextcompiler/internal/embed"
	"github.com/aminmortezaie/contextcompiler/internal/state"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultDSN matches docker-compose.yml credentials.
const DefaultDSN = "postgres://contextcompiler:contextcompiler@localhost:5432/contextcompiler?sslmode=disable"

// Postgres is a pgvector-backed VectorStore.
type Postgres struct {
	pool *pgxpool.Pool
}

func (p *Postgres) Name() string { return "postgres+pgvector" }

// OpenPostgres connects and runs schema migration. Returns error if DB unreachable.
func OpenPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = DefaultDSN
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 8
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	pg := &Postgres{pool: pool}
	if err := pg.Migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return pg, nil
}

// TryOpenPostgres attempts OpenPostgres with a short timeout; on failure returns (nil, err).
func TryOpenPostgres(parent context.Context) (*Postgres, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	return OpenPostgres(ctx, "")
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return nil
}

const migrateSQL = `
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE IF NOT EXISTS entities (
    id          TEXT PRIMARY KEY,
    kind        TEXT NOT NULL,
    title       TEXT NOT NULL,
    text        TEXT NOT NULL,
    ref_ids     TEXT[] NOT NULL DEFAULT '{}',
    meta        JSONB NOT NULL DEFAULT '{}',
    embedding   vector(384),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS entities_kind_idx ON entities (kind);
`

func (p *Postgres) Migrate(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, migrateSQL); err != nil {
		return err
	}
	// HNSW index may fail on empty table in some versions; create if not exists separately.
	_, _ = p.pool.Exec(ctx, `
CREATE INDEX IF NOT EXISTS entities_embedding_hnsw
    ON entities USING hnsw (embedding vector_cosine_ops)
`)
	return nil
}

func (p *Postgres) Upsert(ctx context.Context, entities []state.Entity, embeddings [][]float32) error {
	if len(entities) != len(embeddings) {
		return fmt.Errorf("upsert: %d entities vs %d embeddings", len(entities), len(embeddings))
	}
	batch := &pgx.Batch{}
	for i, e := range entities {
		metaJSON, _ := json.Marshal(e.Meta)
		if e.Meta == nil {
			metaJSON = []byte("{}")
		}
		refs := e.RefIDs
		if refs == nil {
			refs = []string{}
		}
		vecLit := vectorLiteral(embeddings[i])
		batch.Queue(`
INSERT INTO entities (id, kind, title, text, ref_ids, meta, embedding, updated_at)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::vector, now())
ON CONFLICT (id) DO UPDATE SET
  kind = EXCLUDED.kind,
  title = EXCLUDED.title,
  text = EXCLUDED.text,
  ref_ids = EXCLUDED.ref_ids,
  meta = EXCLUDED.meta,
  embedding = EXCLUDED.embedding,
  updated_at = now()
`, e.ID, string(e.Kind), e.Title, e.Text, refs, string(metaJSON), vecLit)
	}
	br := p.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range entities {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) Search(ctx context.Context, queryEmbedding []float32, k int) ([]ScoredEntity, error) {
	if k <= 0 {
		k = 5
	}
	vecLit := vectorLiteral(queryEmbedding)
	rows, err := p.pool.Query(ctx, `
SELECT id, kind, title, text, ref_ids, meta,
       1 - (embedding <=> $1::vector) AS score
FROM entities
WHERE embedding IS NOT NULL
ORDER BY embedding <=> $1::vector
LIMIT $2
`, vecLit, k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScoredEntity
	for rows.Next() {
		var (
			e       state.Entity
			kind    string
			refs    []string
			metaRaw []byte
			score   float64
		)
		if err := rows.Scan(&e.ID, &kind, &e.Title, &e.Text, &refs, &metaRaw, &score); err != nil {
			return nil, err
		}
		e.Kind = state.EntityKind(kind)
		e.RefIDs = refs
		_ = json.Unmarshal(metaRaw, &e.Meta)
		out = append(out, ScoredEntity{Entity: e, Score: score})
	}
	return out, rows.Err()
}

func (p *Postgres) Count(ctx context.Context) (int, error) {
	var n int
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM entities`).Scan(&n)
	return n, err
}

// vectorLiteral formats a float32 slice as a pgvector input string: [1,2,3]
func vectorLiteral(v []float32) string {
	if len(v) == 0 {
		// zero vector of Dim
		v = make([]float32, embed.Dim)
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%g", x)
	}
	b.WriteByte(']')
	return b.String()
}


// Reset truncates the entities table (bench reindex).
func (p *Postgres) Reset(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `TRUNCATE entities`)
	return err
}

// OpenOrMemory tries Postgres; on failure returns a Memory store and a warning string.
func OpenOrMemory(ctx context.Context) (VectorStore, string) {
	pg, err := TryOpenPostgres(ctx)
	if err != nil {
		return NewMemory(), fmt.Sprintf("postgres unavailable (%v); using in-memory vector store", err)
	}
	return pg, ""
}
