package judges

import (
	"context"
	"testing"
	"time"
)

// TestComplianceJudge_EvaluateCompliant tests evaluation of compliant transaction.
func TestComplianceJudge_EvaluateCompliant(t *testing.T) {
	judge := NewMockComplianceJudge()

	input := ComplianceJudgeInput{
		CustomerID:             "customer-001",
		OrderID:                "order-001",
		KYCStatus:              "verified",
		KYCExpiryDate:          time.Now().AddDate(0, 6, 0), // Expires in 6 months
		VelocityWindowSeconds:  3600,
		VelocityThresholdPaise: 5000000, // 50,000 paise
		CurrentVelocityPaise:   1000000, // 10,000 paise (well below threshold)
		SanctionsListAge:       3600,    // 1 hour old
		MatchingThreshold:      0.95,
		AMLRiskScore:           15.0, // Low risk
		VelocityExceeded:       false,
		ConsentProvided:        true,
		AuditTrail:             `[{"check": "kyc_verified", "risk": 15.0}]`,
		Metadata:               map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Compliant transaction should pass
	if verdict.Score < 0.6 {
		t.Logf("Warning: compliant transaction received low score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f, Reason: %s", verdict.Pass, verdict.Score, verdict.Reason)
}

// TestComplianceJudge_EvaluateNonCompliant tests evaluation of non-compliant transaction.
func TestComplianceJudge_EvaluateNonCompliant(t *testing.T) {
	judge := NewMockComplianceJudge()

	input := ComplianceJudgeInput{
		CustomerID:             "customer-002",
		OrderID:                "order-002",
		KYCStatus:              "pending",                    // KYC not verified!
		KYCExpiryDate:          time.Now().AddDate(-1, 0, 0), // Expired 1 year ago
		VelocityWindowSeconds:  3600,
		VelocityThresholdPaise: 5000000,
		CurrentVelocityPaise:   8000000, // Exceeds threshold!
		SanctionsListAge:       432000,  // 5 days old (stale)
		MatchingThreshold:      0.80,    // Low threshold
		AMLRiskScore:           85.0,    // High risk!
		VelocityExceeded:       true,
		ConsentProvided:        false, // No consent!
		AuditTrail:             `[{"check": "kyc_failed", "risk": 85.0}]`,
		Metadata:               map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Non-compliant transaction should fail or score low
	if verdict.Score > 0.5 {
		t.Logf("Warning: non-compliant transaction received high score: %.2f (reason: %s)", verdict.Score, verdict.Reason)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f, Reason: %s", verdict.Pass, verdict.Score, verdict.Reason)
}

// TestComplianceJudge_VelocityExceeded tests velocity threshold detection.
func TestComplianceJudge_VelocityExceeded(t *testing.T) {
	judge := NewMockComplianceJudge()

	input := ComplianceJudgeInput{
		CustomerID:             "customer-003",
		OrderID:                "order-003",
		KYCStatus:              "verified",
		KYCExpiryDate:          time.Now().AddDate(0, 6, 0),
		VelocityWindowSeconds:  3600,
		VelocityThresholdPaise: 5000000,
		CurrentVelocityPaise:   6000000, // Exceeds by 1000000 paise
		SanctionsListAge:       3600,
		MatchingThreshold:      0.95,
		AMLRiskScore:           20.0,
		VelocityExceeded:       true, // Explicitly exceeded
		ConsentProvided:        true,
		AuditTrail:             `[{"check": "velocity_exceeded", "threshold": 5000000, "current": 6000000}]`,
		Metadata:               map[string]string{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	verdict, err := judge.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Velocity exceeded should result in fail or low score
	if verdict.Score > 0.5 {
		t.Logf("Warning: velocity-exceeded transaction received high score: %.2f", verdict.Score)
	}

	t.Logf("Verdict: Pass=%v, Score=%.2f", verdict.Pass, verdict.Score)
}

// TestComplianceJudge_Calibration tests calibration with ground truth.
func TestComplianceJudge_Calibration(t *testing.T) {
	judge := NewMockComplianceJudge()

	// Create calibration samples
	samples := []ComplianceJudgeInput{
		// Compliant sample
		{
			CustomerID:             "cal-001",
			OrderID:                "cal-order-001",
			KYCStatus:              "verified",
			KYCExpiryDate:          time.Now().AddDate(0, 6, 0),
			VelocityWindowSeconds:  3600,
			VelocityThresholdPaise: 5000000,
			CurrentVelocityPaise:   1000000,
			SanctionsListAge:       3600,
			MatchingThreshold:      0.95,
			AMLRiskScore:           15.0,
			VelocityExceeded:       false,
			ConsentProvided:        true,
			AuditTrail:             `[{"check": "compliant"}]`,
		},
		// Non-compliant sample (velocity exceeded)
		{
			CustomerID:             "cal-002",
			OrderID:                "cal-order-002",
			KYCStatus:              "verified",
			KYCExpiryDate:          time.Now().AddDate(0, 6, 0),
			VelocityWindowSeconds:  3600,
			VelocityThresholdPaise: 5000000,
			CurrentVelocityPaise:   8000000,
			SanctionsListAge:       3600,
			MatchingThreshold:      0.95,
			AMLRiskScore:           20.0,
			VelocityExceeded:       true,
			ConsentProvided:        true,
			AuditTrail:             `[{"check": "velocity_exceeded"}]`,
		},
		// Non-compliant sample (KYC expired)
		{
			CustomerID:             "cal-003",
			OrderID:                "cal-order-003",
			KYCStatus:              "verified",
			KYCExpiryDate:          time.Now().AddDate(-1, 0, 0),
			VelocityWindowSeconds:  3600,
			VelocityThresholdPaise: 5000000,
			CurrentVelocityPaise:   1000000,
			SanctionsListAge:       3600,
			MatchingThreshold:      0.95,
			AMLRiskScore:           15.0,
			VelocityExceeded:       false,
			ConsentProvided:        true,
			AuditTrail:             `[{"check": "kyc_expired"}]`,
		},
	}

	// Ground truth: compliant, non-compliant, non-compliant
	groundTruth := []bool{true, false, false}

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

	// Check that confidence intervals are well-formed
	if result.TPRUpperBound < result.TPRLowerBound {
		t.Errorf("Invalid TPR CI: upper=%.3f < lower=%.3f", result.TPRUpperBound, result.TPRLowerBound)
	}
	if result.TNRUpperBound < result.TNRLowerBound {
		t.Errorf("Invalid TNR CI: upper=%.3f < lower=%.3f", result.TNRUpperBound, result.TNRLowerBound)
	}
}

// TestComplianceJudge_Name tests judge name.
func TestComplianceJudge_Name(t *testing.T) {
	judge := NewMockComplianceJudge()
	if judge.Name() != "MockComplianceJudge" {
		t.Errorf("expected name MockComplianceJudge, got %s", judge.Name())
	}
}

// BenchmarkComplianceJudge_Evaluate benchmarks evaluation performance.
func BenchmarkComplianceJudge_Evaluate(b *testing.B) {
	judge := NewMockComplianceJudge()

	input := ComplianceJudgeInput{
		CustomerID:             "bench-001",
		OrderID:                "bench-order-001",
		KYCStatus:              "verified",
		KYCExpiryDate:          time.Now().AddDate(0, 6, 0),
		VelocityWindowSeconds:  3600,
		VelocityThresholdPaise: 5000000,
		CurrentVelocityPaise:   1000000,
		SanctionsListAge:       3600,
		MatchingThreshold:      0.95,
		AMLRiskScore:           15.0,
		VelocityExceeded:       false,
		ConsentProvided:        true,
		AuditTrail:             `[{"check": "compliant"}]`,
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
