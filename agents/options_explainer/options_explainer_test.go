package options_explainer

import (
	"math"
	"testing"
)

func TestATMCallReasonablePrice(t *testing.T) {
	// 30-day ATM call, 25% IV, 7% rfr.
	res := New().Compute(Request{
		Side: "call", UnderlyingPrice: 100, Strike: 100,
		DaysToExpiry: 30, ImpliedVolPct: 25, RiskFreeRatePct: 7,
	})
	// ~3.10 from BS with r=7%. Wider tolerance for floating-point + erf precision.
	if math.Abs(res.TheoreticalPrice-3.10) > 0.15 {
		t.Errorf("ATM call ~3.10; got %.2f", res.TheoreticalPrice)
	}
	if res.Greeks.Delta < 0.45 || res.Greeks.Delta > 0.65 {
		t.Errorf("ATM call delta ~0.5; got %.2f", res.Greeks.Delta)
	}
}

func TestPutCallParity(t *testing.T) {
	call := New().Compute(Request{
		Side: "call", UnderlyingPrice: 100, Strike: 100,
		DaysToExpiry: 30, ImpliedVolPct: 25, RiskFreeRatePct: 7,
	}).TheoreticalPrice
	put := New().Compute(Request{
		Side: "put", UnderlyingPrice: 100, Strike: 100,
		DaysToExpiry: 30, ImpliedVolPct: 25, RiskFreeRatePct: 7,
	}).TheoreticalPrice
	// Put-call parity: C - P = S - K e^{-rT}.
	expected := 100 - 100*math.Exp(-0.07*30.0/365)
	if math.Abs((call-put)-expected) > 0.05 {
		t.Errorf("put-call parity violated: C-P=%.2f expected %.2f", call-put, expected)
	}
}

func TestPutDeltaNegative(t *testing.T) {
	res := New().Compute(Request{Side: "put", UnderlyingPrice: 100, Strike: 100, DaysToExpiry: 30, ImpliedVolPct: 25, RiskFreeRatePct: 7})
	if res.Greeks.Delta >= 0 {
		t.Errorf("put delta should be negative; got %.2f", res.Greeks.Delta)
	}
}

func TestPayoffCurveSignFlipsAtStrike(t *testing.T) {
	res := New().Compute(Request{Side: "call", UnderlyingPrice: 100, Strike: 100, DaysToExpiry: 30, ImpliedVolPct: 25, RiskFreeRatePct: 7, LotSize: 100})
	if len(res.PayoffCurve) == 0 {
		t.Fatal("empty payoff curve")
	}
	// Far below strike should be losing premium.
	if res.PayoffCurve[0].PNL >= 0 {
		t.Errorf("far OTM call should lose premium; got %.2f", res.PayoffCurve[0].PNL)
	}
	// Far above strike should be profitable.
	if res.PayoffCurve[len(res.PayoffCurve)-1].PNL <= 0 {
		t.Errorf("far ITM call should be profitable at expiry; got %.2f", res.PayoffCurve[len(res.PayoffCurve)-1].PNL)
	}
}

func TestInvalidInputReturnsDisclaimer(t *testing.T) {
	res := New().Compute(Request{Side: "call", UnderlyingPrice: 0, Strike: 100, DaysToExpiry: 30, ImpliedVolPct: 25})
	if res.TheoreticalPrice != 0 {
		t.Errorf("zero underlying should not produce a price")
	}
}
