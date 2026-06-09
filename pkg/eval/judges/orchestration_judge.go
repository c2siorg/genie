package judges

import (
	"context"
	"fmt"
	"time"
)

// OrchestrationJudge evaluates workflow orchestration for:
//   - StateValidation: workflow state machine only allows valid transitions
//   - WorkflowSequences: steps execute in correct order (payment before settlement)
//   - MultiAgentConsistency: payment/settlement agents called with proper sequencing
//   - IdempotencyEnforcement: retry requests use idempotency keys
type OrchestrationJudge struct {
	cfg        JudgeConfig
	rubricID   string
	rubricName string
	calibration *CalibrationResult
}

// NewOrchestrationJudge creates a new orchestration judge.
func NewOrchestrationJudge(cfg JudgeConfig) *OrchestrationJudge {
	return &OrchestrationJudge{
		cfg:        cfg,
		rubricID:   "RB-OR-001", // Workflow State Machine Validity (primary)
		rubricName: "Orchestration Evaluation",
	}
}

// Name returns the judge's human-readable name.
func (oj *OrchestrationJudge) Name() string {
	return "OrchestrationJudge"
}

// Evaluate assesses workflow orchestration.
func (oj *OrchestrationJudge) Evaluate(ctx context.Context, input OrchestrationJudgeInput) (Verdict, error) {
	now := time.Now()

	// Build evaluation prompt
	systemPrompt := orchestrationJudgeSystemPrompt()
	userPrompt := orchestrationJudgeUserPrompt(input)

	// Call LLM judge
	judgeResp, err := callLLMJudge(ctx, oj.cfg, systemPrompt, userPrompt)
	if err != nil {
		return Verdict{}, fmt.Errorf("callLLMJudge: %w", err)
	}

	// Apply calibration threshold if available
	score := judgeResp.Score
	if oj.calibration != nil {
		judgeResp.Pass = score >= oj.calibration.OptimalThreshold
	}

	return Verdict{
		Pass:        judgeResp.Pass,
		Score:       score,
		RubricID:    oj.rubricID,
		RubricName:  oj.rubricName,
		Reason:      judgeResp.Reason,
		Evidence:    judgeResp.Evidence,
		EvaluatedAt: now,
	}, nil
}

// Calibrate runs calibration on ground truth samples.
func (oj *OrchestrationJudge) Calibrate(ctx context.Context, samples []OrchestrationJudgeInput, groundTruth []bool) (CalibrationResult, error) {
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
		verdict, err := oj.Evaluate(ctx, sample)
		if err != nil {
			return result, fmt.Errorf("evaluate sample %d: %w", i, err)
		}

		verdicts = append(verdicts, verdict.Pass)
		scores = append(scores, verdict.Score)
		calibrationSamples[i] = CalibrationSample{
			ID:          fmt.Sprintf("orchestration-%d", i),
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
	oj.calibration = &result

	return result, nil
}

// orchestrationJudgeSystemPrompt returns the system prompt for orchestration judge.
func orchestrationJudgeSystemPrompt() string {
	return `You are an Orchestration Evaluation Judge for a multi-agent order processing system.

Your job is to assess whether a workflow follows the correct state machine and step ordering.

Respond ONLY with a JSON object in this exact format (no markdown, no extra text):
{"pass": <boolean>, "score": <float 0.0-1.0>, "reason": "<brief explanation>", "evidence": "<observed facts>"}

Scoring rubric:
  pass=true (score ≥0.8): Valid transition, correct step order (payment before settlement), idempotency key used
  pass=true (score 0.6-0.8): Valid transition, correct step order, minor idempotency concerns
  pass=false (score 0.4-0.6): Invalid transition or wrong order, idempotency unclear
  pass=false (score <0.4): Invalid transition allowed, settlement before payment, no idempotency key

Valid state transitions:
- OrderCreated → PaymentInitiated (always allowed)
- PaymentInitiated → PaymentConfirmed (if payment succeeds)
- PaymentConfirmed → SettlementInitiated (always allowed after payment confirmed)
- SettlementInitiated → SettlementCompleted (if settlement succeeds)
- SettlementCompleted → Fulfilled (always allowed after settlement)
- Any state → Cancelled (error recovery)

Key evaluation points:
1. StateValidation: Is the transition allowed by state machine rules?
2. WorkflowSequences: Is current_step valid after previous_step?
3. MultiAgentConsistency: Payment confirmed before settlement initiated?
4. IdempotencyEnforcement: Is idempotency_key set for retryable operations?

Be precise and reference specific states from the provided context.`
}

// orchestrationJudgeUserPrompt formats the user prompt with orchestration details.
func orchestrationJudgeUserPrompt(input OrchestrationJudgeInput) string {
	stepSeq := ""
	for i, step := range input.StepSequence {
		if i > 0 {
			stepSeq += " → "
		}
		stepSeq += step
	}

	return fmt.Sprintf(`Evaluate workflow orchestration for this order:

Order Details:
- OrderID: %s

Workflow Execution:
- Step Sequence: %s
- Current Step: %s
- Previous Step: %s

Transition Validation:
- Transition Valid: %v

State Conditions:
- Payment Confirmed: %v
- Settlement Initiated: %v (should be after payment confirmed)

Idempotency:
- Idempotency Key: %s

Workflow Audit Trail:
%s

Additional Context:
%v

Orchestration Questions:
1. StateValidation: previous_step ("%s") → current_step ("%s") allowed?
2. WorkflowSequences: Steps in correct order (payment before settlement)?
3. MultiAgentConsistency: payment_confirmed (%v) before settlement_initiated (%v)?
4. IdempotencyEnforcement: idempotency_key set? ("%s")

Assess whether this workflow follows the correct state machine and step ordering.`,
		input.OrderID,
		stepSeq,
		input.CurrentStep,
		input.PreviousStep,
		input.TransitionValid,
		input.PaymentConfirmed,
		input.SettlementInitiated,
		input.IdempotencyKey,
		input.WorkflowAuditTrail,
		input.Metadata,
		input.PreviousStep,
		input.CurrentStep,
		input.PaymentConfirmed,
		input.SettlementInitiated,
		input.IdempotencyKey,
	)
}
