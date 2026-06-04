// Package erupeecompliance provides payment compliance screening for e-Rupee flows,
// including AML screening, velocity checks, and fraud pattern detection.
//
// The compliance pipeline:
// 1. AML screening: beneficiary risk checks (PEP, sanctions, adverse media)
// 2. Velocity monitoring: transaction counts and amounts per time window
// 3. Fraud detection: pattern-based anomaly detection (structuring, round-tripping, etc.)
// 4. Decision: allow, review, or block based on all checks
package erupeecompliance

import (
	"time"
)

// Decision represents the final compliance decision.
type Decision string

const (
	DecisionAllow  Decision = "allow"  // Transaction allowed
	DecisionReview Decision = "review" // Requires manual review
	DecisionBlock  Decision = "block"  // Transaction blocked
)

// AMLResult represents the outcome of AML screening.
type AMLResult string

const (
	AMLPass   AMLResult = "pass"   // No AML concerns
	AMLReview AMLResult = "review" // Elevated risk; requires review
	AMLBlock  AMLResult = "block"  // Transaction blocked (PEP, sanctions, etc.)
)

// VelocityResult represents the outcome of velocity checks.
type VelocityResult string

const (
	VelocityOK      VelocityResult = "ok"      // Within velocity limits
	VelocityWarning VelocityResult = "warning" // Approaching limits
	VelocityBlocked VelocityResult = "blocked" // Velocity limit exceeded
)

// FraudPattern represents a detected fraud pattern.
type FraudPattern string

const (
	PatternStructuring    FraudPattern = "structuring"      // Small txns to avoid limits
	PatternRoundTripping  FraudPattern = "round_tripping"   // Send→receive cycle
	PatternVelocitySpike  FraudPattern = "velocity_spike"   // 100%+ frequency increase
	PatternNewAccountHigh FraudPattern = "new_account_high" // New account, high value
)

// ComplianceCheck represents the complete compliance assessment for a payment.
type ComplianceCheck struct {
	// PaymentID uniquely identifies the payment being checked.
	PaymentID string `json:"payment_id"`

	// AMLResult is the outcome of anti-money laundering screening.
	AMLResult AMLResult `json:"aml_result"` // pass | review | block

	// AMLReason explains the AML result.
	AMLReason string `json:"aml_reason"`

	// VelocityResult is the outcome of velocity checks.
	VelocityResult VelocityResult `json:"velocity_result"` // ok | warning | blocked

	// VelocityReason explains the velocity result.
	VelocityReason string `json:"velocity_reason"`

	// FraudScore is a composite fraud risk score (0-100).
	FraudScore float64 `json:"fraud_score"`

	// DetectedPatterns lists any fraud patterns identified.
	DetectedPatterns []FraudPattern `json:"detected_patterns"`

	// FraudReason explains the fraud score.
	FraudReason string `json:"fraud_reason"`

	// Decision is the final compliance decision (allow/review/block).
	Decision Decision `json:"decision"`

	// DecisionReason explains the final decision.
	DecisionReason string `json:"decision_reason"`

	// CheckedAt is when the check was performed.
	CheckedAt time.Time `json:"checked_at"`
}

// VelocityRecord tracks transaction activity for velocity monitoring.
type VelocityRecord struct {
	// AccountID uniquely identifies the account.
	AccountID string `json:"account_id"`

	// TransactionCount is the number of transactions in the current period.
	TransactionCount int `json:"transaction_count"`

	// TotalAmount is the sum of transaction amounts in the current period (in paise).
	TotalAmount int64 `json:"total_amount"`

	// Period is the time window for this record (hourly|daily).
	Period string `json:"period"` // "hourly" | "daily"

	// LastReset is when this record was last reset.
	LastReset time.Time `json:"last_reset"`

	// UpdatedAt is the last time this record was updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// TransactionRecord represents a single transaction for fraud detection analysis.
type TransactionRecord struct {
	// TransactionID uniquely identifies the transaction.
	TransactionID string `json:"transaction_id"`

	// FromAccountID is the originating account.
	FromAccountID string `json:"from_account_id"`

	// ToAccountID is the receiving account.
	ToAccountID string `json:"to_account_id"`

	// Amount is the transaction amount in paise.
	Amount int64 `json:"amount"`

	// Timestamp is when the transaction occurred.
	Timestamp time.Time `json:"timestamp"`
}

// PaymentRequest represents a payment to be checked for compliance.
type PaymentRequest struct {
	// PaymentID uniquely identifies the payment.
	PaymentID string `json:"payment_id"`

	// FromAccountID is the paying account.
	FromAccountID string `json:"from_account_id"`

	// ToAccountID is the receiving account.
	ToAccountID string `json:"to_account_id"`

	// ToName is the name of the beneficiary.
	ToName string `json:"to_name"`

	// Amount is the payment amount in paise.
	Amount int64 `json:"amount"`

	// Timestamp is when the payment was initiated.
	Timestamp time.Time `json:"timestamp"`

	// AccountAgeSeconds is the age of the originating account in seconds.
	AccountAgeSeconds int64 `json:"account_age_seconds"`
}

// FraudDetectionConfig holds thresholds for fraud pattern detection.
type FraudDetectionConfig struct {
	// StructuringThreshold: number of transactions under limit in one day
	StructuringThreshold int   `json:"structuring_threshold"` // default 5
	StructuringLimit     int64 `json:"structuring_limit"`     // default 10_00_000 (₹10k in paise)

	// RoundTripWindow: seconds within which send→receive is suspicious
	RoundTripWindowSeconds int `json:"round_trip_window_seconds"` // default 3600 (1 hour)
	RoundTripThreshold     int `json:"round_trip_threshold"`      // default 3 occurrences

	// VelocitySpikeThreshold: percentage increase to flag as spike
	VelocitySpikeThreshold float64 `json:"velocity_spike_threshold"` // default 100.0 (100%)

	// NewAccountAgeSeconds: account age threshold for "new" classification
	NewAccountAgeSeconds int64 `json:"new_account_age_seconds"` // default 604800 (7 days)
	NewAccountHighValue  int64 `json:"new_account_high_value"`  // default 50_00_000 (₹50k in paise)
}

// DefaultFraudDetectionConfig returns sensible defaults.
func DefaultFraudDetectionConfig() *FraudDetectionConfig {
	return &FraudDetectionConfig{
		StructuringThreshold:   5,
		StructuringLimit:       10_00_000, // ₹10k in paise
		RoundTripWindowSeconds: 3600,      // 1 hour
		RoundTripThreshold:     3,
		VelocitySpikeThreshold: 100.0,     // 100% increase
		NewAccountAgeSeconds:   604800,    // 7 days
		NewAccountHighValue:    50_00_000, // ₹50k in paise
	}
}

// VelocityConfig holds velocity check thresholds.
type VelocityConfig struct {
	// MaxTransactionsPerHour is the hourly transaction limit per account.
	MaxTransactionsPerHour int `json:"max_transactions_per_hour"` // default 10

	// MaxAmountPerHour is the hourly amount limit per account (in paise).
	MaxAmountPerHour int64 `json:"max_amount_per_hour"` // default 50_00_000 (₹50k)

	// MaxAmountPerDay is the daily amount limit per account (in paise).
	MaxAmountPerDay int64 `json:"max_amount_per_day"` // default 200_00_000 (₹200k)
}

// DefaultVelocityConfig returns sensible defaults.
func DefaultVelocityConfig() *VelocityConfig {
	return &VelocityConfig{
		MaxTransactionsPerHour: 10,
		MaxAmountPerHour:       50_00_000,  // ₹50k in paise
		MaxAmountPerDay:        200_00_000, // ₹200k in paise
	}
}
