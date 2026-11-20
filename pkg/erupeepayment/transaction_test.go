package erupeepayment

import (
	"testing"
	"time"
)

func TestRecordTransaction(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Timestamp:   time.Now(),
		Status:      StatusPending,
	}
	err := log.Record(txn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txn.ID == "" {
		t.Errorf("transaction ID not assigned")
	}
}

func TestRecordTransactionMissingPaymentID(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn := &TransactionRecord{
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
	}
	err := log.Record(txn)
	if err == nil {
		t.Errorf("expected error for missing payment_id")
	}
}

func TestRecordTransactionMissingAccounts(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn := &TransactionRecord{
		PaymentID: "pay_001",
		Amount:    1_000,
	}
	err := log.Record(txn)
	if err == nil {
		t.Errorf("expected error for missing accounts")
	}
}

func TestRecordTransactionNegativeAmount(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      -1_000,
	}
	err := log.Record(txn)
	if err == nil {
		t.Errorf("expected error for negative amount")
	}
}

func TestQueryByAccountID(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	now := time.Now()
	txn1 := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Timestamp:   now,
		Status:      StatusPending,
	}
	txn2 := &TransactionRecord{
		PaymentID:   "pay_002",
		FromAccount: "acct_001",
		ToAccount:   "acct_003",
		Amount:      2_000,
		Timestamp:   now.Add(1 * time.Second),
		Status:      StatusPending,
	}
	log.Record(txn1)
	log.Record(txn2)

	results := log.Query("acct_001", now.Add(-1*time.Second), now.Add(2*time.Second))
	if len(results) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(results))
	}
}

func TestQueryByTimeRange(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	start := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)

	txn1 := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Timestamp:   start,
		Status:      StatusPending,
	}
	txn2 := &TransactionRecord{
		PaymentID:   "pay_002",
		FromAccount: "acct_001",
		ToAccount:   "acct_003",
		Amount:      2_000,
		Timestamp:   start.Add(1 * time.Hour),
		Status:      StatusPending,
	}
	txn3 := &TransactionRecord{
		PaymentID:   "pay_003",
		FromAccount: "acct_001",
		ToAccount:   "acct_004",
		Amount:      3_000,
		Timestamp:   start.Add(3 * time.Hour),
		Status:      StatusPending,
	}
	log.Record(txn1)
	log.Record(txn2)
	log.Record(txn3)

	// Query within first 90 minutes
	results := log.Query("acct_001", start, start.Add(90*time.Minute))
	if len(results) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(results))
	}
}

func TestQueryNoMatches(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	now := time.Now()
	results := log.Query("nonexistent", now, now.Add(1*time.Hour))
	if len(results) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(results))
	}
}

func TestGetStatus(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Status:      StatusPending,
	}
	log.Record(txn)

	status, found := log.GetStatus("pay_001")
	if !found {
		t.Errorf("expected to find payment status")
	}
	if status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, status)
	}
}

func TestGetStatusNotFound(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	_, found := log.GetStatus("nonexistent")
	if found {
		t.Errorf("expected payment not found")
	}
}

func TestGetStatusLatestUpdate(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn1 := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Status:      StatusPending,
	}
	txn2 := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Status:      StatusConfirmed,
		LedgerID:    "ledger_001",
	}
	log.Record(txn1)
	log.Record(txn2)

	status, _ := log.GetStatus("pay_001")
	if status != StatusConfirmed {
		t.Errorf("expected latest status %s, got %s", StatusConfirmed, status)
	}
}

func TestGetByPaymentID(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn1 := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Status:      StatusPending,
	}
	txn2 := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Status:      StatusConfirmed,
		LedgerID:    "ledger_001",
	}
	log.Record(txn1)
	log.Record(txn2)

	results := log.GetByPaymentID("pay_001")
	if len(results) != 2 {
		t.Errorf("expected 2 records, got %d", len(results))
	}
}

func TestGetByPaymentIDNotFound(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	results := log.GetByPaymentID("nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 records, got %d", len(results))
	}
}

func TestTransactionRecordIsCopy(t *testing.T) {
	log := NewInMemoryTransactionLog(nil)
	txn := &TransactionRecord{
		PaymentID:   "pay_001",
		FromAccount: "acct_001",
		ToAccount:   "acct_002",
		Amount:      1_000,
		Status:      StatusPending,
	}
	log.Record(txn)

	retrieved1 := log.Query("acct_001", time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))[0]
	retrieved1.Amount = 99_999
	retrieved2 := log.Query("acct_001", time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))[0]

	if retrieved2.Amount != 1_000 {
		t.Errorf("transaction isolation broken: got %d, want 1000", retrieved2.Amount)
	}
}

func TestConcurrentRecording(t *testing.T) {
	idCounter := 0
	idGen := func() string {
		idCounter++
		return "id_" + string(rune(idCounter))
	}
	log := NewInMemoryTransactionLog(idGen)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			txn := &TransactionRecord{
				PaymentID:   "pay_" + string(rune('A'+idx)), // Use unique letters A-J
				FromAccount: "acct_001",
				ToAccount:   "acct_002",
				Amount:      int64((idx + 1) * 1000),
				Status:      StatusPending,
			}
			log.Record(txn)
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	results := log.Query("acct_001", time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))
	if len(results) != 10 {
		t.Errorf("concurrent recording failed: got %d, want 10", len(results))
	}
}
