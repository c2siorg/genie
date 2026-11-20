package financial_analyst

import (
	"context"
	"testing"

	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
)

func TestExecute_ValidProfile(t *testing.T) {
	a := NewAgent()
	// Pass input as a map to simulate the HTTP/JSON path (DecodeInput round-trip).
	in := map[string]any{
		"profile": map[string]any{
			"user_id":        "u-1",
			"risk_tolerance": "moderate",
		},
	}
	out, err := a.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	opps, ok := out.([]contractv1.SpendingOpportunity)
	if !ok {
		t.Fatalf("output type = %T, want []SpendingOpportunity", out)
	}
	// Data-source integration is still a TODO, so a valid profile yields zero
	// opportunities (not an error).
	if opps == nil {
		t.Fatal("expected non-nil (possibly empty) slice")
	}
}

func TestExecute_MissingUserID(t *testing.T) {
	a := NewAgent()
	_, err := a.Execute(context.Background(), map[string]any{"profile": map[string]any{}})
	if err == nil {
		t.Fatal("expected validation error for missing profile.user_id")
	}
}

func TestExecute_TypedInput(t *testing.T) {
	a := NewAgent()
	// In-process path: a typed struct (e.g. previous agent output wrapped).
	in := contractv1.FinancialAnalystInput{
		Profile: contractv1.UserProfile{UserID: "u-2", RiskTolerance: contractv1.RiskConservative},
	}
	if _, err := a.Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute with typed input: %v", err)
	}
}

func TestComputeTrend(t *testing.T) {
	a := NewAgent()
	if got := a.ComputeTrend([]int64{1, 2}); got != "insufficient_data" {
		t.Fatalf("ComputeTrend(<3) = %q, want insufficient_data", got)
	}
	if got := a.ComputeTrend([]int64{1, 2, 3, 4}); got != "stable" {
		t.Fatalf("ComputeTrend(4 pts) = %q, want stable", got)
	}
}

func TestComputeBenchmarkPercentile_NoPeers(t *testing.T) {
	a := NewAgent()
	if got := a.ComputeBenchmarkPercentile(1000, nil); got != 50 {
		t.Fatalf("percentile with no peers = %d, want 50 (median)", got)
	}
}

func TestValidateOpportunities(t *testing.T) {
	a := NewAgent()
	bad := []contractv1.SpendingOpportunity{{ID: "", Category: "food"}}
	if err := a.ValidateOpportunities(bad); err == nil {
		t.Fatal("expected error for empty opportunity ID")
	}
	good := []contractv1.SpendingOpportunity{{
		ID: "opp-food", Category: "food", Confidence: 0.5,
		CurrentMonthlyPaise: 1000, OptimizedMonthlyPaise: 800,
	}}
	if err := a.ValidateOpportunities(good); err != nil {
		t.Fatalf("unexpected error for valid opportunity: %v", err)
	}
}
