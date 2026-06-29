package advisor

import (
	"context"
	"testing"
)

func TestFinancialAnalyst_Analyze_ValidProfile(t *testing.T) {
	fa := NewFinancialAnalyst()
	ctx := context.Background()
	profile := &UserProfile{
		UserID:           "user-123",
		RiskTolerance:    RiskModerate,
		AnnualIncomePaise: 5000000, // ₹50L/year
	}

	opps, err := fa.Analyze(ctx, profile)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opps == nil {
		t.Fatal("opportunities should not be nil")
	}
}

func TestFinancialAnalyst_Analyze_NilProfile(t *testing.T) {
	fa := NewFinancialAnalyst()
	ctx := context.Background()

	opps, err := fa.Analyze(ctx, nil)

	if err == nil {
		t.Fatal("expected error for nil profile")
	}
	if opps != nil {
		t.Fatal("opportunities should be nil on error")
	}
}

func TestFinancialAnalyst_Analyze_EmptyUserID(t *testing.T) {
	fa := NewFinancialAnalyst()
	ctx := context.Background()
	profile := &UserProfile{UserID: ""}

	opps, err := fa.Analyze(ctx, profile)

	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
	if opps != nil {
		t.Fatal("opportunities should be nil on error")
	}
}

func TestFinancialAnalyst_AnalyzeSpendingCategory_Valid(t *testing.T) {
	fa := NewFinancialAnalyst()
	ctx := context.Background()

	opp, err := fa.AnalyzeSpendingCategory(ctx, "user-123", "food")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opp == nil {
		t.Fatal("opportunity should not be nil")
	}
	if opp.Category != "food" {
		t.Errorf("got category %s, want food", opp.Category)
	}
}

func TestFinancialAnalyst_AnalyzeSpendingCategory_EmptyUserID(t *testing.T) {
	fa := NewFinancialAnalyst()
	ctx := context.Background()

	opp, err := fa.AnalyzeSpendingCategory(ctx, "", "food")

	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
	if opp != nil {
		t.Fatal("opportunity should be nil on error")
	}
}

func TestFinancialAnalyst_AnalyzeSpendingCategory_EmptyCategory(t *testing.T) {
	fa := NewFinancialAnalyst()
	ctx := context.Background()

	opp, err := fa.AnalyzeSpendingCategory(ctx, "user-123", "")

	if err == nil {
		t.Fatal("expected error for empty category")
	}
	if opp != nil {
		t.Fatal("opportunity should be nil on error")
	}
}

func TestFinancialAnalyst_ComputeTrend_Stable(t *testing.T) {
	fa := NewFinancialAnalyst()
	monthlySpend := []int64{100000, 105000, 98000, 102000, 101000, 100000} // Relatively stable

	trend := fa.ComputeTrend(monthlySpend)

	if trend == "" {
		t.Fatal("trend should not be empty")
	}
	// Placeholder implementation returns "stable"
	if trend != "stable" {
		t.Errorf("got trend %s, expected stable for flat spending", trend)
	}
}

func TestFinancialAnalyst_ComputeTrend_InsufficientData(t *testing.T) {
	fa := NewFinancialAnalyst()
	monthlySpend := []int64{100000, 105000} // Only 2 months

	trend := fa.ComputeTrend(monthlySpend)

	if trend != "insufficient_data" {
		t.Errorf("got trend %s, want insufficient_data for < 3 months", trend)
	}
}

func TestFinancialAnalyst_ComputeBenchmarkPercentile_Middle(t *testing.T) {
	fa := NewFinancialAnalyst()
	userSpend := int64(100000)
	peerSpends := []int64{50000, 75000, 100000, 125000, 150000} // User in middle

	percentile := fa.ComputeBenchmarkPercentile(userSpend, peerSpends)

	if percentile < 0 || percentile > 100 {
		t.Errorf("got percentile %d, want 0-100", percentile)
	}
	// Placeholder returns 50
	if percentile != 50 {
		t.Errorf("got percentile %d, expected 50 for middle spending", percentile)
	}
}

func TestFinancialAnalyst_ComputeBenchmarkPercentile_NoData(t *testing.T) {
	fa := NewFinancialAnalyst()
	userSpend := int64(100000)
	peerSpends := []int64{} // No peer data

	percentile := fa.ComputeBenchmarkPercentile(userSpend, peerSpends)

	if percentile != 50 {
		t.Errorf("got percentile %d, want 50 (default) with no peer data", percentile)
	}
}

func TestFinancialAnalyst_ValidateOpportunities_Valid(t *testing.T) {
	fa := NewFinancialAnalyst()
	opps := []SpendingOpportunity{
		{
			ID:                    "opp-1",
			Category:              "food",
			CurrentMonthlyPaise:   50000,
			OptimizedMonthlyPaise: 40000,
			Confidence:            0.85,
		},
	}

	err := fa.ValidateOpportunities(opps)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinancialAnalyst_ValidateOpportunities_MissingID(t *testing.T) {
	fa := NewFinancialAnalyst()
	opps := []SpendingOpportunity{
		{
			ID:       "", // Missing!
			Category: "food",
		},
	}

	err := fa.ValidateOpportunities(opps)

	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestFinancialAnalyst_ValidateOpportunities_InvalidConfidence(t *testing.T) {
	fa := NewFinancialAnalyst()
	opps := []SpendingOpportunity{
		{
			ID:         "opp-1",
			Category:   "food",
			Confidence: 1.5, // Invalid! Should be 0.0-1.0
		},
	}

	err := fa.ValidateOpportunities(opps)

	if err == nil {
		t.Fatal("expected error for invalid confidence")
	}
}

func TestFinancialAnalyst_ValidateOpportunities_OptimizedGreaterThanCurrent(t *testing.T) {
	fa := NewFinancialAnalyst()
	opps := []SpendingOpportunity{
		{
			ID:                    "opp-1",
			Category:              "food",
			CurrentMonthlyPaise:   40000,
			OptimizedMonthlyPaise: 50000, // Optimized should be less than current!
			Confidence:            0.85,
		},
	}

	err := fa.ValidateOpportunities(opps)

	if err == nil {
		t.Fatal("expected error when optimized > current")
	}
}
