package governance

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

func TestRBAC_AllowsWhenRolePresent(t *testing.T) {
	p := RBACPolicy{
		RequiredRolesByType: map[string][]string{
			"finance_question": {"user", "advisor"},
		},
	}
	msg := protocol.Message{Type: "finance_question", Metadata: map[string]any{
		protocol.MetaKeyUserRoles: "user",
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow, got %s", res.Reason)
	}
}

func TestRBAC_DeniesWhenRoleMissing(t *testing.T) {
	p := RBACPolicy{
		RequiredRolesByType: map[string][]string{
			"finance_question": {"advisor"},
		},
	}
	msg := protocol.Message{Type: "finance_question", Metadata: map[string]any{
		protocol.MetaKeyUserRoles: "user",
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionDeny {
		t.Fatalf("expected deny, got %s", res.Reason)
	}
}

func TestRBAC_AdminBypass(t *testing.T) {
	p := RBACPolicy{
		RequiredRolesByType: map[string][]string{
			"finance_question": {"advisor"},
		},
		AdminBypass: true,
	}
	msg := protocol.Message{Type: "finance_question", Metadata: map[string]any{
		protocol.MetaKeyUserRoles: "admin",
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow via admin bypass, got %s", res.Reason)
	}
}

func TestRBAC_UntypedAllowed(t *testing.T) {
	p := RBACPolicy{RequiredRolesByType: map[string][]string{}}
	res, _ := p.Evaluate(context.Background(), protocol.Message{Type: "anything"})
	if res.Decision != DecisionAllow {
		t.Fatalf("untyped should be allowed by default")
	}
}

func TestClassification_DeniesAbove(t *testing.T) {
	p := ClassificationPolicy{
		AllowedTo:      map[string]protocol.Classification{"public_summarizer": protocol.ClassPublic},
		DefaultCeiling: protocol.ClassInternal,
	}
	msg := protocol.Message{To: "public_summarizer", Metadata: map[string]any{
		protocol.MetaKeyClassification: string(protocol.ClassPII),
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionDeny {
		t.Fatalf("expected deny on PII -> public, got %s", res.Reason)
	}
}

func TestClassification_AllowsAtCeiling(t *testing.T) {
	p := ClassificationPolicy{
		AllowedTo: map[string]protocol.Classification{"vault_agent": protocol.ClassSecret},
	}
	msg := protocol.Message{To: "vault_agent", Metadata: map[string]any{
		protocol.MetaKeyClassification: string(protocol.ClassSecret),
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow at ceiling")
	}
}
