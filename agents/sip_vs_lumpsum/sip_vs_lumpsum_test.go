package sip_vs_lumpsum

import "testing"

func TestPositiveDriftLumpsumWinsMoreOften(t *testing.T) {
	res := New().Simulate(Request{
		Amount: 10_00_000, HorizonMonths: 60,
		ExpectedAnnualReturn: 0.12, AnnualVolatility: 0.15,
	})
	if res.LumpsumWinProb < 0.6 {
		t.Errorf("with +12%% drift over 5y lumpsum should win ≥60%%; got %.2f", res.LumpsumWinProb)
	}
}

func TestRecommendationFlipsByWinProb(t *testing.T) {
	highDrift := New().Simulate(Request{
		Amount: 1_00_000, HorizonMonths: 60,
		ExpectedAnnualReturn: 0.15, AnnualVolatility: 0.10, Seed: 1,
	})
	negDrift := New().Simulate(Request{
		Amount: 1_00_000, HorizonMonths: 60,
		ExpectedAnnualReturn: -0.05, AnnualVolatility: 0.10, Seed: 1,
	})
	if highDrift.Recommendation == negDrift.Recommendation {
		t.Errorf("recommendation should flip with drift sign; got same: %q", highDrift.Recommendation)
	}
}

func TestDeterministicWithFixedSeed(t *testing.T) {
	req := Request{Amount: 1_00_000, HorizonMonths: 24, ExpectedAnnualReturn: 0.10, AnnualVolatility: 0.15, Seed: 5}
	a := New().Simulate(req).LumpsumP50
	b := New().Simulate(req).LumpsumP50
	if a != b {
		t.Errorf("same seed should be deterministic: %.2f vs %.2f", a, b)
	}
}

func TestEmptyAmountSafe(t *testing.T) {
	res := New().Simulate(Request{Amount: 0, HorizonMonths: 12, ExpectedAnnualReturn: 0.1, AnnualVolatility: 0.15})
	if res.LumpsumP50 != 0 {
		t.Errorf("zero amount should yield zero p50; got %.2f", res.LumpsumP50)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	if New().Simulate(Request{Amount: 1000, HorizonMonths: 12, ExpectedAnnualReturn: 0.1, AnnualVolatility: 0.1}).Disclaimer == "" {
		t.Errorf("disclaimer missing")
	}
}
