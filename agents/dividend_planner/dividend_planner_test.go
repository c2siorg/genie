package dividend_planner

import "testing"

func TestProjectsTenYears(t *testing.T) {
	res := New().Project(Request{
		Shares: 100, CurrentPrice: 500, DividendPerShare: 10,
		DividendGrowthAnnual: 0.10, PriceAppreciationAnn: 0.10,
		HorizonYears: 10, TDSAndSlabPct: 30, Reinvest: true,
	})
	if len(res.Schedule) != 10 {
		t.Errorf("expected 10 yearly rows; got %d", len(res.Schedule))
	}
	if res.TerminalValue <= 100*500 {
		t.Errorf("terminal should exceed cost basis after 10y of growth; got %.0f", res.TerminalValue)
	}
}

func TestReinvestIncreasesShares(t *testing.T) {
	withReinvest := New().Project(Request{
		Shares: 100, CurrentPrice: 500, DividendPerShare: 10,
		DividendGrowthAnnual: 0.10, PriceAppreciationAnn: 0.05,
		HorizonYears: 5, TDSAndSlabPct: 0, Reinvest: true,
	})
	noReinvest := New().Project(Request{
		Shares: 100, CurrentPrice: 500, DividendPerShare: 10,
		DividendGrowthAnnual: 0.10, PriceAppreciationAnn: 0.05,
		HorizonYears: 5, TDSAndSlabPct: 0, Reinvest: false,
	})
	if withReinvest.Schedule[4].Shares <= 100 {
		t.Errorf("reinvest should grow share count; got %.4f", withReinvest.Schedule[4].Shares)
	}
	if noReinvest.Schedule[4].Shares != 100 {
		t.Errorf("no-reinvest should keep share count constant; got %.4f", noReinvest.Schedule[4].Shares)
	}
}

func TestTaxReducesNetDividend(t *testing.T) {
	highTax := New().Project(Request{
		Shares: 100, CurrentPrice: 100, DividendPerShare: 10,
		HorizonYears: 1, TDSAndSlabPct: 30,
	}).TotalDividend
	noTax := New().Project(Request{
		Shares: 100, CurrentPrice: 100, DividendPerShare: 10,
		HorizonYears: 1, TDSAndSlabPct: 0,
	}).TotalDividend
	if highTax >= noTax {
		t.Errorf("30%% tax should reduce net; got %.0f vs %.0f", highTax, noTax)
	}
}

func TestYieldOnCostGrowsWithDividendGrowth(t *testing.T) {
	res := New().Project(Request{
		Shares: 100, CurrentPrice: 100, DividendPerShare: 5,
		DividendGrowthAnnual: 0.15, PriceAppreciationAnn: 0.10,
		HorizonYears: 15, TDSAndSlabPct: 0,
	})
	// 5%×1.15^15 ≈ ~40% YOC. Sanity check ≥10%.
	if res.YieldOnCostPct < 10 {
		t.Errorf("yield-on-cost should grow significantly; got %.2f%%", res.YieldOnCostPct)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	if New().Project(Request{Shares: 1, CurrentPrice: 100, DividendPerShare: 1, HorizonYears: 1}).Disclaimer == "" {
		t.Errorf("disclaimer missing")
	}
}
