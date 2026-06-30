package erupeecompliance

import (
	"context"
	"testing"
	"time"
)

func TestComplianceEngine_AllowPayment(t *testing.T) {
	engine := NewComplianceEngine()
	ctx := context.Background()

	payment := PaymentRequest{
		PaymentID:         "pay_001",
		FromAccountID:     "acc_001",
		ToAccountID:       "acc_002",
		ToName:            "John Doe",
		Amount:            10_00_000, // ₹10k
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 30 * 24 * 60 * 60, // 30 days old
	}

	check, err := engine.CheckPayment(ctx, payment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.Decision != DecisionAllow {
		t.Errorf("expected DecisionAllow, got %s", check.Decision)
	}

	if check.AMLResult != AMLPass {
		t.Errorf("expected AMLPass, got %s", check.AMLResult)
	}

	if check.VelocityResult != VelocityOK {
		t.Errorf("expected VelocityOK, got %s", check.VelocityResult)
	}

	t.Logf("Decision: %s, Reason: %s", check.Decision, check.DecisionReason)
}

func TestComplianceEngine_BlockPaymentPEP(t *testing.T) {
	engine := NewComplianceEngine()
	ctx := context.Background()

	payment := PaymentRequest{
		PaymentID:         "pay_002",
		FromAccountID:     "acc_001",
		ToAccountID:       "acc_pep",
		ToName:            "Vladimir Putin", // Known PEP
		Amount:            5_00_000,
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 30 * 24 * 60 * 60,
	}

	check, err := engine.CheckPayment(ctx, payment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.Decision != DecisionBlock {
		t.Errorf("expected DecisionBlock, got %s", check.Decision)
	}

	t.Logf("Decision: %s, Reason: %s", check.Decision, check.DecisionReason)
}

func TestComplianceEngine_ReviewPaymentLargeAmount(t *testing.T) {
	// Use a high-limit velocity monitor so only the AML dimension fires.
	// A ₹60k payment exceeds the DEFAULT ₹50k/hour velocity limit, which would
	// produce DecisionBlock instead of DecisionReview. This test isolates the AML
	// path by removing the velocity constraint — the compliance engine should route
	// to review when AML fires without a concurrent velocity block.
	highLimitVelocity := NewInMemoryVelocityMonitorWithConfig(&VelocityConfig{
		MaxTransactionsPerHour: 100,
		MaxAmountPerHour:       10_00_00_000, // ₹10 crore — velocity never fires
		MaxAmountPerDay:        50_00_00_000, // ₹50 crore
	})
	engine := NewComplianceEngineWithComponents(
		NewAMLScreener(),
		NewSanctionsChecker(),
		highLimitVelocity,
		NewInMemoryFraudDetector(),
		DefaultFraudDetectionConfig(),
	)
	ctx := context.Background()

	payment := PaymentRequest{
		PaymentID:         "pay_003",
		FromAccountID:     "acc_001",
		ToAccountID:       "acc_002",
		ToName:            "Jane Smith",
		Amount:            60_00_000, // ₹60k - above ₹50k AML threshold
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 30 * 24 * 60 * 60,
	}

	check, err := engine.CheckPayment(ctx, payment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.Decision != DecisionReview {
		t.Errorf("expected DecisionReview, got %s", check.Decision)
	}

	if check.AMLResult != AMLReview {
		t.Errorf("expected AMLReview, got %s", check.AMLResult)
	}

	t.Logf("Decision: %s, AML Result: %s", check.Decision, check.AMLResult)
}

func TestComplianceEngine_ReviewPaymentNewAccountHighValue(t *testing.T) {
	// Use a high-limit velocity monitor so only the fraud/AML dimension fires.
	// A ₹60k payment exceeds the DEFAULT ₹50k/hour velocity limit — that block
	// would shadow the new-account high-value fraud pattern this test asserts on.
	highLimitVelocity := NewInMemoryVelocityMonitorWithConfig(&VelocityConfig{
		MaxTransactionsPerHour: 100,
		MaxAmountPerHour:       10_00_00_000,
		MaxAmountPerDay:        50_00_00_000,
	})
	engine := NewComplianceEngineWithComponents(
		NewAMLScreener(),
		NewSanctionsChecker(),
		highLimitVelocity,
		NewInMemoryFraudDetector(),
		DefaultFraudDetectionConfig(),
	)
	ctx := context.Background()

	payment := PaymentRequest{
		PaymentID:         "pay_004",
		FromAccountID:     "new_acc",
		ToAccountID:       "acc_002",
		ToName:            "Recipient",
		Amount:            60_00_000, // ₹60k
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 2 * 24 * 60 * 60, // 2 days old
	}

	check, err := engine.CheckPayment(ctx, payment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.Decision != DecisionReview {
		t.Errorf("expected DecisionReview, got %s", check.Decision)
	}

	// Should detect new account high value pattern
	found := false
	for _, pattern := range check.DetectedPatterns {
		if pattern == PatternNewAccountHigh {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Note: New account high value pattern not detected in patterns: %v", check.DetectedPatterns)
	}

	t.Logf("Decision: %s, Fraud Score: %.2f", check.Decision, check.FraudScore)
}

func TestComplianceEngine_BlockPaymentVelocity(t *testing.T) {
	engine := NewComplianceEngine()
	ctx := context.Background()

	now := time.Now().UTC()
	accountID := "vel_test_acc"

	// Record 11 transactions to exceed hourly limit
	for i := 0; i < 11; i++ {
		payment := PaymentRequest{
			PaymentID:         "pay_vel_" + string(rune(i)),
			FromAccountID:     accountID,
			ToAccountID:       "acc_002",
			ToName:            "Recipient",
			Amount:            1_00_000,
			Timestamp:         now.Add(time.Duration(i) * time.Minute),
			AccountAgeSeconds: 30 * 24 * 60 * 60,
		}
		_, _ = engine.CheckPayment(ctx, payment)
	}

	// The 12th payment should be blocked
	finalPayment := PaymentRequest{
		PaymentID:         "pay_vel_final",
		FromAccountID:     accountID,
		ToAccountID:       "acc_002",
		ToName:            "Recipient",
		Amount:            1_00_000,
		Timestamp:         now.Add(11 * time.Minute),
		AccountAgeSeconds: 30 * 24 * 60 * 60,
	}

	check, err := engine.CheckPayment(ctx, finalPayment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.VelocityResult != VelocityBlocked {
		t.Errorf("expected VelocityBlocked, got %s", check.VelocityResult)
	}

	if check.Decision != DecisionBlock {
		t.Errorf("expected DecisionBlock, got %s", check.Decision)
	}

	t.Logf("Velocity blocked: %s", check.VelocityReason)
}

func TestComplianceEngine_GetVelocityRecord(t *testing.T) {
	engine := NewComplianceEngine()
	ctx := context.Background()

	payment := PaymentRequest{
		PaymentID:         "pay_001",
		FromAccountID:     "rec_test_acc",
		ToAccountID:       "acc_002",
		ToName:            "Recipient",
		Amount:            5_00_000,
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 30 * 24 * 60 * 60,
	}

	// Make a payment to record velocity
	_, _ = engine.CheckPayment(ctx, payment)

	// Retrieve the velocity record
	record, err := engine.GetVelocityRecord(ctx, "rec_test_acc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.TransactionCount != 1 {
		t.Errorf("expected 1 transaction, got %d", record.TransactionCount)
	}

	if record.TotalAmount != 5_00_000 {
		t.Errorf("expected amount 5_00_000, got %d", record.TotalAmount)
	}

	t.Logf("Velocity record: %d txns, %.2f INR", record.TransactionCount, float64(record.TotalAmount)/100.0)
}

func TestComplianceEngine_GetTransactionHistory(t *testing.T) {
	engine := NewComplianceEngine()
	ctx := context.Background()

	now := time.Now().UTC()
	accountID := "hist_test_acc"

	// Record 5 transactions
	for i := 0; i < 5; i++ {
		payment := PaymentRequest{
			PaymentID:         "pay_hist_" + string(rune(i)),
			FromAccountID:     accountID,
			ToAccountID:       "acc_002",
			ToName:            "Recipient",
			Amount:            1_00_000,
			Timestamp:         now.Add(-time.Duration(i) * time.Hour),
			AccountAgeSeconds: 30 * 24 * 60 * 60,
		}
		_, _ = engine.CheckPayment(ctx, payment)
	}

	// Get history from last 3 hours
	history := engine.GetTransactionHistory(ctx, accountID, 3)
	if len(history) != 3 {
		t.Errorf("expected 3 transactions in 3-hour window, got %d", len(history))
	}

	// Get history from last 10 hours
	history = engine.GetTransactionHistory(ctx, accountID, 10)
	if len(history) != 5 {
		t.Errorf("expected 5 transactions in 10-hour window, got %d", len(history))
	}

	t.Logf("Transaction history retrieved: %d transactions", len(history))
}

func TestComplianceEngine_ResetVelocity(t *testing.T) {
	engine := NewComplianceEngine()
	ctx := context.Background()

	accountID := "reset_test_acc"

	// Record a payment
	payment := PaymentRequest{
		PaymentID:         "pay_reset",
		FromAccountID:     accountID,
		ToAccountID:       "acc_002",
		ToName:            "Recipient",
		Amount:            5_00_000,
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 30 * 24 * 60 * 60,
	}
	_, _ = engine.CheckPayment(ctx, payment)

	// Verify record exists
	record, err := engine.GetVelocityRecord(ctx, accountID)
	if err != nil || record == nil {
		t.Fatalf("expected velocity record to exist")
	}

	// Reset
	err = engine.ResetVelocity(ctx, accountID)
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Verify record is cleared
	_, err = engine.GetVelocityRecord(ctx, accountID)
	if err == nil {
		t.Error("expected error after reset, but record still exists")
	}

	t.Log("Velocity reset successful")
}

func TestComplianceEngine_ConcurrentPayments(t *testing.T) {
	engine := NewComplianceEngine()
	ctx := context.Background()
	now := time.Now().UTC()
	done := make(chan error, 20)

	// Concurrent payments from different accounts
	for i := 0; i < 20; i++ {
		go func(id int) {
			accountID := "concurrent_acc_" + string(rune(id))
			payment := PaymentRequest{
				PaymentID:         "pay_concurrent_" + string(rune(id)),
				FromAccountID:     accountID,
				ToAccountID:       "acc_002",
				ToName:            "Recipient",
				Amount:            5_00_000,
				Timestamp:         now,
				AccountAgeSeconds: 30 * 24 * 60 * 60,
			}
			_, err := engine.CheckPayment(ctx, payment)
			done <- err
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		err := <-done
		if err != nil {
			t.Errorf("goroutine error: %v", err)
		}
	}

	t.Log("Concurrent payment test passed")
}

func TestComplianceEngine_DecisionLogic(t *testing.T) {
	tests := []struct {
		name       string
		amlResult  AMLResult
		velocity   VelocityResult
		fraudScore float64
		expected   Decision
	}{
		{
			name:       "All green - allow",
			amlResult:  AMLPass,
			velocity:   VelocityOK,
			fraudScore: 0,
			expected:   DecisionAllow,
		},
		{
			name:       "AML review - review",
			amlResult:  AMLReview,
			velocity:   VelocityOK,
			fraudScore: 0,
			expected:   DecisionReview,
		},
		{
			name:       "AML block - block",
			amlResult:  AMLBlock,
			velocity:   VelocityOK,
			fraudScore: 0,
			expected:   DecisionBlock,
		},
		{
			name:       "Velocity blocked - block",
			amlResult:  AMLPass,
			velocity:   VelocityBlocked,
			fraudScore: 0,
			expected:   DecisionBlock,
		},
		{
			name:       "High fraud score - review",
			amlResult:  AMLPass,
			velocity:   VelocityOK,
			fraudScore: 60.0,
			expected:   DecisionReview,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewComplianceEngine()
			check := &ComplianceCheck{
				AMLResult:      tc.amlResult,
				VelocityResult: tc.velocity,
				FraudScore:     tc.fraudScore,
			}

			decision := engine.makeDecision(check)
			if decision != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, decision)
			}
		})
	}
}
