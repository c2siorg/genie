package fx

import (
	"context"
	"fmt"
	"sort"
)

// SettlementPathOptimizer evaluates possible routing options for multi-currency settlement.
// Returns ranked paths with cost/speed tradeoffs. Callers apply policy rules to choose.
type SettlementPathOptimizer interface {
	// FindBestPath evaluates all routing strategies for the given net amounts and
	// returns a ranked list of settlement paths. Higher-ranked paths are preferred
	// based on cost, but callers can override with policy rules (e.g., prefer
	// direct routes if amount < $1M, prefer netting if > $10M).
	FindBestPath(ctx context.Context, netAmounts []NetAmount) ([]SettlementPath, error)
}

// MockPathOptimizer implements settlement path optimization for multi-currency flows.
// It evaluates three strategies: direct, correspondent, and netting pool.
type MockPathOptimizer struct {
	quoter      FXQuoter
	poolManager LiquidityPoolManager
}

// NewMockPathOptimizer creates an optimizer with a quoter and pool manager.
func NewMockPathOptimizer(quoter FXQuoter, poolManager LiquidityPoolManager) *MockPathOptimizer {
	return &MockPathOptimizer{
		quoter:      quoter,
		poolManager: poolManager,
	}
}

// FindBestPath evaluates three routing strategies and returns ranked paths.
func (o *MockPathOptimizer) FindBestPath(ctx context.Context, netAmounts []NetAmount) ([]SettlementPath, error) {
	if len(netAmounts) == 0 {
		return nil, fmt.Errorf("no net amounts provided")
	}

	// Separate inflows and outflows
	inflows := make(map[string]float64)
	outflows := make(map[string]float64)

	for _, na := range netAmounts {
		if na.Amount > 0 {
			inflows[na.Currency] = na.Amount
		} else if na.Amount < 0 {
			outflows[na.Currency] = -na.Amount
		}
	}

	var paths []SettlementPath

	// Try direct paths first (cheapest if available)
	for outCurr, outAmount := range outflows {
		for inCurr, inAmount := range inflows {
			if outCurr == inCurr {
				// Same currency: no FX needed
				paths = append(paths, o.buildDirectPath(outCurr, inCurr, outAmount, inAmount))
			} else {
				// Different currencies: FX needed, try direct route
				path, err := o.buildDirectFXPath(ctx, outCurr, outAmount, inCurr, inAmount)
				if err == nil {
					paths = append(paths, path)
				}
			}
		}
	}

	// Try correspondent routes (USD as intermediary)
	for outCurr, outAmount := range outflows {
		for inCurr, inAmount := range inflows {
			if outCurr != inCurr {
				path, err := o.buildCorrespondentPath(ctx, outCurr, outAmount, inCurr, inAmount)
				if err == nil {
					paths = append(paths, path)
				}
			}
		}
	}

	// Try netting pool approach (aggregate USD pool)
	if len(inflows) > 0 && len(outflows) > 0 {
		path, err := o.buildNettingPoolPath(ctx, inflows, outflows)
		if err == nil {
			paths = append(paths, path)
		}
	}

	// Sort by cost (lower is better)
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].EstimatedCostBps != paths[j].EstimatedCostBps {
			return paths[i].EstimatedCostBps < paths[j].EstimatedCostBps
		}
		// Tie-breaker: prefer faster execution
		return paths[i].ExecutionTimeMs < paths[j].ExecutionTimeMs
	})

	if len(paths) == 0 {
		return nil, fmt.Errorf("no viable settlement paths found")
	}

	return paths, nil
}

// buildDirectPath handles same-currency transfers (no FX).
func (o *MockPathOptimizer) buildDirectPath(outCurr, inCurr string, outAmount, inAmount float64) SettlementPath {
	return SettlementPath{
		Route:              RouteDirect,
		CurrenciesInvolved: []string{outCurr},
		FXRates:            []FXRate{},
		EstimatedCostBps:   10,                     // Minimal settlement cost
		EstimatedCostUSD:   (outAmount * 10) / 10000.0, // Cost in bps
		ExecutionTimeMs:    1000,                   // 1 second domestic
	}
}

// buildDirectFXPath handles direct currency conversion (A -> B).
func (o *MockPathOptimizer) buildDirectFXPath(ctx context.Context, outCurr string, outAmount float64, inCurr string, inAmount float64) (SettlementPath, error) {
	quote, err := o.quoter.GetQuote(ctx, outCurr, outAmount, inCurr)
	if err != nil {
		return SettlementPath{}, err
	}

	// Assume direct FX transfer takes 2-4 hours (cross-border)
	costBps := 75 + quote.FeeBps // Base cost + provider fee

	return SettlementPath{
		Route:              RouteDirect,
		CurrenciesInvolved: []string{outCurr, inCurr},
		FXRates: []FXRate{
			{
				FromCurrency: outCurr,
				ToCurrency:   inCurr,
				Rate:         quote.Rate,
				Timestamp:    quote.ValidUntil.Add(-30 * 1000000000), // Approximate back-dated
				Source:       quote.Provider,
			},
		},
		EstimatedCostBps: costBps,
		EstimatedCostUSD: (outAmount * float64(costBps)) / 10000.0,
		ExecutionTimeMs:  14400000, // 4 hours
	}, nil
}

// buildCorrespondentPath uses USD as intermediary (A -> USD -> B).
func (o *MockPathOptimizer) buildCorrespondentPath(ctx context.Context, outCurr string, outAmount float64, inCurr string, inAmount float64) (SettlementPath, error) {
	// Step 1: outCurr -> USD
	quote1, err := o.quoter.GetQuote(ctx, outCurr, outAmount, "USD")
	if err != nil {
		return SettlementPath{}, err
	}

	// Step 2: USD -> inCurr (need to convert USD amount to inCurr amount)
	quote2, err := o.quoter.GetQuote(ctx, "USD", quote1.ToAmount, inCurr)
	if err != nil {
		return SettlementPath{}, err
	}

	// Total cost: both conversion fees + correspondent banking fees (0.1% per leg)
	totalCostBps := quote1.FeeBps + 10 + quote2.FeeBps + 10

	return SettlementPath{
		Route:              RouteCorrespondent,
		CurrenciesInvolved: []string{outCurr, "USD", inCurr},
		FXRates: []FXRate{
			{
				FromCurrency: outCurr,
				ToCurrency:   "USD",
				Rate:         quote1.Rate,
				Timestamp:    quote1.ValidUntil.Add(-30 * 1000000000),
				Source:       quote1.Provider,
			},
			{
				FromCurrency: "USD",
				ToCurrency:   inCurr,
				Rate:         quote2.Rate,
				Timestamp:    quote2.ValidUntil.Add(-30 * 1000000000),
				Source:       quote2.Provider,
			},
		},
		EstimatedCostBps: totalCostBps,
		EstimatedCostUSD: (outAmount * float64(totalCostBps)) / 10000.0,
		ExecutionTimeMs:  28800000, // 8 hours (two conversions)
	}, nil
}

// buildNettingPoolPath aggregates flows through a USD pool (lowest cost if available).
func (o *MockPathOptimizer) buildNettingPoolPath(ctx context.Context, inflows, outflows map[string]float64) (SettlementPath, error) {
	pools := o.poolManager.GetAllPools()
	if len(pools) == 0 {
		return SettlementPath{}, fmt.Errorf("no pools available")
	}

	// For netting, convert all flows to USD equivalent
	var fxRates []FXRate
	var currencies []string
	totalCostBps := 20 // Base netting cost

	// Convert inflows to USD
	for curr, amount := range inflows {
		if curr != "USD" {
			quote, err := o.quoter.GetQuote(ctx, curr, amount, "USD")
			if err == nil {
				fxRates = append(fxRates, FXRate{
					FromCurrency: curr,
					ToCurrency:   "USD",
					Rate:         quote.Rate,
					Source:       quote.Provider,
				})
				totalCostBps += quote.FeeBps
				currencies = append(currencies, curr)
			}
		}
	}

	// Convert outflows from USD
	for curr, amount := range outflows {
		if curr != "USD" {
			quote, err := o.quoter.GetQuote(ctx, "USD", amount*1.0, curr)
			if err == nil {
				fxRates = append(fxRates, FXRate{
					FromCurrency: "USD",
					ToCurrency:   curr,
					Rate:         quote.Rate,
					Source:       quote.Provider,
				})
				totalCostBps += quote.FeeBps
				currencies = append(currencies, curr)
			}
		}
	}

	currencies = append(currencies, "USD")

	return SettlementPath{
		Route:              RouteNettingPool,
		CurrenciesInvolved: currencies,
		FXRates:            fxRates,
		EstimatedCostBps:   totalCostBps,
		EstimatedCostUSD:   1000.0, // Flat fee for netting services
		ExecutionTimeMs:    86400000, // 24 hours (daily netting cycle)
		Liquidity:          pools,
	}, nil
}
