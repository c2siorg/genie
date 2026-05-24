package aml_monitor

import (
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func fixedNow() time.Time { return time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC) }

func TestCTRThresholdFlagged(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Detect([]finance.Transaction{
		{TransactionID: "t1", AccountID: "A1", Date: "2026-01-15",
			AmountCents: -15_00_000_00, Description: "cash withdrawal"},
	})
	if len(res.STRs) != 1 || res.STRs[0].RuleHit != "ctr_threshold" {
		t.Fatalf("expected CTR draft; got %+v", res.STRs)
	}
}

func TestStructuringFlagged(t *testing.T) {
	a := &Agent{Now: fixedNow}
	// 3 deposits inside a week, each at 85% of CTR.
	res := a.Detect([]finance.Transaction{
		{TransactionID: "t1", AccountID: "A1", Date: "2026-01-01", AmountCents: -8_50_000_00, Description: "cash deposit"},
		{TransactionID: "t2", AccountID: "A1", Date: "2026-01-03", AmountCents: -8_50_000_00, Description: "cash deposit"},
		{TransactionID: "t3", AccountID: "A1", Date: "2026-01-05", AmountCents: -8_50_000_00, Description: "cash deposit"},
	})
	found := false
	for _, s := range res.STRs {
		if s.RuleHit == "structuring" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected structuring STR; got %+v", res.STRs)
	}
}

func TestNoStructuringWhenSpread(t *testing.T) {
	a := &Agent{Now: fixedNow}
	// Same amounts but spread 1 month — outside 7d window.
	res := a.Detect([]finance.Transaction{
		{TransactionID: "t1", AccountID: "A1", Date: "2026-01-01", AmountCents: -8_50_000_00, Description: "cash deposit"},
		{TransactionID: "t2", AccountID: "A1", Date: "2026-02-01", AmountCents: -8_50_000_00, Description: "cash deposit"},
		{TransactionID: "t3", AccountID: "A1", Date: "2026-03-01", AmountCents: -8_50_000_00, Description: "cash deposit"},
	})
	for _, s := range res.STRs {
		if s.RuleHit == "structuring" {
			t.Errorf("should not flag structuring outside 7d window")
		}
	}
}

func TestAdverseMediaMatch(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Detect([]finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-15", AmountCents: -1_00_000_00,
			Description: "payment to ofac-listed entity"},
	})
	if len(res.STRs) == 0 || res.STRs[0].RuleHit != "adverse_media" {
		t.Errorf("expected adverse_media STR; got %+v", res.STRs)
	}
}

func TestWireLTR(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Detect([]finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-15", AmountCents: -60_00_000_00, Description: "wire transfer"},
	})
	if len(res.STRs) == 0 || res.STRs[0].RuleHit != "wire_threshold" {
		t.Errorf("expected wire_threshold STR; got %+v", res.STRs)
	}
}

func TestNoFalsePositiveOnNormalTxns(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Detect([]finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-05", AmountCents: -350_00, Description: "swiggy"},
		{TransactionID: "t2", Date: "2026-01-10", AmountCents: 50_000_00, Description: "salary"},
	})
	if len(res.STRs) != 0 {
		t.Errorf("normal txns should not produce STRs; got %+v", res.STRs)
	}
}
