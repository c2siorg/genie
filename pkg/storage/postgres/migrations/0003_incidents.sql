-- Annexure VI — AI Incident Reporting (RBI FREE-AI Rec 22).
CREATE TABLE IF NOT EXISTS incidents (
    id                    UUID PRIMARY KEY,
    occurred_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    detected_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    use_case              TEXT NOT NULL,
    model                 TEXT NOT NULL DEFAULT '',
    third_party_vendor    TEXT NOT NULL DEFAULT '',
    description           TEXT NOT NULL,
    affected_stakeholders TEXT NOT NULL DEFAULT 'internal',
    severity              TEXT NOT NULL DEFAULT 'low',
    failure_mode          TEXT NOT NULL DEFAULT 'unknown',
    root_cause            TEXT NOT NULL DEFAULT '',
    response_actions      TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'ongoing',
    actor_id              TEXT NOT NULL DEFAULT '',
    metadata              JSONB
);

CREATE INDEX IF NOT EXISTS idx_incidents_failure_mode ON incidents(failure_mode);
CREATE INDEX IF NOT EXISTS idx_incidents_occurred_at ON incidents(occurred_at);

-- Document retention (Rec 15) — soft expiry timestamps purge job will respect.
ALTER TABLE documents      ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE mcp_tokens     ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_documents_expires_at  ON documents(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_mcp_tokens_expires_at ON mcp_tokens(expires_at) WHERE expires_at IS NOT NULL;
