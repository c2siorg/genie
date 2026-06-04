package aml

import (
	"context"
	"time"
)

// BeneficiaryChecker defines the interface for beneficiary risk assessment.
//
// Implementations would typically query external services (OFAC, PEP databases,
// adverse media feeds). This reference implementation provides a mock checker.
type BeneficiaryChecker interface {
	CheckBeneficiary(ctx context.Context, beneficiary Transaction) (*BeneficiaryRiskInfo, error)
}

// MockBeneficiaryChecker is a test implementation that flags known high-risk patterns.
type MockBeneficiaryChecker struct{}

// CheckBeneficiary performs a mock check against known PEP/sanctioned patterns.
//
// In production, this would:
// - Query OFAC Consolidated Sanctions List (CSL)
// - Check UN Security Council lists
// - Search Transparency International's PEP database
// - Call adverse media aggregators
// - Verify against local/regional lists (e.g., FATF grey list)
//
// For now, we use hardcoded demo data to demonstrate the interface.
func (m *MockBeneficiaryChecker) CheckBeneficiary(ctx context.Context, txn Transaction) (*BeneficiaryRiskInfo, error) {
	now := time.Now().UTC()
	info := &BeneficiaryRiskInfo{
		CheckedAt: now,
	}

	// Simulate PEP check: hardcoded known names
	pepNames := map[string]bool{
		"Vladimir Putin": true,
		"Xi Jinping":     true,
		"Kim Jong Un":    true,
	}
	if pepNames[txn.BeneficiaryName] {
		info.IsPEP = true
		info.RiskScore += 40.0
	}

	// Simulate sanctions check: hardcoded known IDs / countries
	sanctionedCountries := map[string]bool{
		"KP": true, // North Korea
		"IR": true, // Iran
		"SY": true, // Syria
	}
	if sanctionedCountries[txn.BeneficiaryCountry] {
		info.IsSanctioned = true
		info.RiskScore += 50.0
	}

	// Simulate adverse media: check for known patterns in name
	adversePatterns := map[string]bool{
		"terrorist":       true,
		"money launderer": true,
		"drug lord":       true,
	}
	for pattern := range adversePatterns {
		// Simple substring check for demo
		if len(txn.BeneficiaryName) > 0 && len(pattern) > 0 {
			// In reality, would use fuzzy matching or ML-based similarity
			info.HasAdverseMedia = false // simplified for demo
		}
	}

	// Cap risk score at 100
	if info.RiskScore > 100 {
		info.RiskScore = 100
	}

	// Build rationale
	reasons := []string{}
	if info.IsPEP {
		reasons = append(reasons, "beneficiary is PEP")
	}
	if info.IsSanctioned {
		reasons = append(reasons, "beneficiary country is sanctioned")
	}
	if info.HasAdverseMedia {
		reasons = append(reasons, "adverse media flag")
	}
	if len(reasons) == 0 {
		info.Rationale = "no elevated risk indicators"
	} else {
		// Simple join for demo; production would use structured logging
		reasons = append(reasons, "")
		info.Rationale = "flagged: " + info.Rationale
	}

	return info, nil
}

// NewMockBeneficiaryChecker returns a demo checker for testing.
func NewMockBeneficiaryChecker() BeneficiaryChecker {
	return &MockBeneficiaryChecker{}
}
