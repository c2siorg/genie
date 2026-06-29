// Package cbdc defines types and ledger state machines for the CBDC Bridge,
// integrating with RBI e-Rupee. It provides an immutable, hash-chain ledger
// with transaction finality modeling and RBI compliance limits.
package cbdc

import (
	"time"
)

// TransactionStatus represents the state of a ledger entry.
type TransactionStatus string

const (
	// StatusPending: transaction committed to ledger, awaiting finality (T+0)
	StatusPending TransactionStatus = "pending"
	// StatusFinalized: transaction confirmed with finality (T+1 RBI processing)
	StatusFinalized TransactionStatus = "finalized"
	// StatusRejected: transaction rejected (failed validation, double-spend, etc.)
	StatusRejected TransactionStatus = "rejected"
)

// LedgerEntry represents a single transaction on the immutable ledger.
type LedgerEntry struct {
	// ID uniquely identifies this ledger entry
	ID string `json:"id"`
	// PaymentID references the originating payment request
	PaymentID string `json:"payment_id"`
	// FromAccount sender's account identifier
	FromAccount string `json:"from_account"`
	// ToAccount receiver's account identifier
	ToAccount string `json:"to_account"`
	// AmountPaise transaction amount in paise (1 rupee = 100 paise)
	AmountPaise int64 `json:"amount_paise"`
	// Timestamp when this entry was committed to the ledger
	Timestamp time.Time `json:"timestamp"`
	// BlockHeight is the block number this entry resides in
	BlockHeight uint64 `json:"block_height"`
	// Hash is the SHA256 hash of this entry's serialized form
	Hash string `json:"hash"`
	// Status is the transaction state (pending, finalized, rejected)
	Status TransactionStatus `json:"status"`
}

// LedgerBlock represents one block in the immutable ledger chain.
type LedgerBlock struct {
	// Height is the block's sequence number (0-indexed)
	Height uint64 `json:"height"`
	// Timestamp when the block was created
	Timestamp time.Time `json:"timestamp"`
	// Transactions in this block (append-only)
	Transactions []LedgerEntry `json:"transactions"`
	// PreviousHash is the hash of the previous block (chain integrity)
	PreviousHash string `json:"previous_hash"`
	// Hash is SHA256(PreviousHash + Serialize(Transactions))
	Hash string `json:"hash"`
}

// CBDCResponse is the result of a payment operation.
type CBDCResponse struct {
	// LedgerID uniquely identifies the ledger entry
	LedgerID string `json:"ledger_id"`
	// BlockHeight is the block number containing this transaction
	BlockHeight uint64 `json:"block_height"`
	// FinalityTimestamp when the transaction is expected to be finalized
	FinalityTimestamp time.Time `json:"finality_timestamp"`
	// Status of the transaction
	Status TransactionStatus `json:"status"`
	// Error message if the operation failed
	Error string `json:"error,omitempty"`
}

// RBILimits defines per-account transaction limits per RBI guidelines.
type RBILimits struct {
	// DailyP2PLimit maximum daily P2P transfer (person-to-person)
	DailyP2PLimit int64 `json:"daily_p2p_limit"`
	// DailyMerchantLimit maximum daily merchant/business transfer
	DailyMerchantLimit int64 `json:"daily_merchant_limit"`
	// SingleTransactionLimit maximum single transaction amount
	SingleTransactionLimit int64 `json:"single_transaction_limit"`
}

// AccountLimitState tracks daily transaction limits for an account.
type AccountLimitState struct {
	// Account identifier
	Account string `json:"account"`
	// DailyP2PUsed amount used toward P2P limit today
	DailyP2PUsed int64 `json:"daily_p2p_used"`
	// DailyMerchantUsed amount used toward merchant limit today
	DailyMerchantUsed int64 `json:"daily_merchant_used"`
	// LastResetAt when the daily limits were last reset
	LastResetAt time.Time `json:"last_reset_at"`
}

// TransactionType is the category of transaction.
type TransactionType string

const (
	// TypeP2P person-to-person transfer
	TypeP2P TransactionType = "p2p"
	// TypeMerchant merchant/business payment
	TypeMerchant TransactionType = "merchant"
)

// DefaultRBILimits returns RBI-compliant limits (in paise).
// Per RBI e-Rupee guidelines:
// - Daily P2P: ₹100,000 (10,000,000 paise)
// - Daily Merchant: ₹500,000 (50,000,000 paise)
// - Single txn: ₹50,000 (5,000,000 paise)
func DefaultRBILimits() RBILimits {
	return RBILimits{
		DailyP2PLimit:          10_000_000, // ₹100,000
		DailyMerchantLimit:     50_000_000, // ₹500,000
		SingleTransactionLimit: 5_000_000,  // ₹50,000
	}
}

// NewAccountLimitState creates a fresh daily limit tracker.
func NewAccountLimitState(account string) AccountLimitState {
	return AccountLimitState{
		Account:     account,
		LastResetAt: time.Now().UTC(),
	}
}
