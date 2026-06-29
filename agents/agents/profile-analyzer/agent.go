// Package profile_analyzer implements the ProfileAnalyzer agent.
// Decoupled from HTTP, storage, and framework-specific code.
// Can run anywhere: backend, frontend (WebWorker), cloud platforms, etc.
//
// Responsibility: Load user financial profile and determine risk tolerance based on KYC data.
// Input: UserID
// Output: UserProfile with risk tolerance, compliance constraints
//
// License: MIT
package profile_analyzer

import (
	"context"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	contractv1 "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/contract/v1"
)

// Agent implements the ProfileAnalyzer agent.
// Stateless - no internal state, all data passed via context/input.
type Agent struct {
	// No dependencies on HTTP, storage, or framework code
	// All external interactions via injected interfaces (see Executor pattern if needed)
}

// NewAgent creates a new ProfileAnalyzer agent.
func NewAgent() *Agent {
	return &Agent{}
}

// Name returns the agent identifier.
func (a *Agent) Name() string {
	return "profile-analyzer"
}

// Version returns the semantic version.
func (a *Agent) Version() string {
	return "1.0.0"
}

// Health checks if agent is ready (always true for stateless agent).
func (a *Agent) Health(ctx context.Context) error {
	return nil
}

// Execute runs the agent.
// Input: contractv1.ProfileAnalyzerInput (user_id at minimum)
// Output: *core.UserProfile
func (a *Agent) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	// Decode into the canonical typed input regardless of how it arrived
	// (typed struct in-process, map/raw JSON over HTTP).
	var req contractv1.ProfileAnalyzerInput
	if err := core.DecodeInput(input, &req); err != nil {
		return nil, core.NewExecutionError(
			a.Name(),
			core.ErrorCodeValidation,
			"invalid input: "+err.Error(),
			err,
		)
	}

	// Validate input
	if err := req.Validate(); err != nil {
		return nil, core.NewExecutionError(
			a.Name(),
			core.ErrorCodeValidation,
			err.Error(),
			err,
		)
	}

	// Analyze KYC status → risk tolerance
	riskTolerance := a.analyzeKYCStatus(req.KYCStatus)

	// Create profile
	profile := &core.UserProfile{
		UserID:                 req.UserID,
		RiskTolerance:          riskTolerance,
		AnnualIncomePaise:      req.AnnualIncomePaise,
		SavingsGoalPaise:       0,                       // Would load from user settings
		InvestmentExperience:   core.ExperienceBeginner, // Would analyze from transaction history
		ComplianceRestrictions: a.extractComplianceConstraints(req.KYCStatus),
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	return profile, nil
}

// analyzeKYCStatus determines risk tolerance based on KYC status.
func (a *Agent) analyzeKYCStatus(kycStatus string) core.RiskTolerance {
	switch kycStatus {
	case "verified_high_value":
		return core.RiskAggressive
	case "verified_standard":
		return core.RiskModerate
	case "pending", "new", "":
		return core.RiskConservative
	default:
		return core.RiskConservative
	}
}

// extractComplianceConstraints identifies restrictions based on KYC status.
func (a *Agent) extractComplianceConstraints(kycStatus string) []string {
	// AML flags: new/pending KYC cannot invest offshore
	if kycStatus == "pending" || kycStatus == "new" {
		return []string{"cannot_invest_offshore", "cannot_use_derivatives"}
	}

	// Standard verified: minor restrictions
	if kycStatus == "verified_standard" {
		return []string{"cannot_use_leverage"}
	}

	// High-value verified: minimal restrictions
	return []string{}
}

// AnalyzeKYCStatus is exported for direct calls (testing, embedding).
func (a *Agent) AnalyzeKYCStatus(kycStatus string) core.RiskTolerance {
	return a.analyzeKYCStatus(kycStatus)
}

// ExtractComplianceConstraints is exported for direct calls.
func (a *Agent) ExtractComplianceConstraints(kycStatus string) []string {
	return a.extractComplianceConstraints(kycStatus)
}
