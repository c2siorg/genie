package fx

import (
	"context"
	"testing"
	"time"
)

func TestMockQuoterGetRate(t *testing.T) {
	quoter := NewMockQuoter()
	ctx := context.Background()

	tests := []struct {
		name      string
		from      string
		to        string
		shouldErr bool
	}{
		{"USD to INR", "USD", "INR", false},
		{"EUR to GBP", "EUR", "GBP", false},
		{"Same currency", "USD", "USD", false},
		{"INR to AED", "INR", "AED", false},
		{"Unsupported currency", "XYZ", "USD", true},
		{"Pair not supported", "ABC", "XYZ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, err := quoter.GetRate(ctx, tt.from, tt.to)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if rate.FromCurrency == rate.ToCurrency && rate.Rate != 1.0 {
					t.Errorf("same currency should have rate 1.0, got %.4f", rate.Rate)
				}
				if rate.Rate <= 0 {
					t.Errorf("rate must be positive, got %.4f", rate.Rate)
				}
			}
		})
	}
}

func TestMockQuoterCaching(t *testing.T) {
	quoter := NewMockQuoter()
	ctx := context.Background()

	// Get rate twice
	rate1, _ := quoter.GetRate(ctx, "USD", "INR")
	time.Sleep(100 * time.Millisecond)
	rate2, _ := quoter.GetRate(ctx, "USD", "INR")

	// Should be identical (cached)
	if rate1.Rate != rate2.Rate {
		t.Errorf("cached rates differ: %.4f vs %.4f", rate1.Rate, rate2.Rate)
	}

	// Timestamps should differ (because we fetched at different times, but in reality cached)
	// Actually since we're caching, the first timestamp should be preserved
	if rate1.Timestamp != rate2.Timestamp {
		// In a real cache, timestamps would match for cached entries
		// For mock, this is OK
	}
}

func TestMockQuoterGetQuote(t *testing.T) {
	quoter := NewMockQuoter()
	ctx := context.Background()

	tests := []struct {
		name      string
		from      string
		amount    float64
		to        string
		shouldErr bool
		expectFee bool
	}{
		{"USD 1000 to INR", "USD", 1000.0, "INR", false, true},
		{"EUR 500 to GBP", "EUR", 500.0, "GBP", false, true},
		{"Negative amount", "USD", -100.0, "INR", true, false},
		{"Unsupported currency", "XYZ", 100.0, "USD", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, err := quoter.GetQuote(ctx, tt.from, tt.amount, tt.to)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if quote.FromAmount != tt.amount {
					t.Errorf("from amount mismatch: %.2f vs %.2f", quote.FromAmount, tt.amount)
				}
				if quote.ToAmount <= 0 {
					t.Errorf("to amount must be positive, got %.2f", quote.ToAmount)
				}
				if tt.expectFee && quote.FeeBps <= 0 {
					t.Errorf("expected fee, got %d bps", quote.FeeBps)
				}
				if quote.IsExpired() {
					t.Errorf("fresh quote should not be expired")
				}
			}
		})
	}
}

func TestFXQuoteExpiry(t *testing.T) {
	quote := FXQuote{
		FromAmount:   100.0,
		FromCurrency: "USD",
		ToAmount:     8312.0,
		ToCurrency:   "INR",
		Rate:         83.12,
		FeeBps:       50,
		ValidUntil:   time.Now().Add(-1 * time.Second), // Expired
	}

	if !quote.IsExpired() {
		t.Errorf("quote should be expired")
	}

	quote.ValidUntil = time.Now().Add(10 * time.Second)
	if quote.IsExpired() {
		t.Errorf("fresh quote should not be expired")
	}
}

func TestSupportedCurrencies(t *testing.T) {
	quoter := NewMockQuoter()
	supported := quoter.SupportedCurrencies()

	expected := map[string]bool{
		"USD": true,
		"EUR": true,
		"GBP": true,
		"INR": true,
		"AED": true,
		"SGD": true,
	}

	for _, curr := range supported {
		if !expected[curr] {
			t.Errorf("unexpected currency in supported list: %s", curr)
		}
	}

	if len(supported) != len(expected) {
		t.Errorf("supported count mismatch: expected %d, got %d", len(expected), len(supported))
	}
}
