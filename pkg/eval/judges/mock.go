package judges

import (
	"context"
	"fmt"
	"time"
)

// MockSettlementJudge is a test double for SettlementJudge that uses deterministic logic.
type MockSettlementJudge struct {
	rubricID   string
	rubricName string
}

// NewMockSettlementJudge creates a new mock settlement judge.
func NewMockSettlementJudge() *MockSettlementJudge {
	return &MockSettlementJudge{
		rubricID:   "RB-SE-003",
		rubricName: "Settlement Evaluation",
	}
}

// Name returns the judge's name.
func (msj *MockSettlementJudge) Name() string {
	return "MockSettlementJudge"
}

// Evaluate assesses a settlement using deterministic rules (no LLM calls).
func (msj *MockSettlementJudge) Evaluate(ctx context.Context, input SettlementJudgeInput) (Verdict, error) {
	var score float64
	var pass bool
	var reason string
	var evidence string

	// Rule 1: Amount must match (100% match = high score)
	amountCorrect := input.SettlementAmount == input.ExpectedAmount
	if !amountCorrect {
		score -= 0.3
		reason = fmt.Sprintf("Amount mismatch: settlement=%d, expected=%d", input.SettlementAmount, input.ExpectedAmount)
	} else {
		score += 0.3
		evidence = fmt.Sprintf("Amount correct: %d paise", input.SettlementAmount)
	}

	// Rule 2: CBDC must be committed
	if !input.CBDCCommitted {
		score -= 0.3
		reason += "; CBDC not committed"
	} else {
		score += 0.3
		evidence += "; CBDC committed"
	}

	// Rule 3: Reconciliation must pass
	if !input.ReconciliationPassed {
		score -= 0.2
		reason += "; reconciliation failed"
	} else {
		score += 0.2
		evidence += "; reconciliation passed"
	}

	// Rule 4: Netting consistency (if netting applied, nettedAmount must be positive)
	if input.NettingApplied && input.NettedAmount <= 0 {
		score -= 0.1
		reason += "; invalid netting"
	} else if input.NettingApplied {
		score += 0.1
		evidence += "; netting applied correctly"
	}

	// Clamp score to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	// Determine pass threshold at 0.6
	pass = score >= 0.6

	return Verdict{
		Pass:        pass,
		Score:       score,
		RubricID:    msj.rubricID,
		RubricName:  msj.rubricName,
		Reason:      reason,
		Evidence:    evidence,
		EvaluatedAt: time.Now(),
	}, nil
}

// Calibrate runs calibration on ground truth samples.
func (msj *MockSettlementJudge) Calibrate(ctx context.Context, samples []SettlementJudgeInput, groundTruth []bool) (CalibrationResult, error) {
	if len(samples) != len(groundTruth) {
		return CalibrationResult{}, fmt.Errorf("samples and groundTruth length mismatch")
	}

	result := CalibrationResult{
		CalibratedAt: time.Now(),
	}

	// Evaluate each sample
	var verdicts []bool
	var scores []float64
	calibrationSamples := make([]CalibrationSample, len(samples))

	for i, sample := range samples {
		verdict, err := msj.Evaluate(ctx, sample)
		if err != nil {
			return result, fmt.Errorf("evaluate sample %d: %w", i, err)
		}

		verdicts = append(verdicts, verdict.Pass)
		scores = append(scores, verdict.Score)
		calibrationSamples[i] = CalibrationSample{
			ID:          fmt.Sprintf("settlement-%d", i),
			GroundTruth: groundTruth[i],
			Verdict:     verdict.Pass,
			Score:       verdict.Score,
			Reason:      verdict.Reason,
		}
	}

	// Compute TPR and TNR
	result.TPR = computeTPR(verdicts, groundTruth)
	result.TNR = computeTNR(verdicts, groundTruth)

	// Compute 95% confidence intervals
	tprCI := bootstrapCI(scores, 1000)
	result.TPRLowerBound = tprCI.LowerBound
	result.TPRUpperBound = tprCI.UpperBound

	tnrCI := bootstrapCI(scores, 1000)
	result.TNRLowerBound = tnrCI.LowerBound
	result.TNRUpperBound = tnrCI.UpperBound

	// Find optimal threshold
	optimalThreshold, _ := findOptimalThreshold(scores, groundTruth)
	result.OptimalThreshold = optimalThreshold

	// Count positive/negative in ground truth
	for _, gt := range groundTruth {
		if gt {
			result.NumPositives++
		} else {
			result.NumNegatives++
		}
	}

	result.Samples = calibrationSamples

	return result, nil
}

// MockComplianceJudge is a test double for ComplianceJudge that uses deterministic logic.
type MockComplianceJudge struct {
	rubricID   string
	rubricName string
}

// NewMockComplianceJudge creates a new mock compliance judge.
func NewMockComplianceJudge() *MockComplianceJudge {
	return &MockComplianceJudge{
		rubricID:   "RB-CM-001",
		rubricName: "Compliance Evaluation",
	}
}

// Name returns the judge's name.
func (mcj *MockComplianceJudge) Name() string {
	return "MockComplianceJudge"
}

// Evaluate assesses compliance using deterministic rules.
func (mcj *MockComplianceJudge) Evaluate(ctx context.Context, input ComplianceJudgeInput) (Verdict, error) {
	var score float64
	var reason string
	var evidence string

	// Rule 1: KYC verified (status must be "verified")
	if input.KYCStatus == "verified" {
		score += 0.3
		evidence = "KYC verified"
	} else {
		score -= 0.3
		reason = "KYC not verified: " + input.KYCStatus
	}

	// Rule 2: AML risk low (score < 30 is considered passing)
	if input.AMLRiskScore < 30 {
		score += 0.3
		evidence += "; AML risk acceptable"
	} else {
		score -= 0.3
		reason += "; AML risk too high"
	}

	// Rule 3: Velocity check
	if !input.VelocityExceeded {
		score += 0.2
		evidence += "; velocity within limits"
	} else {
		score -= 0.2
		reason += "; velocity exceeded"
	}

	// Rule 4: Consent provided
	if input.ConsentProvided {
		score += 0.2
		evidence += "; consent provided"
	} else {
		score -= 0.2
		reason += "; consent not provided"
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	pass := score >= 0.6

	return Verdict{
		Pass:        pass,
		Score:       score,
		RubricID:    mcj.rubricID,
		RubricName:  mcj.rubricName,
		Reason:      reason,
		Evidence:    evidence,
		EvaluatedAt: time.Now(),
	}, nil
}

// Calibrate runs calibration on ground truth samples.
func (mcj *MockComplianceJudge) Calibrate(ctx context.Context, samples []ComplianceJudgeInput, groundTruth []bool) (CalibrationResult, error) {
	if len(samples) != len(groundTruth) {
		return CalibrationResult{}, fmt.Errorf("samples and groundTruth length mismatch")
	}

	result := CalibrationResult{
		CalibratedAt: time.Now(),
	}

	var verdicts []bool
	var scores []float64
	calibrationSamples := make([]CalibrationSample, len(samples))

	for i, sample := range samples {
		verdict, err := mcj.Evaluate(ctx, sample)
		if err != nil {
			return result, fmt.Errorf("evaluate sample %d: %w", i, err)
		}

		verdicts = append(verdicts, verdict.Pass)
		scores = append(scores, verdict.Score)
		calibrationSamples[i] = CalibrationSample{
			ID:          fmt.Sprintf("compliance-%d", i),
			GroundTruth: groundTruth[i],
			Verdict:     verdict.Pass,
			Score:       verdict.Score,
			Reason:      verdict.Reason,
		}
	}

	result.TPR = computeTPR(verdicts, groundTruth)
	result.TNR = computeTNR(verdicts, groundTruth)

	tprCI := bootstrapCI(scores, 1000)
	result.TPRLowerBound = tprCI.LowerBound
	result.TPRUpperBound = tprCI.UpperBound

	tnrCI := bootstrapCI(scores, 1000)
	result.TNRLowerBound = tnrCI.LowerBound
	result.TNRUpperBound = tnrCI.UpperBound

	optimalThreshold, _ := findOptimalThreshold(scores, groundTruth)
	result.OptimalThreshold = optimalThreshold

	for _, gt := range groundTruth {
		if gt {
			result.NumPositives++
		} else {
			result.NumNegatives++
		}
	}

	result.Samples = calibrationSamples
	return result, nil
}

// MockOrchestrationJudge is a test double for OrchestrationJudge.
type MockOrchestrationJudge struct {
	rubricID   string
	rubricName string
}

// NewMockOrchestrationJudge creates a new mock orchestration judge.
func NewMockOrchestrationJudge() *MockOrchestrationJudge {
	return &MockOrchestrationJudge{
		rubricID:   "RB-OR-001",
		rubricName: "Orchestration Evaluation",
	}
}

// Name returns the judge's name.
func (moj *MockOrchestrationJudge) Name() string {
	return "MockOrchestrationJudge"
}

// Evaluate assesses orchestration using deterministic rules.
func (moj *MockOrchestrationJudge) Evaluate(ctx context.Context, input OrchestrationJudgeInput) (Verdict, error) {
	var score float64
	var reason string
	var evidence string

	// Rule 1: Valid transition
	if input.TransitionValid {
		score += 0.35
		evidence = "Transition valid"
	} else {
		score -= 0.35
		reason = "Invalid state transition"
	}

	// Rule 2: Required steps present (e.g., payment_confirmed before settlement)
	hasPaymentConfirmed := input.PaymentConfirmed
	if input.SettlementInitiated && !hasPaymentConfirmed {
		score -= 0.35
		reason += "; missing payment confirmation before settlement"
	} else if len(input.StepSequence) > 0 && hasPaymentConfirmed {
		score += 0.35
		evidence += "; required steps present"
	} else {
		score -= 0.35
		reason += "; insufficient step sequence"
	}

	// Rule 3: Idempotency key present (for retry safety)
	if input.IdempotencyKey != "" {
		score += 0.3
		evidence += "; idempotency key present"
	} else {
		score -= 0.3
		reason += "; missing idempotency key"
	}

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	pass := score >= 0.6

	return Verdict{
		Pass:        pass,
		Score:       score,
		RubricID:    moj.rubricID,
		RubricName:  moj.rubricName,
		Reason:      reason,
		Evidence:    evidence,
		EvaluatedAt: time.Now(),
	}, nil
}

// Calibrate runs calibration.
func (moj *MockOrchestrationJudge) Calibrate(ctx context.Context, samples []OrchestrationJudgeInput, groundTruth []bool) (CalibrationResult, error) {
	if len(samples) != len(groundTruth) {
		return CalibrationResult{}, fmt.Errorf("samples and groundTruth length mismatch")
	}

	result := CalibrationResult{
		CalibratedAt: time.Now(),
	}

	var verdicts []bool
	var scores []float64
	calibrationSamples := make([]CalibrationSample, len(samples))

	for i, sample := range samples {
		verdict, err := moj.Evaluate(ctx, sample)
		if err != nil {
			return result, fmt.Errorf("evaluate sample %d: %w", i, err)
		}

		verdicts = append(verdicts, verdict.Pass)
		scores = append(scores, verdict.Score)
		calibrationSamples[i] = CalibrationSample{
			ID:          fmt.Sprintf("orch-%d", i),
			GroundTruth: groundTruth[i],
			Verdict:     verdict.Pass,
			Score:       verdict.Score,
			Reason:      verdict.Reason,
		}
	}

	result.TPR = computeTPR(verdicts, groundTruth)
	result.TNR = computeTNR(verdicts, groundTruth)
	result.OptimalThreshold = 0.6

	for _, gt := range groundTruth {
		if gt {
			result.NumPositives++
		} else {
			result.NumNegatives++
		}
	}

	result.Samples = calibrationSamples
	return result, nil
}

// MockLineageJudge is a test double for LineageJudge.
type MockLineageJudge struct {
	rubricID   string
	rubricName string
}

// NewMockLineageJudge creates a new mock lineage judge.
func NewMockLineageJudge() *MockLineageJudge {
	return &MockLineageJudge{
		rubricID:   "RB-LN-001",
		rubricName: "Lineage Evaluation",
	}
}

// Name returns the judge's name.
func (mlj *MockLineageJudge) Name() string {
	return "MockLineageJudge"
}

// Evaluate assesses lineage using deterministic rules.
func (mlj *MockLineageJudge) Evaluate(ctx context.Context, input LineageJudgeInput) (Verdict, error) {
	var score float64
	var reason string
	var evidence string

	// Rule 1: Hash chain valid
	if input.HashChainValid {
		score += 0.4
		evidence = "Hash chain valid"
	} else {
		score -= 0.4
		reason = "Hash chain broken"
	}

	// Rule 2: All required fields present
	if input.AllRequiredFieldsPresent {
		score += 0.35
		evidence += "; all required fields present"
	} else {
		score -= 0.35
		reason += "; missing required fields"
	}

	// Rule 3: Timestamps monotonic
	if input.TimestampsMonotonic {
		score += 0.25
		evidence += "; timestamps monotonic"
	} else {
		score -= 0.25
		reason += "; non-monotonic timestamps"
	}

	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	pass := score >= 0.6

	return Verdict{
		Pass:        pass,
		Score:       score,
		RubricID:    mlj.rubricID,
		RubricName:  mlj.rubricName,
		Reason:      reason,
		Evidence:    evidence,
		EvaluatedAt: time.Now(),
	}, nil
}

// Calibrate runs calibration.
func (mlj *MockLineageJudge) Calibrate(ctx context.Context, samples []LineageJudgeInput, groundTruth []bool) (CalibrationResult, error) {
	if len(samples) != len(groundTruth) {
		return CalibrationResult{}, fmt.Errorf("samples and groundTruth length mismatch")
	}

	result := CalibrationResult{
		CalibratedAt: time.Now(),
	}

	var verdicts []bool
	var scores []float64
	calibrationSamples := make([]CalibrationSample, len(samples))

	for i, sample := range samples {
		verdict, err := mlj.Evaluate(ctx, sample)
		if err != nil {
			return result, fmt.Errorf("evaluate sample %d: %w", i, err)
		}

		verdicts = append(verdicts, verdict.Pass)
		scores = append(scores, verdict.Score)
		calibrationSamples[i] = CalibrationSample{
			ID:          fmt.Sprintf("lineage-%d", i),
			GroundTruth: groundTruth[i],
			Verdict:     verdict.Pass,
			Score:       verdict.Score,
			Reason:      verdict.Reason,
		}
	}

	result.TPR = computeTPR(verdicts, groundTruth)
	result.TNR = computeTNR(verdicts, groundTruth)
	result.OptimalThreshold = 0.6

	for _, gt := range groundTruth {
		if gt {
			result.NumPositives++
		} else {
			result.NumNegatives++
		}
	}

	result.Samples = calibrationSamples
	return result, nil
}
