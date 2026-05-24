package fraud

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestFraud_VelocityBurst(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-01", AmountCents: -100_00, Description: "TIME:10:00"},
		{TransactionID: "t2", Date: "2026-01-01", AmountCents: -100_00, Description: "TIME:10:01"},
		{TransactionID: "t3", Date: "2026-01-01", AmountCents: -100_00, Description: "TIME:10:02"},
		{TransactionID: "t4", Date: "2026-01-01", AmountCents: -100_00, Description: "TIME:10:03"},
		{TransactionID: "t5", Date: "2026-01-01", AmountCents: -100_00, Description: "TIME:10:04"},
	}
	signals := a.Detect(txns)
	if len(signals) < 5 {
		t.Fatalf("want at least 5 burst signals (one per txn), got %d", len(signals))
	}
	for _, s := range signals[:5] {
		if s.Pattern != "velocity_burst" {
			t.Errorf("expected velocity_burst, got %q", s.Pattern)
		}
		if s.Severity != "high" {
			t.Errorf("burst severity should be high, got %q", s.Severity)
		}
	}
}

func TestFraud_NoBurstBelowThreshold(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-01", AmountCents: -100_00, Description: "TIME:10:00"},
		{TransactionID: "t2", Date: "2026-01-01", AmountCents: -100_00, Description: "TIME:10:30"},
	}
	for _, s := range a.Detect(txns) {
		if s.Pattern == "velocity_burst" {
			t.Fatalf("did not expect burst with only 2 spaced txns")
		}
	}
}

func TestFraud_ImpossibleTravel(t *testing.T) {
	a := New()
	// Mumbai → Delhi in 15 minutes. ~1150km / 0.25h = 4600 km/h.
	txns := []finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-01", AmountCents: -500_00,
			Description: "COFFEE TIME:10:00 GEO:19.07,72.87"},
		{TransactionID: "t2", Date: "2026-01-01", AmountCents: -500_00,
			Description: "DINNER TIME:10:15 GEO:28.61,77.20"},
	}
	signals := a.Detect(txns)
	found := false
	for _, s := range signals {
		if s.Pattern == "impossible_travel" && s.TransactionID == "t2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected impossible_travel on t2; got %+v", signals)
	}
}

func TestFraud_AfterHoursLargeDebit(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-01", AmountCents: -75_000_00, Description: "TIME:02:30"},
	}
	signals := a.Detect(txns)
	if len(signals) == 0 || signals[0].Pattern != "after_hours_large_debit" {
		t.Fatalf("expected after_hours_large_debit, got %+v", signals)
	}
}

func TestFraud_HighRiskCategory(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-01", AmountCents: -10_000_00,
			Description: "TIME:14:00", Category: "crypto"},
	}
	signals := a.Detect(txns)
	if len(signals) == 0 || signals[0].Pattern != "high_risk_category" {
		t.Fatalf("expected high_risk_category, got %+v", signals)
	}
}

func TestFraud_NoFalsePositiveOnNormalDay(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{TransactionID: "t1", Date: "2026-01-01", AmountCents: -350_00,
			Description: "SWIGGY TIME:13:00", Category: "food"},
		{TransactionID: "t2", Date: "2026-01-02", AmountCents: -250_00,
			Description: "UBER TIME:18:30", Category: "transport"},
		{TransactionID: "t3", Date: "2026-01-03", AmountCents: -2_200_00,
			Description: "ELECTRICITY TIME:11:15", Category: "utilities"},
	}
	if got := a.Detect(txns); len(got) != 0 {
		t.Fatalf("normal txns should produce no signals; got %d: %+v", len(got), got)
	}
}
