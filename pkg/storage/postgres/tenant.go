// tenant.go — tenant-scoped query execution against Postgres RLS.
//
// ─── What this file is ──────────────────────────────────────────────────────
//
// The Go-side runtime that pairs with migrations/0005_rls.sql to enforce
// tenant isolation at the database layer. The migration installs the RLS
// policies; this file is what every application handler calls to make sure
// the connection it's about to query has the right tenant id bound to it.
//
// The contract is one sentence: "every customer-facing read or write
// happens inside a WithTenant(ctx, tenantID, fn) closure, and every
// admin/login/audit operation happens inside WithAdminContext(ctx, fn)."
// If you find a place in the codebase that opens a transaction and runs
// queries WITHOUT going through one of these two helpers, that's a tenant-
// isolation bug — open an incident.
//
// ─── Why a GUC ──────────────────────────────────────────────────────────────
//
// Postgres RLS policies are SQL expressions; they need a value to compare
// the row's tenant column against. There are three ways to supply that
// value:
//
//   (a) Pass it as a query parameter to every SELECT.
//       — Forces every caller to know about RLS; defeats the point.
//   (b) Set a session-level variable on connection open.
//       — Doesn't work with connection pools; the next request on the
//         same physical connection inherits the previous tenant.
//   (c) Set a transaction-local variable.
//       — Exactly what we want. The GUC dies with the txn; the pool can
//         hand the connection to anyone, the next caller sets their own
//         GUC, no leak.
//
// We pick (c). The GUC name is "app.current_tenant"; the value is set via
// set_config(name, value, is_local=true) inside a transaction.
//
// ─── Why set_config, not SET LOCAL ──────────────────────────────────────────
//
// SQL `SET LOCAL app.current_tenant = $1` does not accept bind parameters
// — Postgres rejects parameterised SET. set_config() is the function form;
// it takes a string value and a boolean is_local flag and respects both.
// Using it lets us bind the tenant id safely, so an attacker-controlled
// tenant id (impossible in practice — tenant comes from authenticated
// JWT claims — but defence in depth says assume it could be) can't
// inject SQL.
//
// ─── Defence in depth pairing ───────────────────────────────────────────────
//
// This is the DB-layer half. The bus-layer half is
// pkg/governance.TenantPolicy. The two together implement the rule:
// "a bug in the application code (forgot WHERE clause) is contained by
// the database; a bug in the migration (RLS policy not forced) is
// contained by the bus." Read packages/postgres-rls.md and
// packages/governance-tenant.md side by side.
//
// ─── The admin sentinel ────────────────────────────────────────────────────
//
// Some operations legitimately need cross-tenant reads:
//   - The login handler — user id isn't known until after the email lookup.
//   - The admin-only audit reader — the on-call investigates across tenants.
//   - The admin-only AI inventory endpoint — risk team reads every agent.
//   - Background jobs — retention, reconciliation, KEK rewrap.
//
// All of these call WithAdminContext, which sets the GUC to the literal
// string "__admin__". The RLS policies on `users` and `incidents` include
// an "OR current_setting('app.current_tenant', true) = '__admin__'"
// clause; no other table's policy does. Customer-facing routes never get
// to call WithAdminContext because the HTTP middleware never lets them —
// the route gate is RequireRole(RoleAdmin).
//
// The sentinel is the only legitimate cross-tenant key. A UI contract
// test (pkg/web/handlers/ui_security_test.go) refuses to let the literal
// string "__admin__" appear in any frontend asset, so the sentinel is
// invisible to anyone reading the page source.
//
// ─── Usage examples ────────────────────────────────────────────────────────
//
// Customer-facing read:
//
//     // claims came from the authenticated JWT
//     err := db.WithTenant(r.Context(), claims.Subject, func(ctx context.Context, tx pgx.Tx) error {
//         rows, err := tx.Query(ctx, "SELECT id, description FROM documents")
//         // ↑ No WHERE clause needed. RLS adds the filter implicitly.
//         //   The GUC scopes the connection; rows for any other tenant
//         //   are invisible.
//         return scan(rows)
//     })
//
// Admin-only audit reader:
//
//     err := db.WithAdminContext(r.Context(), func(ctx context.Context, tx pgx.Tx) error {
//         rows, err := tx.Query(ctx, "SELECT id, occurred_at, actor FROM audit_log")
//         // ↑ This route must be gated by RequireRole(RoleAdmin) at the router.
//         //   The admin sentinel is the only legitimate cross-tenant key.
//         return scan(rows)
//     })
//
// Bad — query outside a Wither:
//
//     rows, _ := db.Pool.Query(ctx, "SELECT * FROM documents")
//     // ↑ No GUC set. RLS sees app.current_tenant = '' (missing_ok=true on
//     //   current_setting), the policy compares user_id::text = '', no
//     //   rows match. Returns empty result — fails closed, but the caller
//     //   may interpret "empty" as "nothing for this tenant" which is a
//     //   different bug. Always use a Wither.
//
// ─── FREE-AI alignment ──────────────────────────────────────────────────────
//
// Rec 15 (Data Lifecycle Governance) — "implement data classification and
// access controls aligned with the sensitivity tier." RLS turns access
// control from "the application remembers to filter" into "the database
// refuses unauthorised reads."
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	// AdminTenant is the sentinel that unlocks cross-tenant reads in the
	// RLS policies that include an admin OR clause (today: `users` and
	// `incidents`). Setting the GUC to this exact string is the only way
	// to legitimately read another tenant's data.
	//
	// Acceptable callers (each gated by RequireRole(RoleAdmin) at the
	// router level):
	//   - The login handler (`pkg/web/handlers/users.go` — needs to look
	//     up a user by email before knowing the tenant id).
	//   - The admin-only audit reader.
	//   - The admin-only AI inventory endpoint.
	//   - Background jobs (retention, reconciliation, rewrap).
	//
	// Never use this in a user-request handler that should be tenant-
	// scoped — the dispatch gate (RequireRole) is the only protection
	// against misuse; if a customer-facing route reaches this constant,
	// you have a privilege-escalation bug.
	//
	// A UI contract test pins that this exact string never appears in
	// any frontend asset, so the sentinel cannot leak via page source.
	AdminTenant = "__admin__"

	// tenantSetting is the Postgres GUC name. The RLS policies all read
	// current_setting('app.current_tenant', true) — the second arg is
	// missing_ok=true so an unset GUC returns the empty string instead
	// of raising. That matches our intent: if the GUC is never set
	// (someone bypassed the Wither), every policy check fails and the
	// user sees no rows. Fail closed.
	//
	// The name is namespaced with "app." because Postgres requires
	// custom GUCs to have a prefix; "app." is convention. If you change
	// this string, you MUST update every CREATE POLICY in
	// migrations/0005_rls.sql in the same commit.
	tenantSetting = "app.current_tenant"
)

// ErrNoTenant is returned when WithTenant is called without a tenant id.
//
// Why we error instead of defaulting to the admin sentinel: defaulting
// would convert a missing tenant_id (a real bug at the call site) into a
// silent privilege escalation. Erroring fails the request loudly so the
// bug is visible immediately.
var ErrNoTenant = errors.New("postgres: tenant id is required")

// TenantFunc is the application closure run inside a tenant-scoped txn.
//
// The signature takes a context and the bound pgx.Tx. The tx is the only
// thing the closure should use to talk to the database — using db.Pool
// directly would open a separate connection without the GUC and miss
// RLS entirely. (Lint rule worth adding: ban db.Pool inside a TenantFunc.)
//
// Returning an error from fn rolls back the transaction; returning nil
// commits it. Use this for transactional consistency, not just for RLS —
// a failed business rule should leave the database unchanged.
type TenantFunc func(ctx context.Context, tx pgx.Tx) error

// WithTenant runs fn inside a transaction with app.current_tenant set
// to tenantID via SET LOCAL.
//
// Concurrency: safe. Each call gets its own transaction from the pool.
// The GUC is local to the txn, so two concurrent WithTenant calls cannot
// see each other's tenant context even if they happen to be assigned the
// same physical connection in sequence.
//
// Performance: BeginTx + set_config + Commit add roughly one round-trip
// pair to every query batch. For OLTP workloads this is acceptable
// (sub-millisecond on a local DB); for analytical queries that touch
// many rows the overhead is amortised over the query itself.
//
// Error handling: a non-empty tenantID is required (ErrNoTenant). A
// failed BeginTx, failed set_config, failed fn, or failed Commit all
// propagate up; the txn is rolled back on any error along the way.
//
// Sentinel attack surface: an attacker who could pass tenantID = "__admin__"
// would gain cross-tenant access. This is mitigated by the fact that the
// tenant id always comes from authenticated JWT claims (claims.Subject) —
// no HTTP handler should ever take a tenant id from a request body or
// query string. If you find a handler that does, fix it before the code
// review ends.
func (db *DB) WithTenant(ctx context.Context, tenantID string, fn TenantFunc) error {
	// Reject the empty tenant up front. Defence-in-depth — if the caller
	// forgot to populate tenantID, we don't silently degrade to the admin
	// sentinel; we error loudly.
	if tenantID == "" {
		return ErrNoTenant
	}
	// Delegate to the shared runner. The runner handles BeginTx,
	// set_config, fn invocation, and commit/rollback. We pass the tenant
	// id straight through; runWithSetting binds it safely.
	return db.runWithSetting(ctx, tenantID, fn)
}

// WithAdminContext runs fn inside a transaction with app.current_tenant
// set to the AdminTenant sentinel. The RLS policies that include the
// admin OR clause grant cross-tenant access in this mode.
//
// Acceptable use:
//   - Login handler (user identity is not known until after email lookup)
//   - Admin-only audit log reader
//   - Admin-only AI inventory endpoint
//   - Background jobs (retention, reconciliation)
//
// Never use this in a user-request handler that should be tenant-scoped.
// The HTTP router enforces this by gating every route that calls this
// helper with RequireRole(RoleAdmin) — if a customer can reach the
// route, the route can reach this helper, which is a privilege
// escalation. Audit the router before adding a new caller.
//
// The function does not take a tenant id; the sentinel is hard-coded.
// That's intentional: there's no way to call this helper with a value
// other than the sentinel, so a typo or attacker-controlled string
// cannot accidentally land here.
func (db *DB) WithAdminContext(ctx context.Context, fn TenantFunc) error {
	// Pass the hard-coded sentinel. The same runner that WithTenant uses
	// — set_config doesn't care whether the value is a user id or the
	// sentinel; the RLS policy is what differentiates.
	return db.runWithSetting(ctx, AdminTenant, fn)
}

// runWithSetting is the shared transactional runner. It exists so that
// the two public Wither functions can share their entire body while
// keeping their preconditions distinct (tenant-id-required vs sentinel).
//
// Lifecycle:
//   1. BeginTx — gets a connection from the pool, starts a txn.
//   2. SELECT set_config('app.current_tenant', $1, true) — binds the GUC
//      to the txn. The "true" makes it SET LOCAL semantics.
//   3. Run fn(ctx, tx) — application work. RLS applies to every query
//      on tx because the GUC is set on this connection.
//   4. Commit (on nil error) or Rollback (on any error).
//
// On any failure mid-flight, the txn is rolled back. The GUC dies with
// the txn — there is no leak path even if Commit itself errors.
//
// Note: we deliberately don't return the txn or the commit error to the
// closure. If fn needs to know whether commit succeeded, it should do
// its own commit accounting; the helper's job is "RLS context + atomic
// txn," not "fully-featured txn manager."
func (db *DB) runWithSetting(ctx context.Context, tenant string, fn TenantFunc) error {
	// BeginTx with default options — read committed, no read-only flag.
	// If a future caller needs SERIALIZABLE or ReadOnly txns, add a
	// WithTenantOpts variant rather than overloading this helper.
	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		// Wrap so the caller sees "begin tx" in the chain — helps debug
		// pool exhaustion vs network errors vs auth errors.
		return fmt.Errorf("begin tx: %w", err)
	}
	// SET LOCAL is scoped to the current txn. The pool may reuse this
	// connection for a different request after commit/rollback; that
	// request's SET LOCAL (or its absence) overrides.
	//
	// We use set_config(name, value, is_local) because parameterised SET
	// rejects non-literal values; set_config takes a string and respects
	// the local flag. Binding via $1/$2 is safe — Postgres parameterises
	// the value, not the GUC name.
	//
	// If this Exec fails, we explicitly rollback (ignoring the rollback
	// error — there's nothing useful to do with it; the original error
	// is more informative) and return the wrapped error.
	if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", tenantSetting, tenant); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("set tenant: %w", err)
	}
	// Run the application closure. fn does whatever queries it needs;
	// RLS is now active and will filter rows according to the GUC.
	//
	// If fn returns an error, we rollback. A returned error means the
	// business logic decided "don't commit" — possibly because a
	// validation failed, possibly because a downstream query failed.
	// Either way, the partial state from fn's earlier statements is
	// discarded.
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	// fn succeeded; commit. If commit itself errors (network, conflict,
	// constraint), the txn is automatically rolled back by Postgres and
	// the caller sees the commit error.
	return tx.Commit(ctx)
}
