package prepayment_advisor

import "testing"

func TestPrepay_HighestRatePicked(t *testing.T) {
	a := New()
	plan := a.Compute(Request{
		Loans: []Loan{
			{Name: "home", OutstandingRupees: 50_00_000, APR: 0.085, MonthsRemaining: 240, TaxDeductible: true},
			{Name: "personal", OutstandingRupees: 2_00_000, APR: 0.16, MonthsRemaining: 36},
		},
		PrepaymentAmount: 1_00_000,
		BorrowerSlabPct:  30,
	})
	if len(plan.Suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}
	if plan.Suggestions[0].LoanName != "personal" {
		t.Errorf("personal loan should rank above tax-deductible home loan; got order %+v", plan.Suggestions)
	}
}

func TestPrepay_FixedRateFlag(t *testing.T) {
	a := New()
	plan := a.Compute(Request{
		Loans: []Loan{
			{Name: "fixed", OutstandingRupees: 5_00_000, APR: 0.11, MonthsRemaining: 60, IsFixedRate: true},
		},
		PrepaymentAmount: 1_00_000,
	})
	if len(plan.Suggestions) == 0 || len(plan.Suggestions[0].Flags) == 0 {
		t.Fatal("expected fixed-rate flag")
	}
}

func TestPrepay_TaxAdjustmentLowersEffectiveRate(t *testing.T) {
	withSlab := effectiveRate(Loan{APR: 0.09, TaxDeductible: true}, 30)
	withoutSlab := effectiveRate(Loan{APR: 0.09, TaxDeductible: true}, 0)
	if withSlab >= withoutSlab {
		t.Errorf("30%% slab should lower effective rate; %.4f vs %.4f", withSlab, withoutSlab)
	}
}

func TestPrepay_NoLoansEmptyPlan(t *testing.T) {
	plan := New().Compute(Request{PrepaymentAmount: 10000})
	if len(plan.Suggestions) != 0 {
		t.Errorf("expected empty plan; got %+v", plan)
	}
}

func TestPrepay_InterestSavingsPositive(t *testing.T) {
	plan := New().Compute(Request{
		Loans:            []Loan{{Name: "L", OutstandingRupees: 5_00_000, APR: 0.12, MonthsRemaining: 60}},
		PrepaymentAmount: 50_000,
	})
	if plan.TotalSavingINR <= 0 {
		t.Errorf("expected positive savings; got %.2f", plan.TotalSavingINR)
	}
}
