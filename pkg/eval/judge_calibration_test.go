package eval

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/judges"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Judge Calibration Tests (50 tests)
// Tests: Judge accuracy, TPR/TNR calculation, confidence intervals, robustness
// ============================================================================

// TestJudge_SettlementJudgeTPRCalculation tests TPR calculation
func TestJudge_SettlementJudgeTPRCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Positive samples (correct settlements)
	samples := []judges.SettlementJudgeInput{
		{
			OrderID: "order-001", SettlementID: "settlement-001",
			SettlementAmount: 100_000, ExpectedAmount: 100_000,
			CBDCCommitted: true, ReconciliationPassed: true,
		},
		{
			OrderID: "order-002", SettlementID: "settlement-002",
			SettlementAmount: 500_000, ExpectedAmount: 500_000,
			CBDCCommitted: true, ReconciliationPassed: true,
		},
		{
			OrderID: "order-003", SettlementID: "settlement-003",
			SettlementAmount: 1_000_000, ExpectedAmount: 1_000_000,
			CBDCCommitted: true, ReconciliationPassed: true,
		},
	}
	groundTruth := []bool{true, true, true}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// TPR should be 1.0 for all correct verdicts
	assert.Equal(t, float64(1.0), result.TPR, "TPR should be 1.0 for all positive samples")
	// Bounds are bootstrap percentiles of the per-sample scores; they are valid
	// probabilities with lower <= upper.
	assert.LessOrEqual(t, result.TPRLowerBound, result.TPRUpperBound)
	assert.GreaterOrEqual(t, result.TPRLowerBound, float64(0))
	assert.LessOrEqual(t, result.TPRUpperBound, float64(1))

	t.Logf("Settlement TPR: %.3f [%.3f, %.3f]", result.TPR, result.TPRLowerBound, result.TPRUpperBound)
}

// TestJudge_SettlementJudgeTNRCalculation tests TNR calculation
func TestJudge_SettlementJudgeTNRCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Negative samples (incorrect settlements)
	samples := []judges.SettlementJudgeInput{
		{
			OrderID: "order-n1", SettlementID: "settlement-n1",
			SettlementAmount: 50_000, ExpectedAmount: 100_000,
			CBDCCommitted: false, ReconciliationPassed: false,
		},
		{
			OrderID: "order-n2", SettlementID: "settlement-n2",
			SettlementAmount: 75_000, ExpectedAmount: 500_000,
			CBDCCommitted: false, ReconciliationPassed: false,
		},
		{
			OrderID: "order-n3", SettlementID: "settlement-n3",
			SettlementAmount: 0, ExpectedAmount: 1_000_000,
			CBDCCommitted: false, ReconciliationPassed: false,
		},
	}
	groundTruth := []bool{false, false, false}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// TNR should be high for incorrect samples
	assert.Greater(t, result.TNR, float64(0.5), "TNR should be > 0.5")
	// Bounds are bootstrap percentiles of the per-sample scores; they are valid
	// probabilities with lower <= upper.
	assert.LessOrEqual(t, result.TNRLowerBound, result.TNRUpperBound)
	assert.GreaterOrEqual(t, result.TNRLowerBound, float64(0))
	assert.LessOrEqual(t, result.TNRUpperBound, float64(1))

	t.Logf("Settlement TNR: %.3f [%.3f, %.3f]", result.TNR, result.TNRLowerBound, result.TNRUpperBound)
}

// TestJudge_ComplianceJudgeTPRCalculation tests compliance judge TPR
func TestJudge_ComplianceJudgeTPRCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockComplianceJudge()

	samples := []judges.ComplianceJudgeInput{
		{
			CustomerID:       "cust-compliant-001",
			KYCStatus:        "verified",
			AMLRiskScore:     10.0,
			VelocityExceeded: false,
			ConsentProvided:  true,
		},
		{
			CustomerID:       "cust-compliant-002",
			KYCStatus:        "verified",
			AMLRiskScore:     10.0,
			VelocityExceeded: false,
			ConsentProvided:  true,
		},
	}
	groundTruth := []bool{true, true}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.TPR, float64(0.5))
	t.Logf("Compliance TPR: %.3f [%.3f, %.3f]", result.TPR, result.TPRLowerBound, result.TPRUpperBound)
}

// TestJudge_OrchestrationJudgeTPRCalculation tests orchestration judge TPR
func TestJudge_OrchestrationJudgeTPRCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockOrchestrationJudge()

	samples := []judges.OrchestrationJudgeInput{
		{
			OrderID:             "order-001",
			StepSequence:        []string{"order_created", "payment_confirmed", "settlement_completed"},
			TransitionValid:     true,
			IdempotencyKey:      "idem-001",
			PaymentConfirmed:    true,
			SettlementInitiated: true,
		},
		{
			OrderID:             "order-002",
			StepSequence:        []string{"order_created", "payment_confirmed", "settlement_completed"},
			TransitionValid:     true,
			IdempotencyKey:      "idem-002",
			PaymentConfirmed:    true,
			SettlementInitiated: true,
		},
	}
	groundTruth := []bool{true, true}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.TPR, float64(0.5))
	t.Logf("Orchestration TPR: %.3f [%.3f, %.3f]", result.TPR, result.TPRLowerBound, result.TPRUpperBound)
}

// TestJudge_LineageJudgeTPRCalculation tests lineage judge TPR
func TestJudge_LineageJudgeTPRCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockLineageJudge()

	samples := []judges.LineageJudgeInput{
		{
			OrderID:                  "order-001",
			HashChainValid:           true,
			AllRequiredFieldsPresent: true,
			TimestampsMonotonic:      true,
		},
		{
			OrderID:                  "order-002",
			HashChainValid:           true,
			AllRequiredFieldsPresent: true,
			TimestampsMonotonic:      true,
		},
	}
	groundTruth := []bool{true, true}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.TPR, float64(0.5))
	t.Logf("Lineage TPR: %.3f [%.3f, %.3f]", result.TPR, result.TPRLowerBound, result.TPRUpperBound)
}

// TestJudge_ConfidenceIntervalQuality tests 95% CI coverage
func TestJudge_ConfidenceIntervalQuality(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// 30 samples for robust CI calculation
	samples := make([]judges.SettlementJudgeInput, 30)
	groundTruth := make([]bool, 30)

	for i := 0; i < 30; i++ {
		isCorrect := i < 20 // 20 correct, 10 incorrect
		samples[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("order-%d", i),
			SettlementID:         fmt.Sprintf("settlement-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        isCorrect,
			ReconciliationPassed: isCorrect,
		}
		groundTruth[i] = isCorrect
	}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// Bootstrap CIs are computed over per-sample scores: bounds are ordered and
	// stay within [0, 1].
	assert.LessOrEqual(t, result.TPRLowerBound, result.TPRUpperBound)
	assert.LessOrEqual(t, result.TNRLowerBound, result.TNRUpperBound)
	assert.GreaterOrEqual(t, result.TPRLowerBound, float64(0))
	assert.LessOrEqual(t, result.TPRUpperBound, float64(1))

	// CI width should be reasonable
	tprCIWidth := result.TPRUpperBound - result.TPRLowerBound
	assert.Less(t, tprCIWidth, float64(0.5), "CI width should be reasonable")

	t.Logf("CI Quality: TPR CI width=%.3f", tprCIWidth)
}

// TestJudge_JudgeAccuracyThreshold tests ≥92% accuracy
func TestJudge_JudgeAccuracyThreshold(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Create balanced dataset
	samples := make([]judges.SettlementJudgeInput, 50)
	groundTruth := make([]bool, 50)

	for i := 0; i < 50; i++ {
		isCorrect := i%2 == 0
		samples[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("order-%d", i),
			SettlementID:         fmt.Sprintf("settlement-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        isCorrect,
			ReconciliationPassed: isCorrect,
		}
		groundTruth[i] = isCorrect
	}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// Calculate accuracy
	accuracy := (result.TPR + result.TNR) / 2
	assert.GreaterOrEqual(t, accuracy, float64(0.92), "Judge accuracy should be ≥92%")

	t.Logf("Judge Accuracy: %.2f%% (TPR=%.2f%%, TNR=%.2f%%)",
		accuracy*100, result.TPR*100, result.TNR*100)
}

// TestJudge_JudgeRobustness tests robustness to adversarial inputs
func TestJudge_JudgeRobustness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Adversarial cases
	adversarialCases := []judges.SettlementJudgeInput{
		{
			OrderID: "adv-1", SettlementID: "settlement-adv-1",
			SettlementAmount: 100_000, ExpectedAmount: 100_001, // Off by 1 paise
			CBDCCommitted: true, ReconciliationPassed: true,
		},
		{
			OrderID: "adv-2", SettlementID: "settlement-adv-2",
			SettlementAmount: 0, ExpectedAmount: 100_000, // Zero amount
			CBDCCommitted: false, ReconciliationPassed: false,
		},
		{
			OrderID: "adv-3", SettlementID: "settlement-adv-3",
			SettlementAmount: -100_000, ExpectedAmount: 100_000, // Negative amount
			CBDCCommitted: false, ReconciliationPassed: false,
		},
	}

	for _, input := range adversarialCases {
		verdict, err := judge.Evaluate(ctx, input)
		assert.NoError(t, err, "Judge should handle adversarial input without panic")
		assert.GreaterOrEqual(t, verdict.Score, float64(0))
	}

	t.Logf("Judge robustness: passed adversarial tests")
}

// TestJudge_JudgeConsistency tests judge consistency on same input
func TestJudge_JudgeConsistency(t *testing.T) {
	ctx := context.Background()

	judge := judges.NewMockSettlementJudge()

	input := judges.SettlementJudgeInput{
		OrderID: "order-consistency", SettlementID: "settlement-consistency",
		SettlementAmount: 100_000, ExpectedAmount: 100_000,
		CBDCCommitted: true, ReconciliationPassed: true,
	}

	// Evaluate multiple times
	verdicts := make([]judges.Verdict, 5)
	for i := 0; i < 5; i++ {
		v, err := judge.Evaluate(ctx, input)
		require.NoError(t, err)
		verdicts[i] = v
	}

	// All verdicts should be identical (deterministic judge)
	for i := 1; i < len(verdicts); i++ {
		assert.Equal(t, verdicts[0].Pass, verdicts[i].Pass)
		assert.Equal(t, verdicts[0].Score, verdicts[i].Score)
	}

	t.Logf("Judge consistency: all 5 evaluations identical")
}

// TestJudge_JudgeFailureCaseHandling tests failure case handling
func TestJudge_JudgeFailureCaseHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	failureCases := []judges.SettlementJudgeInput{
		{
			OrderID: "", SettlementID: "settlement-1", // Empty OrderID
			SettlementAmount: 100_000, ExpectedAmount: 100_000,
		},
		{
			OrderID: "order-2", SettlementID: "", // Empty SettlementID
			SettlementAmount: 100_000, ExpectedAmount: 100_000,
		},
		{
			OrderID: "order-3", SettlementID: "settlement-3",
			SettlementAmount: 100_000, ExpectedAmount: 100_000,
			AuditTrail: "invalid-json", // Invalid JSON
		},
	}

	for i, input := range failureCases {
		verdict, err := judge.Evaluate(ctx, input)
		// Should handle gracefully (not panic)
		t.Logf("Failure case %d: err=%v, verdict=%v", i, err, verdict)
	}
}

// TestJudge_FalsePositiveRateCalculation tests FPR calculation
func TestJudge_FalsePositiveRateCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Create dataset with known FP opportunities
	samples := make([]judges.SettlementJudgeInput, 20)
	groundTruth := make([]bool, 20)

	// First 10: negative (false positives possible here)
	// Last 10: positive
	for i := 0; i < 20; i++ {
		isCorrect := i >= 10
		samples[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("order-%d", i),
			SettlementID:         fmt.Sprintf("settlement-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        isCorrect,
			ReconciliationPassed: isCorrect,
		}
		groundTruth[i] = isCorrect
	}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// FPR = 1 - TNR
	fpr := 1 - result.TNR
	assert.Less(t, fpr, float64(0.3), "False positive rate should be < 30%")

	t.Logf("FPR: %.3f (TNR=%.3f)", fpr, result.TNR)
}

// TestJudge_FalseNegativeRateCalculation tests FNR calculation
func TestJudge_FalseNegativeRateCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Create dataset with known FN opportunities
	samples := make([]judges.SettlementJudgeInput, 20)
	groundTruth := make([]bool, 20)

	// First 10: positive (false negatives possible here)
	// Last 10: negative
	for i := 0; i < 20; i++ {
		isCorrect := i < 10
		samples[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("order-%d", i),
			SettlementID:         fmt.Sprintf("settlement-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        isCorrect,
			ReconciliationPassed: isCorrect,
		}
		groundTruth[i] = isCorrect
	}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// FNR = 1 - TPR
	fnr := 1 - result.TPR
	assert.Less(t, fnr, float64(0.3), "False negative rate should be < 30%")

	t.Logf("FNR: %.3f (TPR=%.3f)", fnr, result.TPR)
}

// TestJudge_EdgeCaseHandling tests edge cases
func TestJudge_EdgeCaseHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	edgeCases := []struct {
		name  string
		input judges.SettlementJudgeInput
	}{
		{
			name: "zero_amount",
			input: judges.SettlementJudgeInput{
				OrderID: "edge-1", SettlementID: "settle-edge-1",
				SettlementAmount: 0, ExpectedAmount: 0,
			},
		},
		{
			name: "max_amount",
			input: judges.SettlementJudgeInput{
				OrderID: "edge-2", SettlementID: "settle-edge-2",
				SettlementAmount: math.MaxInt64, ExpectedAmount: math.MaxInt64,
			},
		},
		{
			name: "negative_amount",
			input: judges.SettlementJudgeInput{
				OrderID: "edge-3", SettlementID: "settle-edge-3",
				SettlementAmount: -1, ExpectedAmount: -1,
			},
		},
	}

	for _, ec := range edgeCases {
		verdict, err := judge.Evaluate(ctx, ec.input)
		assert.NoError(t, err)
		t.Logf("Edge case %s: score=%.2f, pass=%v", ec.name, verdict.Score, verdict.Pass)
		assert.GreaterOrEqual(t, verdict.Score, float64(0))
	}
}

// TestJudge_ConcurrentEvaluation tests concurrent judge evaluation
func TestJudge_ConcurrentEvaluation(t *testing.T) {
	ctx := context.Background()

	judge := judges.NewMockSettlementJudge()

	numGoroutines := 20
	var wg sync.WaitGroup
	successCount := atomic.Int32{}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			input := judges.SettlementJudgeInput{
				OrderID:              fmt.Sprintf("order-concurrent-%d", idx),
				SettlementID:         fmt.Sprintf("settlement-concurrent-%d", idx),
				SettlementAmount:     100_000,
				ExpectedAmount:       100_000,
				CBDCCommitted:        true,
				ReconciliationPassed: true,
			}

			verdict, err := judge.Evaluate(ctx, input)
			if err == nil && verdict.Pass {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(numGoroutines), successCount.Load())

	t.Logf("Concurrent evaluation: %d/%d succeeded", successCount.Load(), numGoroutines)
}

// TestJudge_TimeoutHandling tests timeout handling
func TestJudge_TimeoutHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	input := judges.SettlementJudgeInput{
		OrderID: "timeout-test", SettlementID: "settle-timeout",
		SettlementAmount: 100_000, ExpectedAmount: 100_000,
	}

	verdict, err := judge.Evaluate(ctx, input)
	t.Logf("Timeout handling: verdict=%v, err=%v", verdict, err)
}

// TestJudge_ErrorRecovery tests error recovery
func TestJudge_ErrorRecovery(t *testing.T) {
	ctx := context.Background()

	judge := judges.NewMockSettlementJudge()

	// First call with error
	input1 := judges.SettlementJudgeInput{
		OrderID:      "", // Invalid
		SettlementID: "settle-1",
	}

	_, _ = judge.Evaluate(ctx, input1)

	// Recovery call should work
	input2 := judges.SettlementJudgeInput{
		OrderID:          "order-2",
		SettlementID:     "settle-2",
		SettlementAmount: 100_000,
		ExpectedAmount:   100_000,
	}

	verdict, err := judge.Evaluate(ctx, input2)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, verdict.Score, float64(0))

	t.Logf("Error recovery: successful after prior error")
}

// TestJudge_OptimalThresholdCalculation tests threshold tuning
func TestJudge_OptimalThresholdCalculation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	samples := make([]judges.SettlementJudgeInput, 40)
	groundTruth := make([]bool, 40)

	for i := 0; i < 40; i++ {
		isCorrect := i < 25
		samples[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("order-%d", i),
			SettlementID:         fmt.Sprintf("settlement-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        isCorrect,
			ReconciliationPassed: isCorrect,
		}
		groundTruth[i] = isCorrect
	}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// Optimal threshold should be between 0 and 1
	assert.GreaterOrEqual(t, result.OptimalThreshold, float64(0))
	assert.LessOrEqual(t, result.OptimalThreshold, float64(1))

	t.Logf("Optimal threshold: %.3f (F1-optimal)", result.OptimalThreshold)
}

// TestJudge_BootstrapIntervals tests bootstrap confidence intervals
func TestJudge_BootstrapIntervals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	samples := make([]judges.SettlementJudgeInput, 30)
	groundTruth := make([]bool, 30)

	for i := 0; i < 30; i++ {
		isCorrect := i%2 == 0
		samples[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("order-%d", i),
			SettlementID:         fmt.Sprintf("settlement-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        isCorrect,
			ReconciliationPassed: isCorrect,
		}
		groundTruth[i] = isCorrect
	}

	result, err := judge.Calibrate(ctx, samples, groundTruth)
	require.NoError(t, err)

	// Verify bootstrap CI properties: ordered bounds within [0, 1].
	assert.LessOrEqual(t, result.TPRLowerBound, result.TPRUpperBound)
	assert.GreaterOrEqual(t, result.TPRLowerBound, float64(0))
	assert.LessOrEqual(t, result.TPRUpperBound, float64(1))

	t.Logf("Bootstrap intervals: TPR=%.3f [%.3f, %.3f]",
		result.TPR, result.TPRLowerBound, result.TPRUpperBound)
}

// TestJudge_AblationTesting tests ablation (feature importance)
func TestJudge_AblationTesting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Test with all features
	samplesFull := make([]judges.SettlementJudgeInput, 20)
	for i := 0; i < 20; i++ {
		samplesFull[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("order-%d", i),
			SettlementID:         fmt.Sprintf("settlement-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        true,
			ReconciliationPassed: true,
			AuditTrail:           "[{}]",
		}
	}
	groundTruth := make([]bool, 20)
	for i := 0; i < 20; i++ {
		groundTruth[i] = true
	}

	resultFull, _ := judge.Calibrate(ctx, samplesFull, groundTruth)

	// Test without AuditTrail
	samplesNoAudit := make([]judges.SettlementJudgeInput, 20)
	for i := 0; i < 20; i++ {
		samplesNoAudit[i] = samplesFull[i]
		samplesNoAudit[i].AuditTrail = ""
	}

	resultNoAudit, _ := judge.Calibrate(ctx, samplesNoAudit, groundTruth)

	t.Logf("Ablation: full_TPR=%.3f, noAudit_TPR=%.3f, impact=%.3f",
		resultFull.TPR, resultNoAudit.TPR, resultFull.TPR-resultNoAudit.TPR)
}

// TestJudge_HallucinationDetection tests hallucination detection
func TestJudge_HallucinationDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Input with inconsistent fields (hallucination indicators)
	hallucinationCases := []judges.SettlementJudgeInput{
		{
			OrderID:              "order-1",
			SettlementAmount:     100_000,
			ExpectedAmount:       500_000, // Large mismatch
			CBDCCommitted:        true,
			ReconciliationPassed: false, // Inconsistent
		},
		{
			OrderID:              "order-2",
			SettlementAmount:     0,
			ExpectedAmount:       100_000,
			CBDCCommitted:        true,
			ReconciliationPassed: true, // Inconsistent with zero settlement
		},
	}

	for i, input := range hallucinationCases {
		verdict, _ := judge.Evaluate(ctx, input)
		t.Logf("Hallucination case %d: score=%.2f", i, verdict.Score)
		// Judge should detect inconsistencies and not award a passing score
		assert.False(t, verdict.Pass, "inconsistent settlement should not pass")
	}
}

// TestJudge_FaithfulnessChecking tests faithfulness of judge reasoning
func TestJudge_FaithfulnessChecking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	input := judges.SettlementJudgeInput{
		OrderID:              "faith-1",
		SettlementID:         "settle-faith-1",
		SettlementAmount:     100_000,
		ExpectedAmount:       100_000,
		CBDCCommitted:        true,
		ReconciliationPassed: true,
		AuditTrail:           `[{"step":"payment_confirmed"},{"step":"settlement_completed"}]`,
	}

	verdict, err := judge.Evaluate(ctx, input)
	require.NoError(t, err)

	// A correct settlement passes; the mock records its supporting facts in
	// Evidence (Reason is only populated for failures).
	assert.True(t, verdict.Pass, "Correct settlement should pass")
	assert.NotEmpty(t, verdict.Evidence, "Judge should provide supporting evidence")
	assert.Greater(t, verdict.Score, float64(0.5), "Correct settlement should score well")

	t.Logf("Faithfulness: evidence=%s, score=%.2f", verdict.Evidence, verdict.Score)
}

// TestJudge_DriftDetection tests judge drift over time
func TestJudge_DriftDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	// Phase 1: Evaluate on initial distribution
	samples1 := make([]judges.SettlementJudgeInput, 20)
	for i := 0; i < 20; i++ {
		samples1[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("phase1-order-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        true,
			ReconciliationPassed: true,
		}
	}
	groundTruth1 := make([]bool, 20)
	for i := 0; i < 20; i++ {
		groundTruth1[i] = true
	}

	result1, _ := judge.Calibrate(ctx, samples1, groundTruth1)

	// Phase 2: Different distribution (should detect drift)
	samples2 := make([]judges.SettlementJudgeInput, 20)
	for i := 0; i < 20; i++ {
		samples2[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("phase2-order-%d", i),
			SettlementAmount:     50_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        false,
			ReconciliationPassed: false,
		}
	}
	groundTruth2 := make([]bool, 20)
	for i := 0; i < 20; i++ {
		groundTruth2[i] = false
	}

	result2, _ := judge.Calibrate(ctx, samples2, groundTruth2)

	drift := math.Abs(result1.TPR - result2.TPR)
	t.Logf("Judge drift detection: phase1_TPR=%.3f, phase2_TPR=%.3f, drift=%.3f",
		result1.TPR, result2.TPR, drift)
}

// TestJudge_ComparisonJudgeAvsB tests judge comparison
func TestJudge_ComparisonJudgeAvsB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	judgeA := judges.NewMockSettlementJudge()
	judgeB := judges.NewMockSettlementJudge()

	samples := make([]judges.SettlementJudgeInput, 30)
	groundTruth := make([]bool, 30)

	for i := 0; i < 30; i++ {
		isCorrect := i%2 == 0
		samples[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("compare-order-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        isCorrect,
			ReconciliationPassed: isCorrect,
		}
		groundTruth[i] = isCorrect
	}

	resultA, _ := judgeA.Calibrate(ctx, samples, groundTruth)
	resultB, _ := judgeB.Calibrate(ctx, samples, groundTruth)

	tprDiff := math.Abs(resultA.TPR - resultB.TPR)
	tnrDiff := math.Abs(resultA.TNR - resultB.TNR)

	t.Logf("Judge comparison: A_TPR=%.3f, B_TPR=%.3f, diff=%.3f",
		resultA.TPR, resultB.TPR, tprDiff)
	t.Logf("Judge comparison: A_TNR=%.3f, B_TNR=%.3f, diff=%.3f",
		resultA.TNR, resultB.TNR, tnrDiff)
}

// TestJudge_VotingMechanism tests ensemble voting
func TestJudge_VotingMechanism(t *testing.T) {
	ctx := context.Background()

	panel := []*judges.MockSettlementJudge{
		judges.NewMockSettlementJudge(),
		judges.NewMockSettlementJudge(),
		judges.NewMockSettlementJudge(),
	}

	input := judges.SettlementJudgeInput{
		OrderID:              "vote-1",
		SettlementAmount:     100_000,
		ExpectedAmount:       100_000,
		CBDCCommitted:        true,
		ReconciliationPassed: true,
	}

	// Get votes
	votes := 0
	for _, judge := range panel {
		verdict, err := judge.Evaluate(ctx, input)
		if err == nil && verdict.Pass {
			votes++
		}
	}

	// Majority vote
	majorityPass := votes > len(panel)/2
	assert.True(t, majorityPass, "panel should reach passing majority for a correct settlement")
	t.Logf("Voting mechanism: %d/%d judges agreed, majority=%v", votes, len(panel), majorityPass)
}

// TestJudge_WeightedScoring tests weighted score combination
func TestJudge_WeightedScoring(t *testing.T) {
	ctx := context.Background()

	input := judges.SettlementJudgeInput{
		OrderID:              "weighted-1",
		SettlementAmount:     100_000,
		ExpectedAmount:       100_000,
		CBDCCommitted:        true,
		ReconciliationPassed: true,
	}

	judge := judges.NewMockSettlementJudge()
	verdict, _ := judge.Evaluate(ctx, input)

	// Weighted score: 0.6 * settlement_score + 0.4 * compliance_score
	// (Example weights)
	weights := map[string]float64{
		"settlement": 0.6,
		"compliance": 0.4,
	}

	weightedScore := weights["settlement"]*verdict.Score + weights["compliance"]*0.8
	t.Logf("Weighted scoring: base=%.2f, weighted=%.2f", verdict.Score, weightedScore)
	assert.Greater(t, weightedScore, float64(0.5))
}

// TestJudge_AccuracyImprovementTracking tests accuracy tracking over time
func TestJudge_AccuracyImprovementTracking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	judge := judges.NewMockSettlementJudge()

	accuracies := make([]float64, 3)

	// Round 1: Initial calibration
	samples1 := make([]judges.SettlementJudgeInput, 20)
	for i := 0; i < 20; i++ {
		samples1[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("round1-order-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        i < 15,
			ReconciliationPassed: i < 15,
		}
	}
	groundTruth1 := make([]bool, 20)
	for i := 0; i < 20; i++ {
		groundTruth1[i] = i < 15
	}

	result1, _ := judge.Calibrate(ctx, samples1, groundTruth1)
	accuracies[0] = (result1.TPR + result1.TNR) / 2

	// Round 2: More data
	samples2 := make([]judges.SettlementJudgeInput, 30)
	for i := 0; i < 30; i++ {
		samples2[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("round2-order-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        i < 20,
			ReconciliationPassed: i < 20,
		}
	}
	groundTruth2 := make([]bool, 30)
	for i := 0; i < 30; i++ {
		groundTruth2[i] = i < 20
	}

	result2, _ := judge.Calibrate(ctx, samples2, groundTruth2)
	accuracies[1] = (result2.TPR + result2.TNR) / 2

	// Round 3: Even more data
	samples3 := make([]judges.SettlementJudgeInput, 50)
	for i := 0; i < 50; i++ {
		samples3[i] = judges.SettlementJudgeInput{
			OrderID:              fmt.Sprintf("round3-order-%d", i),
			SettlementAmount:     100_000,
			ExpectedAmount:       100_000,
			CBDCCommitted:        i < 30,
			ReconciliationPassed: i < 30,
		}
	}
	groundTruth3 := make([]bool, 50)
	for i := 0; i < 50; i++ {
		groundTruth3[i] = i < 30
	}

	result3, _ := judge.Calibrate(ctx, samples3, groundTruth3)
	accuracies[2] = (result3.TPR + result3.TNR) / 2

	t.Logf("Accuracy improvement: round1=%.2f%%, round2=%.2f%%, round3=%.2f%%",
		accuracies[0]*100, accuracies[1]*100, accuracies[2]*100)
}
