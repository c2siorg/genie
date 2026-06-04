// bridge.go — CBDC Bridge for RBI e-Rupee integration.
//
// The bridge orchestrates payment initiation and finality, simulating
// RBI processing delays and graceful fallback to local ledger.
package cbdc

import (
	"fmt"
	"sync"
	"time"
)

// CBDCBridge defines the interface for payment operations on the CBDC ledger.
type CBDCBridge interface {
	// InitiatePayment commits a payment to the ledger (T+0).
	InitiatePayment(paymentID, fromAccount, toAccount string, amountPaise int64) (*CBDCResponse, error)

	// FinalizePayment advances a payment to finalized status (T+1 simulation).
	FinalizePayment(paymentID string) (*CBDCResponse, error)

	// GetPaymentStatus retrieves the current status of a payment.
	GetPaymentStatus(paymentID string) (*CBDCResponse, error)

	// GetBlock retrieves a ledger block by height.
	GetBlock(height uint64) (*LedgerBlock, error)

	// ProcessFinality processes the finality queue (call periodically).
	ProcessFinality() error
}

// MockCBDCBridge is a mock RBI CBDC service with configurable finality delay.
type MockCBDCBridge struct {
	mu                  sync.RWMutex
	ledger              Ledger
	finalityDelayBlocks uint64 // blocks to wait before finalization (1-5 typical)
	rbiAvailable        bool   // simulation of RBI backend availability
	lastError           error
}

// NewMockCBDCBridge creates a bridge with the given finality delay (in blocks).
func NewMockCBDCBridge(finalityDelayBlocks uint64) *MockCBDCBridge {
	if finalityDelayBlocks == 0 {
		finalityDelayBlocks = 1 // Default: T+1
	}
	if finalityDelayBlocks > 5 {
		finalityDelayBlocks = 5 // Cap at 5 blocks
	}

	return &MockCBDCBridge{
		ledger:              NewInMemoryLedger(),
		finalityDelayBlocks: finalityDelayBlocks,
		rbiAvailable:        true,
	}
}

// InitiatePayment commits a payment to the ledger at T+0.
func (b *MockCBDCBridge) InitiatePayment(paymentID, fromAccount, toAccount string, amountPaise int64) (*CBDCResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.rbiAvailable {
		// Graceful fallback: still commit to local ledger
		_, err := b.ledger.CommitTransaction(paymentID, fromAccount, toAccount, amountPaise)
		if err != nil {
			return nil, fmt.Errorf("RBI backend unavailable, local commit failed: %w", err)
		}
		// Return pending response even if backend is down
	} else {
		// Normal flow: commit to ledger
		_, err := b.ledger.CommitTransaction(paymentID, fromAccount, toAccount, amountPaise)
		if err != nil {
			b.lastError = err
			return nil, err
		}
	}

	// Get the committed entry
	entry, found, err := b.ledger.GetStatus(paymentID)
	if err != nil || !found {
		return nil, fmt.Errorf("failed to retrieve committed transaction: %w", err)
	}

	// Calculate expected finality timestamp (now + delay blocks)
	// Rough estimate: ~1 second per block (configurable in real implementation)
	finalityTime := time.Now().UTC().Add(time.Duration(b.finalityDelayBlocks) * time.Second)

	return &CBDCResponse{
		LedgerID:          entry.ID,
		BlockHeight:       entry.BlockHeight,
		FinalityTimestamp: finalityTime,
		Status:            StatusPending,
	}, nil
}

// FinalizePayment advances a payment to finalized status (manual finality).
func (b *MockCBDCBridge) FinalizePayment(paymentID string) (*CBDCResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, found, err := b.ledger.GetStatus(paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction status: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("payment_id %s not found in ledger", paymentID)
	}

	// Finalize by advancing past the delay threshold
	_, err = b.ledger.FinalizeTransactions(0) // Force finalization of all pending in cutoff blocks
	if err != nil {
		return nil, fmt.Errorf("finality processing failed: %w", err)
	}

	// Retrieve updated status
	updatedEntry, found, err := b.ledger.GetStatus(paymentID)
	if err != nil || !found {
		return nil, fmt.Errorf("failed to retrieve finalized transaction: %w", err)
	}

	return &CBDCResponse{
		LedgerID:          updatedEntry.ID,
		BlockHeight:       updatedEntry.BlockHeight,
		FinalityTimestamp: updatedEntry.Timestamp.Add(time.Duration(b.finalityDelayBlocks) * time.Second),
		Status:            updatedEntry.Status,
	}, nil
}

// GetPaymentStatus retrieves the current status of a payment.
func (b *MockCBDCBridge) GetPaymentStatus(paymentID string) (*CBDCResponse, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	entry, found, err := b.ledger.GetStatus(paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction status: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("payment_id %s not found in ledger", paymentID)
	}

	// Calculate expected finality time based on block height
	currentHeight := b.ledger.GetLatestBlockHeight()
	blockAge := currentHeight - entry.BlockHeight

	var finalityTime time.Time
	if entry.Status == StatusFinalized {
		finalityTime = entry.Timestamp.Add(time.Duration(b.finalityDelayBlocks) * time.Second)
	} else {
		blocksRemaining := int64(b.finalityDelayBlocks) - int64(blockAge)
		if blocksRemaining <= 0 {
			blocksRemaining = 1
		}
		finalityTime = time.Now().UTC().Add(time.Duration(blocksRemaining) * time.Second)
	}

	return &CBDCResponse{
		LedgerID:          entry.ID,
		BlockHeight:       entry.BlockHeight,
		FinalityTimestamp: finalityTime,
		Status:            entry.Status,
	}, nil
}

// GetBlock retrieves a ledger block by height.
func (b *MockCBDCBridge) GetBlock(height uint64) (*LedgerBlock, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	block, found, err := b.ledger.QueryBlock(height)
	if err != nil {
		return nil, fmt.Errorf("failed to query block: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("block height %d not found", height)
	}

	return block, nil
}

// ProcessFinality processes the finality queue periodically.
// Call this on a timer (e.g., every second) to age transactions toward finality.
func (b *MockCBDCBridge) ProcessFinality() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Advance pending transactions in old blocks to finalized status
	_, err := b.ledger.FinalizeTransactions(b.finalityDelayBlocks)
	if err != nil {
		b.lastError = err
		return fmt.Errorf("finality processing failed: %w", err)
	}

	return nil
}

// SetRBIAvailability simulates RBI backend availability (for testing).
func (b *MockCBDCBridge) SetRBIAvailability(available bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rbiAvailable = available
}

// GetLastError returns the last error encountered (for testing/debugging).
func (b *MockCBDCBridge) GetLastError() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastError
}

// GetLedger returns the underlying ledger (for testing).
func (b *MockCBDCBridge) GetLedger() Ledger {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ledger
}
