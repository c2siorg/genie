package advisorpipeline_test

import (
	"context"
	"testing"

	financial_analyst "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/financial-analyst"
	profile_analyzer "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/profile-analyzer"
	recommendation_generator "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/agents/recommendation-generator"
	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
)

// TestAdvisorPipeline_EndToEnd wires the three migrated agents together exactly
// as the backend handler will: each stage's output is reshaped into the next
// stage's input (the "glue"). It proves the agents compose and that the
// canonical contractv1 types flow through all three without conversion.
func TestAdvisorPipeline_EndToEnd(t *testing.T) {
	ctx := context.Background()
	pa := profile_analyzer.NewAgent()
	fa := financial_analyst.NewAgent()
	rg := recommendation_generator.NewAgent()

	// Stage 1: profile analysis.
	profOut, err := pa.Execute(ctx, map[string]any{
		"user_id":             "u-42",
		"kyc_status":          "verified_high_value",
		"annual_income_paise": int64(120000000),
	})
	if err != nil {
		t.Fatalf("profile-analyzer: %v", err)
	}
	profile, ok := profOut.(*contractv1.UserProfile)
	if !ok {
		t.Fatalf("stage1 output %T, want *UserProfile", profOut)
	}
	if profile.RiskTolerance != contractv1.RiskAggressive {
		t.Fatalf("high-value KYC → risk %q, want aggressive", profile.RiskTolerance)
	}

	// Glue 1→2: wrap the profile as the analyst's input.
	faOut, err := fa.Execute(ctx, contractv1.FinancialAnalystInput{Profile: *profile})
	if err != nil {
		t.Fatalf("financial-analyst: %v", err)
	}
	opps, ok := faOut.([]contractv1.SpendingOpportunity)
	if !ok {
		t.Fatalf("stage2 output %T, want []SpendingOpportunity", faOut)
	}

	// The analyst's data-source integration is still a TODO, so it returns no
	// opportunities. Inject a synthetic opportunity so we can exercise stage 3
	// (this is what real transaction data will eventually supply).
	if len(opps) == 0 {
		opps = []contractv1.SpendingOpportunity{{
			ID:                      "opp-dining",
			Category:                "dining",
			MonthlySavingsPotential: 30000,
			BenchmarkPercentile:     90,
			Confidence:              0.8,
		}}
	}

	// Glue 2→3: combine profile + opportunities for the generator.
	rgOut, err := rg.Execute(ctx, contractv1.RecommendationGeneratorInput{
		Profile:       *profile,
		Opportunities: opps,
	})
	if err != nil {
		t.Fatalf("recommendation-generator: %v", err)
	}
	recs, ok := rgOut.([]contractv1.Recommendation)
	if !ok {
		t.Fatalf("stage3 output %T, want []Recommendation", rgOut)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d recommendations, want 1", len(recs))
	}
	// Aggressive profile → 0.85 confidence multiplier flows through to impact.
	if recs[0].EstimatedImpact.Confidence != 0.85 {
		t.Errorf("impact confidence = %v, want 0.85 (aggressive)", recs[0].EstimatedImpact.Confidence)
	}
	if recs[0].UserID != "u-42" {
		t.Errorf("recommendation UserID = %q, want u-42", recs[0].UserID)
	}
}
