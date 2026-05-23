package policy

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/sovereignty"
)

const samplePolicy = `
version: "1.0"
board_approved_on: "2025-08-13"
owner: "CRO"
governance:
  admin_bypass: true
  rbac:
    finance_question: ["user", "admin"]
sovereignty:
  home_region: "in"
data:
  block_pii: true
  block_prompt_injection: true
consent:
  type_to_category:
    portfolio_request: "portfolio"
explainability:
  applies_to: ["recommendations"]
limits:
  required_metadata:
    finance_question: ["user_id", "trace_id"]
`

func TestParse_AndComposite(t *testing.T) {
	p, err := Parse([]byte(samplePolicy))
	if err != nil {
		t.Fatal(err)
	}
	if p.Risk.MaxContentLengthBytes != 256*1024 {
		t.Fatalf("expected default content limit applied, got %d", p.Risk.MaxContentLengthBytes)
	}
	ledger := compliance.NewInMemoryLedger()
	composite := p.BuildComposite(ledger)

	// RBAC denies unauthenticated finance_question.
	res, _ := composite.Evaluate(context.Background(), protocol.Message{
		Type:     "finance_question",
		Metadata: map[string]any{"user_id": "u-1", "trace_id": "t-1"},
	})
	if res.Decision != governance.DecisionDeny {
		t.Fatalf("expected RBAC deny (no roles), got %s", res.Reason)
	}

	// PII pattern in content is denied.
	res, _ = composite.Evaluate(context.Background(), protocol.Message{
		Type:     "something_else",
		Content:  "leaked email a@b.com",
		Metadata: map[string]any{},
	})
	if res.Decision != governance.DecisionDeny {
		t.Fatalf("expected PII deny, got %s", res.Reason)
	}

	// Sovereignty defaults pulled in.
	if p.Sovereignty.HomeRegion != string(sovereignty.RegionIN) {
		t.Errorf("home_region: %q", p.Sovereignty.HomeRegion)
	}
}
