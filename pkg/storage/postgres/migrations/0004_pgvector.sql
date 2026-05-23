-- pgvector extension + embeddings table for pkg/rag/pgvector.
-- Idempotent so re-running the migration set is safe.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS rag_embeddings (
    namespace   TEXT NOT NULL,
    chunk_id    TEXT NOT NULL,
    source      TEXT NOT NULL DEFAULT '',
    title       TEXT NOT NULL DEFAULT '',
    body        TEXT NOT NULL,
    metadata    JSONB,
    embedding   vector NOT NULL, -- dimension is enforced per-namespace in app code
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, chunk_id)
);

-- No ANN index by default — Genie supports multiple embedder dimensions
-- (HashEmbedder=256, Ollama nomic-embed-text=768) and ivfflat / hnsw require
-- the column to declare a fixed dimension. Sequential scan is fine for demo
-- corpora; once you standardize on one embedder, ALTER the column to
-- vector(N) and add e.g.:
--   CREATE INDEX idx_rag_embeddings_cosine ON rag_embeddings
--     USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
