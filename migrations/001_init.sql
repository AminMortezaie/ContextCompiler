-- Context Compiler schema: entities + pgvector embeddings (dim 384).
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

-- HNSW for cosine similarity; works well for offline deterministic vectors.
CREATE INDEX IF NOT EXISTS entities_embedding_hnsw
    ON entities USING hnsw (embedding vector_cosine_ops);

CREATE INDEX IF NOT EXISTS entities_kind_idx ON entities (kind);
