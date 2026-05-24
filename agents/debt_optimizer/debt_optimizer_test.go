package debt_optimizer

import "testing"

func TestAvalanche_PrioritisesHighestAPR(t *testing.T) {
	a := New()
	plan := a.Compute(Request{
		Debts: []Debt{
			{Name: "card", BalanceRupees: 10_000, APR: 0.36, MinPaymentRupees: 500},
			{Name: "personal", BalanceRupees: 100_000, APR: 0.14, MinPaymentRupees: 3_000},
		},
		ExtraPerMonth: 5_000,
		Strategy:      StrategyAvalanche,
	})
	if plan.Order[0] != "card" {
		t.Errorf("avalanche should pay card (36%%) first; got %v", plan.Order)
	}
}

func TestSnowball_PrioritisesSmallestBalance(t *testing.T) {
	a := New()
	plan := a.Compute(Request{
		Debts: []Debt{
			{Name: "big", BalanceRupees: 100_000, APR: 0.36, MinPaymentRupees: 3_000},
			{Name: "tiny", BalanceRupees: 5_000, APR: 0.14, MinPaymentRupees: 500},
		},
		ExtraPerMonth: 5_000,
		Strategy:      StrategySnowball,
	})
	if plan.Order[0] != "tiny" {
		t.Errorf("snowball should pay tiny (smallest balance) first; got %v", plan.Order)
	}
}

func TestAvalanche_LowerTotalInterestThanSnowballForSameMix(t *testing.T) {
	mk := func() []Debt {
		return []Debt{
			{Name: "card", BalanceRupees: 80_000, APR: 0.36, MinPaymentRupees: 2_000},
			{Name: "personal", BalanceRupees: 20_000, APR: 0.14, MinPaymentRupees: 1_000},
		}
	}
	av := New().Compute(Request{Debts: mk(), Strategy: StrategyAvalanche, ExtraPerMonth: 5_000})
	sn := New().Compute(Request{Debts: mk(), Strategy: StrategySnowball, ExtraPerMonth: 5_000})
	if av.TotalInterest >= sn.TotalInterest {
		t.Errorf("avalanche interest=%.0f should be < snowball=%.0f", av.TotalInterest, sn.TotalInterest)
	}
}

func TestEmptyDebts(t *testing.T) {
	plan := New().Compute(Request{})
	if plan.MonthsToFree != 0 || len(plan.Order) != 0 {
		t.Errorf("empty debts should yield empty plan; got %+v", plan)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	plan := New().Compute(Request{Debts: []Debt{{Name: "x", BalanceRupees: 1000, APR: 0.1, MinPaymentRupees: 100}}, ExtraPerMonth: 100})
	if plan.Disclaimer == "" {
		t.Errorf("disclaimer missing")
	}
}
