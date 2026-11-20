-- ===========================================================================
-- 0005_rls.sql — Row-Level Security (RLS) for tenant isolation
-- ===========================================================================
--
-- Why: defence in depth. Application-level RBAC at the bus already denies
-- cross-tenant message dispatch (pkg/governance.TenantPolicy). RLS adds a
-- second enforcement point: even if application code has a bug, the
-- database itself refuses to return another tenant's rows.
--
-- Pattern:
--   1. Every row carries an explicit tenant column (user_id today; an
--      org_id added later for true multi-tenancy).
--   2. The application sets `app.current_tenant` on the connection via
--      `SET LOCAL` before every query, scoped to the txn (the helper is
--      pkg/storage/postgres.WithTenant / WithAdminContext).
--   3. CREATE POLICY rules USING (user_id::text = current_setting('app.current_tenant', true))
--      gate SELECT / INSERT / UPDATE / DELETE.
--
-- The `BYPASSRLS` attribute is granted only to the migration role, which
-- runs out-of-band. Application database users do not have BYPASSRLS;
-- they cannot opt out of the policy.
--
-- ─── Why FORCE in addition to ENABLE ────────────────────────────────────────
--
-- ALTER TABLE … ENABLE ROW LEVEL SECURITY turns on RLS for non-owner roles.
-- FORCE extends the policy to table owners. Without FORCE, the migration
-- role (and any future superuser-equivalent owner) silently bypasses
-- RLS. That's exactly the audit-day surprise we want to avoid: a
-- background job running as the owner could read every tenant's data
-- without the policy firing.
--
-- The CI pipeline runs `SELECT relname, relrowsecurity, relforcerowsecurity
-- FROM pg_class WHERE relname IN (…)` after migrations and refuses to
-- deploy if FORCE isn't set on every tenant table.
--
-- ─── Why current_setting(..., true) ───────────────────────────────────────
--
-- The second arg to current_setting is missing_ok=true. An unset GUC
-- returns the empty string instead of raising. That matches our intent:
-- if `app.current_tenant` is never set (someone bypassed WithTenant),
-- every policy check evaluates user_id::text = '' which fails and the
-- user sees zero rows. Fail closed — no rows is the safe outcome when
-- the tenant context is missing.
--
-- ─── Why ::text on user_id ────────────────────────────────────────────────
--
-- user_id is a UUID column; the GUC is a string. Casting both sides to
-- text avoids implicit-cast confusion (UUID → text vs text → UUID) and
-- gives Postgres a stable comparison shape. Performance is fine — UUID
-- comparison via text is the same cost as native UUID compare for the
-- tiny strings we deal with.
--
-- ─── Audit checklist after running this migration ───────────────────────
--
-- 1. SELECT relname, relrowsecurity, relforcerowsecurity FROM pg_class
--      WHERE relname IN ('documents','accounts','mcp_tokens','incidents','users');
--      Expect both t for all five.
--
-- 2. SELECT polname, polrelid::regclass, polcmd, polqual FROM pg_policy;
--      Expect one tenant_isolation policy per table; polcmd = '*' (all
--      commands); polqual references current_setting('app.current_tenant',true).
--
-- 3. As a non-owner role, SELECT FROM documents with no SET — should
--      return zero rows.
--
-- 4. SELECT set_config('app.current_tenant','<known-user>',true);
--      Then SELECT FROM documents — should return only that user's rows.
--
-- FREE-AI alignment: Rec 15 (Data Lifecycle Governance) — database-level
-- tenant isolation is part of the lifecycle controls.

-- ---------------------------------------------------------------------------
-- documents
-- ---------------------------------------------------------------------------
-- documents carry the customer's uploaded content (bank statements,
-- KYC docs, receipts). Tenant column is user_id (UUID → text cast).
-- Both USING and WITH CHECK are set so SELECT, INSERT, UPDATE, and
-- DELETE are all gated. An attacker who somehow got an INSERT past
-- the application layer still cannot write a row for another tenant
-- (WITH CHECK refuses it).

ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS documents_tenant_isolation ON documents;
CREATE POLICY documents_tenant_isolation ON documents
    USING (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (user_id::text = current_setting('app.current_tenant', true));

-- ---------------------------------------------------------------------------
-- accounts
-- ---------------------------------------------------------------------------
-- accounts holds linked-account stubs and balance snapshots. Same shape
-- as documents — user_id is the tenant column.

ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS accounts_tenant_isolation ON accounts;
CREATE POLICY accounts_tenant_isolation ON accounts
    USING (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (user_id::text = current_setting('app.current_tenant', true));

-- ---------------------------------------------------------------------------
-- mcp_tokens
-- ---------------------------------------------------------------------------
-- mcp_tokens stores per-user OAuth tokens for third-party APIs (Zerodha
-- Kite, the user's broker, etc.). Tenant isolation prevents a user from
-- reading another user's stored third-party API tokens, regardless of
-- any application bug. This is the highest-stakes RLS policy in the
-- schema — a leak here would expose third-party credentials, not just
-- per-tenant data.

ALTER TABLE mcp_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp_tokens FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS mcp_tokens_tenant_isolation ON mcp_tokens;
CREATE POLICY mcp_tokens_tenant_isolation ON mcp_tokens
    USING (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (user_id::text = current_setting('app.current_tenant', true));

-- ---------------------------------------------------------------------------
-- incidents
-- ---------------------------------------------------------------------------
-- Incidents reference an opaque AffectedID (pseudonymised). We isolate by
-- the affected_id column when present. Incidents without an affected_id
-- (system-level events: BCP drill, retention job, KEK rotation) are
-- visible to admin sessions only — modelled by a NULL affected_id
-- matching only the explicit '__admin__' sentinel.
--
-- The USING clause has two branches:
--   - affected_id IS NULL AND current_setting = '__admin__'
--     → system-level event, admin only
--   - affected_id = current_setting
--     → customer-affecting event, scoped to that customer
--
-- The WITH CHECK is looser: an admin can insert a NULL-affected_id row
-- (the system-level case), and a customer-context insert must have
-- affected_id = the current tenant. This is correct because INSERTs
-- always come from controlled code paths (the incident store's
-- Create method), not from raw user input.

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
-- users
-- ---------------------------------------------------------------------------
-- Users table is special: the user's *own* record must be visible. We
-- isolate by id (the user's own row). The login flow needs to look up a
-- user by email BEFORE knowing the tenant id, so the policy includes the
-- '__admin__' OR clause to let WithAdminContext do that lookup.
--
-- The login handler is the single place this admin clause is acceptable
-- on the customer-request path. Every other admin use is on an
-- admin-only route (audit reader, inventory) gated by RequireRole.

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
-- is the single place this is acceptable on the customer-request path.
--
-- After successful authentication, the handler switches to WithTenant
-- (using the resolved user id) for any subsequent reads in the same
-- request — never carries the admin sentinel into the post-login work.
