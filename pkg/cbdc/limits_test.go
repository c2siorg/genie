package cbdc

import (
	"strings"
	"testing"
	"time"
)

func TestNewRBILimitsValidator(t *testing.T) {
	validator := NewRBILimitsValidator()
	if validator == nil {
		t.Fatal("validator should not be nil")
	}

	limits := validator.GetLimits()
	if limits.DailyP2PLimit != 10_000_000 {
		t.Errorf("P2P limit should be 10M paise, got %d", limits.DailyP2PLimit)
	}
	if limits.DailyMerchantLimit != 50_000_000 {
		t.Errorf("merchant limit should be 50M paise, got %d", limits.DailyMerchantLimit)
	}
	if limits.SingleTransactionLimit != 5_000_000 {
		t.Errorf("single txn limit should be 5M paise, got %d", limits.SingleTransactionLimit)
	}
}

func TestNewRBILimitsValidatorWithCustomLimits(t *testing.T) {
	customLimits := RBILimits{
		DailyP2PLimit:          1_000_000,
		DailyMerchantLimit:     5_000_000,
		SingleTransactionLimit: 500_000,
	}

	validator := NewRBILimitsValidatorWithLimits(customLimits)
	limits := validator.GetLimits()

	if limits.DailyP2PLimit != 1_000_000 {
		t.Error("custom P2P limit not applied")
	}
}

func TestValidatePaymentSingleTransactionLimit(t *testing.T) {
	validator := NewRBILimitsValidator()

	tests := []struct {
		name      string
		amount    int64
		wantAllow bool
	}{
		{"at limit", 5_000_000, true},
		{"below limit", 2_500_000, true},
		{"exceeds limit", 5_000_001, false},
		{"double limit", 10_000_000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _ := validator.ValidatePayment("account1", tt.amount, TypeP2P)
			if allowed != tt.wantAllow {
				t.Errorf("amount %d: got %v, want %v", tt.amount, allowed, tt.wantAllow)
			}
		})
	}
}

func TestValidatePaymentP2PDaily(t *testing.T) {
	validator := NewRBILimitsValidator()

	// First transaction: ₹50,000
	allowed, msg := validator.ValidatePayment("account1", 5_000_000, TypeP2P)
	if !allowed {
		t.Errorf("first transaction should be allowed: %s", msg)
	}
	validator.CommitTransaction("account1", 5_000_000, TypeP2P)

	// Second transaction: ₹50,000 (total ₹100,000)
	allowed, msg = validator.ValidatePayment("account1", 5_000_000, TypeP2P)
	if !allowed {
		t.Errorf("second transaction should be allowed: %s", msg)
	}
	validator.CommitTransaction("account1", 5_000_000, TypeP2P)

	// Third transaction: ₹50,000 (total would be ₹150,000, exceeds ₹100,000 limit)
	allowed, msg = validator.ValidatePayment("account1", 5_000_000, TypeP2P)
	if allowed {
		t.Error("should reject transaction exceeding P2P limit")
	}

	if !strings.Contains(msg, "P2P limit exceeded") {
		t.Errorf("error message should mention P2P limit, got: %s", msg)
	}
}

func TestValidatePaymentMerchantDaily(t *testing.T) {
	validator := NewRBILimitsValidator()

	// Merchant limit is 500K (50,000,000 paise), single txn limit is 50K (5M paise)
	// Commit 10 transactions of 4M paise each = 40M total
	for i := 0; i < 10; i++ {
		validator.CommitTransaction("account2", 4_000_000, TypeMerchant)
	}

	// Try to commit 5M more (4M + 5M = 9M remaining, within 50M daily limit)
	allowed, msg := validator.ValidatePayment("account2", 5_000_000, TypeMerchant)
	if !allowed {
		t.Errorf("should allow transaction within limit: %s", msg)
	}
	validator.CommitTransaction("account2", 5_000_000, TypeMerchant)

	// Try to commit 1M more (45M used, 5M remaining, but trying 1M)
	// This should succeed as 45M + 1M = 46M < 50M limit
	allowed, msg = validator.ValidatePayment("account2", 1_000_000, TypeMerchant)
	if !allowed {
		t.Errorf("should allow transaction: %s", msg)
	}
	validator.CommitTransaction("account2", 1_000_000, TypeMerchant)

	// Now we're at 46M, try to commit 5M more - should fail (46M + 5M > 50M)
	allowed, msg = validator.ValidatePayment("account2", 5_000_000, TypeMerchant)
	if allowed {
		t.Error("should reject transaction exceeding merchant limit")
	}

	if !strings.Contains(msg, "Merchant limit exceeded") {
		t.Errorf("error message should mention merchant limit, got: %s", msg)
	}
}

func TestValidatePaymentNegativeAndZero(t *testing.T) {
	validator := NewRBILimitsValidator()

	tests := []struct {
		name   string
		amount int64
	}{
		{"zero amount", 0},
		{"negative amount", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, msg := validator.ValidatePayment("account", tt.amount, TypeP2P)
			if allowed {
				t.Errorf("should reject %s", tt.name)
			}
			if !strings.Contains(msg, "positive") {
				t.Errorf("error should mention positive amount, got: %s", msg)
			}
		})
	}
}

func TestGetAccountLimits(t *testing.T) {
	validator := NewRBILimitsValidator()

	// Account not yet tracked
	state, found := validator.GetAccountLimits("new-account")
	if found {
		t.Error("new account should not exist yet")
	}

	// Use account
	validator.ValidatePayment("new-account", 1_000_000, TypeP2P)
	validator.CommitTransaction("new-account", 1_000_000, TypeP2P)

	// Now should exist
	state, found = validator.GetAccountLimits("new-account")
	if !found {
		t.Fatal("account should exist after use")
	}

	if state.DailyP2PUsed != 1_000_000 {
		t.Errorf("P2P used should be 1M, got %d", state.DailyP2PUsed)
	}

	if state.DailyMerchantUsed != 0 {
		t.Errorf("merchant used should be 0, got %d", state.DailyMerchantUsed)
	}
}

func TestResetAccountLimits(t *testing.T) {
	validator := NewRBILimitsValidator()

	// Use account
	validator.CommitTransaction("reset-account", 1_000_000, TypeP2P)
	validator.CommitTransaction("reset-account", 2_000_000, TypeMerchant)

	// Verify usage
	state, _ := validator.GetAccountLimits("reset-account")
	if state.DailyP2PUsed != 1_000_000 || state.DailyMerchantUsed != 2_000_000 {
		t.Error("usage not recorded correctly")
	}

	// Reset
	validator.ResetAccountLimits("reset-account")

	// Verify reset
	state, _ = validator.GetAccountLimits("reset-account")
	if state.DailyP2PUsed != 0 || state.DailyMerchantUsed != 0 {
		t.Error("limits not reset")
	}
}

func TestUpdateLimits(t *testing.T) {
	validator := NewRBILimitsValidator()

	// Update limits
	newLimits := RBILimits{
		DailyP2PLimit:          1_000_000,
		DailyMerchantLimit:     5_000_000,
		SingleTransactionLimit: 500_000,
	}

	validator.UpdateLimits(newLimits)

	// Verify updated
	limits := validator.GetLimits()
	if limits.DailyP2PLimit != 1_000_000 {
		t.Error("P2P limit not updated")
	}

	// Test validation with new limits
	allowed, _ := validator.ValidatePayment("account", 501_000, TypeP2P)
	if allowed {
		t.Error("should reject transaction exceeding new single limit")
	}
}

func TestDailyLimitReset(t *testing.T) {
	validator := NewRBILimitsValidator()

	// Create account state with usage
	validator.CommitTransaction("reset-account", 1_000_000, TypeP2P)

	state, _ := validator.GetAccountLimits("reset-account")
	if state.DailyP2PUsed == 0 {
		t.Fatal("usage should be recorded")
	}

	// Manually set LastResetAt to yesterday (force reset on next operation)
	yesterday := time.Now().UTC().Add(-25 * time.Hour)
	validator.mu.Lock()
	validator.accounts["reset-account"].LastResetAt = yesterday
	validator.mu.Unlock()

	// Sleep a tiny bit to ensure time changes
	time.Sleep(10 * time.Millisecond)

	// Next validate should trigger reset
	validator.ValidatePayment("reset-account", 1_000_000, TypeP2P)

	// Check if reset occurred
	state, _ = validator.GetAccountLimits("reset-account")
	if state.DailyP2PUsed != 0 {
		t.Errorf("P2P used should be reset to 0, got %d", state.DailyP2PUsed)
	}

	// LastResetAt should be much more recent (at least >= to a time after yesterday)
	if !state.LastResetAt.After(yesterday) {
		t.Errorf("LastResetAt should have been updated. old=%v, new=%v", yesterday, state.LastResetAt)
	}
}

func TestCommitTransactionUnknownType(t *testing.T) {
	validator := NewRBILimitsValidator()

	err := validator.CommitTransaction("account", 1_000_000, TransactionType("unknown"))
	if err == nil {
		t.Error("should error on unknown transaction type")
	}
}

func TestValidatePaymentUnknownType(t *testing.T) {
	validator := NewRBILimitsValidator()

	allowed, msg := validator.ValidatePayment("account", 1_000_000, TransactionType("unknown"))
	if allowed {
		t.Error("should not allow unknown transaction type")
	}

	if !strings.Contains(msg, "unknown transaction type") {
		t.Errorf("error should mention unknown type, got: %s", msg)
	}
}

func TestSeparateAccountLimits(t *testing.T) {
	// Use custom limits with larger single txn limit to avoid conflicts
	customLimits := RBILimits{
		DailyP2PLimit:          100_000_000, // 1M rupees
		DailyMerchantLimit:     500_000_000, // 5M rupees
		SingleTransactionLimit: 50_000_000,  // 500K rupees (increased from 50K)
	}
	validator := NewRBILimitsValidatorWithLimits(customLimits)

	// Account 1: commit 10M twice = 20M
	validator.CommitTransaction("account1", 10_000_000, TypeP2P)
	validator.CommitTransaction("account1", 10_000_000, TypeP2P)

	// Account 2 uses separate allocation - should have fresh 100M limit
	allowed, msg := validator.ValidatePayment("account2", 10_000_000, TypeP2P)
	if !allowed {
		t.Errorf("account2 should have separate P2P limit: %s", msg)
	}

	validator.CommitTransaction("account2", 10_000_000, TypeP2P)

	// Account 1: commit 50M more (hitting single txn limit) = 70M total
	allowed, _ = validator.ValidatePayment("account1", 50_000_000, TypeP2P)
	if !allowed {
		t.Error("account1 should allow 50M more (total 70M < 100M)")
	}
	validator.CommitTransaction("account1", 50_000_000, TypeP2P)

	// Account 1: try 50M more (70M + 50M > 100M) - should fail
	allowed1, _ := validator.ValidatePayment("account1", 50_000_000, TypeP2P)
	if allowed1 {
		t.Error("account1 should reject exceeding limit")
	}

	// Account 2: should still have plenty (has used 10M of 100M)
	allowed2, _ := validator.ValidatePayment("account2", 50_000_000, TypeP2P)
	if !allowed2 {
		t.Error("account2 should have separate allocation")
	}
}

func TestMixedTransactionTypes(t *testing.T) {
	// Use custom limits with larger caps
	customLimits := RBILimits{
		DailyP2PLimit:          100_000_000, // 1M rupees
		DailyMerchantLimit:     500_000_000, // 5M rupees
		SingleTransactionLimit: 50_000_000,  // 500K rupees (increased from 50K)
	}
	validator := NewRBILimitsValidatorWithLimits(customLimits)

	account := "mixed-account"

	// P2P: 10M paise
	validator.CommitTransaction(account, 10_000_000, TypeP2P)

	// Merchant: 10M paise
	validator.CommitTransaction(account, 10_000_000, TypeMerchant)

	// P2P: 10M more (total 20M, within 100M P2P daily limit)
	allowed, _ := validator.ValidatePayment(account, 10_000_000, TypeP2P)
	if !allowed {
		t.Error("should allow P2P within limit")
	}
	validator.CommitTransaction(account, 10_000_000, TypeP2P)

	// Merchant: 20M more (total 30M, within 500M merchant daily limit)
	allowed, _ = validator.ValidatePayment(account, 20_000_000, TypeMerchant)
	if !allowed {
		t.Error("should allow merchant within limit")
	}
	validator.CommitTransaction(account, 20_000_000, TypeMerchant)

	// P2P: Now at 20M used, add 50M more for exactly 70M
	allowed, _ = validator.ValidatePayment(account, 50_000_000, TypeP2P)
	if !allowed {
		t.Error("should allow P2P (70M < 100M)")
	}
	validator.CommitTransaction(account, 50_000_000, TypeP2P)

	// P2P: try 50M more (70M + 50M = 120M > 100M limit) - should fail
	allowed, _ = validator.ValidatePayment(account, 50_000_000, TypeP2P)
	if allowed {
		t.Error("should reject P2P exceeding daily limit")
	}

	// Merchant: still has room (30M + 20M = 50M < 500M)
	allowed, _ = validator.ValidatePayment(account, 20_000_000, TypeMerchant)
	if !allowed {
		t.Error("should allow merchant with separate tracking")
	}
}

func TestDefaultRBILimits(t *testing.T) {
	limits := DefaultRBILimits()

	if limits.DailyP2PLimit != 10_000_000 {
		t.Errorf("default P2P limit incorrect: %d", limits.DailyP2PLimit)
	}

	if limits.DailyMerchantLimit != 50_000_000 {
		t.Errorf("default merchant limit incorrect: %d", limits.DailyMerchantLimit)
	}

	if limits.SingleTransactionLimit != 5_000_000 {
		t.Errorf("default single txn limit incorrect: %d", limits.SingleTransactionLimit)
	}
}

func TestNewAccountLimitState(t *testing.T) {
	state := NewAccountLimitState("test-account")

	if state.Account != "test-account" {
		t.Error("account not set")
	}

	if state.DailyP2PUsed != 0 || state.DailyMerchantUsed != 0 {
		t.Error("usage should be zero")
	}

	if state.LastResetAt.IsZero() {
		t.Error("LastResetAt should be set")
	}
}
