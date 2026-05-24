package var_calculator

import (
	"math"
	"testing"
)

func TestParametricVaR95(t *testing.T) {
	// Returns with mean=0, sd=0.01. VaR95 ≈ 1.645σ = 0.01645 (1.645%).
	returns := []float64{}
	for i := 0; i < 100; i++ {
		// Alternating +/− to keep mean ≈ 0 but sd > 0.
		if i%2 == 0 {
			returns = append(returns, 0.01)
		} else {
			returns = append(returns, -0.01)
		}
	}
	res := New().Compute(Request{Returns: returns, PortfolioValue: 1_00_000, ConfidencePct: 95})
	if math.Abs(res.Parametric.VaRPct-1.645) > 0.2 {
		t.Errorf("parametric VaR95 should be ~1.645%%; got %.3f%%", res.Parametric.VaRPct)
	}
}

func TestHistoricalVaRRupees(t *testing.T) {
	// 100 returns, worst is -5%.
	returns := []float64{}
	for i := 0; i < 99; i++ {
		returns = append(returns, 0.0)
	}
	returns = append(returns, -0.05)
	res := New().Compute(Request{Returns: returns, PortfolioValue: 1_00_000, ConfidencePct: 99})
	if math.Abs(res.Historical.VaRINR-5000) > 100 {
		t.Errorf("hist VaR @99%% should be ~₹5000; got %.0f", res.Historical.VaRINR)
	}
}

func TestEmptyReturnsSafe(t *testing.T) {
	res := New().Compute(Request{PortfolioValue: 1000})
	if res.Note == "" {
		t.Errorf("note should mention missing returns; got %+v", res)
	}
}

func TestHorizonSqrtScaling(t *testing.T) {
	returns := []float64{}
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			returns = append(returns, 0.01)
		} else {
			returns = append(returns, -0.01)
		}
	}
	a := New().Compute(Request{Returns: returns, PortfolioValue: 1, ConfidencePct: 95, HorizonDays: 1}).Parametric.VaRPct
	b := New().Compute(Request{Returns: returns, PortfolioValue: 1, ConfidencePct: 95, HorizonDays: 16}).Parametric.VaRPct
	// √16 = 4. Should scale by ~4.
	if math.Abs(b/a-4) > 0.5 {
		t.Errorf("VaR should scale by √horizon; got %.2f", b/a)
	}
}

func TestExpectedShortfallAtLeastVaR(t *testing.T) {
	returns := []float64{-0.05, -0.03, -0.02, -0.01, 0.0, 0.01, 0.02, 0.03}
	res := New().Compute(Request{Returns: returns, PortfolioValue: 1_00_000, ConfidencePct: 95})
	if res.Historical.ESINR < res.Historical.VaRINR {
		t.Errorf("ES should be ≥VaR; got ES=%.0f VaR=%.0f", res.Historical.ESINR, res.Historical.VaRINR)
	}
}
