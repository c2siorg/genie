// Package financial_analyst implements the FinancialAnalyst agent.
// Decoupled from HTTP, storage, and framework-specific code — runs anywhere.
//
// Responsibility: analyze a user's financial profile / transaction history to
// identify spending opportunities (categories where the user could optimize).
// Input:  contractv1.FinancialAnalystInput (a UserProfile)
// Output: []contractv1.SpendingOpportunity
//
// Migrated from pkg/advisor/analyst.go (FinancialAnalyst). Business logic is
// preserved as-is; only the interface shape changed to agents/core.Agent.
//
// License: MIT
package financial_analyst

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
)

// Agent implements the FinancialAnalyst agent. Stateless.
type Agent struct{}

// NewAgent creates a new FinancialAnalyst agent.
func NewAgent() *Agent { return &Agent{} }

// Name returns the agent identifier.
func (a *Agent) Name() string { return "financial-analyst" }

// Version returns the semantic version.
func (a *Agent) Version() string { return "1.0.0" }

// Health checks readiness (always healthy for a stateless agent).
func (a *Agent) Health(ctx context.Context) error { return nil }

// Execute decodes a FinancialAnalystInput, validates it, and returns the
// spending opportunities found for the profile.
func (a *Agent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	var req contractv1.FinancialAnalystInput
	if err := core.DecodeInput(input, &req); err != nil {
		return nil, core.NewExecutionError(a.Name(), core.ErrorCodeValidation, "invalid input: "+err.Error(), err)
	}
	if err := req.Validate(); err != nil {
		return nil, core.NewExecutionError(a.Name(), core.ErrorCodeValidation, err.Error(), err)
	}

	opportunities, err := a.Analyze(ctx, &req.Profile)
	if err != nil {
		return nil, core.NewExecutionError(a.Name(), core.ErrorCodeInternal, "analyze failed", err)
	}
	return opportunities, nil
}

// Analyze examines a user's profile/transaction history to find spending
// opportunities. Ported from advisor.FinancialAnalyst.Analyze — the data-source
// integration (Commerce transaction history, peer benchmarks) remains a TODO, so
// it currently returns no opportunities for a valid profile rather than
// fabricating data.
func (a *Agent) Analyze(ctx context.Context, profile *contractv1.UserProfile) ([]contractv1.SpendingOpportunity, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile required")
	}
	if profile.UserID == "" {
		return nil, fmt.Errorf("user_id required")
	}

	// TODO: Load transaction history (Phase 2 Commerce), group by category,
	// compute monthly spend + trends, compare to peer benchmarks, return the
	// top opportunities by savings potential.
	opportunities := []contractv1.SpendingOpportunity{}
	return opportunities, nil
}

// AnalyzeSpendingCategory examines a single spending category in detail.
// Ported from advisor.FinancialAnalyst.AnalyzeSpendingCategory.
func (a *Agent) AnalyzeSpendingCategory(ctx context.Context, userID, category string) (*contractv1.SpendingOpportunity, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id required")
	}
	if category == "" {
		return nil, fmt.Errorf("category required")
	}
	return &contractv1.SpendingOpportunity{
		ID:                  fmt.Sprintf("opp-%s", category),
		Category:            category,
		CurrentMonthlyPaise: 0,
		Confidence:          0.0,
		Trend:               "stable",
		AnalyzedAt:          time.Now(),
	}, nil
}

// ComputeTrend analyzes spending direction over time. Returns "increasing",
// "decreasing", "stable", or "insufficient_data". Ported verbatim.
func (a *Agent) ComputeTrend(monthlySpend []int64) string {
	if len(monthlySpend) < 3 {
		return "insufficient_data"
	}
	// TODO: linear regression on spend over time; slope thresholds for
	// increasing/decreasing. Placeholder returns stable.
	return "stable"
}

// ComputeBenchmarkPercentile compares a user's spend to a peer segment.
// Returns 0 (lowest) to 100 (highest); 50 if no peer data. Ported verbatim.
func (a *Agent) ComputeBenchmarkPercentile(userSpend int64, peerSpends []int64) int {
	if len(peerSpends) == 0 {
		return 50
	}
	// TODO: percentile = (peers spending less than user / total) * 100.
	return 50
}

// ValidateOpportunities ensures opportunities are well-formed. Ported verbatim.
func (a *Agent) ValidateOpportunities(opps []contractv1.SpendingOpportunity) error {
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
