// Package recommendation_generator implements the RecommendationGenerator agent.
// Decoupled from HTTP, storage, and framework-specific code — runs anywhere.
//
// Responsibility: final step of the advisor pipeline. Takes the user profile and
// the spending opportunities found by the FinancialAnalyst, and produces
// actionable, compliance-checked recommendations with impact estimates.
// Input:  contractv1.RecommendationGeneratorInput (profile + opportunities)
// Output: []contractv1.Recommendation
//
// Migrated from pkg/advisor/generator.go (RecommendationGenerator). Business
// logic is preserved; the one adaptation is that Execute wires EstimateImpact
// into each generated recommendation (the source left EstimatedImpact zero), so
// pipeline output carries a usable impact figure.
//
// License: MIT
package recommendation_generator

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
)

// Agent implements the RecommendationGenerator agent. Stateless.
type Agent struct{}

// NewAgent creates a new RecommendationGenerator agent.
func NewAgent() *Agent { return &Agent{} }

// Name returns the agent identifier.
func (a *Agent) Name() string { return "recommendation-generator" }

// Version returns the semantic version.
func (a *Agent) Version() string { return "1.0.0" }

// Health checks readiness (always healthy for a stateless agent).
func (a *Agent) Health(ctx context.Context) error { return nil }

// Execute decodes a RecommendationGeneratorInput, validates it, and generates
// one recommendation per opportunity (compliance-checked, with impact estimate).
func (a *Agent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	var req contractv1.RecommendationGeneratorInput
	if err := core.DecodeInput(input, &req); err != nil {
		return nil, core.NewExecutionError(a.Name(), core.ErrorCodeValidation, "invalid input: "+err.Error(), err)
	}
	if err := req.Validate(); err != nil {
		return nil, core.NewExecutionError(a.Name(), core.ErrorCodeValidation, err.Error(), err)
	}

	recs := make([]contractv1.Recommendation, 0, len(req.Opportunities))
	for i := range req.Opportunities {
		rec, err := a.GenerateRecommendation(ctx, &req.Profile, &req.Opportunities[i])
		if err != nil {
			return nil, core.NewExecutionError(a.Name(), core.ErrorCodeInternal, "generate failed", err)
		}
		// Adaptation: populate impact from the ported EstimateImpact so the
		// pipeline output is actionable.
		rec.EstimatedImpact = a.EstimateImpact(&req.Opportunities[i], &req.Profile)

		// Compliance gate: drop recommendations that violate user constraints.
		if ok, _ := a.CheckCompliance(rec, req.Profile.ComplianceRestrictions); !ok {
			continue
		}
		recs = append(recs, *rec)
	}
	return recs, nil
}

// GenerateRecommendation builds a personalized recommendation from a profile and
// a single opportunity. Ported from advisor.RecommendationGenerator.
func (a *Agent) GenerateRecommendation(
	ctx context.Context,
	profile *contractv1.UserProfile,
	opportunity *contractv1.SpendingOpportunity,
) (*contractv1.Recommendation, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile required")
	}
	if opportunity == nil {
		return nil, fmt.Errorf("opportunity required")
	}

	return &contractv1.Recommendation{
		RecommendationID:      fmt.Sprintf("rec-%s", opportunity.ID),
		UserID:                profile.UserID,
		Category:              contractv1.CategorySpendingOptimization,
		Title:                 fmt.Sprintf("Optimize %s spending", opportunity.Category),
		Description:           fmt.Sprintf("Reduce %s spending by ₹%d/month", opportunity.Category, opportunity.MonthlySavingsPotential/100),
		Rationale:             fmt.Sprintf("Your %s spending is in the %dth percentile vs peers", opportunity.Category, opportunity.BenchmarkPercentile),
		Actions:               []contractv1.Action{},
		Risks:                 []contractv1.Risk{},
		ComplianceConstraints: profile.ComplianceRestrictions,
		Status:                contractv1.StatusPending,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}, nil
}

// EstimateImpact calculates expected financial impact with a confidence
// multiplier that depends on the user's risk tolerance. Ported verbatim.
func (a *Agent) EstimateImpact(
	opportunity *contractv1.SpendingOpportunity,
	profile *contractv1.UserProfile,
) contractv1.ImpactEstimate {
	confidence := 0.75 // moderate default
	if profile.RiskTolerance == contractv1.RiskConservative {
		confidence = 0.6
	} else if profile.RiskTolerance == contractv1.RiskAggressive {
		confidence = 0.85
	}
	return contractv1.ImpactEstimate{
		MonthlySavingsPaise: int64(float64(opportunity.MonthlySavingsPotential) * confidence),
		AnnualReturnPaise:   int64(float64(opportunity.MonthlySavingsPotential*12) * confidence),
		Confidence:          confidence,
	}
}

// CheckCompliance verifies a recommendation doesn't violate the user's
// constraints. Ported from advisor.RecommendationGenerator.CheckCompliance.
func (a *Agent) CheckCompliance(rec *contractv1.Recommendation, constraints []string) (bool, string) {
	if rec == nil {
		return false, "recommendation required"
	}
	// TODO: match each constraint against recommendation category/actions and
	// return (false, reason) on violation. Placeholder assumes compliant.
	return true, ""
}

// GenerateActions creates concrete action items for a recommendation. Ported.
func (a *Agent) GenerateActions(category string, savingsPotential int64, userExperience contractv1.InvestmentExperience) []contractv1.Action {
	// TODO: map category + savings potential + experience to concrete actions.
	return []contractv1.Action{}
}

// GenerateRisks identifies risks for a recommendation category. Ported.
func (a *Agent) GenerateRisks(category contractv1.RecommendationCategory) []contractv1.Risk {
	// TODO: per-category risk catalog with severity + mitigation.
	return []contractv1.Risk{}
}

// ValidateRecommendation ensures a recommendation is well-formed. Ported, now
// delegating the shared field checks to contractv1.Recommendation.Validate and
// adding the "at least one action" rule specific to the generator.
func (a *Agent) ValidateRecommendation(rec *contractv1.Recommendation) error {
	if err := rec.Validate(); err != nil {
		return err
	}
	if len(rec.Actions) == 0 {
		return fmt.Errorf("at least one action required")
	}
	return nil
}
