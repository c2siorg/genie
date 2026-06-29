// Package fx provides foreign exchange rate management, quotation, and settlement
// path optimization for multi-currency transactions. It integrates with the
// settlement coordinator to find optimal cross-border payment routes.
//
// The design follows a three-layer approach:
// 1. FXRate: raw exchange rate data from providers
// 2. FXQuote: quoted rates for specific transaction amounts (includes fees)
// 3. SettlementPath: optimized routing with cost/time tradeoffs
package fx

import (
	"time"
)

// Source identifies the FX rate provider.
type Source string

const (
	SourceXE    Source = "xe.com"
	SourceOanda Source = "oanda"
)

// FXRate represents a single exchange rate from one currency to another.
// Rates are snapshots from a specific provider at a specific timestamp.
type FXRate struct {
	FromCurrency string    `json:"from_currency"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"` // multiplier: from_amount * rate = to_amount
	Timestamp    time.Time `json:"timestamp"`
	Source       Source    `json:"source"` // provider
}

// LiquidityPool tracks available balance in a specific currency from a specific source.
// Used for netting pools and correspondent banking arrangements.
type LiquidityPool struct {
	Currency          string    `json:"currency"`
	AvailableBalance  float64   `json:"available_balance"`   // in base units (e.g. dollars, rupees)
	Provider          string    `json:"provider"`            // "bank" | "correspondent" | "pool"
	SettlementCostBps int       `json:"settlement_cost_bps"` // basis points (100 bps = 1%)
	LastUpdated       time.Time `json:"last_updated"`
}

// SettlementRoute describes the path a payment takes through the FX network.
type SettlementRoute string

const (
	RouteDirect        SettlementRoute = "direct"        // A → B directly
	RouteCorrespondent SettlementRoute = "correspondent" // A → USD → B (or other intermediary)
	RouteNettingPool   SettlementRoute = "netting_pool"  // Aggregated pool netting
)

// SettlementPath represents one possible routing option for multi-currency settlement.
// The optimizer returns ranked paths; callers choose based on policy (speed vs. cost).
type SettlementPath struct {
	Route              SettlementRoute `json:"route"`
	CurrenciesInvolved []string        `json:"currencies_involved"`
	FXRates            []FXRate        `json:"fx_rates"`           // rates used in this path
	EstimatedCostBps   int             `json:"estimated_cost_bps"` // total cost including all conversions
	EstimatedCostUSD   float64         `json:"estimated_cost_usd"` // cost in USD equivalent
	ExecutionTimeMs    int             `json:"execution_time_ms"`  // est. settlement time
	Liquidity          []LiquidityPool `json:"liquidity"`          // pools involved
}

// NetAmount represents a currency amount to be settled (or netted).
type NetAmount struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"` // positive: inflow, negative: outflow
}

// FXQuote is a firm quote for a specific transaction amount, valid for a TTL.
type FXQuote struct {
	FromAmount   float64   `json:"from_amount"`
	FromCurrency string    `json:"from_currency"`
	ToAmount     float64   `json:"to_amount"`
	ToCurrency   string    `json:"to_currency"`
	Rate         float64   `json:"rate"`
	FeeBps       int       `json:"fee_bps"`    // fee in basis points
	FeeAmount    float64   `json:"fee_amount"` // fee in from_currency
	ValidUntil   time.Time `json:"valid_until"`
	Provider     Source    `json:"provider"`
}

// IsExpired checks if the quote is no longer valid.
func (q FXQuote) IsExpired() bool {
	return time.Now().After(q.ValidUntil)
}
