// Package erupeepayment provides core fund transfer and account management
// for e-Rupee (CBDC) payments. It integrates with the CBDC Bridge for ledger
// commitment and provides laminar tracing for all payments.
//
// Types define:
// - PaymentRequest: initiated transfers (pending → confirming → confirmed/failed/reversed)
// - Account: e-Rupee wallets with balance and status tracking
// - TransactionRecord: immutable ledger entries for audit trails
// - PaymentStatus: state machine for payment lifecycle
package erupeepayment

import "time"

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	// StatusPending: payment initiated, awaiting ledger commitment
	StatusPending PaymentStatus = "pending"
	// StatusConfirming: submitted to CBDC Bridge, awaiting confirmation
	StatusConfirming PaymentStatus = "confirming"
	// StatusConfirmed: ledger committed, payment final
	StatusConfirmed PaymentStatus = "confirmed"
	// StatusFailed: payment rejected (insufficient balance, invalid account, etc.)
	StatusFailed PaymentStatus = "failed"
	// StatusReversed: payment reversed after confirmation (refund scenario)
	StatusReversed PaymentStatus = "reversed"
)

// AccountType distinguishes between personal and merchant e-Rupee wallets.
type AccountType string

const (
	// TypePersonal: individual citizen wallet
	TypePersonal AccountType = "personal"
	// TypeMerchant: business/merchant wallet
	TypeMerchant AccountType = "merchant"
)

// AccountStatus tracks the operational state of an account.
type AccountStatus string

const (
	// StatusActive: account in good standing
	StatusActive AccountStatus = "active"
	// StatusFrozen: account temporarily suspended (AML/compliance hold)
	StatusFrozen AccountStatus = "frozen"
	// StatusClosed: account permanently closed
	StatusClosed AccountStatus = "closed"
)

// PaymentRequest represents a fund transfer request.
// All amounts are in paise (₹1 = 100 paise) to avoid floating-point issues.
type PaymentRequest struct {
	// ID uniquely identifies this payment (UUIDv7 recommended)
	ID string `json:"id"`
	// FromAccount is the sender's e-Rupee account ID
	FromAccount string `json:"from_account"`
	// ToAccount is the recipient's e-Rupee account ID
	ToAccount string `json:"to_account"`
	// AmountPaise is the transfer amount in paise
	AmountPaise int64 `json:"amount_paise"`
	// Timestamp records when the payment was initiated
	Timestamp time.Time `json:"timestamp"`
	// Reference is a memo or reference code (e.g., "INV-2026-001")
	Reference string `json:"reference"`
	// Metadata holds caller-provided context (purpose, category, etc.)
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Account represents an e-Rupee wallet.
type Account struct {
	// ID uniquely identifies the account (UUIDv7 recommended)
	ID string `json:"id"`
	// HolderID links to the account holder (person or merchant)
	HolderID string `json:"holder_id"`
	// BalancePaise is the current balance in paise
	BalancePaise int64 `json:"balance_paise"`
	// Type distinguishes personal from merchant
	Type AccountType `json:"type"`
	// Status tracks account operational state
	Status AccountStatus `json:"status"`
	// CreatedAt records account creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt records the last balance or status change
	UpdatedAt time.Time `json:"updated_at"`
}

// TransactionRecord is an immutable ledger entry for audit trails.
// One PaymentRequest may generate two TransactionRecords (debit + credit).
type TransactionRecord struct {
	// ID uniquely identifies this ledger entry
	ID string `json:"id"`
	// PaymentID links to the originating PaymentRequest
	PaymentID string `json:"payment_id"`
	// FromAccount is the sender's account ID
	FromAccount string `json:"from_account"`
	// ToAccount is the recipient's account ID
	ToAccount string `json:"to_account"`
	// Amount in paise
	Amount int64 `json:"amount"`
	// Timestamp records when the transaction occurred
	Timestamp time.Time `json:"timestamp"`
	// Status is the payment lifecycle state
	Status PaymentStatus `json:"status"`
	// LedgerID is the CBDC Bridge ledger commit ID (for reconciliation)
	LedgerID string `json:"ledger_id,omitempty"`
}
