package judges

import (
	"context"
	"fmt"
	"time"
)

// ComplianceJudge evaluates compliance enforcement for:
//   - KYCJustification: KYC status valid and not expired
//   - AMLThresholds: AML risk score and velocity thresholds enforced
//   - VelocityEnforcement: transaction velocity limits enforced
//   - AuditTrails: compliance decisions fully logged with evidence
type ComplianceJudge struct {
	cfg        JudgeConfig
	rubricID   string
	rubricName string
	calibration *CalibrationResult
}

// NewComplianceJudge creates a new compliance judge.
func NewComplianceJudge(cfg JudgeConfig) *ComplianceJudge {
	return &ComplianceJudge{
		cfg:        cfg,
		rubricID:   "RB-CO-001", // Velocity Monitoring (primary)
		rubricName: "Compliance Evaluation",
	}
}

// Name returns the judge's human-readable name.
func (cj *ComplianceJudge) Name() string {
	return "ComplianceJudge"
}

// Evaluate assesses compliance enforcement for a transaction.
func (cj *ComplianceJudge) Evaluate(ctx context.Context, input ComplianceJudgeInput) (Verdict, error) {
	now := time.Now()

	// Build evaluation prompt
	systemPrompt := complianceJudgeSystemPrompt()
	userPrompt := complianceJudgeUserPrompt(input)

	// Call LLM judge
	judgeResp, err := callLLMJudge(ctx, cj.cfg, systemPrompt, userPrompt)
	if err != nil {
		return Verdict{}, fmt.Errorf("callLLMJudge: %w", err)
	}

	// Apply calibration threshold if available
	score := judgeResp.Score
	if cj.calibration != nil {
		judgeResp.Pass = score >= cj.calibration.OptimalThreshold
	}

	return Verdict{
		Pass:        judgeResp.Pass,
		Score:       score,
		RubricID:    cj.rubricID,
		RubricName:  cj.rubricName,
		Reason:      judgeResp.Reason,
		Evidence:    judgeResp.Evidence,
		EvaluatedAt: now,
	}, nil
}

// Calibrate runs calibration on ground truth samples.
func (cj *ComplianceJudge) Calibrate(ctx context.Context, samples []ComplianceJudgeInput, groundTruth []bool) (CalibrationResult, error) {
	if len(samples) != len(groundTruth) {
		return CalibrationResult{}, fmt.Errorf("samples and groundTruth length mismatch")
	}

	if len(samples) == 0 {
		return CalibrationResult{}, fmt.Errorf("no samples provided")
	}

	result := CalibrationResult{
		CalibratedAt: time.Now(),
	}

	// Evaluate each sample
	var verdicts []bool
	var scores []float64
	calibrationSamples := make([]CalibrationSample, len(samples))

	for i, sample := range samples {
		verdict, err := cj.Evaluate(ctx, sample)
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

	// Update internal calibration
	cj.calibration = &result

	return result, nil
}

// complianceJudgeSystemPrompt returns the system prompt for compliance judge.
func complianceJudgeSystemPrompt() string {
	return `You are a Compliance Evaluation Judge for an e-Rupee financial system.

Your job is to assess whether a transaction complies with KYC/AML/velocity requirements.

Respond ONLY with a JSON object in this exact format (no markdown, no extra text):
{"pass": <boolean>, "score": <float 0.0-1.0>, "reason": "<brief explanation>", "evidence": "<observed facts>"}

Scoring rubric:
  pass=true (score ≥0.8): KYC verified and not expired, velocity within threshold, AML risk acceptable, audit trail complete
  pass=true (score 0.6-0.8): KYC verified, velocity acceptable, audit trail mostly complete
  pass=false (score 0.4-0.6): KYC pending or expired, velocity exceeded, AML risk high, audit gaps
  pass=false (score <0.4): KYC failed, velocity severely exceeded, AML risk critical, audit trail missing

Key evaluation points:
1. KYCJustification: Is customer KYC status "verified"? Not expired?
2. AMLThresholds: Is AML risk score acceptable? (typically <30 = low, 30-70 = medium, >70 = high)
3. VelocityEnforcement: Is current_velocity within threshold? No exceeding limit?
4. AuditTrails: Is compliance decision fully logged with evidence (threshold, score, matching)?

Be precise and reference specific values from the provided context.`
}

// complianceJudgeUserPrompt formats the user prompt with compliance details.
func complianceJudgeUserPrompt(input ComplianceJudgeInput) string {
	return fmt.Sprintf(`Evaluate compliance for this transaction:

Customer Details:
- CustomerID: %s
- OrderID: %s

KYC Status:
- Status: %s
- Expiry Date: %v

AML Risk:
- Risk Score: %.1f/100 (0-30=low, 30-70=medium, >70=high)

Velocity Monitoring:
- Window: %d seconds
- Threshold: %d paise
- Current Velocity: %d paise
- Exceeded: %v

Sanctions/List Management:
- Sanctions List Age: %d seconds
- Matching Threshold: %.2f (0.0-1.0)

Data Consent:
- Consent Provided: %v

Audit Trail:
%s

Additional Context:
%v

Compliance Questions:
1. KYCJustification: kyc_status == "verified" && not_expired?
2. AMLThresholds: aml_risk_score acceptable (typically <70)?
3. VelocityEnforcement: current_velocity (%d) <= threshold (%d)?
4. AuditTrails: Decision fully logged with evidence?

Assess whether this transaction complies with all KYC/AML/velocity requirements.`,
		input.CustomerID,
		input.OrderID,
		input.KYCStatus,
		input.KYCExpiryDate,
		input.AMLRiskScore,
		input.VelocityWindowSeconds,
		input.VelocityThresholdPaise,
		input.CurrentVelocityPaise,
		input.VelocityExceeded,
		input.SanctionsListAge,
		input.MatchingThreshold,
		input.ConsentProvided,
		input.AuditTrail,
		input.Metadata,
		input.CurrentVelocityPaise,
		input.VelocityThresholdPaise,
	)
}
