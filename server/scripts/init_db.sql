CREATE EXTENSION IF NOT EXISTS vector;

-- Creating tables so we can apply permissions and RLS
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    transaction_id VARCHAR NOT NULL UNIQUE,
    amount DOUBLE PRECISION,
    merchant VARCHAR,
    date TIMESTAMP
);

CREATE INDEX IF NOT EXISTS ix_transactions_user_id ON transactions (user_id);

CREATE TABLE IF NOT EXISTS semantic_memories (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    transaction_id VARCHAR REFERENCES transactions(transaction_id) ON DELETE CASCADE,
    content TEXT,
    embedding VECTOR(384)
);

CREATE INDEX IF NOT EXISTS ix_semantic_memories_user_id ON semantic_memories (user_id);
-- HNSW index for vector similarity search
CREATE INDEX IF NOT EXISTS ix_semantic_memories_embedding_hnsw 
    ON semantic_memories USING hnsw (embedding vector_cosine_ops) 
    WITH (m = 16, ef_construction = 64);

-- Roles
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'app_worker_role') THEN
        CREATE ROLE app_worker_role WITH LOGIN PASSWORD 'worker_password';
    ELSE
        ALTER ROLE app_worker_role WITH LOGIN PASSWORD 'worker_password';
    END IF;

    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'llm_reader_role') THEN
        CREATE ROLE llm_reader_role WITH LOGIN PASSWORD 'reader_password';
    ELSE
        ALTER ROLE llm_reader_role WITH LOGIN PASSWORD 'reader_password';
    END IF;
END
$$;

-- Grant scoped permissions on transactions table
GRANT SELECT, INSERT, UPDATE ON transactions TO app_worker_role;
GRANT SELECT ON transactions TO llm_reader_role;

-- Also grant on semantic_memories
GRANT SELECT, INSERT, UPDATE ON semantic_memories TO app_worker_role;
GRANT SELECT ON semantic_memories TO llm_reader_role;

-- Enable Row-Level Security (RLS) on transactions table
ALTER TABLE transactions ENABLE ROW LEVEL SECURITY;

-- Create an RLS policy named tenant_isolation_policy
CREATE POLICY tenant_isolation_policy ON transactions
    FOR ALL
    USING (user_id = current_setting('app.current_user_id')::uuid);

ALTER TABLE semantic_memories ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_policy_memories ON semantic_memories
    FOR ALL
    USING (user_id = current_setting('app.current_user_id')::uuid);
