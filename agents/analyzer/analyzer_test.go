package analyzer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestAnalyzer_FansOutAndTotals(t *testing.T) {
	in := []finance.Transaction{
		{Category: "income:salary", AmountCents: 5000000, Currency: "INR"},
		{Category: "food:delivery", AmountCents: -45000},
		{Category: "food:delivery", AmountCents: -30000},
		{Category: "housing:rent", AmountCents: -2500000},
	}
	payload, _ := finance.MarshalTransactions(in)
	msg := agent.NewMessage("enricher", ID, agent.RoleAgent, TypeIn, payload, nil)

	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("want 3 fan-out messages, got %d", len(out))
	}
	var res Result
	if err := json.Unmarshal([]byte(out[0].Content), &res); err != nil {
		t.Fatal(err)
	}
	if res.TotalIncomeCents != 5000000 {
		t.Errorf("income: %d", res.TotalIncomeCents)
	}
	if res.TotalExpenseCents != 2575000 {
		t.Errorf("expense: %d", res.TotalExpenseCents)
	}
	if res.NetCents != 5000000-2575000 {
		t.Errorf("net: %d", res.NetCents)
	}
	if res.ByCategory[0].Category != "housing:rent" {
		t.Errorf("top category should be housing:rent, got %q", res.ByCategory[0].Category)
	}
}
