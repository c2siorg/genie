package normalizer

import (
	"context"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestNormalize_SetsCurrencyAndMerchant(t *testing.T) {
	in := []finance.Transaction{
		{Date: "2026-01-03", Description: "Swiggy Order #4823", AmountCents: -45000},
	}
	payload, _ := finance.MarshalTransactions(in)
	msg := agent.NewMessage("ingestor", ID, agent.RoleAgent, TypeIn, payload, map[string]any{
		"account_id": "acct-1",
		"currency":   "INR",
	})

	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	txns, _ := finance.UnmarshalTransactions(out[0].Content)
	if got := txns[0].Merchant; got != "swiggy" {
		t.Fatalf("merchant: want swiggy, got %q", got)
	}
	if got := txns[0].Currency; got != "INR" {
		t.Fatalf("currency: want INR, got %q", got)
	}
	if got := txns[0].AccountID; got != "acct-1" {
		t.Fatalf("account_id: want acct-1, got %q", got)
	}
}
