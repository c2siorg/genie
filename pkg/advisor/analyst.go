package advisor

import (
	"context"
	"fmt"
	"time"
)

// FinancialAnalyst analyzes user transaction history to identify spending patterns and opportunities.
// Part of Phase 7 recommendation pipeline.
// Receives ProfileAnalyzer output, produces opportunities for RecommendationGenerator.
type FinancialAnalyst struct {
	// Add storage, logging, etc. as needed
}

// NewFinancialAnalyst creates a new financial analyst.
func NewFinancialAnalyst() *FinancialAnalyst {
	return &FinancialAnalyst{}
}

// SpendingOpportunity represents a potential area for financial improvement.
type SpendingOpportunity struct {
	ID                      string    `json:"id"`
	Category                string    `json:"category"` // spending category where opportunity exists
	CurrentMonthlyPaise     int64     `json:"current_monthly_paise"`
	OptimizedMonthlyPaise   int64     `json:"optimized_monthly_paise"`
	MonthlySavingsPotential int64     `json:"monthly_savings_potential"` // current - optimized
	Confidence              float64   `json:"confidence"`                 // 0.0-1.0
	PrimaryDriver           string    `json:"primary_driver"`             // what's causing overspend?
	BenchmarkPercentile     int       `json:"benchmark_percentile"`       // user vs peers (0-100)
	Trend                   string    `json:"trend"`                      // "increasing", "stable", "decreasing"
	AnalyzedAt              time.Time `json:"analyzed_at"`
}

// Analyze examines user's transaction history to find spending opportunities.
// Input: UserProfile with spending history
// Output: SpendingOpportunity array for RecommendationGenerator
func (fa *FinancialAnalyst) Analyze(ctx context.Context, profile *UserProfile) ([]SpendingOpportunity, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile required")
	}
	if profile.UserID == "" {
		return nil, fmt.Errorf("user_id required")
	}

	// TODO: Load from Phase 2 (Commerce) - transaction history for user
	// TODO: Group by category (food, transport, entertainment, utilities, etc.)
	// TODO: Calculate monthly spend per category
	// TODO: Compute trends (is category spend increasing? stable? decreasing?)
	// TODO: Compare to peer benchmarks (via external data or stored baseline)
	// TODO: Identify outliers (categories where user spends significantly more than peers)
	// TODO: Return top 3-5 opportunities (highest savings potential)

	opportunities := []SpendingOpportunity{}
	// Placeholder logic - will be filled with real transaction analysis

	return opportunities, nil
}

// AnalyzeSpendingCategory examines a single spending category in detail.
// Returns current spend, peer average, and savings potential.
func (fa *FinancialAnalyst) AnalyzeSpendingCategory(ctx context.Context, userID string, category string) (*SpendingOpportunity, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id required")
	}
	if category == "" {
		return nil, fmt.Errorf("category required")
	}

	// TODO: Query Commerce API for user's transactions in this category
	// TODO: Calculate user's average monthly spend
	// TODO: Fetch peer benchmark for this category (by income bracket, age, location)
	// TODO: If user > peer average + 1 SD, flag as opportunity
	// TODO: Compute savings potential (user spend - (peer average + 20% buffer))
	// TODO: Analyze trend (is it getting worse over time?)

	opportunity := &SpendingOpportunity{
		ID:                  fmt.Sprintf("opp-%s", category),
		Category:            category,
		CurrentMonthlyPaise: 0, // Will be loaded from data
		Confidence:          0.0,
		Trend:               "stable",
		AnalyzedAt:          time.Now(),
	}

	return opportunity, nil
}

// ComputeTrend analyzes spending direction over time.
// Input: last 12 months of spending in a category
// Output: "increasing" if growing, "decreasing" if shrinking, "stable" otherwise
func (fa *FinancialAnalyst) ComputeTrend(monthlySpend []int64) string {
	if len(monthlySpend) < 3 {
		return "insufficient_data"
	}

	// TODO: Linear regression on spending over time
	// TODO: If slope > 2% per month: "increasing"
	// TODO: If slope < -2% per month: "decreasing"
	// TODO: Otherwise: "stable"

	// Placeholder
	return "stable"
}

// ComputeBenchmarkPercentile compares user's spending to peer segment.
// Input: user's category spend, peer distribution
// Output: percentile (0=lowest spender, 100=highest spender)
func (fa *FinancialAnalyst) ComputeBenchmarkPercentile(userSpend int64, peerSpends []int64) int {
	if len(peerSpends) == 0 {
		return 50 // Default to median if no peer data
	}

	// TODO: Count how many peers spend less than user
	// TODO: percentile = (count_less_than_user / total_peers) * 100

	// Placeholder
	return 50
}

// ValidateOpportunities ensures opportunities are reasonable.
func (fa *FinancialAnalyst) ValidateOpportunities(opps []SpendingOpportunity) error {
	for i, opp := range opps {
		if opp.ID == "" {
			return fmt.Errorf("opportunity %d: id required", i)
		}
		if opp.Category == "" {
			return fmt.Errorf("opportunity %d: category required", i)
		}
		if opp.Confidence < 0 || opp.Confidence > 1 {
			return fmt.Errorf("opportunity %d: confidence must be 0.0-1.0", i)
		}
		if opp.CurrentMonthlyPaise < opp.OptimizedMonthlyPaise {
			return fmt.Errorf("opportunity %d: optimized must be <= current", i)
		}
	}
	return nil
}
