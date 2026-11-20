package kyc

import (
	"context"
	"time"

	"github.com/c2siorg/genie/pkg/compliance"
)

// IdentityVerifierAgent validates extracted identity fields against expected patterns.
type IdentityVerifierAgent struct {
	biometric BiometricVerifier
	address   AddressValidator
	auditLog  compliance.AuditLog
}

// VerifyIdentity checks name/DOB/address consistency.
// Returns field-level match confidence and an overall confidence score.
func (a *IdentityVerifierAgent) VerifyIdentity(
	ctx context.Context,
	extractedText string,
	formData map[string]any,
	docType string,
) (*VerificationResult, error) {
	// TODO: implement
	panic("not implemented")
}

// BiometricVerifier defines the interface for biometric identity verification.
type BiometricVerifier interface {
	// VerifyBiometric performs biometric verification (e.g., V-CIP, Aadhaar).
	VerifyBiometric(ctx context.Context, docType string, rawBytes []byte) (bool, float64, error)
}

// AddressValidator defines the interface for address validation.
type AddressValidator interface {
	// ValidateAddress checks address consistency against postal databases.
	ValidateAddress(ctx context.Context, address string, postcode string) (bool, float64, error)
}

// VerificationResult holds the output of identity verification.
type VerificationResult struct {
	RequestID          string
	FieldMatches       FieldMatchDetails
	OverallConfidence  float64
	VerificationMethod string
	VerifiedAt         time.Time
	VerifierID         string
	Issues             []string
}

// FieldMatchDetails breaks down match results per field.
type FieldMatchDetails struct {
	Name     MatchResult
	DOB      MatchResult
	Address  MatchResult
	IDNumber MatchResult
}

// MatchResult represents the result of matching a single field.
type MatchResult struct {
	IsMatch    bool
	Confidence float64
	Extracted  string
	Expected   string
	Note       string
}
