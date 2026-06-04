package fx

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FXQuoter interface provides exchange rates and firm quotes for multi-currency settlement.
// Implementations must be thread-safe and handle provider unavailability gracefully.
type FXQuoter interface {
	// GetRate returns the current mid-market rate from one currency to another.
	// Falls back to cached rates if the provider is unavailable.
	GetRate(ctx context.Context, from, to string) (FXRate, error)

	// GetQuote returns a firm quote for a specific amount, valid for a TTL.
	// The quote includes the conversion amount, fees, and provider information.
	GetQuote(ctx context.Context, from string, amount float64, to string) (FXQuote, error)

	// SupportedCurrencies returns the list of currencies this quoter can handle.
	SupportedCurrencies() []string
}

// MockQuoter provides test rates for phase 1 currencies without external API calls.
// Useful for development and unit testing. In production, replace with real provider.
type MockQuoter struct {
	mu             sync.RWMutex
	rateCache      map[string]map[string]FXRate
	cacheExpiry    map[string]time.Time
	midRates       map[string]map[string]float64
	providerFeeBps int
	cacheTTL       time.Duration
}

// NewMockQuoter creates a quoter with mock rates for phase 1 currencies.
// Phase 1: USD, EUR, GBP, INR, AED, SGD
func NewMockQuoter() *MockQuoter {
	mq := &MockQuoter{
		rateCache:      make(map[string]map[string]FXRate),
		cacheExpiry:    make(map[string]time.Time),
		providerFeeBps: 50, // 0.5% conversion fee
		cacheTTL:       60 * time.Second,
	}

	// Phase 1 mid-market rates (as of 2024-05-31).
	// In production, these come from live provider APIs.
	mq.midRates = map[string]map[string]float64{
		"USD": {
			"USD": 1.0,
			"EUR": 0.92,
			"GBP": 0.79,
			"INR": 83.12,
			"AED": 3.67,
			"SGD": 1.34,
		},
		"EUR": {
			"USD": 1.087,
			"EUR": 1.0,
			"GBP": 0.859,
			"INR": 90.35,
			"AED": 3.991,
			"SGD": 1.456,
		},
		"GBP": {
			"USD": 1.266,
			"EUR": 1.164,
			"GBP": 1.0,
			"INR": 105.18,
			"AED": 4.645,
			"SGD": 1.695,
		},
		"INR": {
			"USD": 0.01203,
			"EUR": 0.01106,
			"GBP": 0.00951,
			"INR": 1.0,
			"AED": 0.04414,
			"SGD": 0.01612,
		},
		"AED": {
			"USD": 0.2723,
			"EUR": 0.2506,
			"GBP": 0.2154,
			"INR": 22.647,
			"AED": 1.0,
			"SGD": 0.3651,
		},
		"SGD": {
			"USD": 0.7461,
			"EUR": 0.6872,
			"GBP": 0.5900,
			"INR": 62.033,
			"AED": 2.739,
			"SGD": 1.0,
		},
	}

	return mq
}

// GetRate returns the current mid-market rate, from cache or falls back to mid-rate.
func (mq *MockQuoter) GetRate(ctx context.Context, from, to string) (FXRate, error) {
	from = normalizeCode(from)
	to = normalizeCode(to)

	if from == to {
		return FXRate{
			FromCurrency: from,
			ToCurrency:   to,
			Rate:         1.0,
			Timestamp:    time.Now(),
			Source:       SourceXE,
		}, nil
	}

	// Try cache first
	mq.mu.RLock()
	cached := mq.rateCache[from]
	if cached != nil {
		if rate, ok := cached[to]; ok {
			if time.Now().Before(mq.cacheExpiry[from+to]) {
				mq.mu.RUnlock()
				return rate, nil
			}
		}
	}
	mq.mu.RUnlock()

	// Fall back to mid-rate if available
	mq.mu.RLock()
	rates := mq.midRates[from]
	if rates == nil {
		mq.mu.RUnlock()
		return FXRate{}, fmt.Errorf("unsupported currency: %s", from)
	}
	midRate, ok := rates[to]
	if !ok {
		mq.mu.RUnlock()
		return FXRate{}, fmt.Errorf("unsupported pair: %s/%s", from, to)
	}
	mq.mu.RUnlock()

	rate := FXRate{
		FromCurrency: from,
		ToCurrency:   to,
		Rate:         midRate,
		Timestamp:    time.Now(),
		Source:       SourceXE,
	}

	// Cache the rate
	mq.mu.Lock()
	if mq.rateCache[from] == nil {
		mq.rateCache[from] = make(map[string]FXRate)
	}
	mq.rateCache[from][to] = rate
	mq.cacheExpiry[from+to] = time.Now().Add(mq.cacheTTL)
	mq.mu.Unlock()

	return rate, nil
}

// GetQuote returns a firm quote with fees applied for a specific amount.
func (mq *MockQuoter) GetQuote(ctx context.Context, from string, amount float64, to string) (FXQuote, error) {
	if amount < 0 {
		return FXQuote{}, fmt.Errorf("amount must be positive, got %.2f", amount)
	}

	rate, err := mq.GetRate(ctx, from, to)
	if err != nil {
		return FXQuote{}, err
	}

	from = normalizeCode(from)
	to = normalizeCode(to)

	// Calculate fee and net amount
	feeAmount := amount * float64(mq.providerFeeBps) / 10000.0 // bps to decimal
	netAmount := amount - feeAmount
	toAmount := netAmount * rate.Rate

	quote := FXQuote{
		FromAmount:   amount,
		FromCurrency: from,
		ToAmount:     toAmount,
		ToCurrency:   to,
		Rate:         rate.Rate,
		FeeBps:       mq.providerFeeBps,
		FeeAmount:    feeAmount,
		ValidUntil:   time.Now().Add(30 * time.Second), // quotes valid for 30 seconds
		Provider:     SourceXE,
	}

	return quote, nil
}

// SupportedCurrencies returns the list of phase 1 currencies.
func (mq *MockQuoter) SupportedCurrencies() []string {
	return []string{"USD", "EUR", "GBP", "INR", "AED", "SGD"}
}

// normalizeCode converts currency codes to uppercase for consistency.
func normalizeCode(code string) string {
	if len(code) == 3 {
		// For now, assume 3-letter codes are currency codes.
		upper := code
		if len(code) >= 1 {
			// Simple uppercase for any input
			for i := 0; i < len(code); i++ {
				c := code[i]
				if c >= 'a' && c <= 'z' {
					upper = upper[:i] + string(c-32) + upper[i+1:]
				}
			}
		}
		return upper
	}
	return code
}
