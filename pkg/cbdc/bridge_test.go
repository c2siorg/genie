package cbdc

import (
	"strings"
	"testing"
	"time"
)

func TestNewMockCBDCBridge(t *testing.T) {
	tests := []struct {
		name          string
		delayBlocks   uint64
		expectedDelay uint64
	}{
		{"zero delay defaults to 1", 0, 1},
		{"delay 1", 1, 1},
		{"delay 3", 3, 3},
		{"delay 5 (max)", 5, 5},
		{"delay 6 capped to 5", 6, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bridge := NewMockCBDCBridge(tt.delayBlocks)
			// We can't directly access finalityDelayBlocks, but we can test behavior
			if bridge == nil {
				t.Fatal("bridge should not be nil")
			}
		})
	}
}

func TestInitiatePayment(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	resp, err := bridge.InitiatePayment("pay001", "account1", "account2", 100000)
	if err != nil {
		t.Fatalf("InitiatePayment failed: %v", err)
	}

	if resp == nil {
		t.Fatal("response should not be nil")
	}

	if resp.LedgerID == "" {
		t.Error("ledger ID should not be empty")
	}

	if resp.Status != StatusPending {
		t.Errorf("status should be pending, got %s", resp.Status)
	}

	if resp.BlockHeight != 0 {
		t.Errorf("first transaction should be in block 0, got %d", resp.BlockHeight)
	}

	// Finality timestamp should be in the future
	if resp.FinalityTimestamp.Before(time.Now().UTC()) {
		t.Error("finality timestamp should be in the future")
	}
}

func TestInitiatePaymentErrors(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	tests := []struct {
		name string
		call func() (*CBDCResponse, error)
	}{
		{
			"empty payment ID",
			func() (*CBDCResponse, error) {
				return bridge.InitiatePayment("", "account1", "account2", 100000)
			},
		},
		{
			"empty from account",
			func() (*CBDCResponse, error) {
				return bridge.InitiatePayment("pay002", "", "account2", 100000)
			},
		},
		{
			"empty to account",
			func() (*CBDCResponse, error) {
				return bridge.InitiatePayment("pay003", "account1", "", 100000)
			},
		},
		{
			"zero amount",
			func() (*CBDCResponse, error) {
				return bridge.InitiatePayment("pay004", "account1", "account2", 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call()
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestDoubleSpendOnBridge(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	_, err := bridge.InitiatePayment("pay-double", "account1", "account2", 100000)
	if err != nil {
		t.Fatalf("first initiate failed: %v", err)
	}

	// Attempt duplicate
	_, err = bridge.InitiatePayment("pay-double", "account1", "account3", 50000)
	if err == nil || !strings.Contains(err.Error(), "double-spend") {
		t.Errorf("expected double-spend error, got: %v", err)
	}
}

func TestGetPaymentStatus(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	// Initiate
	resp, _ := bridge.InitiatePayment("pay-status", "account1", "account2", 100000)

	// Get status
	status, err := bridge.GetPaymentStatus("pay-status")
	if err != nil {
		t.Fatalf("GetPaymentStatus failed: %v", err)
	}

	if status.LedgerID != resp.LedgerID {
		t.Error("ledger ID mismatch")
	}

	if status.Status != StatusPending {
		t.Errorf("status should still be pending, got %s", status.Status)
	}

	// Non-existent
	_, err = bridge.GetPaymentStatus("nonexistent")
	if err == nil {
		t.Error("should error on nonexistent payment")
	}
}

func TestFinalizePayment(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	// Initiate
	_, _ = bridge.InitiatePayment("pay-finalize", "account1", "account2", 100000)

	// Finalize
	resp, err := bridge.FinalizePayment("pay-finalize")
	if err != nil {
		t.Fatalf("FinalizePayment failed: %v", err)
	}

	if resp.Status != StatusFinalized {
		t.Errorf("status should be finalized, got %s", resp.Status)
	}
}

func TestFinalizePaymentNotFound(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	_, err := bridge.FinalizePayment("nonexistent")
	if err == nil {
		t.Error("should error on nonexistent payment")
	}
}

func TestGetBlock(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	// Initiate to create block
	_, _ = bridge.InitiatePayment("pay-block", "account1", "account2", 100000)

	// Get block
	block, err := bridge.GetBlock(0)
	if err != nil {
		t.Fatalf("GetBlock failed: %v", err)
	}

	if block.Height != 0 {
		t.Errorf("block height should be 0, got %d", block.Height)
	}

	// Non-existent block
	_, err = bridge.GetBlock(999)
	if err == nil {
		t.Error("should error on nonexistent block")
	}
}

func TestProcessFinality(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	// Initiate multiple
	for i := 1; i <= 3; i++ {
		pid := "pay-proc-" + string(rune(48+i))
		bridge.InitiatePayment(pid, "account1", "account2", 100000)
	}

	// All should be pending
	for i := 1; i <= 3; i++ {
		pid := "pay-proc-" + string(rune(48+i))
		status, _ := bridge.GetPaymentStatus(pid)
		if status.Status != StatusPending {
			t.Errorf("payment %s should be pending", pid)
		}
	}

	// Process finality (will finalize blocks older than 1 block)
	err := bridge.ProcessFinality()
	if err != nil {
		t.Fatalf("ProcessFinality failed: %v", err)
	}

	// Create new block to age the previous one
	bridge.InitiatePayment("pay-proc-new", "account1", "account2", 100000)

	// Process again
	err = bridge.ProcessFinality()
	if err != nil {
		t.Fatalf("ProcessFinality failed: %v", err)
	}

	// Now check if older ones are finalized
	status, _ := bridge.GetPaymentStatus("pay-proc-1")
	if status.Status != StatusFinalized {
		t.Errorf("old payment should be finalized, got %s", status.Status)
	}
}

func TestSetRBIAvailability(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	// Normal case - RBI available
	resp, err := bridge.InitiatePayment("pay-rbi-1", "account1", "account2", 100000)
	if err != nil {
		t.Fatalf("payment should succeed when RBI available: %v", err)
	}
	if resp.Status != StatusPending {
		t.Error("payment should be pending")
	}

	// Simulate RBI backend down
	bridge.SetRBIAvailability(false)

	// Should still work with fallback to local ledger
	resp, err = bridge.InitiatePayment("pay-rbi-2", "account1", "account2", 100000)
	if err != nil {
		t.Fatalf("payment should succeed with RBI fallback: %v", err)
	}
	if resp.Status != StatusPending {
		t.Error("payment should still be pending")
	}

	// Restore availability
	bridge.SetRBIAvailability(true)
	resp, err = bridge.InitiatePayment("pay-rbi-3", "account1", "account2", 100000)
	if err != nil {
		t.Fatalf("payment should succeed after restoring RBI: %v", err)
	}
}

func TestBridgeFinality(t *testing.T) {
	// Test with different finality delays
	for delayBlocks := uint64(1); delayBlocks <= 3; delayBlocks++ {
		t.Run("finality delay "+string(rune(48+delayBlocks)), func(t *testing.T) {
			bridge := NewMockCBDCBridge(delayBlocks)

			// Initiate
			resp1, _ := bridge.InitiatePayment("pay-fin-1", "account1", "account2", 100000)
			initialFinality := resp1.FinalityTimestamp

			// Check status
			status, _ := bridge.GetPaymentStatus("pay-fin-1")
			if status.Status != StatusPending {
				t.Error("should be pending")
			}

			// Finality timestamp should be approximately consistent (within 1 second)
			diff := status.FinalityTimestamp.Sub(initialFinality)
			if diff < -1*time.Second || diff > 1*time.Second {
				t.Errorf("finality timestamp changed unexpectedly: diff=%v", diff)
			}
		})
	}
}

func TestMultiplePayments(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	// Initiate 50 payments
	for i := 1; i <= 50; i++ {
		pid := "pay-multi-" + formatID(i)
		resp, err := bridge.InitiatePayment(pid, "account1", "account2", int64(100000*i))
		if err != nil {
			t.Fatalf("payment %d failed: %v", i, err)
		}
		if resp.Status != StatusPending {
			t.Errorf("payment %d status should be pending", i)
		}
	}

	// Verify all can be retrieved
	for i := 1; i <= 50; i++ {
		pid := "pay-multi-" + formatID(i)
		status, err := bridge.GetPaymentStatus(pid)
		if err != nil || status.Status != StatusPending {
			t.Errorf("payment %d retrieval failed", i)
		}
	}
}

func TestBridgeErrorTracking(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	// Should be no error initially
	if err := bridge.GetLastError(); err != nil {
		t.Errorf("should have no error initially, got: %v", err)
	}

	// Try invalid payment
	bridge.InitiatePayment("", "account1", "account2", 100000)
	if err := bridge.GetLastError(); err == nil {
		t.Error("should have captured error")
	}
}

func TestBridgeGetLedger(t *testing.T) {
	bridge := NewMockCBDCBridge(1)

	ledger := bridge.GetLedger()
	if ledger == nil {
		t.Fatal("ledger should not be nil")
	}

	// Verify ledger works
	bridge.InitiatePayment("pay-ledger", "account1", "account2", 100000)

	entry, found, _ := ledger.GetStatus("pay-ledger")
	if !found {
		t.Error("payment should be in ledger")
	}
	if entry.PaymentID != "pay-ledger" {
		t.Error("payment ID mismatch")
	}
}
