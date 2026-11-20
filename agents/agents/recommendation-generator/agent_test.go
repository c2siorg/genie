package recommendation_generator

import (
	"context"
	"testing"

	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
)

func sampleInput() contractv1.RecommendationGeneratorInput {
	return contractv1.RecommendationGeneratorInput{
		Profile: contractv1.UserProfile{
			UserID:        "u-1",
			RiskTolerance: contractv1.RiskModerate,
		},
		Opportunities: []contractv1.SpendingOpportunity{{
			ID:                      "opp-food",
			Category:                "food",
			MonthlySavingsPotential: 50000, // ₹500 in paise
			BenchmarkPercentile:     82,
		}},
	}
}

func TestExecute_GeneratesRecommendationPerOpportunity(t *testing.T) {
	a := NewAgent()
	out, err := a.Execute(context.Background(), sampleInput())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	recs, ok := out.([]contractv1.Recommendation)
	if !ok {
		t.Fatalf("output type = %T, want []Recommendation", out)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d recommendations, want 1", len(recs))
	}
	rec := recs[0]
	if rec.UserID != "u-1" {
		t.Errorf("UserID = %q, want u-1", rec.UserID)
	}
	if rec.RecommendationID != "rec-opp-food" {
		t.Errorf("RecommendationID = %q, want rec-opp-food", rec.RecommendationID)
	}
	// Moderate risk tolerance → 0.75 confidence multiplier.
	if rec.EstimatedImpact.Confidence != 0.75 {
		t.Errorf("confidence = %v, want 0.75 for moderate", rec.EstimatedImpact.Confidence)
	}
	wantMonthly := int64(float64(50000) * 0.75)
	if rec.EstimatedImpact.MonthlySavingsPaise != wantMonthly {
		t.Errorf("monthly savings = %d, want %d", rec.EstimatedImpact.MonthlySavingsPaise, wantMonthly)
	}
}

func TestExecute_NoOpportunities_Rejected(t *testing.T) {
	a := NewAgent()
	in := contractv1.RecommendationGeneratorInput{
		Profile: contractv1.UserProfile{UserID: "u-1"},
	}
	if _, err := a.Execute(context.Background(), in); err == nil {
		t.Fatal("expected validation error when no opportunities supplied")
	}
}

func TestEstimateImpact_ConfidenceByRiskTolerance(t *testing.T) {
	a := NewAgent()
	opp := &contractv1.SpendingOpportunity{MonthlySavingsPotential: 10000}
	cases := map[contractv1.RiskTolerance]float64{
		contractv1.RiskConservative: 0.6,
		contractv1.RiskModerate:     0.75,
		contractv1.RiskAggressive:   0.85,
	}
	for rt, want := range cases {
		got := a.EstimateImpact(opp, &contractv1.UserProfile{RiskTolerance: rt})
		if got.Confidence != want {
			t.Errorf("risk %q: confidence = %v, want %v", rt, got.Confidence, want)
		}
	}
}

func TestValidateRecommendation_RequiresActions(t *testing.T) {
	a := NewAgent()
	rec := &contractv1.Recommendation{
		RecommendationID: "r-1", UserID: "u-1", Title: "x",
		EstimatedImpact: contractv1.ImpactEstimate{Confidence: 0.5},
	}
	if err := a.ValidateRecommendation(rec); err == nil {
		t.Fatal("expected error: recommendation has no actions")
	}
	rec.Actions = []contractv1.Action{{ID: "a-1"}}
	if err := a.ValidateRecommendation(rec); err != nil {
		t.Fatalf("unexpected error for valid recommendation: %v", err)
	}
}
