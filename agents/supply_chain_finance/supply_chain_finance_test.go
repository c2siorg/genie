package supply_chain_finance

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

func TestAggregatesByBuyer(t *testing.T) {
	r := New().Recommend(Request{
		SupplierID: "s-1",
		Invoices: []Invoice{
			{InvoiceID: "i1", BuyerID: "B1", AmountRupees: 100_000, DaysOutstanding: 20},
			{InvoiceID: "i2", BuyerID: "B1", AmountRupees: 50_000, DaysOutstanding: 40},
			{InvoiceID: "i3", BuyerID: "B2", AmountRupees: 50_000, DaysOutstanding: 10},
		},
	})
	if len(r.BuyerSlices) != 2 {
		t.Fatalf("expected 2 buyers, got %d", len(r.BuyerSlices))
	}
	if r.BuyerSlices[0].BuyerID != "B1" {
		t.Errorf("expected B1 as top buyer; got %s", r.BuyerSlices[0].BuyerID)
	}
	if r.BuyerSlices[0].WorstDays != 40 {
		t.Errorf("expected worst-days = 40 for B1; got %d", r.BuyerSlices[0].WorstDays)
	}
}

func TestConcentrationWarn(t *testing.T) {
	r := New().Recommend(Request{
		SupplierID: "s-2",
		Invoices: []Invoice{
			{BuyerID: "B1", AmountRupees: 700_000, DaysOutstanding: 30},
			{BuyerID: "B2", AmountRupees: 300_000, DaysOutstanding: 30},
		},
	})
	if r.ConcentrationOK {
		t.Errorf("70%% top-buyer share should trip concentration warn")
	}
}

func TestTREDSCandidates(t *testing.T) {
	r := New().Recommend(Request{
		SupplierID: "s-3",
		Invoices: []Invoice{
			{InvoiceID: "old", BuyerID: "B1", AmountRupees: 50_000, DaysOutstanding: 60, GSTeInvoiced: true},
			{InvoiceID: "fresh", BuyerID: "B1", AmountRupees: 50_000, DaysOutstanding: 5, GSTeInvoiced: true},
			{InvoiceID: "noeinv", BuyerID: "B2", AmountRupees: 50_000, DaysOutstanding: 90, GSTeInvoiced: false},
		},
	})
	if len(r.TREDSCandidates) != 1 || r.TREDSCandidates[0].InvoiceID != "old" {
		t.Errorf("expected only 'old' as TReDS candidate; got %+v", r.TREDSCandidates)
	}
}

func TestHealthyChainNoActions(t *testing.T) {
	r := New().Recommend(Request{
		SupplierID: "s-4",
		Invoices: []Invoice{
			{BuyerID: "B1", AmountRupees: 200_000, DaysOutstanding: 10, GSTeInvoiced: true},
			{BuyerID: "B2", AmountRupees: 200_000, DaysOutstanding: 15, GSTeInvoiced: true},
			{BuyerID: "B3", AmountRupees: 200_000, DaysOutstanding: 20, GSTeInvoiced: true},
		},
	})
	if !r.ConcentrationOK {
		t.Errorf("balanced book should pass concentration check")
	}
	if len(r.TREDSCandidates) != 0 {
		t.Errorf("no aged invoices → no TReDS candidates; got %d", len(r.TREDSCandidates))
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(Request{SupplierID: "s", Invoices: []Invoice{{BuyerID: "B", AmountRupees: 1}}})
	msg := agent.NewMessage("u", ID, agent.RoleUser, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimer(t *testing.T) {
	r := New().Recommend(Request{})
	if r.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}
