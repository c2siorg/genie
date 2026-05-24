package working_capital

import "testing"

func TestCCCComputation(t *testing.T) {
	res := New().Forecast(Request{
		MonthlyRevenue: 10_00_000, GrossMarginPct: 40,
		OperatingCostsMonthly: 2_00_000, OpeningCashINR: 10_00_000,
		DSO: 60, DIO: 30, DPO: 45, HorizonMonths: 6,
	})
	if res.CCC != 60+30-45 {
		t.Errorf("CCC=%d want 45", res.CCC)
	}
}

func TestNegativeCashFlagsRunway(t *testing.T) {
	res := New().Forecast(Request{
		MonthlyRevenue: 5_00_000, GrossMarginPct: 20,
		OperatingCostsMonthly: 6_00_000, // burning cash
		OpeningCashINR: 3_00_000,
		DSO: 30, DIO: 0, DPO: 0, HorizonMonths: 12,
	})
	if res.RunwayMonths >= 12 {
		t.Errorf("burn-rate scenario should have <12 months runway; got %d", res.RunwayMonths)
	}
}

func TestHealthyCycleStableRunway(t *testing.T) {
	res := New().Forecast(Request{
		MonthlyRevenue: 50_00_000, GrossMarginPct: 60,
		OperatingCostsMonthly: 10_00_000, OpeningCashINR: 1_00_00_000,
		DSO: 30, DIO: 15, DPO: 30, HorizonMonths: 12,
	})
	if res.RunwayMonths != 12 {
		t.Errorf("healthy cycle should preserve runway over horizon; got %d", res.RunwayMonths)
	}
}

func TestLongCCCRecommendsTReDS(t *testing.T) {
	res := New().Forecast(Request{
		MonthlyRevenue: 10_00_000, GrossMarginPct: 40,
		DSO: 100, DIO: 60, DPO: 30, HorizonMonths: 6,
	})
	if res.Recommendation == "" || !contains(res.Recommendation, "TReDS") {
		t.Errorf("long CCC should mention TReDS; got %q", res.Recommendation)
	}
}

func TestForecastLengthMatchesHorizon(t *testing.T) {
	res := New().Forecast(Request{
		MonthlyRevenue: 5_00_000, GrossMarginPct: 30,
		OperatingCostsMonthly: 1_00_000, HorizonMonths: 9, DSO: 30, DPO: 30,
	})
	if len(res.Forecast) != 9 {
		t.Errorf("want 9 rows; got %d", len(res.Forecast))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
