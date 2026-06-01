package kyc

import "context"

// ApprovalPolicy defines the rules for auto-approval, manual review, and auto-flag decisions.
type ApprovalPolicy interface {
	// EvaluateApprovalPolicy applies policy rules to an onboarding request and returns the decision.
	EvaluateApprovalPolicy(ctx context.Context, request *OnboardingRequest) (*ApprovalPolicyResult, error)
	// ComputeRiskScore calculates a synthetic risk score for the request.
	ComputeRiskScore(req *OnboardingRequest) float64
	// IsHighRiskJurisdiction checks if a jurisdiction is high-risk per policy.
	IsHighRiskJurisdiction(jurisdiction string) bool
	// IsHighRiskOccupation checks if an occupation is high-risk per FATF guidance.
	IsHighRiskOccupation(occupationCode string) bool
}

// EvaluateApprovalPolicy applies policy rules to an onboarding request.
// Returns the recommended action (auto_approve, manual_review, or auto_flag).
func EvaluateApprovalPolicy(
	ctx context.Context,
	request *OnboardingRequest,
	policy ApprovalPolicy,
) (*ApprovalPolicyResult, error) {
	// TODO: implement
	panic("not implemented")
}

// ComputeRiskScore calculates a synthetic risk score based on all verifications.
// Returns 0.0 (lowest risk) to 1.0 (highest risk).
// Factors: identity confidence (30%), sanctions hit (100% if hit), jurisdiction risk (20%),
// high-risk occupation (10%), and document issues (15%).
func ComputeRiskScore(req *OnboardingRequest) float64 {
	// TODO: implement
	panic("not implemented")
}

// IsHighRiskJurisdiction checks if a jurisdiction is high-risk per policy YAML.
// High-risk jurisdictions (Tier C) typically include FATF grey-list and higher-risk countries.
func IsHighRiskJurisdiction(jurisdiction string) bool {
	// TODO: implement
	panic("not implemented")
}

// IsHighRiskOccupation checks if an occupation code is high-risk per FATF guidance.
// Examples: lawyers, accountants, dealers in precious metals/stones, real estate agents.
func IsHighRiskOccupation(occupationCode string) bool {
	// TODO: implement
	panic("not implemented")
}
