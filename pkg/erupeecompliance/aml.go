package erupeecompliance

import (
	"context"
	"fmt"
)

// ComplianceChecker defines the interface for payment compliance screening.
type ComplianceChecker interface {
	// CheckPayment performs AML screening and returns the compliance result.
	CheckPayment(ctx context.Context, payment PaymentRequest) (*ComplianceCheck, error)
}

// AMLScreener performs anti-money laundering screening on payments.
type AMLScreener struct {
	// In production, would integrate with pkg/aml.RiskScore
	// For now, we use simplified mock checking
	knownPEPs      map[string]bool
	sanctionedList map[string]bool
	adverseMedia   map[string]bool
}

// NewAMLScreener creates a new AML screening instance.
func NewAMLScreener() *AMLScreener {
	return &AMLScreener{
		knownPEPs: map[string]bool{
			"Vladimir Putin":     true,
			"Xi Jinping":         true,
			"Kim Jong Un":        true,
			"Nicolás Maduro":     true,
			"Bashar al-Assad":    true,
		},
		sanctionedList: map[string]bool{
			"KP": true, // North Korea
			"IR": true, // Iran
			"SY": true, // Syria
			"CU": true, // Cuba
			"MM": true, // Myanmar
		},
		adverseMedia: map[string]bool{
			"terrorist": true,
			"money launderer": true,
			"drug lord": true,
		},
	}
}

// CheckPayment performs AML screening on a payment request.
func (a *AMLScreener) CheckPayment(ctx context.Context, payment PaymentRequest) (*ComplianceCheck, error) {
	check := &ComplianceCheck{
		PaymentID:        payment.PaymentID,
		AMLResult:        AMLPass,
		VelocityResult:   VelocityOK,
		Decision:         DecisionAllow,
		DetectedPatterns: []FraudPattern{},
	}

	// Perform AML screening
	amlResult, amlReason := a.screenAML(ctx, payment)
	check.AMLResult = amlResult
	check.AMLReason = amlReason

	// If AML blocks, set decision accordingly
	if amlResult == AMLBlock {
		check.Decision = DecisionBlock
		check.DecisionReason = fmt.Sprintf("AML screening blocked: %s", amlReason)
		return check, nil
	}

	// If AML review required, escalate decision
	if amlResult == AMLReview {
		check.Decision = DecisionReview
		check.DecisionReason = fmt.Sprintf("AML screening requires review: %s", amlReason)
	}

	return check, nil
}

// screenAML performs the actual AML screening logic.
// Returns (AMLResult, reason).
func (a *AMLScreener) screenAML(ctx context.Context, payment PaymentRequest) (AMLResult, string) {
	// Check beneficiary against PEP list
	if a.knownPEPs[payment.ToName] {
		return AMLBlock, fmt.Sprintf("Beneficiary '%s' is on PEP list", payment.ToName)
	}

	// Check if amount exceeds a high-risk threshold for new beneficiaries
	// This would integrate with pkg/aml for full scoring
	if payment.Amount > 50_00_000 { // ₹50k
		return AMLReview, fmt.Sprintf("Large payment (%.2f INR) requires AML review", float64(payment.Amount)/100.0)
	}

	// If no issues found, pass
	return AMLPass, "AML screening passed"
}

// SanctionsChecker performs sanctions list checking.
type SanctionsChecker struct {
	sanctionedCountries map[string]bool
	sanctionedAccounts  map[string]bool
}

// NewSanctionsChecker creates a new sanctions checking instance.
func NewSanctionsChecker() *SanctionsChecker {
	return &SanctionsChecker{
		sanctionedCountries: map[string]bool{
			"KP": true, // North Korea
			"IR": true, // Iran
			"SY": true, // Syria
			"CU": true, // Cuba
			"MM": true, // Myanmar
		},
		sanctionedAccounts: map[string]bool{},
	}
}

// CheckAccount checks if an account/beneficiary is on any sanctions list.
// Returns (isBlocked, reason).
func (s *SanctionsChecker) CheckAccount(ctx context.Context, accountID, country string) (bool, string) {
	// Check if account is directly sanctioned
	if s.sanctionedAccounts[accountID] {
		return true, fmt.Sprintf("Account %s is on OFAC/UN sanctions list", accountID)
	}

	// Check if country is sanctioned
	if s.sanctionedCountries[country] {
		return true, fmt.Sprintf("Country %s is subject to sanctions", country)
	}

	return false, ""
}
