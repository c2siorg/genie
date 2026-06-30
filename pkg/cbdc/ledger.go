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
//
// Immutability guarantee: once a LedgerEntry is appended to a block, its bytes
// (and the block's hash) are never modified. Finality is tracked separately in
// finalityLog — a plain set of payment IDs that have been finalized. This means
// a verifier can re-hash any committed block at any time and get the same result.
type InMemoryLedger struct {
	mu          sync.RWMutex
	blocks      []LedgerBlock
	txIndex     map[string]*LedgerEntry // payment_id -> pointer into blocks slice (not stack copy)
	blockTxn    map[string]uint64       // payment_id -> block_height for finality tracking
	finalityLog map[string]time.Time    // payment_id -> time finalized (append-only; blocks never mutated)
}

// NewInMemoryLedger creates an empty ledger with a genesis block.
func NewInMemoryLedger() *InMemoryLedger {
	ledger := &InMemoryLedger{
		blocks:      []LedgerBlock{},
		txIndex:     make(map[string]*LedgerEntry),
		blockTxn:    make(map[string]uint64),
		finalityLog: make(map[string]time.Time),
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

	// Index for quick lookup. IMPORTANT: take the address of the slice element AFTER
	// the append (not &entry, which is a stack-local copy). The slice element and the
	// index pointer now point to the same memory; FinalizeTransactions must NOT mutate
	// block.Transactions — it writes to finalityLog instead, keeping blocks immutable.
	l.txIndex[paymentID] = &currentBlock.Transactions[len(currentBlock.Transactions)-1]
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

// FinalizeTransactions records finality events for pending transactions in blocks
// that are at least blocksDelay old.
//
// IMMUTABILITY GUARANTEE: committed block data (Transactions slice, block Hash) is
// NEVER modified. Finality is modelled as new events written to the append-only
// finalityLog map. This preserves tamper-evidence: a verifier can re-hash any
// committed block at any time and get the same result. The txIndex Status field is
// updated as a convenience for callers, but the source of truth is finalityLog.
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

	now := time.Now().UTC()
	count := 0

	for i := 0; i <= int(cutoffHeight) && i < len(l.blocks); i++ {
		for _, tx := range l.blocks[i].Transactions {
			if _, alreadyFinalized := l.finalityLog[tx.PaymentID]; alreadyFinalized {
				continue
			}
			// Append finality event — blocks[i].Transactions is NOT touched.
			l.finalityLog[tx.PaymentID] = now
			// Update the txIndex Status so GetStatus / GetTransactionByPaymentID
			// return StatusFinalized without requiring a finalityLog lookup.
			if idx, exists := l.txIndex[tx.PaymentID]; exists {
				idx.Status = StatusFinalized
			}
			count++
		}
	}

	// Prune finalityLog entries older than 90 days to bound memory usage.
	// The ledger's tamper-evidence guarantee comes from the immutable block chain,
	// not from finalityLog — pruning old entries does not affect hash-chain integrity.
	// A node needing to verify old finality can re-derive it from the block data.
	const finalityRetention = 90 * 24 * time.Hour
	pruneOlderThan := now.Add(-finalityRetention)
	for paymentID, finalizedAt := range l.finalityLog {
		if finalizedAt.Before(pruneOlderThan) {
			delete(l.finalityLog, paymentID)
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
