package erupeecompliance

import (
	"context"
	"fmt"
	"time"
)

// ComplianceEngine is the main integration point for all compliance checks.
// It orchestrates AML screening, velocity monitoring, and fraud detection.
type ComplianceEngine struct {
	amlScreener      *AMLScreener
	sanctionsChecker *SanctionsChecker
	velocityMonitor  VelocityMonitor
	fraudDetector    FraudDetector
	fraudConfig      *FraudDetectionConfig
}

// NewComplianceEngine creates a new compliance engine with default components.
func NewComplianceEngine() *ComplianceEngine {
	return &ComplianceEngine{
		amlScreener:      NewAMLScreener(),
		sanctionsChecker: NewSanctionsChecker(),
		velocityMonitor:  NewInMemoryVelocityMonitor(),
		fraudDetector:    NewInMemoryFraudDetector(),
		fraudConfig:      DefaultFraudDetectionConfig(),
	}
}

// NewComplianceEngineWithComponents creates a compliance engine with custom components.
func NewComplianceEngineWithComponents(
	amlScreener *AMLScreener,
	sanctionsChecker *SanctionsChecker,
	velocityMonitor VelocityMonitor,
	fraudDetector FraudDetector,
	fraudConfig *FraudDetectionConfig,
) *ComplianceEngine {
	return &ComplianceEngine{
		amlScreener:      amlScreener,
		sanctionsChecker: sanctionsChecker,
		velocityMonitor:  velocityMonitor,
		fraudDetector:    fraudDetector,
		fraudConfig:      fraudConfig,
	}
}

// CheckPayment performs comprehensive compliance checking on a payment.
// Returns a complete ComplianceCheck with all screening results.
func (e *ComplianceEngine) CheckPayment(ctx context.Context, payment PaymentRequest) (*ComplianceCheck, error) {
	check := &ComplianceCheck{
		PaymentID:        payment.PaymentID,
		AMLResult:        AMLPass,
		VelocityResult:   VelocityOK,
		FraudScore:       0.0,
		Decision:         DecisionAllow,
		DetectedPatterns: []FraudPattern{},
		CheckedAt:        time.Now().UTC(),
	}

	// Step 1: AML Screening
	amlCheck, err := e.amlScreener.CheckPayment(ctx, payment)
	if err != nil {
		return nil, fmt.Errorf("AML screening error: %w", err)
	}
	check.AMLResult = amlCheck.AMLResult
	check.AMLReason = amlCheck.AMLReason

	// Step 2: Sanctions Check
	if check.AMLResult != AMLBlock {
		blocked, reason := e.sanctionsChecker.CheckAccount(ctx, payment.ToAccountID, "")
		if blocked {
			check.AMLResult = AMLBlock
			check.AMLReason = reason
		}
	}

	// Step 3: Velocity Check
	velocityAllowed, velocityReason := e.velocityMonitor.CheckVelocity(ctx, payment.FromAccountID)
	if velocityAllowed {
		check.VelocityResult = VelocityOK
		check.VelocityReason = "Within velocity limits"
	} else {
		check.VelocityResult = VelocityBlocked
		check.VelocityReason = velocityReason
	}

	// Step 4: Fraud Detection
	// Get transaction history for the account
	history := e.fraudDetector.GetHistory(ctx, payment.FromAccountID, 72) // Last 72 hours
	patterns, fraudScore, fraudReason := e.fraudDetector.DetectPatterns(ctx, payment.FromAccountID, history)
	check.FraudScore = fraudScore
	check.DetectedPatterns = patterns
	check.FraudReason = fraudReason

	// Check for new account high value
	isNewHighValue, newAccScore, _ := CheckNewAccountHighValue(payment, e.fraudConfig)
	if isNewHighValue {
		check.DetectedPatterns = append(check.DetectedPatterns, PatternNewAccountHigh)
		check.FraudScore += newAccScore
		if check.FraudScore > 100.0 {
			check.FraudScore = 100.0
		}
	}

	// Step 5: Record the transaction for future fraud analysis
	txn := TransactionRecord{
		TransactionID: payment.PaymentID,
		FromAccountID: payment.FromAccountID,
		ToAccountID:   payment.ToAccountID,
		Amount:        payment.Amount,
		Timestamp:     payment.Timestamp,
	}
	_ = e.fraudDetector.RecordTransaction(ctx, txn)

	// Step 6: Record velocity metrics
	_ = e.velocityMonitor.RecordTransaction(ctx, payment.FromAccountID, payment.Amount, payment.Timestamp)

	// Step 7: Make final decision
	check.Decision = e.makeDecision(check)
	check.DecisionReason = e.makeDecisionReason(check)

	return check, nil
}

// makeDecision derives the final compliance decision from all checks.
func (e *ComplianceEngine) makeDecision(check *ComplianceCheck) Decision {
	// Block decision takes precedence
	if check.AMLResult == AMLBlock || check.VelocityResult == VelocityBlocked {
		return DecisionBlock
	}

	// Review decision if AML is under review or fraud risk is moderate
	if check.AMLResult == AMLReview || check.FraudScore >= 50.0 {
		return DecisionReview
	}

	// Warning-level velocity or high fraud score warrants review
	if check.VelocityResult == VelocityWarning || check.FraudScore >= 30.0 {
		return DecisionReview
	}

	return DecisionAllow
}

// makeDecisionReason generates a human-readable reason for the decision.
func (e *ComplianceEngine) makeDecisionReason(check *ComplianceCheck) string {
	switch check.Decision {
	case DecisionBlock:
		if check.AMLResult == AMLBlock {
			return fmt.Sprintf("Blocked: %s", check.AMLReason)
		}
		if check.VelocityResult == VelocityBlocked {
			return fmt.Sprintf("Blocked: %s", check.VelocityReason)
		}
		return "Blocked: Failed compliance checks"

	case DecisionReview:
		reasons := []string{}
		if check.AMLResult == AMLReview {
			reasons = append(reasons, fmt.Sprintf("AML review required: %s", check.AMLReason))
		}
		if check.VelocityResult == VelocityWarning {
			reasons = append(reasons, fmt.Sprintf("Velocity warning: %s", check.VelocityReason))
		}
		if check.FraudScore > 30.0 {
			reasons = append(reasons, fmt.Sprintf("Fraud risk (%.0f%%): %s", check.FraudScore, check.FraudReason))
		}
		if len(reasons) > 0 {
			return fmt.Sprintf("Review required: %v", reasons)
		}
		return "Review required: Multiple risk factors"

	default:
		return "Allowed: All compliance checks passed"
	}
}

// GetVelocityRecord returns the current velocity record for an account.
func (e *ComplianceEngine) GetVelocityRecord(ctx context.Context, accountID string) (*VelocityRecord, error) {
	return e.velocityMonitor.GetRecord(ctx, accountID)
}

// GetTransactionHistory returns recent transaction history for an account.
func (e *ComplianceEngine) GetTransactionHistory(ctx context.Context, accountID string, lookbackHours int) []TransactionRecord {
	return e.fraudDetector.GetHistory(ctx, accountID, lookbackHours)
}

// ResetVelocity resets velocity records for an account (admin operation).
func (e *ComplianceEngine) ResetVelocity(ctx context.Context, accountID string) error {
	return e.velocityMonitor.Reset(ctx, accountID)
}
