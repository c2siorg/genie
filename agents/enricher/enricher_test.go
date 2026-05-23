package enricher

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

func TestEnrich_AssignsCategories(t *testing.T) {
	in := []finance.Transaction{
		{Merchant: "swiggy", Description: "swiggy"},
		{Description: "Uber ride"},
		{Description: "Unknown vendor"},
	}
	payload, _ := finance.MarshalTransactions(in)
	msg := agent.NewMessage("normalizer", ID, agent.RoleAgent, TypeIn, payload, nil)

	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	txns, _ := finance.UnmarshalTransactions(out[0].Content)
	if txns[0].Category != "food:delivery" {
		t.Errorf("swiggy: want food:delivery, got %q", txns[0].Category)
	}
	if txns[1].Category != "transport:ride" {
		t.Errorf("uber: want transport:ride, got %q", txns[1].Category)
	}
	if txns[2].Category != "uncategorized" {
		t.Errorf("unknown: want uncategorized, got %q", txns[2].Category)
	}
}
