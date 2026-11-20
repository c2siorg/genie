package commercesettlement

import (
	"fmt"
	"sync"
	"time"
)

// SettlementBatchManager defines operations for managing settlement batches.
type SettlementBatchManager interface {
	// CreateBatch creates a new settlement batch for the given date and merchants.
	// Returns the created batch and any error.
	CreateBatch(settlementDate time.Time, merchantIDs []string) (*SettlementBatch, error)

	// GetBatch retrieves a batch by ID.
	GetBatch(batchID string) (*SettlementBatch, error)

	// QueryByDate retrieves all batches for a given settlement date.
	QueryByDate(settlementDate time.Time) ([]*SettlementBatch, error)

	// UpdateBatchStatus updates the status of a batch.
	UpdateBatchStatus(batchID string, status SettlementStatus) error

	// AddMerchantAmounts adds merchant receivables to a batch.
	// amountsPaise is a map of merchantID -> amount in paise.
	AddMerchantAmounts(batchID string, amountsPaise map[string]int64) error
}

// InMemoryBatchManager is a thread-safe, in-memory implementation of SettlementBatchManager.
// It stores batches keyed by batch ID and indexed by settlement date for queries.
type InMemoryBatchManager struct {
	mu      sync.RWMutex
	batches map[string]*SettlementBatch   // keyed by batch ID
	byDate  map[string][]*SettlementBatch // keyed by settlement date (YYYY-MM-DD)
}

// NewInMemoryBatchManager creates a new in-memory batch manager.
func NewInMemoryBatchManager() *InMemoryBatchManager {
	return &InMemoryBatchManager{
		batches: make(map[string]*SettlementBatch),
		byDate:  make(map[string][]*SettlementBatch),
	}
}

// CreateBatch creates a new settlement batch.
func (m *InMemoryBatchManager) CreateBatch(settlementDate time.Time, merchantIDs []string) (*SettlementBatch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	batch := NewSettlementBatch(settlementDate, merchantIDs)
	m.batches[batch.ID] = batch

	// Index by date (YYYY-MM-DD format).
	dateKey := settlementDate.Format("2006-01-02")
	m.byDate[dateKey] = append(m.byDate[dateKey], batch)

	return batch, nil
}

// GetBatch retrieves a batch by ID.
func (m *InMemoryBatchManager) GetBatch(batchID string) (*SettlementBatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	batch, ok := m.batches[batchID]
	if !ok {
		return nil, fmt.Errorf("batch not found: %s", batchID)
	}
	return batch, nil
}

// QueryByDate retrieves all batches for a given settlement date.
func (m *InMemoryBatchManager) QueryByDate(settlementDate time.Time) ([]*SettlementBatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dateKey := settlementDate.Format("2006-01-02")
	batches, ok := m.byDate[dateKey]
	if !ok {
		return []*SettlementBatch{}, nil
	}
	return batches, nil
}

// UpdateBatchStatus updates a batch's status.
func (m *InMemoryBatchManager) UpdateBatchStatus(batchID string, status SettlementStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	batch, ok := m.batches[batchID]
	if !ok {
		return fmt.Errorf("batch not found: %s", batchID)
	}
	batch.Status = status
	if status == StatusSettled {
		now := time.Now()
		batch.SettledAt = &now
	}
	return nil
}

// AddMerchantAmounts adds merchant receivables to a batch.
func (m *InMemoryBatchManager) AddMerchantAmounts(batchID string, amountsPaise map[string]int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	batch, ok := m.batches[batchID]
	if !ok {
		return fmt.Errorf("batch not found: %s", batchID)
	}

	for merchantID, amountPaise := range amountsPaise {
		batch.AddEntry(merchantID, amountPaise)
	}

	return nil
}
