package kyc

import (
	"context"
	"time"

	"github.com/c2siorg/genie/pkg/kyc"
)

// This file defines the sub-agent interfaces that OnboardingAgentSupervisor
// orchestrates. They are kept as interfaces (rather than concrete types) so the
// supervisor can be unit-tested with mocks and so each sub-agent can be swapped
// independently. The concrete agents in this package
// (DocumentProcessorAgent, IdentityVerifierAgent, SanctionsCheckerAgent)
// satisfy the first three interfaces.

// DocumentProcessor ingests a raw document, stores it securely, runs OCR, and
// returns the extracted, structured document.
type DocumentProcessor interface {
	ProcessDocument(ctx context.Context, docType string, rawFileBytes []byte) (*kyc.Document, error)
}

// IdentityVerifier validates extracted identity fields against form data and
// returns field-level and overall match confidence.
type IdentityVerifier interface {
	VerifyIdentity(ctx context.Context, extractedText string, formData map[string]any, docType string) (*VerificationResult, error)
}

// SanctionsChecker screens an applicant's name/DOB/jurisdiction against
// configured watchlists (OFAC SDN, UN, MHA, etc.).
type SanctionsChecker interface {
	CheckSanctions(ctx context.Context, name string, dob *time.Time, jurisdiction string) (*kyc.SanctionsResult, error)
}

// OnboardingApprover evaluates the approval policy for a completed onboarding
// request and returns the policy decision (auto-approve / manual-review / flag).
type OnboardingApprover interface {
	EvaluateApproval(ctx context.Context, request *kyc.OnboardingRequest) (*kyc.ApprovalPolicyResult, error)
}
