package judges

import (
	"context"
	"testing"
	"time"
)

// TestOrchestrationJudge_EvaluateValidSequence tests evaluation of valid state sequence.
func TestOrchestrationJudge_EvaluateValidSequence(t *testing.T) {
	judge := NewMockOrchestrationJudge()

	input := OrchestrationJudgeInput{
		OrderID:             "order-001",
		StepSequence:        []string{"order_created", "payment_initiated", "payment_confirmed", "settlement_initiated", "settlement_completed", "fulfilled"},
		CurrentStep:         "settlement_completed",
		PreviousStep:        "settlement_initiated",
		TransitionValid:     true,
		IdempotencyKey:      "idempotency-001",
		PaymentConfirmed:    true,
		SettlementInitiated: true,
		WorkflowAuditTrail:  `[{"step": "payment_confirmed"}, {"step": "settlement_initiated"}, {"step": "settlement_completed"}]`,
		Metadata:            map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Valid sequence should pass or score high
	if verdict.Score < 0.6 {
		t.Logf("Warning: valid sequence received low score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f, Reason: %s", verdict.Pass, verdict.Score, verdict.Reason)
}

// TestOrchestrationJudge_EvaluateInvalidSequence tests evaluation of invalid state sequence.
func TestOrchestrationJudge_EvaluateInvalidSequence(t *testing.T) {
	judge := NewMockOrchestrationJudge()

	input := OrchestrationJudgeInput{
		OrderID:             "order-002",
		StepSequence:        []string{"order_created", "settlement_initiated"}, // Settlement before payment!
		CurrentStep:         "settlement_initiated",
		PreviousStep:        "order_created",
		TransitionValid:     false, // Invalid transition
		IdempotencyKey:      "",
		PaymentConfirmed:    false, // Payment not confirmed
		SettlementInitiated: true,
		WorkflowAuditTrail:  `[{"step": "order_created"}, {"step": "settlement_initiated"}]`,
		Metadata:            map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Invalid sequence should fail or score low
	if verdict.Score > 0.5 {
		t.Logf("Warning: invalid sequence received high score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f, Reason: %s", verdict.Pass, verdict.Score, verdict.Reason)
}

// TestOrchestrationJudge_EvaluateNoIdempotencyKey tests handling of missing idempotency key.
func TestOrchestrationJudge_EvaluateNoIdempotencyKey(t *testing.T) {
	judge := NewMockOrchestrationJudge()

	input := OrchestrationJudgeInput{
		OrderID:             "order-003",
		StepSequence:        []string{"order_created", "payment_initiated", "payment_confirmed"},
		CurrentStep:         "payment_confirmed",
		PreviousStep:        "payment_initiated",
		TransitionValid:     true,
		IdempotencyKey:      "", // Missing idempotency key!
		PaymentConfirmed:    true,
		SettlementInitiated: false,
		WorkflowAuditTrail:  `[{"step": "payment_initiated"}, {"step": "payment_confirmed"}]`,
		Metadata:            map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Missing idempotency key should result in lower score
	t.Logf("Verdict: Pass=%v, Score=%.2f (idempotency key missing)", verdict.Pass, verdict.Score)
}

// TestOrchestrationJudge_Calibration tests calibration with ground truth.
func TestOrchestrationJudge_Calibration(t *testing.T) {
	judge := NewMockOrchestrationJudge()

	// Create calibration samples
	samples := []OrchestrationJudgeInput{
		// Valid sequence sample
		{
			OrderID:             "cal-001",
			StepSequence:        []string{"order_created", "payment_initiated", "payment_confirmed", "settlement_initiated", "settlement_completed"},
			CurrentStep:         "settlement_completed",
			PreviousStep:        "settlement_initiated",
			TransitionValid:     true,
			IdempotencyKey:      "idempotency-001",
			PaymentConfirmed:    true,
			SettlementInitiated: true,
			WorkflowAuditTrail:  `[{"step": "payment_confirmed"}, {"step": "settlement_completed"}]`,
		},
		// Invalid sequence (settlement before payment)
		{
			OrderID:             "cal-002",
			StepSequence:        []string{"order_created", "settlement_initiated"},
			CurrentStep:         "settlement_initiated",
			PreviousStep:        "order_created",
			TransitionValid:     false,
			IdempotencyKey:      "",
			PaymentConfirmed:    false,
			SettlementInitiated: true,
			WorkflowAuditTrail:  `[{"step": "order_created"}, {"step": "settlement_initiated"}]`,
		},
		// Valid sequence with idempotency
		{
			OrderID:             "cal-003",
			StepSequence:        []string{"order_created", "payment_initiated", "payment_confirmed"},
			CurrentStep:         "payment_confirmed",
			PreviousStep:        "payment_initiated",
			TransitionValid:     true,
			IdempotencyKey:      "idempotency-003",
			PaymentConfirmed:    true,
			SettlementInitiated: false,
			WorkflowAuditTrail:  `[{"step": "payment_initiated"}, {"step": "payment_confirmed"}]`,
		},
	}

	// Ground truth: valid, invalid, valid
	groundTruth := []bool{true, false, true}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	if err != nil {
		t.Fatalf("Calibrate failed: %v", err)
	}

	t.Logf("Calibration Results:")
	t.Logf("  Samples: %d positive, %d negative", result.NumPositives, result.NumNegatives)
	t.Logf("  TPR: %.3f (CI: [%.3f, %.3f])", result.TPR, result.TPRLowerBound, result.TPRUpperBound)
	t.Logf("  TNR: %.3f (CI: [%.3f, %.3f])", result.TNR, result.TNRLowerBound, result.TNRUpperBound)
	t.Logf("  Optimal Threshold: %.3f", result.OptimalThreshold)

	// Verify confidence intervals
	if result.TPRUpperBound < result.TPRLowerBound {
		t.Errorf("Invalid TPR CI: upper=%.3f < lower=%.3f", result.TPRUpperBound, result.TPRLowerBound)
	}
	if result.TNRUpperBound < result.TNRLowerBound {
		t.Errorf("Invalid TNR CI: upper=%.3f < lower=%.3f", result.TNRUpperBound, result.TNRLowerBound)
	}
}

// TestOrchestrationJudge_Name tests judge name.
func TestOrchestrationJudge_Name(t *testing.T) {
	judge := NewMockOrchestrationJudge()
	if judge.Name() != "MockOrchestrationJudge" {
		t.Errorf("expected name MockOrchestrationJudge, got %s", judge.Name())
	}
}

// BenchmarkOrchestrationJudge_Evaluate benchmarks evaluation performance.
func BenchmarkOrchestrationJudge_Evaluate(b *testing.B) {
	judge := NewMockOrchestrationJudge()

	input := OrchestrationJudgeInput{
		OrderID:             "bench-001",
		StepSequence:        []string{"order_created", "payment_initiated", "payment_confirmed"},
		CurrentStep:         "payment_confirmed",
		PreviousStep:        "payment_initiated",
		TransitionValid:     true,
		IdempotencyKey:      "bench-idempotency-001",
		PaymentConfirmed:    true,
		SettlementInitiated: false,
		WorkflowAuditTrail:  `[{"step": "payment_initiated"}, {"step": "payment_confirmed"}]`,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := judge.Evaluate(ctx, input)
		if err != nil {
			b.Fatalf("Evaluate failed: %v", err)
		}
	}
}
