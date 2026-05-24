package subscription_detector

import (
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

func TestDetectsNetflixMonthly(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Merchant: "netflix", Date: "2026-01-05", AmountCents: -649_00},
		{Merchant: "netflix", Date: "2026-02-05", AmountCents: -649_00},
		{Merchant: "netflix", Date: "2026-03-06", AmountCents: -649_00},
	}
	res := a.Detect(txns)
	if len(res.Subscriptions) != 1 || res.Subscriptions[0].Merchant != "netflix" {
		t.Fatalf("expected netflix subscription; got %+v", res)
	}
	if res.Subscriptions[0].AnnualisedINR <= 0 {
		t.Errorf("annualised should be positive")
	}
}

func TestIgnoresOneOffPurchases(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Merchant: "amazon", Date: "2026-01-05", AmountCents: -2_500_00},
		{Merchant: "amazon", Date: "2026-01-25", AmountCents: -800_00},
	}
	if len(a.Detect(txns).Subscriptions) != 0 {
		t.Errorf("should not flag one-off purchases as subscription")
	}
}

func TestZombieWarningOnPriceJump(t *testing.T) {
	a := New()
	txns := []finance.Transaction{
		{Merchant: "gym", Date: "2026-01-01", AmountCents: -1_000_00},
		{Merchant: "gym", Date: "2026-02-01", AmountCents: -1_000_00},
		{Merchant: "gym", Date: "2026-03-01", AmountCents: -1_500_00},
	}
	res := a.Detect(txns)
	if len(res.Subscriptions) == 0 || res.Subscriptions[0].ZombieWarning == "" {
		t.Errorf("expected zombie warning for 50%% price hike; got %+v", res)
	}
}

func TestRejectsIrregularCadence(t *testing.T) {
	a := New()
	// Day-of-month varies wildly (1, 14, 28) → std > 7.
	txns := []finance.Transaction{
		{Merchant: "irregular", Date: "2026-01-01", AmountCents: -500_00},
		{Merchant: "irregular", Date: "2026-02-14", AmountCents: -500_00},
		{Merchant: "irregular", Date: "2026-03-28", AmountCents: -500_00},
	}
	if len(a.Detect(txns).Subscriptions) != 0 {
		t.Errorf("irregular-cadence merchant should not be flagged")
	}
}

func TestRecommendationOnlyWhenSubsFound(t *testing.T) {
	a := New()
	if a.Detect(nil).Recommendation != "" {
		t.Errorf("no recommendation when no subs")
	}
}
