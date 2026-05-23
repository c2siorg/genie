-- MCP per-user tokens, encrypted at rest via pkg/crypto envelope.
CREATE TABLE IF NOT EXISTS mcp_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,                       -- e.g. "zerodha-kite"
    endpoint    TEXT NOT NULL,                       -- MCP endpoint URL
    payload     JSONB NOT NULL,                      -- crypto.EncryptedPayload
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_mcp_tokens_user ON mcp_tokens(user_id);

-- Consent ledger (RBI / DPDP alignment). Records the explicit user consent
-- for each data category (e.g. "transactions", "portfolio") and whether it
-- has been revoked.
CREATE TABLE IF NOT EXISTS consents (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category    TEXT NOT NULL,
    purpose     TEXT NOT NULL,
    granted     BOOLEAN NOT NULL DEFAULT TRUE,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_consents_user ON consents(user_id);

-- Append-only audit log with hash chaining for tamper-evidence.
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor       TEXT NOT NULL,           -- user id, agent id, "system"
    action      TEXT NOT NULL,           -- e.g. "consent.grant", "msg.deny"
    target      TEXT NOT NULL DEFAULT '',
    details     JSONB,
    prev_hash   BYTEA NOT NULL,          -- hash of the previous row
    row_hash    BYTEA NOT NULL           -- hash(prev_hash || canonical row)
);
