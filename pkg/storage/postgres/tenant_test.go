// tenant_test.go — unit tests for the tenant-scoped helper.
//
// The integration test against a live Postgres lives in tests/ — it boots a
// docker container and verifies RLS denial end-to-end. This file covers the
// parts that don't need a database: input validation, sentinel handling.
package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestWithTenantRejectsEmpty(t *testing.T) {
	// We can construct a DB stub with a nil pool since we never reach the
	// pool: the empty-tenant check fires first.
	db := &DB{Pool: nil}
	err := db.WithTenant(context.Background(), "", func(ctx context.Context, tx pgx.Tx) error {
		t.Fatal("fn should not run with empty tenant")
		return nil
	})
	if err != ErrNoTenant {
		t.Errorf("expected ErrNoTenant, got %v", err)
	}
}

func TestAdminTenantSentinelValue(t *testing.T) {
	// Hard-coded so accidental refactors don't shift the policy contract.
	if AdminTenant != "__admin__" {
		t.Errorf("AdminTenant sentinel must remain '__admin__'; got %q", AdminTenant)
	}
	if tenantSetting != "app.current_tenant" {
		t.Errorf("tenantSetting must remain 'app.current_tenant' to match RLS policy; got %q", tenantSetting)
	}
}
