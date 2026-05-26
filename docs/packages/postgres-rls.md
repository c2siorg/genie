# pkg/storage/postgres — tenant context + Row-Level Security

> **Where:** `pkg/storage/postgres/tenant.go` + `migrations/0005_rls.sql`
> **Lines of code:** ~100 (Go) + ~100 (SQL) · **Tests:** 2 (contract) + integration coverage in `tests/security_envelope_test.go`
> **FREE-AI alignment:** Rec 15 (Data Lifecycle Governance), Rec 22 (Tamper-Evident Audit)

---

## Overview

Tenant isolation as a **database-enforced contract**, not an
application-enforced one. The motivation is straightforward: every
application has bugs. If the only thing standing between user A and
user B's data is the `WHERE user_id = $1` clause in a hand-written
SQL query, then a single missed filter is a cross-tenant leak. RLS
moves the filter into the database itself — Postgres refuses to
return rows whose tenant column doesn't match the session's
`app.current_tenant` GUC, no matter what the query says.

This package is the small Go layer that sets the GUC at the start of
every txn (via `SET LOCAL`), plus the migration that enables RLS on
every table that carries a tenant column.

Pairs with `pkg/governance.TenantPolicy` (the bus-layer check). The
two together implement defence in depth: a bug in the bus check still
hits RLS at the DB; a bug in RLS configuration still has the bus.

---

## Surface

```go
// AdminTenant is the sentinel that unlocks cross-tenant reads in
// admin-context RLS policies. Use only at login + the admin-only
// audit/inventory endpoints.
const AdminTenant = "__admin__"

// ErrNoTenant is returned when WithTenant is called without a tenant.
var ErrNoTenant = errors.New("postgres: tenant id is required")

// TenantFunc is the application closure run inside a tenant-scoped txn.
type TenantFunc func(ctx context.Context, tx pgx.Tx) error

// WithTenant runs fn inside a transaction with app.current_tenant set
// to tenantID via SET LOCAL.
func (db *DB) WithTenant(ctx context.Context, tenantID string, fn TenantFunc) error

// WithAdminContext runs fn with app.current_tenant = '__admin__'.
// Acceptable only at login, the admin audit reader, and background jobs.
func (db *DB) WithAdminContext(ctx context.Context, fn TenantFunc) error
```

Both helpers `BeginTx` → `SELECT set_config('app.current_tenant', $1, true)` → run fn → commit/rollback.

---

## Why SET LOCAL via set_config

```go
tx.Exec(ctx, "SELECT set_config($1, $2, true)", tenantSetting, tenant)
```

Three things to notice:

1. **`SET LOCAL`** (the third arg `true` to `set_config`) scopes the
   GUC to the current transaction. When the txn commits or rolls back,
   the GUC is gone. The pool may then hand the same physical connection
   to another request — that request's `SET LOCAL` (or its absence)
   takes effect. Without `LOCAL`, the GUC would leak to the next user
   of the connection.
2. **`set_config(name, value, is_local)`** is used instead of the
   `SET LOCAL …` statement because the SQL form doesn't accept
   parameterised values. `set_config` takes a string and respects the
   local flag, so we can safely bind the tenant id.
3. The call goes through `tx.Exec` (not `db.Pool.Exec`) so the GUC is
   set on the same connection the subsequent application queries run on.
   The pool guarantees a transaction owns a single connection for its
   lifetime.

---

## The RLS migration (0005_rls.sql)

Every table that carries a tenant column gets two statements:

```sql
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE documents FORCE ROW LEVEL SECURITY;
```

- `ENABLE` turns RLS on for non-owner roles.
- `FORCE` extends the policy to table owners. Without `FORCE`, the
  migration role and any future superuser would silently bypass RLS,
  which is exactly the audit-day surprise we want to avoid.

Then the per-table policy:

```sql
CREATE POLICY documents_tenant_isolation ON documents
    USING (user_id::text = current_setting('app.current_tenant', true))
    WITH CHECK (user_id::text = current_setting('app.current_tenant', true));
```

- `USING` filters rows on SELECT / UPDATE / DELETE.
- `WITH CHECK` validates rows on INSERT / UPDATE — you cannot insert
  a row whose `user_id` is a tenant other than the session's.
- The `, true)` second arg to `current_setting` is "missing_ok=true":
  an unset GUC returns the empty string instead of raising. That
  matches our intent: if `app.current_tenant` is never set, every
  policy check fails and the user sees no rows.

The migration covers `documents`, `accounts`, `mcp_tokens`,
`incidents`, and `users`. Each table has the right column shape:

| Table | Tenant column | Notes |
|---|---|---|
| documents | `user_id::text` | Default isolation pattern |
| accounts | `user_id::text` | Same |
| mcp_tokens | `user_id::text` | Crucial — third-party API tokens must never cross tenants |
| incidents | `affected_id` (nullable) | Null = system-level, visible to `__admin__` only |
| users | `id::text` | The user's own row is visible to themselves |

The `users` and `incidents` policies include an `OR` clause that
opens cross-tenant access when `current_setting('app.current_tenant',
true) = '__admin__'`. This is the only legitimate cross-tenant access
pattern (login lookup, audit reader). Application code that wants it
must call `WithAdminContext` — there's no way to set the sentinel
"by accident."

---

## Wire flow (single request)

```
HTTP handler ─┐
              │ 1. JWT mid extracts user_id from claims
              ▼
db.WithTenant(ctx, userID, func(ctx, tx) error {
    // 2. set_config('app.current_tenant', user_id, true)   ← inside txn
    rows, err := tx.Query(ctx, "SELECT id FROM documents")  // ← no WHERE needed
    //   → Postgres applies the RLS policy and returns only
    //     this user's rows, regardless of the query text
})
              │ 3. commit or rollback, GUC dies with the txn
              ▼
pool returns connection — next request starts clean
```

The middle step is what's new. Today, application code typically
writes `WHERE user_id = $1` everywhere — a missed clause is a leak.
With RLS, the clause is implicit and the DB enforces it.

---

## Defence in depth

This package is one of two tenant-isolation layers:

| Layer | Where | What it catches |
|---|---|---|
| Bus | `pkg/governance.TenantPolicy` | Cross-tenant message dispatch before the agent runs |
| DB | this package + 0005_rls.sql | Cross-tenant SQL read/write after the agent runs |

A bug in either layer is contained by the other. A missed
`tenant_id` check at the bus still hits RLS at the DB and gets
denied there. A missing `FORCE ROW LEVEL SECURITY` clause still has
the bus check.

The other security primitives shipped in the same wave:

- **`pkg/auth/tokenexchange`** — gives the upstream API a token whose
  `Subject` is the user and `Actor` is the agent. The audit log can
  reconstruct "user A, via agent X, read row R" even though the SQL
  itself was issued by the agent's connection.
- **`pkg/agent.Tier`** — only production-tier agents are eligible for
  customer-facing dispatch, which constrains who can even reach the
  query path that the RLS policy guards.

---

## What this package does *not* do

- **It doesn't replace `WHERE user_id = $1`**. Application queries
  should still include the clause where the developer knows the
  tenant — it's documentation and a fail-fast when the GUC is unset.
  RLS is the "second pair of eyes," not the primary check.
- **It doesn't enforce role-based access**. A user with `__admin__`
  context can read any row. RBAC at the HTTP middleware layer
  decides who is allowed to call `WithAdminContext` in the first
  place.
- **It doesn't audit-log the tenant context per query**. The audit
  layer logs the request's authenticated user; the DB-side GUC is
  derived from that. If the two ever diverge, the request-level audit
  is the source of truth.
- **It doesn't handle multi-tenant `org_id` semantics yet**. The
  current model treats `user_id` as the tenant for consumer banking.
  Corporate banking (one org = many users) needs a second column
  + policy update — tracked in the operations doc.

---

## Tests

`pkg/storage/postgres/tenant_test.go` covers:

- `WithTenant("", …)` returns `ErrNoTenant`.
- `AdminTenant` constant is the exact string `"__admin__"` so a doc
  drift on the sentinel cannot pass review unnoticed.

The integration test in `tests/security_envelope_test.go` exercises
RLS conceptually by asserting the `pkg/governance.TenantPolicy` bus
layer rejects untenanted messages before they ever reach a handler
that would query Postgres — both layers must be in place for the
end-to-end envelope to hold.

A live-DB integration test (spin up Postgres, run the migration,
attempt cross-tenant read) belongs in a separate `tests/dbintegration`
package; it's not yet in this repo because the CI doesn't currently
spin up Postgres. The contract is documented and exercised in
production via the manual ops test in `docs/operations.md`.

---

## FREE-AI mapping

- **Rec 15 — Data Lifecycle Governance.** "Implement data classification
  and access controls aligned with the sensitivity tier." RLS turns
  the access control from "the application remembers to filter" into
  "the database refuses unauthorised reads."
- **Rec 22 — Tamper-Evident Audit.** RLS doesn't write audit entries
  itself, but it complements the `pkg/audit` log: even if an audit
  entry is missing for a row read, the read couldn't have happened
  in the first place if the tenant context was wrong.

---

## Pointers

- Migration: `pkg/storage/postgres/migrations/0005_rls.sql`
- Helpers: `pkg/storage/postgres/tenant.go`
- Tests: `pkg/storage/postgres/tenant_test.go`,
  `tests/security_envelope_test.go`
- Bus-layer counterpart: [governance-tenant.md](governance-tenant.md)
- Operational runbook: `docs/operations.md` (RLS migration section)
