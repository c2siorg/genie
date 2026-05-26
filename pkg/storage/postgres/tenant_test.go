// tenant_test.go — contract tests for the tenant-scoping helpers.
//
// ─── What these tests are ───────────────────────────────────────────────────
//
// These live alongside tenant.go and run on every `go test ./...`.
// They are NOT integration tests — they don't spin up Postgres. The
// integration test (live DB + RLS verify) belongs in a separate
// tests/dbintegration package once CI has a Postgres step.
//
// ─── What we pin here ──────────────────────────────────────────────────────
//
//   - WithTenant rejects the empty tenant id with ErrNoTenant.
//   - The AdminTenant sentinel is exactly "__admin__" — any drift would
//     break the RLS policies in 0005_rls.sql which compare against this
//     exact string literal.
//   - The tenantSetting GUC name is exactly "app.current_tenant" — same
//     reason; the SQL CREATE POLICY references this string.
//
// ─── Why pin the sentinel ───────────────────────────────────────────────────
//
// The migration SQL hard-codes "__admin__" in the USING / WITH CHECK
// clauses. If a future refactor changed the Go constant without
// updating the SQL, the bypass would silently stop working (or worse,
// a different unintended string would become a bypass). The test
// fails the build on any drift, forcing whoever changed the constant
// to also update the SQL in the same commit.
package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestWithTenantRejectsEmpty asserts that the helper refuses to run
// the closure with an empty tenant id.
//
// Why this matters: defaulting an empty tenant id to the admin sentinel
// (or to "anyone") would convert a missing-claim bug at the call site
// into a silent privilege escalation. Erroring fails the request loudly
// so the bug is visible immediately.
//
// We pass a closure that t.Fatal's — it should never run, because the
// precondition check fires before the runner gets to invoke it. If
// we ever see "fn should not run" fired here, the precondition has
// regressed.
func TestWithTenantRejectsEmpty(t *testing.T) {
	// We can construct a DB stub with a nil pool since we never reach the
	// pool: the empty-tenant check fires first. If a future refactor
	// moves the check after BeginTx, this test would nil-deref —
	// which is a louder signal than a silent behavioural change.
	db := &DB{Pool: nil}
	// Pass a closure that fails the test if it runs. The precondition
	// check should fire before the runner ever calls fn.
	err := db.WithTenant(context.Background(), "", func(ctx context.Context, tx pgx.Tx) error {
		t.Fatal("fn should not run with empty tenant")
		return nil
	})
	// Compare against the sentinel error. ErrNoTenant is exported so
	// downstream code can errors.Is for it; this test reads the same
	// surface a caller would.
	if err != ErrNoTenant {
		t.Errorf("expected ErrNoTenant, got %v", err)
	}
}

// TestAdminTenantSentinelValue pins the exact string values of the
// admin sentinel and the GUC name.
//
// Both strings appear hard-coded in migrations/0005_rls.sql:
//   - The CREATE POLICY clauses on `users` and `incidents` literally
//     compare against '__admin__'.
//   - Every CREATE POLICY clause reads current_setting('app.current_tenant', true).
//
// If either Go constant drifts, the SQL stops matching the runtime
// value and the security model silently breaks. This test forces the
// two to stay in sync at compile time (well, test time — but CI
// catches it before deploy).
//
// To deliberately change either string:
//   1. Update the constant in tenant.go.
//   2. Update every reference in migrations/0005_rls.sql.
//   3. Update this test.
//   All three in the same commit, or the build fails.
func TestAdminTenantSentinelValue(t *testing.T) {
	// Hard-coded so accidental refactors don't shift the policy contract.
	if AdminTenant != "__admin__" {
		t.Errorf("AdminTenant sentinel must remain '__admin__' to match the RLS policy clauses in migrations/0005_rls.sql; got %q", AdminTenant)
	}
	// Same reason — the GUC name appears in every CREATE POLICY in the
	// migration. A drift here would mean WithTenant sets a GUC the
	// policies don't read.
	if tenantSetting != "app.current_tenant" {
		t.Errorf("tenantSetting must remain 'app.current_tenant' to match RLS policy; got %q", tenantSetting)
	}
}
