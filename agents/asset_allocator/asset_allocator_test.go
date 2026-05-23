package asset_allocator

import "testing"

func TestYoungAggressiveHighEquity(t *testing.T) {
	plan := New().Compute(Request{
		Age: 25, HorizonYears: 30, RiskTolerance: RiskAggressive,
	})
	if plan.Target.Equity < 0.80 {
		t.Errorf("young+aggressive+long-horizon should be ≥80%% equity; got %.2f", plan.Target.Equity)
	}
}

func TestOldConservativeLowEquity(t *testing.T) {
	plan := New().Compute(Request{
		Age: 65, HorizonYears: 5, RiskTolerance: RiskConservative,
	})
	if plan.Target.Equity > 0.30 {
		t.Errorf("old+conservative should be ≤30%% equity; got %.2f", plan.Target.Equity)
	}
}

func TestRebalanceBuyAndSell(t *testing.T) {
	plan := New().Compute(Request{
		Age: 30, HorizonYears: 20, RiskTolerance: RiskModerate,
		Current: Current{
			EquityINR: 1_00_000, DebtINR: 9_00_000, GoldINR: 0, CashINR: 0, // way too debt-heavy
		},
	})
	hasBuyEquity := false
	hasSellDebt := false
	for _, r := range plan.Rebalance {
		if r.Asset == "equity" && r.Action == "buy" {
			hasBuyEquity = true
		}
		if r.Asset == "debt" && r.Action == "sell" {
			hasSellDebt = true
		}
	}
	if !hasBuyEquity || !hasSellDebt {
		t.Errorf("expected buy equity + sell debt; got %+v", plan.Rebalance)
	}
}

func TestAllocationSumsToOne(t *testing.T) {
	plan := New().Compute(Request{Age: 35, HorizonYears: 15, RiskTolerance: RiskModerate})
	sum := plan.Target.Equity + plan.Target.Debt + plan.Target.Gold + plan.Target.Cash
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("target allocation should sum to 1; got %.4f", sum)
	}
}

func TestEmptyCurrentNoZeroDiv(t *testing.T) {
	plan := New().Compute(Request{Age: 30, HorizonYears: 10, RiskTolerance: RiskModerate})
	if plan.CurrentAlloc.Equity != 0 {
		t.Errorf("empty current should have zero alloc; got %.4f", plan.CurrentAlloc.Equity)
	}
}
