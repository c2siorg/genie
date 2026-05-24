package invoice_discounter

import (
	"testing"
	"time"
)

func asOf(s string) func() time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return func() time.Time { return t }
}

func TestCheapestSelectedFirst(t *testing.T) {
	a := &Agent{Now: asOf("2026-05-01")}
	plan := a.Plan(Request{
		AnnualisedRateBp: 950,
		TargetCashINR:    5_00_000,
		Invoices: []Invoice{
			{ID: "i1", FaceValueRupees: 5_00_000, DueOn: "2026-06-30", CounterpartyRating: "BBB"},
			{ID: "i2", FaceValueRupees: 5_00_000, DueOn: "2026-06-30", CounterpartyRating: "AAA"},
		},
	})
	if len(plan.Selected) == 0 || plan.Selected[0].InvoiceID != "i2" {
		t.Errorf("AAA should be selected before BBB; got %+v", plan.Selected)
	}
}

func TestPastDueExcluded(t *testing.T) {
	a := &Agent{Now: asOf("2026-05-01")}
	plan := a.Plan(Request{
		AnnualisedRateBp: 950,
		TargetCashINR:    10_00_000,
		Invoices: []Invoice{
			{ID: "i1", FaceValueRupees: 5_00_000, DueOn: "2026-04-01", CounterpartyRating: "AAA"}, // past
		},
	})
	if len(plan.Selected) != 0 {
		t.Errorf("past-due invoice should be excluded; got %+v", plan.Selected)
	}
}

func TestUnfundedGapReported(t *testing.T) {
	a := &Agent{Now: asOf("2026-05-01")}
	plan := a.Plan(Request{
		AnnualisedRateBp: 950, TargetCashINR: 50_00_000,
		Invoices: []Invoice{
			{ID: "i1", FaceValueRupees: 2_00_000, DueOn: "2026-06-30", CounterpartyRating: "AAA"},
		},
	})
	if plan.UnfundedGapINR <= 0 {
		t.Errorf("expected unfunded gap; got %.0f", plan.UnfundedGapINR)
	}
}

func TestRatingPremiumIncreasesDiscount(t *testing.T) {
	a := &Agent{Now: asOf("2026-05-01")}
	aaa := a.Plan(Request{AnnualisedRateBp: 950, TargetCashINR: 1_00_00_000,
		Invoices: []Invoice{{ID: "x", FaceValueRupees: 10_00_000, DueOn: "2026-08-01", CounterpartyRating: "AAA"}}})
	bbb := a.Plan(Request{AnnualisedRateBp: 950, TargetCashINR: 1_00_00_000,
		Invoices: []Invoice{{ID: "x", FaceValueRupees: 10_00_000, DueOn: "2026-08-01", CounterpartyRating: "BBB"}}})
	if aaa.TotalDiscountINR >= bbb.TotalDiscountINR {
		t.Errorf("BBB should cost more than AAA; got %.0f vs %.0f", bbb.TotalDiscountINR, aaa.TotalDiscountINR)
	}
}

func TestEmptyInvoicesEmptyPlan(t *testing.T) {
	if plan := New().Plan(Request{}); len(plan.Selected) != 0 {
		t.Errorf("empty invoices should yield empty plan; got %+v", plan.Selected)
	}
}
