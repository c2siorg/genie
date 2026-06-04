// ledger.go — Immutable, hash-chain ledger implementation for CBDC transactions.
//
// The ledger is append-only with block-based storage. Each block contains
// multiple transactions and chains to the previous block via hash integrity.
// Transactions progress from pending → finalized with configurable delay.
package cbdc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Ledger defines the interface for CBDC transaction storage and retrieval.
type Ledger interface {
	// CommitTransaction appends a transaction to the ledger and returns its ledger ID.
	CommitTransaction(paymentID, fromAccount, toAccount string, amountPaise int64) (ledgerID string, err error)

	// GetStatus returns the current status of a transaction by payment ID.
	GetStatus(paymentID string) (ledgerEntry *LedgerEntry, found bool, err error)

	// QueryBlock retrieves a block by height.
	QueryBlock(height uint64) (block *LedgerBlock, found bool, err error)

	// FinalizeTransactions advances pending transactions to finalized status.
	// This simulates T+1 RBI processing (e.g., finalize blocks created 1+ blocks ago).
	FinalizeTransactions(blocksDelay uint64) (count int, err error)

	// GetLatestBlockHeight returns the height of the most recent block.
	GetLatestBlockHeight() uint64

	// GetTransactionByPaymentID retrieves a transaction by payment ID across all blocks.
	GetTransactionByPaymentID(paymentID string) (entry *LedgerEntry, found bool, err error)
}

// InMemoryLedger is a thread-safe, in-memory ledger with block-based storage.
type InMemoryLedger struct {
	mu       sync.RWMutex
	blocks   []LedgerBlock
	txIndex  map[string]*LedgerEntry // payment_id -> entry for O(1) lookup
	blockTxn map[string]uint64       // payment_id -> block_height for finality tracking
}

// NewInMemoryLedger creates an empty ledger with a genesis block.
func NewInMemoryLedger() *InMemoryLedger {
	ledger := &InMemoryLedger{
		blocks:   []LedgerBlock{},
		txIndex:  make(map[string]*LedgerEntry),
		blockTxn: make(map[string]uint64),
	}

	// Create genesis block (height 0, no transactions)
	genesisBlock := LedgerBlock{
		Height:       0,
		Timestamp:    time.Now().UTC(),
		Transactions: []LedgerEntry{},
		PreviousHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Hash:         ledger.computeBlockHash("0000000000000000000000000000000000000000000000000000000000000000", []LedgerEntry{}),
	}
	ledger.blocks = append(ledger.blocks, genesisBlock)

	return ledger
}

// CommitTransaction appends a transaction to the current or new block.
func (l *InMemoryLedger) CommitTransaction(paymentID, fromAccount, toAccount string, amountPaise int64) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if paymentID == "" || fromAccount == "" || toAccount == "" || amountPaise <= 0 {
		return "", fmt.Errorf("invalid transaction parameters")
	}

	// Check for duplicate payment ID
	if _, exists := l.txIndex[paymentID]; exists {
		return "", fmt.Errorf("payment_id %s already exists (double-spend prevention)", paymentID)
	}

	now := time.Now().UTC()
	entryID := fmt.Sprintf("ledger-%d", now.UnixNano())

	entry := LedgerEntry{
		ID:          entryID,
		PaymentID:   paymentID,
		FromAccount: fromAccount,
		ToAccount:   toAccount,
		AmountPaise: amountPaise,
		Timestamp:   now,
		Status:      StatusPending,
	}

	// Add to current block or create new one if limit reached
	const txnsPerBlock = 100
	currentBlock := &l.blocks[len(l.blocks)-1]

	if len(currentBlock.Transactions) >= txnsPerBlock {
		// Create new block
		newHeight := currentBlock.Height + 1
		newBlock := LedgerBlock{
			Height:       newHeight,
			Timestamp:    now,
			Transactions: []LedgerEntry{},
			PreviousHash: currentBlock.Hash,
		}
		newBlock.Hash = l.computeBlockHash(newBlock.PreviousHash, newBlock.Transactions)
		l.blocks = append(l.blocks, newBlock)
		currentBlock = &l.blocks[len(l.blocks)-1]
	}

	// Set block height and compute entry hash
	entry.BlockHeight = currentBlock.Height
	entry.Hash = l.computeEntryHash(entry)

	// Append to block
	currentBlock.Transactions = append(currentBlock.Transactions, entry)
	// Recompute block hash with new transaction
	currentBlock.Hash = l.computeBlockHash(currentBlock.PreviousHash, currentBlock.Transactions)

	// Index for quick lookup
	l.txIndex[paymentID] = &entry
	l.blockTxn[paymentID] = entry.BlockHeight

	return entry.ID, nil
}

// GetStatus retrieves the status of a transaction by payment ID.
func (l *InMemoryLedger) GetStatus(paymentID string) (*LedgerEntry, bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry, exists := l.txIndex[paymentID]
	if !exists {
		return nil, false, nil
	}

	// Return a copy to prevent external mutation
	entryCopy := *entry
	return &entryCopy, true, nil
}

// QueryBlock retrieves a block by height.
func (l *InMemoryLedger) QueryBlock(height uint64) (*LedgerBlock, bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if height >= uint64(len(l.blocks)) {
		return nil, false, nil
	}

	blockCopy := l.blocks[height]
	return &blockCopy, true, nil
}

// FinalizeTransactions advances pending transactions to finalized.
// Only finalizes transactions in blocks that are at least blocksDelay old.
func (l *InMemoryLedger) FinalizeTransactions(blocksDelay uint64) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.blocks) == 0 {
		return 0, nil
	}

	latestHeight := l.blocks[len(l.blocks)-1].Height
	cutoffHeight := int64(latestHeight) - int64(blocksDelay)

	if cutoffHeight < 0 {
		cutoffHeight = 0
	}

	count := 0

	// Iterate through blocks and finalize old pending transactions
	for i := 0; i < len(l.blocks); i++ {
		if uint64(i) > uint64(cutoffHeight) {
			continue // Skip blocks newer than cutoff
		}

		block := &l.blocks[i]
		for j := range block.Transactions {
			if block.Transactions[j].Status == StatusPending {
				block.Transactions[j].Status = StatusFinalized
				// Update index
				if entry, exists := l.txIndex[block.Transactions[j].PaymentID]; exists {
					entry.Status = StatusFinalized
				}
				count++
			}
		}

		// Recompute block hash after status changes
		if i > 0 {
			block.Hash = l.computeBlockHash(l.blocks[i-1].Hash, block.Transactions)
		}
	}

	return count, nil
}

// GetLatestBlockHeight returns the height of the most recent block.
func (l *InMemoryLedger) GetLatestBlockHeight() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.blocks) == 0 {
		return 0
	}

	return l.blocks[len(l.blocks)-1].Height
}

// GetTransactionByPaymentID retrieves a transaction by payment ID.
func (l *InMemoryLedger) GetTransactionByPaymentID(paymentID string) (*LedgerEntry, bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry, exists := l.txIndex[paymentID]
	if !exists {
		return nil, false, nil
	}

	entryCopy := *entry
	return &entryCopy, true, nil
}

// computeBlockHash returns SHA256(previousHash || serializedTransactions).
func (l *InMemoryLedger) computeBlockHash(previousHash string, transactions []LedgerEntry) string {
	h := sha256.New()

	// Write previous hash
	h.Write([]byte(previousHash))

	// Write serialized transactions
	for _, txn := range transactions {
		data, _ := json.Marshal(txn)
		h.Write(data)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// computeEntryHash returns SHA256(serializedEntry).
func (l *InMemoryLedger) computeEntryHash(entry LedgerEntry) string {
	// Temporarily clear hash to avoid circular computation
	entry.Hash = ""
	data, _ := json.Marshal(entry)
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
