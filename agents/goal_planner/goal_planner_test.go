package goal_planner

import "testing"

func TestSimulate_HighSuccessForOverFundedGoal(t *testing.T) {
	a := New()
	plan := a.Simulate(Request{
		CurrentCorpus:        50_00_000,
		MonthlyContribution:  50_000,
		TargetCorpus:         60_00_000,
		HorizonMonths:        120,
		ExpectedAnnualReturn: 0.10,
		AnnualVolatility:     0.15,
	})
	if plan.SuccessProbability < 0.9 {
		t.Errorf("over-funded goal should have >90%% success; got %.2f", plan.SuccessProbability)
	}
}

func TestSimulate_LowSuccessForStretchGoal(t *testing.T) {
	a := New()
	plan := a.Simulate(Request{
		CurrentCorpus:        1_00_000,
		MonthlyContribution:  5_000,
		TargetCorpus:         1_00_00_000, // ₹1cr in 5yr — unrealistic
		HorizonMonths:        60,
		ExpectedAnnualReturn: 0.10,
		AnnualVolatility:     0.15,
	})
	if plan.SuccessProbability > 0.5 {
		t.Errorf("stretch goal should have <50%% success; got %.2f", plan.SuccessProbability)
	}
}

func TestSimulate_PercentilesOrdered(t *testing.T) {
	plan := New().Simulate(Request{
		CurrentCorpus: 10_00_000, MonthlyContribution: 10_000,
		TargetCorpus: 50_00_000, HorizonMonths: 120,
		ExpectedAnnualReturn: 0.12, AnnualVolatility: 0.18,
	})
	if !(plan.P10Corpus <= plan.P50Corpus && plan.P50Corpus <= plan.P90Corpus) {
		t.Errorf("p10≤p50≤p90 violated: %.0f / %.0f / %.0f", plan.P10Corpus, plan.P50Corpus, plan.P90Corpus)
	}
}

func TestSimulate_RequiredMonthlySensible(t *testing.T) {
	plan := New().Simulate(Request{
		CurrentCorpus: 0, MonthlyContribution: 0,
		TargetCorpus: 10_00_000, HorizonMonths: 120,
		ExpectedAnnualReturn: 0.10, AnnualVolatility: 0.15,
	})
	if plan.RequiredMonthlyINR < 1000 || plan.RequiredMonthlyINR > 50_000 {
		t.Errorf("required monthly for ₹10L/10yr@10%% should be ~₹4-5k; got ₹%.0f", plan.RequiredMonthlyINR)
	}
}

func TestSimulate_DeterministicWithFixedSeed(t *testing.T) {
	req := Request{
		CurrentCorpus: 1_00_000, MonthlyContribution: 5000, TargetCorpus: 5_00_000,
		HorizonMonths: 60, ExpectedAnnualReturn: 0.10, AnnualVolatility: 0.15, Seed: 1,
	}
	p1 := New().Simulate(req).SuccessProbability
	p2 := New().Simulate(req).SuccessProbability
	if p1 != p2 {
		t.Errorf("same seed should produce identical results: %.4f vs %.4f", p1, p2)
	}
}
