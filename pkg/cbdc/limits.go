// limits.go — RBI compliance limits validator for CBDC transactions.
//
// Enforces per-account daily limits and single-transaction caps
// as defined by RBI e-Rupee guidelines.
package cbdc

import (
	"fmt"
	"sync"
	"time"
)

// RBILimitsValidator enforces transaction limits per RBI guidelines.
type RBILimitsValidator struct {
	mu       sync.RWMutex
	limits   RBILimits
	accounts map[string]*AccountLimitState
}

// NewRBILimitsValidator creates a validator with RBI default limits.
func NewRBILimitsValidator() *RBILimitsValidator {
	return &RBILimitsValidator{
		limits:   DefaultRBILimits(),
		accounts: make(map[string]*AccountLimitState),
	}
}

// NewRBILimitsValidatorWithLimits creates a validator with custom limits.
func NewRBILimitsValidatorWithLimits(limits RBILimits) *RBILimitsValidator {
	return &RBILimitsValidator{
		limits:   limits,
		accounts: make(map[string]*AccountLimitState),
	}
}

// ValidatePayment checks if a transaction adheres to RBI limits.
// Returns (allowed, reason) where reason explains any rejection.
func (v *RBILimitsValidator) ValidatePayment(account string, amount int64, txnType TransactionType) (bool, string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if amount <= 0 {
		return false, "transaction amount must be positive"
	}

	// Check single transaction limit
	if amount > v.limits.SingleTransactionLimit {
		return false, fmt.Sprintf(
			"transaction exceeds single limit: %d paise (₹%.2f) > %d paise (₹%.2f)",
			amount, float64(amount)/100.0,
			v.limits.SingleTransactionLimit, float64(v.limits.SingleTransactionLimit)/100.0,
		)
	}

	// Get or create account limit state
	state, exists := v.accounts[account]
	if !exists {
		state = &AccountLimitState{
			Account:     account,
			LastResetAt: time.Now().UTC(),
		}
		v.accounts[account] = state
	} else {
		// Check if daily reset is needed (UTC midnight boundary)
		v.resetDailyLimitsIfNeeded(state)
	}

	// Check daily limits based on transaction type
	switch txnType {
	case TypeP2P:
		if state.DailyP2PUsed+amount > v.limits.DailyP2PLimit {
			return false, fmt.Sprintf(
				"P2P limit exceeded: %d paise used + %d paise requested > %d paise daily limit (₹%.2f)",
				state.DailyP2PUsed, amount,
				v.limits.DailyP2PLimit, float64(v.limits.DailyP2PLimit)/100.0,
			)
		}

	case TypeMerchant:
		if state.DailyMerchantUsed+amount > v.limits.DailyMerchantLimit {
			return false, fmt.Sprintf(
				"Merchant limit exceeded: %d paise used + %d paise requested > %d paise daily limit (₹%.2f)",
				state.DailyMerchantUsed, amount,
				v.limits.DailyMerchantLimit, float64(v.limits.DailyMerchantLimit)/100.0,
			)
		}

	default:
		return false, fmt.Sprintf("unknown transaction type: %s", txnType)
	}

	return true, ""
}

// CommitTransaction records a payment against the account's daily limits.
// Must be called after ValidatePayment returns true.
func (v *RBILimitsValidator) CommitTransaction(account string, amount int64, txnType TransactionType) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	state, exists := v.accounts[account]
	if !exists {
		state = &AccountLimitState{
			Account:     account,
			LastResetAt: time.Now().UTC(),
		}
		v.accounts[account] = state
	} else {
		v.resetDailyLimitsIfNeeded(state)
	}

	// Update limit counters
	switch txnType {
	case TypeP2P:
		state.DailyP2PUsed += amount

	case TypeMerchant:
		state.DailyMerchantUsed += amount

	default:
		return fmt.Errorf("unknown transaction type: %s", txnType)
	}

	return nil
}

// GetAccountLimits retrieves the current limit state for an account.
func (v *RBILimitsValidator) GetAccountLimits(account string) (*AccountLimitState, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	state, exists := v.accounts[account]
	if !exists {
		return nil, false
	}

	// Return a copy
	stateCopy := *state
	return &stateCopy, true
}

// ResetAccountLimits resets daily limits for an account (for testing/admin).
func (v *RBILimitsValidator) ResetAccountLimits(account string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.accounts[account] = &AccountLimitState{
		Account:     account,
		LastResetAt: time.Now().UTC(),
	}
}

// UpdateLimits changes the global limits (for testing/admin).
func (v *RBILimitsValidator) UpdateLimits(limits RBILimits) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if limits.DailyP2PLimit > 0 {
		v.limits.DailyP2PLimit = limits.DailyP2PLimit
	}
	if limits.DailyMerchantLimit > 0 {
		v.limits.DailyMerchantLimit = limits.DailyMerchantLimit
	}
	if limits.SingleTransactionLimit > 0 {
		v.limits.SingleTransactionLimit = limits.SingleTransactionLimit
	}
}

// GetLimits returns the current global limits.
func (v *RBILimitsValidator) GetLimits() RBILimits {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.limits
}

// resetDailyLimitsIfNeeded checks if the account's daily limits should be reset (UTC midnight).
// Assumes the caller holds the mutex lock.
func (v *RBILimitsValidator) resetDailyLimitsIfNeeded(state *AccountLimitState) {
	now := time.Now().UTC()
	lastReset := state.LastResetAt.UTC()

	// Check if we've crossed a UTC midnight boundary
	if now.Year() != lastReset.Year() ||
		now.YearDay() != lastReset.YearDay() {
		// New day in UTC, reset limits
		state.DailyP2PUsed = 0
		state.DailyMerchantUsed = 0
		state.LastResetAt = now
	}
}
