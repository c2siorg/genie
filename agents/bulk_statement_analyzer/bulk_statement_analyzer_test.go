package bulk_statement_analyzer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestDedupsInterAccountTransfer(t *testing.T) {
	req := Request{
		ApplicantID: "a-1",
		Transactions: []Txn{
			{AccountID: "A", Date: "2026-01-01", Description: "Transfer to B", Amount: 5000, Type: "debit"},
			{AccountID: "B", Date: "2026-01-01", Description: "Transfer to B", Amount: 5000, Type: "credit"},
			{AccountID: "A", Date: "2026-01-03", Description: "Salary", Amount: 50000, Type: "credit"},
		},
	}
	s := New().Consolidate(req)
	if s.TxnCountDedup != 1 {
		t.Errorf("expected 1 txn after dedup; got %d", s.TxnCountDedup)
	}
	if s.TotalCredit != 50000 {
		t.Errorf("expected credit=50000 after dedup; got %.2f", s.TotalCredit)
	}
	if s.TotalDebit != 0 {
		t.Errorf("expected debit=0 after transfer dedup; got %.2f", s.TotalDebit)
	}
}

func TestExactDuplicateRemoved(t *testing.T) {
	req := Request{
		Transactions: []Txn{
			{AccountID: "A", Date: "2026-01-01", Description: "Swiggy", Amount: 450, Type: "debit"},
			{AccountID: "A", Date: "2026-01-01", Description: "Swiggy", Amount: 450, Type: "debit"},
		},
	}
	s := New().Consolidate(req)
	if s.TxnCountDedup != 1 {
		t.Errorf("expected duplicate to be folded; got %d txns", s.TxnCountDedup)
	}
}

func TestAccountCount(t *testing.T) {
	req := Request{
		Transactions: []Txn{
			{AccountID: "A", Date: "2026-01-01", Description: "x", Amount: 1, Type: "debit"},
			{AccountID: "B", Date: "2026-01-01", Description: "y", Amount: 1, Type: "debit"},
			{AccountID: "C", Date: "2026-01-01", Description: "z", Amount: 1, Type: "debit"},
		},
	}
	s := New().Consolidate(req)
	if s.AccountCount != 3 {
		t.Errorf("expected 3 accounts, got %d", s.AccountCount)
	}
}

func TestCategorisation(t *testing.T) {
	req := Request{
		Transactions: []Txn{
			{AccountID: "A", Date: "2026-01-01", Description: "Rent payment Jan", Amount: 25000, Type: "debit"},
			{AccountID: "A", Date: "2026-01-02", Description: "Swiggy order", Amount: 500, Type: "debit"},
			{AccountID: "A", Date: "2026-01-03", Description: "Uber ride", Amount: 200, Type: "debit"},
		},
	}
	s := New().Consolidate(req)
	if s.TopCategories["housing:rent"] != 25000 {
		t.Errorf("expected rent category at 25000; got %+v", s.TopCategories)
	}
}

func TestDurationMonths(t *testing.T) {
	req := Request{
		Transactions: []Txn{
			{AccountID: "A", Date: "2026-01-01", Description: "x", Amount: 100, Type: "credit"},
			{AccountID: "A", Date: "2026-04-30", Description: "y", Amount: 100, Type: "credit"},
		},
	}
	s := New().Consolidate(req)
	if s.DurationMonths != 4 {
		t.Errorf("expected 4 months; got %d", s.DurationMonths)
	}
}

func TestMonthlyAverage(t *testing.T) {
	req := Request{
		Transactions: []Txn{
			{AccountID: "A", Date: "2026-01-01", Description: "x", Amount: 60000, Type: "credit"},
			{AccountID: "A", Date: "2026-02-01", Description: "x", Amount: 60000, Type: "credit"},
			{AccountID: "A", Date: "2026-03-01", Description: "x", Amount: 60000, Type: "credit"},
		},
	}
	s := New().Consolidate(req)
	if s.MonthlyAverage != 60000 {
		t.Errorf("expected monthly avg 60000 over 3 months, got %.2f", s.MonthlyAverage)
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(Request{Transactions: []Txn{{AccountID: "A", Date: "2026-01-01", Description: "x", Amount: 1, Type: "credit"}}})
	msg := agent.NewMessage("up", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected single dispatch of %s; got %+v", TypeOut, out)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	if New().Consolidate(Request{}).Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}
