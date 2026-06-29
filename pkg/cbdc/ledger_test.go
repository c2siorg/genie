package cbdc

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewInMemoryLedger(t *testing.T) {
	ledger := NewInMemoryLedger()

	if ledger == nil {
		t.Fatal("ledger should not be nil")
	}

	if height := ledger.GetLatestBlockHeight(); height != 0 {
		t.Errorf("initial block height should be 0, got %d", height)
	}

	// Verify genesis block exists
	genesisBlock, found, err := ledger.QueryBlock(0)
	if err != nil || !found {
		t.Fatal("genesis block should exist")
	}

	if genesisBlock.Height != 0 || len(genesisBlock.Transactions) != 0 {
		t.Error("genesis block should be empty")
	}

	if genesisBlock.PreviousHash != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Error("genesis block should have zero previous hash")
	}

	if len(genesisBlock.Hash) != 64 {
		t.Errorf("block hash should be 64 hex chars, got %d", len(genesisBlock.Hash))
	}
}

func TestCommitTransaction(t *testing.T) {
	ledger := NewInMemoryLedger()

	tests := []struct {
		name        string
		paymentID   string
		fromAccount string
		toAccount   string
		amount      int64
		wantErr     bool
	}{
		{
			name:        "valid transaction",
			paymentID:   "pay001",
			fromAccount: "account1",
			toAccount:   "account2",
			amount:      100000,
			wantErr:     false,
		},
		{
			name:        "invalid: empty payment id",
			paymentID:   "",
			fromAccount: "account1",
			toAccount:   "account2",
			amount:      100000,
			wantErr:     true,
		},
		{
			name:        "invalid: zero amount",
			paymentID:   "pay002",
			fromAccount: "account1",
			toAccount:   "account2",
			amount:      0,
			wantErr:     true,
		},
		{
			name:        "invalid: negative amount",
			paymentID:   "pay003",
			fromAccount: "account1",
			toAccount:   "account2",
			amount:      -100,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledgerID, err := ledger.CommitTransaction(tt.paymentID, tt.fromAccount, tt.toAccount, tt.amount)

			if (err != nil) != tt.wantErr {
				t.Errorf("CommitTransaction error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && ledgerID == "" {
				t.Error("ledgerID should not be empty on success")
			}
		})
	}
}

func TestDoubleSpendPrevention(t *testing.T) {
	ledger := NewInMemoryLedger()

	paymentID := "pay-unique-001"
	_, err := ledger.CommitTransaction(paymentID, "account1", "account2", 100000)
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}

	// Attempt double-spend with same payment ID
	_, err = ledger.CommitTransaction(paymentID, "account1", "account3", 50000)
	if err == nil || !strings.Contains(err.Error(), "double-spend") {
		t.Errorf("expected double-spend error, got: %v", err)
	}
}

func TestGetStatus(t *testing.T) {
	ledger := NewInMemoryLedger()

	paymentID := "pay002"
	ledgerID, _ := ledger.CommitTransaction(paymentID, "account1", "account2", 100000)

	entry, found, err := ledger.GetStatus(paymentID)
	if err != nil || !found {
		t.Fatalf("GetStatus failed: err=%v, found=%v", err, found)
	}

	if entry.ID != ledgerID {
		t.Errorf("entry ID mismatch: got %s, want %s", entry.ID, ledgerID)
	}

	if entry.Status != StatusPending {
		t.Errorf("status should be pending, got %s", entry.Status)
	}

	if entry.AmountPaise != 100000 {
		t.Errorf("amount mismatch: got %d, want 100000", entry.AmountPaise)
	}

	if entry.FromAccount != "account1" || entry.ToAccount != "account2" {
		t.Error("account mismatch")
	}

	// Test non-existent payment
	_, found, err = ledger.GetStatus("nonexistent")
	if found {
		t.Error("should not find nonexistent payment")
	}
}

func TestHashChainIntegrity(t *testing.T) {
	ledger := NewInMemoryLedger()

	// Commit multiple transactions
	for i := 1; i <= 5; i++ {
		paymentID := "pay" + string(rune(48+i))
		_, err := ledger.CommitTransaction(paymentID, "account1", "account2", int64(100000*i))
		if err != nil {
			t.Fatalf("commit %d failed: %v", i, err)
		}
	}

	// Verify all blocks have valid hash chain
	for height := uint64(1); height <= ledger.GetLatestBlockHeight(); height++ {
		block, found, err := ledger.QueryBlock(height)
		if err != nil || !found {
			t.Fatalf("block %d not found", height)
		}

		// Get previous block and verify hash chain
		if height > 0 {
			prevBlock, found, err := ledger.QueryBlock(height - 1)
			if err != nil || !found {
				t.Fatalf("previous block %d not found", height-1)
			}

			if block.PreviousHash != prevBlock.Hash {
				t.Errorf("hash chain broken at block %d", height)
			}
		}

		// Verify hash is 64 hex chars
		if len(block.Hash) != 64 {
			t.Errorf("block %d hash invalid length: %d", height, len(block.Hash))
		}
	}
}

func TestBlockCreation(t *testing.T) {
	ledger := NewInMemoryLedger()

	// Commit 150 transactions to trigger block creation (limit is 100 per block)
	for i := 1; i <= 150; i++ {
		paymentID := "pay" + formatID(i)
		_, err := ledger.CommitTransaction(paymentID, "account1", "account2", int64(100000))
		if err != nil {
			t.Fatalf("commit %d failed: %v", i, err)
		}
	}

	// Should have created multiple blocks
	latestHeight := ledger.GetLatestBlockHeight()
	if latestHeight < 1 {
		t.Errorf("should have multiple blocks, got height %d", latestHeight)
	}

	// Genesis block (0) should have 100 txns (first 100)
	block0, found0, _ := ledger.QueryBlock(0)
	if !found0 {
		t.Fatal("block 0 should exist")
	}
	if len(block0.Transactions) != 100 {
		t.Errorf("block 0 should have 100 txns, got %d", len(block0.Transactions))
	}

	// Block 1 should have remaining 50 txns
	block1, found1, _ := ledger.QueryBlock(1)
	if !found1 {
		t.Fatal("block 1 should exist")
	}
	if len(block1.Transactions) != 50 {
		t.Errorf("block 1 should have 50 txns, got %d", len(block1.Transactions))
	}
}

func TestFinalizeTransactions(t *testing.T) {
	ledger := NewInMemoryLedger()

	// Commit 205 transactions to fill blocks 0 (100), 1 (100), and 2 (5)
	for i := 1; i <= 205; i++ {
		pid := "pay-" + formatID(i)
		_, _ = ledger.CommitTransaction(pid, "account1", "account2", 100000)
	}

	// Verify block structure
	latestHeight := ledger.GetLatestBlockHeight()
	if latestHeight != 2 {
		t.Fatalf("should have 3 blocks (0, 1, 2), latest height is %d", latestHeight)
	}

	// All should be pending initially
	for i := 1; i <= 205; i++ {
		pid := "pay-" + formatID(i)
		entry, _, _ := ledger.GetStatus(pid)
		if entry.Status != StatusPending {
			t.Errorf("%s should be pending", pid)
		}
	}

	// Finalize with 1-block delay
	// Latest height = 2, cutoff = 2 - 1 = 1
	// Finalizes blocks 0 and 1 (all with index <= 1)
	// Block 0: 100 txns
	// Block 1: 100 txns
	// Total: 200 txns finalized, block 2 stays pending
	count, err := ledger.FinalizeTransactions(1)
	if err != nil {
		t.Fatalf("FinalizeTransactions failed: %v", err)
	}

	if count != 200 {
		t.Errorf("should have finalized 200 txns, got %d", count)
	}

	// Check status of block 2 (should still be pending)
	block2, _, _ := ledger.QueryBlock(2)
	for _, txn := range block2.Transactions {
		if txn.Status != StatusPending {
			t.Errorf("block 2 txn %s should be pending, got %s", txn.PaymentID, txn.Status)
		}
	}
}

func TestQueryBlock(t *testing.T) {
	ledger := NewInMemoryLedger()

	// Commit transaction to block 0 (genesis)
	_, _ = ledger.CommitTransaction("pay1", "account1", "account2", 100000)

	// Query existing block
	block, found, err := ledger.QueryBlock(0)
	if err != nil || !found {
		t.Fatal("block 0 should exist")
	}

	if len(block.Transactions) == 0 {
		t.Error("block 0 should contain transaction")
	}

	// Query non-existent block
	_, found, _ = ledger.QueryBlock(999)
	if found {
		t.Error("block 999 should not exist")
	}
}

func TestConcurrentTransactions(t *testing.T) {
	ledger := NewInMemoryLedger()
	var wg sync.WaitGroup
	errs := make([]error, 0)
	var mu sync.Mutex

	// 100 concurrent transactions
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			paymentID := "pay-concurrent-" + formatID(idx)
			_, err := ledger.CommitTransaction(paymentID, "account1", "account2", 100000)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("concurrent transactions failed: %v", errs[0])
	}

	// Verify all were committed
	for i := 0; i < 100; i++ {
		paymentID := "pay-concurrent-" + formatID(i)
		_, found, _ := ledger.GetStatus(paymentID)
		if !found {
			t.Errorf("transaction %s not found", paymentID)
		}
	}
}

func TestGetLatestBlockHeight(t *testing.T) {
	ledger := NewInMemoryLedger()

	if height := ledger.GetLatestBlockHeight(); height != 0 {
		t.Errorf("initial height should be 0, got %d", height)
	}

	// Add 150 transactions to create blocks
	for i := 1; i <= 150; i++ {
		_, _ = ledger.CommitTransaction("pay"+formatID(i), "account1", "account2", 100000)
	}

	if height := ledger.GetLatestBlockHeight(); height < 1 {
		t.Errorf("height should be at least 1, got %d", height)
	}
}

func TestGetTransactionByPaymentID(t *testing.T) {
	ledger := NewInMemoryLedger()

	paymentID := "unique-pay-001"
	_, _ = ledger.CommitTransaction(paymentID, "account1", "account2", 500000)

	entry, found, err := ledger.GetTransactionByPaymentID(paymentID)
	if err != nil || !found {
		t.Fatalf("GetTransactionByPaymentID failed: err=%v, found=%v", err, found)
	}

	if entry.PaymentID != paymentID {
		t.Errorf("payment ID mismatch: got %s, want %s", entry.PaymentID, paymentID)
	}

	// Non-existent
	_, found, _ = ledger.GetTransactionByPaymentID("nonexistent")
	if found {
		t.Error("should not find nonexistent transaction")
	}
}

func TestTransactionTimestampAndBlockHeight(t *testing.T) {
	ledger := NewInMemoryLedger()

	before := time.Now().UTC()
	_, _ = ledger.CommitTransaction("pay-time-test", "account1", "account2", 100000)
	after := time.Now().UTC()

	entry, _, _ := ledger.GetStatus("pay-time-test")

	if entry.Timestamp.Before(before) || entry.Timestamp.After(after.Add(time.Second)) {
		t.Errorf("timestamp not in expected range: before=%v, ts=%v, after=%v", before, entry.Timestamp, after)
	}

	if entry.BlockHeight != 0 {
		t.Errorf("first transaction should be in block 0, got %d", entry.BlockHeight)
	}
}

// Helper function to format integers with leading zeros
func formatID(n int) string {
	if n < 10 {
		return "000" + string(rune(48+n))
	}
	if n < 100 {
		return "00" + string(rune(48+n/10)) + string(rune(48+n%10))
	}
	return "0" + string(rune(48+n/100)) + string(rune(48+(n/10)%10)) + string(rune(48+n%10))
}
