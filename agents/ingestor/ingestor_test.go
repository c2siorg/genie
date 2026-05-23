package ingestor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestHandleMessage_ParsesCSV(t *testing.T) {
	csv := strings.TrimSpace(`
date,description,category,amount,type
2026-01-01,Salary,Income,50000,credit
2026-01-03,Swiggy,Food,450,debit
`)
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeRawCSV, csv, map[string]any{"account_id": "acct-1"})

	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 output message, got %d", len(out))
	}
	if out[0].To != NextAgent {
		t.Fatalf("expected To=%q, got %q", NextAgent, out[0].To)
	}
	txns, err := finance.UnmarshalTransactions(out[0].Content)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("expected 2 txns, got %d", len(txns))
	}
	if txns[1].AmountCents != -45000 {
		t.Fatalf("expected debit -45000, got %d", txns[1].AmountCents)
	}
	if txns[0].AmountCents != 5000000 {
		t.Fatalf("expected credit 5000000, got %d", txns[0].AmountCents)
	}
}

func TestHandleMessage_IgnoresUnknownType(t *testing.T) {
	msg := agent.NewMessage("user", ID, agent.RoleUser, "other_type", "irrelevant", nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output for unknown type, got %v", out)
	}
}

func TestHandleMessage_RejectsEmpty(t *testing.T) {
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeRawCSV, "", nil)
	if _, err := New().HandleMessage(context.Background(), msg, testEnv{}); err == nil {
		t.Fatal("expected error for empty content")
	}
}
