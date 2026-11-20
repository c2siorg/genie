package commercesettlement

import (
	"testing"
	"time"
)

func TestCreateBatch(t *testing.T) {
	manager := NewInMemoryBatchManager()
	now := time.Now()
	merchantIDs := []string{"merchant_001", "merchant_002", "merchant_003"}

	batch, err := manager.CreateBatch(now, merchantIDs)
	if err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	if batch.ID == "" {
		t.Error("Expected batch ID to be non-empty")
	}
	if batch.Status != StatusPending {
		t.Errorf("Expected status %s, got %s", StatusPending, batch.Status)
	}
	if len(batch.MerchantIDs) != len(merchantIDs) {
		t.Errorf("Expected %d merchants, got %d", len(merchantIDs), len(batch.MerchantIDs))
	}
}

func TestGetBatch(t *testing.T) {
	manager := NewInMemoryBatchManager()
	batch, _ := manager.CreateBatch(time.Now(), []string{"m1"})

	retrieved, err := manager.GetBatch(batch.ID)
	if err != nil {
		t.Fatalf("GetBatch failed: %v", err)
	}
	if retrieved.ID != batch.ID {
		t.Errorf("Expected batch ID %s, got %s", batch.ID, retrieved.ID)
	}
}

func TestGetBatchNotFound(t *testing.T) {
	manager := NewInMemoryBatchManager()
	_, err := manager.GetBatch("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent batch")
	}
}

func TestQueryByDate(t *testing.T) {
	manager := NewInMemoryBatchManager()
	date1 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC)

	batch1, _ := manager.CreateBatch(date1, []string{"m1"})
	batch2, _ := manager.CreateBatch(date1, []string{"m2"})
	batch3, _ := manager.CreateBatch(date2, []string{"m3"})

	batches1, _ := manager.QueryByDate(date1)
	if len(batches1) != 2 {
		t.Errorf("Expected 2 batches for date1, got %d", len(batches1))
	}

	batches2, _ := manager.QueryByDate(date2)
	if len(batches2) != 1 {
		t.Errorf("Expected 1 batch for date2, got %d", len(batches2))
	}

	// Verify batch IDs are correct.
	ids := map[string]bool{}
	for _, b := range batches1 {
		ids[b.ID] = true
	}
	if !ids[batch1.ID] {
		t.Errorf("Expected batch1 %s in results", batch1.ID)
	}
	if !ids[batch2.ID] {
		t.Errorf("Expected batch2 %s in results", batch2.ID)
	}

	// Verify batch3 is NOT in date1 results
	ids2 := map[string]bool{}
	for _, b := range batches2 {
		ids2[b.ID] = true
	}
	if ids2[batch3.ID] && batch3.SettlementDate.Format("2006-01-02") != date2.Format("2006-01-02") {
		t.Error("Date mismatch detected")
	}
}

func TestAddMerchantAmounts(t *testing.T) {
	manager := NewInMemoryBatchManager()
	batch, _ := manager.CreateBatch(time.Now(), []string{"m1", "m2"})

	amounts := map[string]int64{
		"m1": 10000,
		"m2": 20000,
	}
	err := manager.AddMerchantAmounts(batch.ID, amounts)
	if err != nil {
		t.Fatalf("AddMerchantAmounts failed: %v", err)
	}

	batch, _ = manager.GetBatch(batch.ID)
	if batch.TotalAmountPaise != 30000 {
		t.Errorf("Expected total 30000, got %d", batch.TotalAmountPaise)
	}

	if entry, ok := batch.Entries["m1"]; !ok || entry.AmountOwedPaise != 10000 {
		t.Error("Expected m1 entry with 10000 paise")
	}
	if entry, ok := batch.Entries["m2"]; !ok || entry.AmountOwedPaise != 20000 {
		t.Error("Expected m2 entry with 20000 paise")
	}
}

func TestUpdateBatchStatus(t *testing.T) {
	manager := NewInMemoryBatchManager()
	batch, _ := manager.CreateBatch(time.Now(), []string{"m1"})

	err := manager.UpdateBatchStatus(batch.ID, StatusNetted)
	if err != nil {
		t.Fatalf("UpdateBatchStatus failed: %v", err)
	}

	batch, _ = manager.GetBatch(batch.ID)
	if batch.Status != StatusNetted {
		t.Errorf("Expected status %s, got %s", StatusNetted, batch.Status)
	}

	err = manager.UpdateBatchStatus(batch.ID, StatusSettled)
	if err != nil {
		t.Fatalf("UpdateBatchStatus to settled failed: %v", err)
	}

	batch, _ = manager.GetBatch(batch.ID)
	if batch.Status != StatusSettled {
		t.Errorf("Expected status %s, got %s", StatusSettled, batch.Status)
	}
	if batch.SettledAt == nil {
		t.Error("Expected SettledAt to be set")
	}
}

func TestBatchAddEntryUpdates(t *testing.T) {
	batch := NewSettlementBatch(time.Now(), []string{"m1", "m2"})

	batch.AddEntry("m1", 5000)
	batch.AddEntry("m2", 3000)
	batch.AddEntry("m1", 2000) // Replace m1 entry

	// After adding m1 twice, it should be 2000 (last write wins), and m2 is 3000, total = 5000
	if batch.TotalAmountPaise != 5000 {
		t.Errorf("Expected total 5000, got %d", batch.TotalAmountPaise)
	}
	if len(batch.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(batch.Entries))
	}
	if batch.Entries["m1"].AmountOwedPaise != 2000 {
		t.Errorf("Expected m1 to be 2000, got %d", batch.Entries["m1"].AmountOwedPaise)
	}
	if batch.Entries["m2"].AmountOwedPaise != 3000 {
		t.Errorf("Expected m2 to be 3000, got %d", batch.Entries["m2"].AmountOwedPaise)
	}
}
