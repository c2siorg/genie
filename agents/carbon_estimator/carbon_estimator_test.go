package carbon_estimator

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestCompute_BasicFootprint(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Date: "2026-01-05", AmountCents: -5_000_00, Category: "fuel"},
		{Date: "2026-01-10", AmountCents: -2_000_00, Category: "groceries"},
	}
	res := a.Compute(txns)
	// fuel=5000×0.0024=12 + groceries=2000×0.0003=0.6 = 12.6
	if res.TotalKgCO2e < 12 || res.TotalKgCO2e > 13 {
		t.Errorf("total kgCO2e=%.2f want ~12.6", res.TotalKgCO2e)
	}
}

func TestCompute_TopEmittersRankedFirst(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Date: "2026-01-05", AmountCents: -10_000_00, Category: "fuel"},
		{Date: "2026-01-10", AmountCents: -500_00, Category: "food"},
	}
	res := a.Compute(txns)
	if res.ByCategory[0].Category != "fuel" {
		t.Errorf("fuel should top the list; got %s", res.ByCategory[0].Category)
	}
}

func TestCompute_MoMTrend(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Date: "2026-01-05", AmountCents: -1_000_00, Category: "fuel"},
		{Date: "2026-02-05", AmountCents: -3_000_00, Category: "fuel"},
	}
	res := a.Compute(txns)
	if res.MoMChangePct < 100 {
		t.Errorf("3x fuel spend MoM should be ≥100%% growth; got %.1f", res.MoMChangePct)
	}
}

func TestCompute_SuggestionsPresent(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Date: "2026-01-05", AmountCents: -5_000_00, Category: "fuel"},
	}
	res := a.Compute(txns)
	if len(res.Suggestions) == 0 {
		t.Errorf("expected at least one reduction suggestion")
	}
}

func TestCompute_OnlyDebitsCounted(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Date: "2026-01-05", AmountCents: 5_000_00, Category: "income"}, // credit, should be ignored
	}
	res := a.Compute(txns)
	if res.TotalKgCO2e != 0 {
		t.Errorf("credits should not contribute; got %.2f", res.TotalKgCO2e)
	}
}

func TestCompute_UnknownCategoryFallback(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Date: "2026-01-05", AmountCents: -1_000_00, Category: "unknown-cat"},
	}
	if a.Compute(txns).TotalKgCO2e <= 0 {
		t.Errorf("unknown category should use fallback factor")
	}
}
