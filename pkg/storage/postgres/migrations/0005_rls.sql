-- Row-Level Security (RLS) for tenant isolation.
--
-- Why: defence in depth. Application-level RBAC at the bus already denies
-- cross-tenant message dispatch (pkg/governance.RBACPolicy). RLS adds a
-- second enforcement point: even if application code has a bug, the
-- database itself refuses to return another tenant's rows.
--
-- Pattern:
--   1. Every row carries an explicit tenant column (user_id today; an
--      org_id added later for true multi-tenancy).
--   2. The application sets `app.current_tenant` on the connection via
--      `SET LOCAL` before every query, scoped to the txn.
--   3. CREATE POLICY rules USING (user_id::text = current_setting('app.current_tenant', true))
--      gate SELECT / INSERT / UPDATE / DELETE.
--
-- The `BYPASSRLS` attribute is granted only to the migration role, which
-- runs out-of-band. Application database users do not have BYPASSRLS;
-- they cannot opt out of the policy.
--
-- FREE-AI alignment: Rec 15 (Data Lifecycle Governance) — database-level
-- tenant isolation is part of the lifecycle controls.

-- ---------------------------------------------------------------------------
-- documents
-- ---------------------------------------------------------------------------

ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS documents_tenant_isolation ON documents;
CREATE POLICY documents_tenant_isolation ON documents
    USING (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (user_id::text = current_setting('app.current_tenant', true));

-- ---------------------------------------------------------------------------
-- accounts
-- ---------------------------------------------------------------------------

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS accounts_tenant_isolation ON accounts;
CREATE POLICY accounts_tenant_isolation ON accounts
    USING (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (user_id::text = current_setting('app.current_tenant', true));

-- ---------------------------------------------------------------------------
-- mcp_tokens
-- Tenant isolation prevents a user from reading another user's stored
-- third-party API tokens, regardless of any application bug.
-- ---------------------------------------------------------------------------

ALTER TABLE mcp_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp_tokens FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS mcp_tokens_tenant_isolation ON mcp_tokens;
CREATE POLICY mcp_tokens_tenant_isolation ON mcp_tokens
    USING (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (user_id::text = current_setting('app.current_tenant', true));

-- ---------------------------------------------------------------------------
-- incidents
-- Incidents reference an opaque AffectedID (pseudonymised). We isolate by
-- the affected_id column when present. Incidents without an affected_id
-- (system-level events) are visible to admin sessions only — modelled by
-- a NULL tenant matching only the explicit '__admin__' sentinel.
-- ---------------------------------------------------------------------------

ALTER TABLE incidents ENABLE ROW LEVEL SECURITY;
ALTER TABLE incidents FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS incidents_tenant_isolation ON incidents;
CREATE POLICY incidents_tenant_isolation ON incidents
    USING (
        affected_id IS NULL AND current_setting('app.current_tenant', true) = '__admin__'
        OR affected_id = current_setting('app.current_tenant', true)
    )
    WITH CHECK (
        affected_id IS NULL OR affected_id = current_setting('app.current_tenant', true)
    );

-- ---------------------------------------------------------------------------
-- Users table is special: the user's *own* record must be visible.
-- We isolate by id (the user's own row).
-- ---------------------------------------------------------------------------

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS users_self_isolation ON users;
CREATE POLICY users_self_isolation ON users
    USING (
        id::text = current_setting('app.current_tenant', true)
        OR current_setting('app.current_tenant', true) = '__admin__'
    )
    WITH CHECK (
        id::text = current_setting('app.current_tenant', true)
        OR current_setting('app.current_tenant', true) = '__admin__'
    );

-- Note: login flow must use the '__admin__' tenant context because the
-- user's identity isn't known until after email lookup. The login handler
-- is the single place this is acceptable.
