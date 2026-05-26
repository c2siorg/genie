// tenant.go — tenant-scoped query execution against Postgres RLS.
//
// The Row-Level Security policies in migrations/0005_rls.sql read the
// `app.current_tenant` GUC. The application sets this GUC inside a
// transaction via `SET LOCAL` before running queries. The setting is
// scoped to the txn — pgxpool may hand the same connection back to a
// different request, and the next user's SET LOCAL will overwrite ours.
// Using SET LOCAL avoids leaking the tenant across requests on a shared
// connection.
//
// Usage from a handler:
//
//	err := db.WithTenant(ctx, userID, func(ctx context.Context, tx pgx.Tx) error {
//	    rows, err := tx.Query(ctx, "SELECT id, description FROM documents WHERE id = $1", docID)
//	    // RLS filters out rows for any other tenant; no application check needed.
//	    return scan(rows)
//	})
//
// For admin / login flows that legitimately need to read across tenants
// (the email→user lookup at sign-in, the audit-log reader, etc.), call
// WithAdminContext. This is the only place '__admin__' is acceptable.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	// AdminTenant is the sentinel that unlocks cross-tenant reads in the
	// RLS policies that include the OR clause. Use only at the login
	// handler and the admin-only audit / inventory endpoints.
	AdminTenant = "__admin__"

	tenantSetting = "app.current_tenant"
)

// ErrNoTenant is returned when WithTenant is called without a tenant id.
var ErrNoTenant = errors.New("postgres: tenant id is required")

// TenantFunc is the application closure run inside a tenant-scoped txn.
// The tx is bound to the GUC set on the connection; queries through it
// see only rows that match the tenant.
type TenantFunc func(ctx context.Context, tx pgx.Tx) error

// WithTenant runs fn inside a transaction with app.current_tenant set
// to tenantID via SET LOCAL. The setting is scoped to the txn, so when
// the pool hands the connection back to another request the next caller's
// SET LOCAL (or its absence) takes effect.
//
// Returning an error from fn rolls back the transaction. Returning nil
// commits it.
func (db *DB) WithTenant(ctx context.Context, tenantID string, fn TenantFunc) error {
	if tenantID == "" {
		return ErrNoTenant
	}
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
func (db *DB) WithAdminContext(ctx context.Context, fn TenantFunc) error {
	return db.runWithSetting(ctx, AdminTenant, fn)
}

func (db *DB) runWithSetting(ctx context.Context, tenant string, fn TenantFunc) error {
	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// SET LOCAL is scoped to the current txn. The pool may reuse this
	// connection for a different request after commit/rollback; that
	// request's SET LOCAL (or its absence) overrides.
	//
	// We use set_config(name, value, is_local) because parameterised SET
	// rejects non-literal values; set_config takes a string and respects
	// the local flag.
	if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", tenantSetting, tenant); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("set tenant: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
