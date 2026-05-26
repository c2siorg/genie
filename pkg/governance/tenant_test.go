package governance

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

func mkMsg(typ string, meta map[string]any) protocol.Message {
	return protocol.Message{
		ID: "m", From: "u", To: "agent",
		Role: protocol.RoleUser, Type: typ, Content: "x", Metadata: meta,
	}
}

func TestTenantPolicyDeniesMissingTenant(t *testing.T) {
	p := TenantPolicy{}
	res, _ := p.Evaluate(context.Background(), mkMsg("finance_question", nil))
	if res.Decision != DecisionDeny {
		t.Errorf("missing tenant_id must deny; got %s", res.Decision)
	}
}

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

func TestTenantPolicyAppliesToFilter(t *testing.T) {
	// AppliesTo lists only "finance_question"; an "audit_read" message
	// without tenant_id should pass because the policy doesn't apply.
	p := TenantPolicy{AppliesTo: []string{"finance_question"}}
	res, _ := p.Evaluate(context.Background(), mkMsg("audit_read", nil))
	if res.Decision != DecisionAllow {
		t.Errorf("policy with AppliesTo filter should not block other types; got %s", res.Decision)
	}
}

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
