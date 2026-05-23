package governance

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

func TestConsentPolicy_DenyMissing(t *testing.T) {
	ledger := compliance.NewInMemoryLedger()
	p := ConsentPolicy{
		Ledger:    ledger,
		TypeToCat: map[string]compliance.ConsentCategory{"portfolio_request": compliance.CategoryPortfolio},
	}
	msg := protocol.Message{Type: "portfolio_request", Metadata: map[string]any{
		protocol.MetaKeyUserID: "u-1",
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionDeny {
		t.Fatalf("expected deny without consent, got %s", res.Reason)
	}
	_, _ = ledger.Grant(context.Background(), "u-1", compliance.CategoryPortfolio, "show holdings")
	res, _ = p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow after grant, got %s", res.Reason)
	}
}

func TestExplainability_RequiresRationale(t *testing.T) {
	p := ExplainabilityPolicy{AppliesTo: []string{"recommendations"}}

	// Missing rationale -> deny
	res, _ := p.Evaluate(context.Background(), protocol.Message{
		Type:    "recommendations",
		Content: `{"recommendations":[{"title":"X"}]}`,
	})
	if res.Decision != DecisionDeny {
		t.Fatalf("expected deny missing rationale, got %s", res.Reason)
	}

	// Rationale at root
	res, _ = p.Evaluate(context.Background(), protocol.Message{
		Type:    "recommendations",
		Content: `{"rationale":"net cashflow negative"}`,
	})
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow with rationale, got %s", res.Reason)
	}

	// Rationale nested in items
	res, _ = p.Evaluate(context.Background(), protocol.Message{
		Type:    "recommendations",
		Content: `{"recommendations":[{"title":"X","rationale":"because"}]}`,
	})
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow with nested rationale, got %s", res.Reason)
	}
}
