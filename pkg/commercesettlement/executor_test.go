package commercesettlement

import (
	"testing"
	"time"
)

func TestExecuteBatchSuccess(t *testing.T) {
	batchMgr := NewInMemoryBatchManager()
	paymentBridge := NewCBDCPaymentAdapter()
	executor := NewDefaultSettlementExecutor(batchMgr, paymentBridge)

	// Create and populate batch.
	batch, _ := batchMgr.CreateBatch(time.Now(), []string{"m1", "m2"})
	batchMgr.AddMerchantAmounts(batch.ID, map[string]int64{
		"m1": 100,
		"m2": 80,
	})

	// Net positions: m1 owes 20, m2 is owed 20.
	netPositions := map[string]int64{
		"m1": 20,
		"m2": -20,
	}

	success, txnIDs, err := executor.ExecuteBatch(batch.ID, netPositions)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}
	if !success {
		t.Error("Expected successful execution")
	}
	if len(txnIDs) == 0 {
		t.Error("Expected at least one settlement transaction")
	}

	// Verify batch is marked as settled.
	batch, _ = batchMgr.GetBatch(batch.ID)
	if batch.Status != StatusSettled {
		t.Errorf("Expected status %s, got %s", StatusSettled, batch.Status)
	}
}

func TestExecuteBatchIdempotency(t *testing.T) {
	batchMgr := NewInMemoryBatchManager()
	paymentBridge := NewCBDCPaymentAdapter()
	executor := NewDefaultSettlementExecutor(batchMgr, paymentBridge)

	// Create batch.
	batch, _ := batchMgr.CreateBatch(time.Now(), []string{"m1", "m2"})
	batchMgr.AddMerchantAmounts(batch.ID, map[string]int64{
		"m1": 100,
		"m2": 80,
	})

	netPositions := map[string]int64{
		"m1": 20,
		"m2": -20,
	}

	// Execute once.
	success1, txnIDs1, err1 := executor.ExecuteBatch(batch.ID, netPositions)
	if err1 != nil {
		t.Fatalf("First ExecuteBatch failed: %v", err1)
	}

	// Execute again (should be idempotent).
	success2, txnIDs2, err2 := executor.ExecuteBatch(batch.ID, netPositions)
	if err2 != nil {
		t.Fatalf("Second ExecuteBatch failed: %v", err2)
	}

	// Both should succeed and return same txn IDs.
	if !success1 || !success2 {
		t.Error("Both executions should succeed")
	}
	if len(txnIDs1) != len(txnIDs2) {
		t.Errorf("Expected same txn count, got %d and %d", len(txnIDs1), len(txnIDs2))
	}
}

func TestExecuteBatchNettingValidation(t *testing.T) {
	batchMgr := NewInMemoryBatchManager()
	paymentBridge := NewCBDCPaymentAdapter()
	executor := NewDefaultSettlementExecutor(batchMgr, paymentBridge)

	batch, _ := batchMgr.CreateBatch(time.Now(), []string{"m1", "m2"})
	batchMgr.AddMerchantAmounts(batch.ID, map[string]int64{
		"m1": 100,
		"m2": 80,
	})

	// Invalid net positions (negative sum).
	invalidNetPositions := map[string]int64{
		"m1": 50,
		"m2": -100, // Sum = -50, which is negative and invalid
	}

	_, _, err := executor.ExecuteBatch(batch.ID, invalidNetPositions)
	if err == nil {
		t.Error("Expected error for negative net positions sum")
	}
}

func TestVerifySettlement(t *testing.T) {
	batchMgr := NewInMemoryBatchManager()
	paymentBridge := NewCBDCPaymentAdapter()
	executor := NewDefaultSettlementExecutor(batchMgr, paymentBridge)

	batch, _ := batchMgr.CreateBatch(time.Now(), []string{"m1", "m2"})
	batchMgr.AddMerchantAmounts(batch.ID, map[string]int64{
		"m1": 100,
		"m2": 80,
	})

	// Before execution, should not be verified.
	verified, err := executor.VerifySettlement(batch.ID)
	if err != nil {
		t.Fatalf("VerifySettlement failed: %v", err)
	}
	if verified {
		t.Error("Expected settlement to not be verified before execution")
	}

	// Execute settlement with balanced netting.
	netPositions := map[string]int64{
		"m1": 20,
		"m2": -20,
	}
	success, txnIDs, err := executor.ExecuteBatch(batch.ID, netPositions)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}
	if !success {
		t.Error("Expected successful execution")
	}
	if len(txnIDs) == 0 {
		t.Error("Expected settlement transactions")
	}
	t.Logf("Execution success: %v, txns: %d", success, len(txnIDs))

	// After execution, should be verified.
	verified, err = executor.VerifySettlement(batch.ID)
	if err != nil {
		t.Fatalf("VerifySettlement after execution failed: %v", err)
	}

	// Debug: check batch status
	batchAfter, _ := batchMgr.GetBatch(batch.ID)
	t.Logf("Batch status after execution: %v", batchAfter.Status)
	t.Logf("Verified flag: %v", verified)

	if !verified {
		t.Error("Expected settlement to be verified after execution")
	}
}

func TestCBDCPaymentAdapterCreatePayment(t *testing.T) {
	adapter := NewCBDCPaymentAdapter()

	txnID, err := adapter.CreatePayment("m1", "m2", 1000, map[string]any{})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if txnID == "" {
		t.Error("Expected non-empty transaction ID")
	}
}

func TestCBDCPaymentAdapterInvalidInput(t *testing.T) {
	adapter := NewCBDCPaymentAdapter()

	// Negative amount.
	_, err := adapter.CreatePayment("m1", "m2", -100, map[string]any{})
	if err == nil {
		t.Error("Expected error for negative amount")
	}

	// Missing from merchant.
	_, err = adapter.CreatePayment("", "m2", 100, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing from merchant")
	}

	// Missing to merchant.
	_, err = adapter.CreatePayment("m1", "", 100, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing to merchant")
	}
}

func TestAuditLog(t *testing.T) {
	log := NewAuditLog()

	batchID := "batch-123"
	log.LogAction(batchID, "batch_created", map[string]any{"merchants": 3})
	log.LogAction(batchID, "settlement_executed", map[string]any{"txn_count": 2})

	entries := log.GetEntries(batchID)
	if len(entries) != 2 {
		t.Errorf("Expected 2 audit entries, got %d", len(entries))
	}

	for _, entry := range entries {
		if entry.BatchID != batchID {
			t.Errorf("Expected batch ID %s, got %s", batchID, entry.BatchID)
		}
		if entry.ID == "" {
			t.Error("Expected non-empty audit entry ID")
		}
	}

	// Query for non-existent batch.
	emptyEntries := log.GetEntries("nonexistent")
	if len(emptyEntries) != 0 {
		t.Errorf("Expected 0 entries for nonexistent batch, got %d", len(emptyEntries))
	}
}

func TestExecuteBatchWithMultipleMerchants(t *testing.T) {
	batchMgr := NewInMemoryBatchManager()
	paymentBridge := NewCBDCPaymentAdapter()
	executor := NewDefaultSettlementExecutor(batchMgr, paymentBridge)

	// Three merchants.
	batch, _ := batchMgr.CreateBatch(time.Now(), []string{"m1", "m2", "m3"})
	batchMgr.AddMerchantAmounts(batch.ID, map[string]int64{
		"m1": 100,
		"m2": 80,
		"m3": 60,
	})

	// Net positions after netting.
	netPositions := map[string]int64{
		"m1": 50,
		"m2": 20,
		"m3": -70,
	}

	success, txnIDs, err := executor.ExecuteBatch(batch.ID, netPositions)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}
	if !success {
		t.Error("Expected successful execution")
	}

	// Should have 2 transactions (m1 and m2 are net payers).
	if len(txnIDs) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(txnIDs))
	}
}
