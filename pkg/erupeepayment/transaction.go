package erupeepayment

import (
	"fmt"
	"sync"
	"time"
)

// TransactionLog defines queries and recording operations on the transaction ledger.
type TransactionLog interface {
	// Record adds a transaction to the immutable log.
	Record(txn *TransactionRecord) error
	// Query returns all transactions for an account within a time range (inclusive).
	Query(accountID string, since, until time.Time) []*TransactionRecord
	// GetStatus returns the current payment status for a given payment ID.
	// Returns false if the payment ID is not found.
	GetStatus(paymentID string) (PaymentStatus, bool)
	// GetByPaymentID returns all transaction records for a given payment ID.
	GetByPaymentID(paymentID string) []*TransactionRecord
}

// InMemoryTransactionLog is an append-only, queryable transaction ledger.
// Immutability is enforced: all returned records are copies.
type InMemoryTransactionLog struct {
	mu            sync.RWMutex
	transactions  []*TransactionRecord
	paymentStatus map[string]PaymentStatus // cache of most recent status per payment
	idGen         func() string             // injectable ID generator for testing
}

// NewInMemoryTransactionLog creates a new in-memory transaction log.
// idGen is used to generate transaction IDs; if nil, a simple counter is used.
func NewInMemoryTransactionLog(idGen func() string) *InMemoryTransactionLog {
	if idGen == nil {
		counter := 0
		idGen = func() string {
			counter++
			return fmt.Sprintf("txn_%d", counter)
		}
	}
	return &InMemoryTransactionLog{
		transactions:  make([]*TransactionRecord, 0),
		paymentStatus: make(map[string]PaymentStatus),
		idGen:         idGen,
	}
}

// Record adds a transaction to the log. ID is auto-assigned if empty.
func (l *InMemoryTransactionLog) Record(txn *TransactionRecord) error {
	if txn == nil {
		return fmt.Errorf("transaction cannot be nil")
	}
	if txn.PaymentID == "" {
		return fmt.Errorf("payment_id is required")
	}
	if txn.FromAccount == "" || txn.ToAccount == "" {
		return fmt.Errorf("from_account and to_account are required")
	}
	if txn.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if txn.ID == "" {
		txn.ID = l.idGen()
	}
	if txn.Timestamp.IsZero() {
		txn.Timestamp = time.Now()
	}

	// Append to immutable log
	l.transactions = append(l.transactions, txn)
	// Update status cache
	l.paymentStatus[txn.PaymentID] = txn.Status

	return nil
}

// Query returns all transactions for an account within [since, until] inclusive.
func (l *InMemoryTransactionLog) Query(accountID string, since, until time.Time) []*TransactionRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*TransactionRecord
	for _, txn := range l.transactions {
		// Match transactions where this account is either sender or receiver
		if (txn.FromAccount == accountID || txn.ToAccount == accountID) &&
			!txn.Timestamp.Before(since) && !txn.Timestamp.After(until) {
			// Return a copy to prevent external mutations
			cpy := *txn
			result = append(result, &cpy)
		}
	}
	return result
}

// GetStatus returns the current payment status for a given payment ID.
func (l *InMemoryTransactionLog) GetStatus(paymentID string) (PaymentStatus, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	status, ok := l.paymentStatus[paymentID]
	return status, ok
}

// GetByPaymentID returns all transaction records for a given payment ID.
func (l *InMemoryTransactionLog) GetByPaymentID(paymentID string) []*TransactionRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*TransactionRecord
	for _, txn := range l.transactions {
		if txn.PaymentID == paymentID {
			cpy := *txn
			result = append(result, &cpy)
		}
	}
	return result
}
