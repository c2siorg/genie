package fx

import (
	"context"
	"testing"
)

func TestMockPathOptimizerFindBestPath(t *testing.T) {
	quoter := NewMockQuoter()
	poolManager := NewMockLiquidityPoolManager()
	optimizer := NewMockPathOptimizer(quoter, poolManager)
	ctx := context.Background()

	tests := []struct {
		name      string
		amounts   []NetAmount
		shouldErr bool
		minPaths  int
	}{
		{
			name: "Two matching inflow/outflow same currency",
			amounts: []NetAmount{
				{Currency: "USD", Amount: 100_000.0},
				{Currency: "USD", Amount: -100_000.0},
			},
			shouldErr: false,
			minPaths:  1, // Direct path for same currency
		},
		{
			name: "Two currencies simple",
			amounts: []NetAmount{
				{Currency: "USD", Amount: 100_000.0},
				{Currency: "INR", Amount: -5_000_000.0},
			},
			shouldErr: false,
			minPaths:  1, // At least direct path
		},
		{
			name: "Multi-currency netting",
			amounts: []NetAmount{
				{Currency: "USD", Amount: -100_000.0},
				{Currency: "EUR", Amount: 50_000.0},
				{Currency: "GBP", Amount: 30_000.0},
				{Currency: "INR", Amount: -5_000_000.0},
			},
			shouldErr: false,
			minPaths:  2, // At least direct + correspondent paths
		},
		{
			name:      "No amounts",
			amounts:   []NetAmount{},
			shouldErr: true,
			minPaths:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths, err := optimizer.FindBestPath(ctx, tt.amounts)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(paths) < tt.minPaths {
					t.Errorf("expected at least %d paths, got %d", tt.minPaths, len(paths))
				}

				// Verify paths are ranked by cost
				for i := 0; i < len(paths)-1; i++ {
					if paths[i].EstimatedCostBps > paths[i+1].EstimatedCostBps {
						t.Errorf("paths not sorted by cost: path[%d]=%d > path[%d]=%d",
							i, paths[i].EstimatedCostBps, i+1, paths[i+1].EstimatedCostBps)
					}
				}
			}
		})
	}
}

func TestDirectPath(t *testing.T) {
	quoter := NewMockQuoter()
	poolManager := NewMockLiquidityPoolManager()
	optimizer := NewMockPathOptimizer(quoter, poolManager)

	// Same currency: should produce a direct path with minimal cost
	path := optimizer.buildDirectPath("USD", "USD", 100_000.0, 100_000.0)

	if path.Route != RouteDirect {
		t.Errorf("expected direct route, got %v", path.Route)
	}
	if path.EstimatedCostBps > 50 {
		t.Errorf("same-currency cost should be minimal, got %d bps", path.EstimatedCostBps)
	}
	if path.ExecutionTimeMs > 5000 {
		t.Errorf("same-currency should settle quickly, got %d ms", path.ExecutionTimeMs)
	}
}

func TestDirectFXPath(t *testing.T) {
	quoter := NewMockQuoter()
	poolManager := NewMockLiquidityPoolManager()
	optimizer := NewMockPathOptimizer(quoter, poolManager)
	ctx := context.Background()

	// USD to INR direct
	path, err := optimizer.buildDirectFXPath(ctx, "USD", 100_000.0, "INR", 5_000_000.0)
	if err != nil {
		t.Fatalf("buildDirectFXPath failed: %v", err)
	}

	if path.Route != RouteDirect {
		t.Errorf("expected direct route, got %v", path.Route)
	}
	if len(path.FXRates) != 1 {
		t.Errorf("expected 1 FX rate, got %d", len(path.FXRates))
	}
	if path.FXRates[0].FromCurrency != "USD" || path.FXRates[0].ToCurrency != "INR" {
		t.Errorf("incorrect rate currencies")
	}
	if path.ExecutionTimeMs < 3600000 { // Should be several hours for cross-border
		t.Errorf("cross-border transfer should take hours, got %d ms", path.ExecutionTimeMs)
	}
}

func TestCorrespondentPath(t *testing.T) {
	quoter := NewMockQuoter()
	poolManager := NewMockLiquidityPoolManager()
	optimizer := NewMockPathOptimizer(quoter, poolManager)
	ctx := context.Background()

	// EUR to GBP via USD correspondent
	path, err := optimizer.buildCorrespondentPath(ctx, "EUR", 100_000.0, "GBP", 86_400.0)
	if err != nil {
		t.Fatalf("buildCorrespondentPath failed: %v", err)
	}

	if path.Route != RouteCorrespondent {
		t.Errorf("expected correspondent route, got %v", path.Route)
	}
	if len(path.FXRates) != 2 {
		t.Errorf("expected 2 FX rates (2 conversions), got %d", len(path.FXRates))
	}
	if path.FXRates[0].ToCurrency != "USD" {
		t.Errorf("first leg should convert to USD")
	}
	if path.FXRates[1].FromCurrency != "USD" {
		t.Errorf("second leg should convert from USD")
	}

	// Cost should include 2 conversion fees + correspondent banking fees
	if path.EstimatedCostBps < 100 {
		t.Errorf("correspondent path cost too low: %d bps", path.EstimatedCostBps)
	}

	// Should take longer than direct (two hops)
	if path.ExecutionTimeMs < 28800000 { // 8 hours minimum
		t.Errorf("correspondent path should be slower, got %d ms", path.ExecutionTimeMs)
	}
}

func TestNettingPoolPath(t *testing.T) {
	quoter := NewMockQuoter()
	poolManager := NewMockLiquidityPoolManager()
	optimizer := NewMockPathOptimizer(quoter, poolManager)
	ctx := context.Background()

	inflows := map[string]float64{
		"EUR": 100_000.0,
		"GBP": 50_000.0,
	}
	outflows := map[string]float64{
		"INR": 5_000_000.0,
		"AED": 1_000_000.0,
	}

	path, err := optimizer.buildNettingPoolPath(ctx, inflows, outflows)
	if err != nil {
		t.Fatalf("buildNettingPoolPath failed: %v", err)
	}

	if path.Route != RouteNettingPool {
		t.Errorf("expected netting pool route, got %v", path.Route)
	}

	// Netting should have USD in the currencies
	hasUSD := false
	for _, curr := range path.CurrenciesInvolved {
		if curr == "USD" {
			hasUSD = true
			break
		}
	}
	if !hasUSD {
		t.Errorf("netting pool should include USD as intermediary")
	}

	// Netting is slowest but lowest cost
	if path.ExecutionTimeMs < 86400000 { // 24 hours for daily netting
		t.Errorf("netting should take ~24 hours, got %d ms", path.ExecutionTimeMs)
	}
}

func TestRealWorldScenario(t *testing.T) {
	// Scenario: $100k USD + 5M INR inflow, £30k GBP outflow
	quoter := NewMockQuoter()
	poolManager := NewMockLiquidityPoolManager()
	optimizer := NewMockPathOptimizer(quoter, poolManager)
	ctx := context.Background()

	netAmounts := []NetAmount{
		{Currency: "USD", Amount: 100_000.0},
		{Currency: "INR", Amount: 5_000_000.0},
		{Currency: "GBP", Amount: -30_000.0},
	}

	paths, err := optimizer.FindBestPath(ctx, netAmounts)
	if err != nil {
		t.Fatalf("FindBestPath failed: %v", err)
	}

	if len(paths) == 0 {
		t.Fatalf("expected at least one path")
	}

	// Best path should be cost-optimal
	bestPath := paths[0]
	t.Logf("Best path: route=%v cost=%d bps execution=%d ms",
		bestPath.Route, bestPath.EstimatedCostBps, bestPath.ExecutionTimeMs)

	// Verify key currencies are involved
	involvesCurrencies := make(map[string]bool)
	for _, curr := range bestPath.CurrenciesInvolved {
		involvesCurrencies[curr] = true
	}

	// At least USD and GBP should be in any viable path
	if !involvesCurrencies["USD"] {
		t.Errorf("best path should involve USD")
	}
	if !involvesCurrencies["GBP"] {
		t.Errorf("best path should involve GBP")
	}
}

func TestPathSortingByCost(t *testing.T) {
	quoter := NewMockQuoter()
	poolManager := NewMockLiquidityPoolManager()
	optimizer := NewMockPathOptimizer(quoter, poolManager)
	ctx := context.Background()

	// Multi-leg scenario that produces multiple path options
	netAmounts := []NetAmount{
		{Currency: "USD", Amount: -1_000_000.0},
		{Currency: "EUR", Amount: 500_000.0},
		{Currency: "INR", Amount: 50_000_000.0},
	}

	paths, err := optimizer.FindBestPath(ctx, netAmounts)
	if err != nil {
		t.Fatalf("FindBestPath failed: %v", err)
	}

	// Verify paths are sorted by cost
	for i := 0; i < len(paths)-1; i++ {
		if paths[i].EstimatedCostBps > paths[i+1].EstimatedCostBps {
			t.Errorf("paths[%d] cost %d > paths[%d] cost %d (not sorted)",
				i, paths[i].EstimatedCostBps, i+1, paths[i+1].EstimatedCostBps)
		}
	}
}
