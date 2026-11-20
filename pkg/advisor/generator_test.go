package advisor

import (
	"context"
	"testing"
)

func TestRecommendationGenerator_GenerateRecommendation_Valid(t *testing.T) {
	rg := NewRecommendationGenerator()
	ctx := context.Background()
	profile := &UserProfile{
		UserID:           "user-123",
		RiskTolerance:    RiskModerate,
		AnnualIncomePaise: 5000000,
	}
	opportunity := &SpendingOpportunity{
		ID:                      "opp-food",
		Category:                "food",
		CurrentMonthlyPaise:     50000,
		OptimizedMonthlyPaise:   40000,
		MonthlySavingsPotential: 10000,
		Confidence:              0.85,
		BenchmarkPercentile:     75,
	}

	rec, err := rg.GenerateRecommendation(ctx, profile, opportunity)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec == nil {
		t.Fatal("recommendation should not be nil")
	}
	if rec.UserID != "user-123" {
		t.Errorf("got user_id %s, want user-123", rec.UserID)
	}
	if rec.Status != StatusPending {
		t.Errorf("got status %s, want pending", rec.Status)
	}
}

func TestRecommendationGenerator_GenerateRecommendation_NilProfile(t *testing.T) {
	rg := NewRecommendationGenerator()
	ctx := context.Background()
	opportunity := &SpendingOpportunity{ID: "opp-1", Category: "food"}

	rec, err := rg.GenerateRecommendation(ctx, nil, opportunity)

	if err == nil {
		t.Fatal("expected error for nil profile")
	}
	if rec != nil {
		t.Fatal("recommendation should be nil on error")
	}
}

func TestRecommendationGenerator_GenerateRecommendation_NilOpportunity(t *testing.T) {
	rg := NewRecommendationGenerator()
	ctx := context.Background()
	profile := &UserProfile{UserID: "user-123"}

	rec, err := rg.GenerateRecommendation(ctx, profile, nil)

	if err == nil {
		t.Fatal("expected error for nil opportunity")
	}
	if rec != nil {
		t.Fatal("recommendation should be nil on error")
	}
}

func TestRecommendationGenerator_GenerateMultipleRecommendations_Valid(t *testing.T) {
	rg := NewRecommendationGenerator()
	ctx := context.Background()
	profile := &UserProfile{
		UserID:        "user-123",
		RiskTolerance: RiskModerate,
	}
	opportunities := []SpendingOpportunity{
		{ID: "opp-1", Category: "food", MonthlySavingsPotential: 10000},
		{ID: "opp-2", Category: "transport", MonthlySavingsPotential: 5000},
	}

	recs, err := rg.GenerateMultipleRecommendations(ctx, profile, opportunities)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recs == nil {
		t.Fatal("recommendations should not be nil")
	}
}

func TestRecommendationGenerator_GenerateMultipleRecommendations_NilProfile(t *testing.T) {
	rg := NewRecommendationGenerator()
	ctx := context.Background()
	opportunities := []SpendingOpportunity{
		{ID: "opp-1", Category: "food"},
	}

	recs, err := rg.GenerateMultipleRecommendations(ctx, nil, opportunities)

	if err == nil {
		t.Fatal("expected error for nil profile")
	}
	if recs != nil {
		t.Fatal("recommendations should be nil on error")
	}
}

func TestRecommendationGenerator_GenerateMultipleRecommendations_EmptyOpportunities(t *testing.T) {
	rg := NewRecommendationGenerator()
	ctx := context.Background()
	profile := &UserProfile{UserID: "user-123"}

	recs, err := rg.GenerateMultipleRecommendations(ctx, profile, []SpendingOpportunity{})

	if err == nil {
		t.Fatal("expected error for empty opportunities")
	}
	if recs != nil {
		t.Fatal("recommendations should be nil on error")
	}
}

func TestRecommendationGenerator_GenerateActions(t *testing.T) {
	rg := NewRecommendationGenerator()

	actions := rg.GenerateActions("food", 10000, ExperienceBeginner)

	if actions == nil {
		t.Fatal("actions should not be nil")
	}
	// Should return list of actions (food-related)
}

func TestRecommendationGenerator_GenerateRisks(t *testing.T) {
	rg := NewRecommendationGenerator()

	risks := rg.GenerateRisks(CategorySpendingOptimization)

	if risks == nil {
		t.Fatal("risks should not be nil")
	}
	// Should return list of risks for spending optimization
}

func TestRecommendationGenerator_EstimateImpact_Conservative(t *testing.T) {
	rg := NewRecommendationGenerator()
	opportunity := &SpendingOpportunity{
		MonthlySavingsPotential: 10000,
	}
	profile := &UserProfile{
		RiskTolerance: RiskConservative,
	}

	impact := rg.EstimateImpact(opportunity, profile)

	if impact.Confidence != 0.6 {
		t.Errorf("got confidence %f, want 0.6 for conservative", impact.Confidence)
	}
	expectedMonthly := int64(6000) // 10000 * 0.6
	if impact.MonthlySavingsPaise != expectedMonthly {
		t.Errorf("got monthly %d, want %d", impact.MonthlySavingsPaise, expectedMonthly)
	}
}

func TestRecommendationGenerator_EstimateImpact_Moderate(t *testing.T) {
	rg := NewRecommendationGenerator()
	opportunity := &SpendingOpportunity{
		MonthlySavingsPotential: 10000,
	}
	profile := &UserProfile{
		RiskTolerance: RiskModerate,
	}

	impact := rg.EstimateImpact(opportunity, profile)

	if impact.Confidence != 0.75 {
		t.Errorf("got confidence %f, want 0.75 for moderate", impact.Confidence)
	}
}

func TestRecommendationGenerator_EstimateImpact_Aggressive(t *testing.T) {
	rg := NewRecommendationGenerator()
	opportunity := &SpendingOpportunity{
		MonthlySavingsPotential: 10000,
	}
	profile := &UserProfile{
		RiskTolerance: RiskAggressive,
	}

	impact := rg.EstimateImpact(opportunity, profile)

	if impact.Confidence != 0.85 {
		t.Errorf("got confidence %f, want 0.85 for aggressive", impact.Confidence)
	}
}

func TestRecommendationGenerator_CheckCompliance_Compliant(t *testing.T) {
	rg := NewRecommendationGenerator()
	rec := &Recommendation{
		RecommendationID: "rec-1",
		Category:         CategorySpendingOptimization,
	}
	constraints := []string{"cannot_invest_offshore"}

	ok, reason := rg.CheckCompliance(rec, constraints)

	if !ok {
		t.Errorf("expected compliance, got: %s", reason)
	}
}

func TestRecommendationGenerator_CheckCompliance_NilRecommendation(t *testing.T) {
	rg := NewRecommendationGenerator()

	ok, reason := rg.CheckCompliance(nil, []string{})

	if ok {
		t.Fatal("expected non-compliant for nil recommendation")
	}
	if reason == "" {
		t.Fatal("reason should be provided")
	}
}

func TestRecommendationGenerator_ValidateRecommendation_Valid(t *testing.T) {
	rg := NewRecommendationGenerator()
	rec := &Recommendation{
		RecommendationID: "rec-1",
		UserID:           "user-123",
		Title:            "Optimize spending",
		EstimatedImpact: ImpactEstimate{
			Confidence: 0.85,
		},
		Actions: []Action{
			{ID: "act-1", Type: ActionReduceCategory},
		},
	}

	err := rg.ValidateRecommendation(rec)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecommendationGenerator_ValidateRecommendation_NilRecommendation(t *testing.T) {
	rg := NewRecommendationGenerator()

	err := rg.ValidateRecommendation(nil)

	if err == nil {
		t.Fatal("expected error for nil recommendation")
	}
}

func TestRecommendationGenerator_ValidateRecommendation_MissingTitle(t *testing.T) {
	rg := NewRecommendationGenerator()
	rec := &Recommendation{
		RecommendationID: "rec-1",
		UserID:           "user-123",
		Title:            "", // Missing!
	}

	err := rg.ValidateRecommendation(rec)

	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestRecommendationGenerator_ValidateRecommendation_InvalidConfidence(t *testing.T) {
	rg := NewRecommendationGenerator()
	rec := &Recommendation{
		RecommendationID: "rec-1",
		UserID:           "user-123",
		Title:            "Test",
		EstimatedImpact: ImpactEstimate{
			Confidence: 1.5, // Invalid!
		},
	}

	err := rg.ValidateRecommendation(rec)

	if err == nil {
		t.Fatal("expected error for invalid confidence")
	}
}

func TestRecommendationGenerator_ValidateRecommendation_NoActions(t *testing.T) {
	rg := NewRecommendationGenerator()
	rec := &Recommendation{
		RecommendationID: "rec-1",
		UserID:           "user-123",
		Title:            "Test",
		EstimatedImpact: ImpactEstimate{
			Confidence: 0.85,
		},
		Actions: []Action{}, // Empty!
	}

	err := rg.ValidateRecommendation(rec)

	if err == nil {
		t.Fatal("expected error for no actions")
	}
}
