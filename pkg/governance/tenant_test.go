// tenant_test.go — contract tests for the bus-layer TenantPolicy.
//
// ─── What's pinned here ────────────────────────────────────────────────────
//
// Seven tests covering every branch of TenantPolicy.Evaluate plus the
// metaStringSlice coercion helper:
//
//   1. Missing tenant_id → deny.
//   2. Matching tenant + expected_tenant → allow.
//   3. Mismatched tenant + expected_tenant → deny (cross-tenant attempt).
//   4. Message type not in AppliesTo → allow (policy not applicable).
//   5. Admin role bypasses when AdminBypass=true.
//   6. Admin role does NOT bypass when AdminBypass=false (default-off).
//   7. metaStringSlice coerces []any → []string with non-string elements dropped.
//
// ─── Why each test exists ──────────────────────────────────────────────────
//
// Each test is named after the invariant it pins, and the failure
// message names the security consequence ("cross-tenant attempt") so
// the on-call engineer reading the CI log understands what regressed.
package governance

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// mkMsg is a tiny constructor for protocol.Message — keeps the test
// bodies short by hiding the From/To/Role boilerplate that doesn't
// vary across these tests.
//
// Only typ (message type) and meta (metadata map) actually vary; the
// rest are fixed defaults that don't influence policy decisions.
func mkMsg(typ string, meta map[string]any) protocol.Message {
	return protocol.Message{
		ID: "m", From: "u", To: "agent",
		Role: protocol.RoleUser, Type: typ, Content: "x", Metadata: meta,
	}
}

// TestTenantPolicyDeniesMissingTenant — the most basic invariant.
// A message without a tenant_id in metadata must be denied. No
// AppliesTo filter is set, so the policy applies to every type;
// no role hints, no expected_tenant — just a bare message.
//
// Failure mode if this regressed: any agent could be invoked
// without a tenant context, leading to RLS-empty results or worse,
// admin-tenanted operations from an unauthenticated path.
func TestTenantPolicyDeniesMissingTenant(t *testing.T) {
	p := TenantPolicy{}
	res, _ := p.Evaluate(context.Background(), mkMsg("finance_question", nil))
	if res.Decision != DecisionDeny {
		t.Errorf("missing tenant_id must deny; got %s", res.Decision)
	}
}

// TestTenantPolicyAllowsMatchingTenant — the happy path.
// tenant_id present AND matches expected_tenant → allow.
// This is what every well-formed customer-facing message should look
// like coming off the HTTP intake.
func TestTenantPolicyAllowsMatchingTenant(t *testing.T) {
	p := TenantPolicy{}
	res, _ := p.Evaluate(context.Background(), mkMsg("finance_question", map[string]any{
		"tenant_id":       "user-1",
		"expected_tenant": "user-1",
	}))
	if res.Decision != DecisionAllow {
		t.Errorf("matching tenant should allow; got %s (%s)", res.Decision, res.Reason)
	}
}

// TestTenantPolicyDeniesMismatch — the cross-tenant attempt.
// tenant_id != expected_tenant → deny. This is the classic
// confused-deputy attempt: the caller has authenticated as user-1
// but is trying to act on user-2.
//
// The denial reason should include both ids (verified implicitly
// — if the reason string ever stops including them, this test still
// passes but the on-call experience degrades; consider tightening
// the assertion to grep the reason).
func TestTenantPolicyDeniesMismatch(t *testing.T) {
	p := TenantPolicy{}
	res, _ := p.Evaluate(context.Background(), mkMsg("finance_question", map[string]any{
		"tenant_id":       "user-1",
		"expected_tenant": "user-2",
	}))
	if res.Decision != DecisionDeny {
		t.Errorf("tenant mismatch must deny; got %s", res.Decision)
	}
}

// TestTenantPolicyAppliesToFilter — the AppliesTo allow-list.
// AppliesTo lists only "finance_question"; an "audit_read" message
// without tenant_id should pass because the policy doesn't apply.
// This is how system-level message types (heartbeat, fallback) avoid
// being denied for not carrying a tenant.
func TestTenantPolicyAppliesToFilter(t *testing.T) {
	// AppliesTo lists only "finance_question"; an "audit_read" message
	// without tenant_id should pass because the policy doesn't apply.
	p := TenantPolicy{AppliesTo: []string{"finance_question"}}
	res, _ := p.Evaluate(context.Background(), mkMsg("audit_read", nil))
	if res.Decision != DecisionAllow {
		t.Errorf("policy with AppliesTo filter should not block other types; got %s", res.Decision)
	}
}

// TestTenantPolicyAdminBypass — the admin opt-in bypass.
// With AdminBypass=true, a user_roles array containing "admin"
// permits cross-tenant routing. Used by the audit-reader policy
// instance, the quarterly export, etc.
//
// The test deliberately mismatches tenant_id and expected_tenant —
// without the bypass, that would deny. The admin role is what makes
// the difference.
func TestTenantPolicyAdminBypass(t *testing.T) {
	p := TenantPolicy{AdminBypass: true}
	res, _ := p.Evaluate(context.Background(), mkMsg("audit_read", map[string]any{
		"tenant_id":   "user-1",
		"user_roles":  []string{"user", "admin"},
		"expected_tenant": "user-99",
	}))
	if res.Decision != DecisionAllow {
		t.Errorf("admin role with bypass should allow cross-tenant; got %s", res.Decision)
	}
}

// TestTenantPolicyAdminBypassRequiresOptIn — the security-critical
// counterpart to the previous test.
//
// Same message shape (admin role + cross-tenant attempt), but the
// policy instance has AdminBypass=false (the default). The admin
// role MUST NOT bypass — because this represents a customer-facing
// route's policy instance, and on those routes admin should not
// silently cross tenants.
//
// Regression here would mean an admin token on a customer-facing
// route silently gets cross-tenant access — privilege escalation.
func TestTenantPolicyAdminBypassRequiresOptIn(t *testing.T) {
	p := TenantPolicy{} // AdminBypass false
	res, _ := p.Evaluate(context.Background(), mkMsg("audit_read", map[string]any{
		"tenant_id":       "user-1",
		"user_roles":      []string{"admin"},
		"expected_tenant": "user-99",
	}))
	if res.Decision != DecisionDeny {
		t.Errorf("admin should NOT bypass when policy doesn't opt-in; got %s", res.Decision)
	}
}

// TestMetaStringSliceHandlesAnySlice — the JSON round-trip helper.
//
// Messages crossing the bus often carry roles as []any after JSON
// round-trip (encoding/json can't recover []string from wire bytes
// alone — every list comes back as []any). The metaStringSlice
// helper coerces []any → []string, dropping non-string elements
// silently.
//
// The dropped element (42 below) is the security-relevant part: an
// attacker-controlled JSON could inject non-string elements into the
// roles array. Dropping them is the safe choice — the worst case is
// "admin role is dropped, fewer bypasses" which is fail-closed.
//
// A future refactor that "tightens" the type assertion (refuses to
// coerce []any at all, only []string) would break every real
// production message whose JSON intake stage produces []any. This
// test makes that breakage loud at CI time rather than silent at
// runtime.
func TestMetaStringSliceHandlesAnySlice(t *testing.T) {
	// Messages crossing the bus often carry roles as []any after JSON
	// round-trip; the helper must coerce.
	got := metaStringSlice(map[string]any{
		"user_roles": []any{"user", "admin", 42}, // 42 is dropped
	}, "user_roles")
	if len(got) != 2 || got[0] != "user" || got[1] != "admin" {
		t.Errorf("expected [user admin]; got %v", got)
	}
}
