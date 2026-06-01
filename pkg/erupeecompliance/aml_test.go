package erupeecompliance

import (
	"context"
	"testing"
	"time"
)

func TestAMLScreener_PassFlow(t *testing.T) {
	screener := NewAMLScreener()
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

	check, err := screener.CheckPayment(ctx, payment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.AMLResult != AMLPass {
		t.Errorf("expected AMLPass, got %s", check.AMLResult)
	}

	if check.Decision != DecisionAllow {
		t.Errorf("expected DecisionAllow, got %s", check.Decision)
	}
}

func TestAMLScreener_PEPBlock(t *testing.T) {
	screener := NewAMLScreener()
	ctx := context.Background()

	payment := PaymentRequest{
		PaymentID:         "pay_002",
		FromAccountID:     "acc_001",
		ToAccountID:       "acc_pep",
		ToName:            "Vladimir Putin", // Known PEP
		Amount:            5_00_000,         // ₹5k
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 30 * 24 * 60 * 60,
	}

	check, err := screener.CheckPayment(ctx, payment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.AMLResult != AMLBlock {
		t.Errorf("expected AMLBlock, got %s", check.AMLResult)
	}

	if check.Decision != DecisionBlock {
		t.Errorf("expected DecisionBlock, got %s", check.Decision)
	}
}

func TestAMLScreener_LargeAmountReview(t *testing.T) {
	screener := NewAMLScreener()
	ctx := context.Background()

	payment := PaymentRequest{
		PaymentID:         "pay_003",
		FromAccountID:     "acc_001",
		ToAccountID:       "acc_002",
		ToName:            "Jane Smith",
		Amount:            60_00_000, // ₹60k - above ₹50k threshold
		Timestamp:         time.Now().UTC(),
		AccountAgeSeconds: 30 * 24 * 60 * 60,
	}

	check, err := screener.CheckPayment(ctx, payment)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if check.AMLResult != AMLReview {
		t.Errorf("expected AMLReview, got %s", check.AMLResult)
	}

	if check.Decision != DecisionReview {
		t.Errorf("expected DecisionReview, got %s", check.Decision)
	}
}

func TestSanctionsChecker_BlockSanctionedCountry(t *testing.T) {
	checker := NewSanctionsChecker()
	ctx := context.Background()

	blocked, reason := checker.CheckAccount(ctx, "acc_iran", "IR")
	if !blocked {
		t.Error("expected sanctions check to block Iran")
	}

	if reason == "" {
		t.Error("expected reason to be provided")
	}

	t.Logf("Sanctions check reason: %s", reason)
}

func TestSanctionsChecker_AllowNormalCountry(t *testing.T) {
	checker := NewSanctionsChecker()
	ctx := context.Background()

	blocked, reason := checker.CheckAccount(ctx, "acc_india", "IN")
	if blocked {
		t.Error("expected sanctions check to allow India")
	}

	if reason != "" {
		t.Logf("Unexpected reason: %s", reason)
	}
}
