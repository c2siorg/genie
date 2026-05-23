package aa_fetcher

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestAAFetcher_DeniesWithoutConsent(t *testing.T) {
	client := NewInMemoryFIClient()
	client.Seed("u-1", "acct-1", Statement{AccountID: "acct-1", Currency: "INR"})
	ledger := compliance.NewInMemoryLedger()
	a := New(client, ledger)
	_, err := a.HandleMessage(context.Background(), agent.NewMessage("supervisor", ID, agent.RoleAgent, TypeIn, "", map[string]any{
		protocol.MetaKeyUserID: "u-1",
		"account_id":           "acct-1",
	}), testEnv{})
	if err == nil || !strings.Contains(err.Error(), "consent") {
		t.Fatalf("want consent error, got %v", err)
	}
}

func TestAAFetcher_HappyPath(t *testing.T) {
	client := NewInMemoryFIClient()
	client.Seed("u-1", "acct-1", Statement{
		AccountID:    "acct-1",
		Currency:     "INR",
		Transactions: []map[string]any{{"amount_cents": -500, "description": "swiggy"}},
	})
	ledger := compliance.NewInMemoryLedger()
	_, _ = ledger.Grant(context.Background(), "u-1", compliance.CategoryTransactions, "monthly summary")

	a := New(client, ledger)
	out, err := a.HandleMessage(context.Background(), agent.NewMessage("supervisor", ID, agent.RoleAgent, TypeIn, "", map[string]any{
		protocol.MetaKeyUserID: "u-1",
		"account_id":           "acct-1",
	}), testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("unexpected output: %+v", out)
	}
	var stmt Statement
	_ = json.Unmarshal([]byte(out[0].Content), &stmt)
	if stmt.Classification != protocol.ClassPII {
		t.Errorf("expected PII classification, got %s", stmt.Classification)
	}
}
