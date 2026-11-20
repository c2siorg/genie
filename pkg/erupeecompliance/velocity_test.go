package erupeecompliance

import (
	"context"
	"testing"
	"time"
)

func TestVelocityMonitor_WithinLimits(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	accountID := "acc_001"

	// Record one transaction
	err := monitor.RecordTransaction(ctx, accountID, 1_00_000, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check velocity
	allowed, reason := monitor.CheckVelocity(ctx, accountID)
	if !allowed {
		t.Errorf("expected allowed, got %s", reason)
	}
}

func TestVelocityMonitor_ExceededTransactionLimit(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	accountID := "acc_001"
	now := time.Now().UTC()

	// Record 11 transactions (limit is 10 per hour)
	for i := 0; i < 11; i++ {
		err := monitor.RecordTransaction(ctx, accountID, 1_00_000, now.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Check velocity - should be blocked
	allowed, reason := monitor.CheckVelocity(ctx, accountID)
	if allowed {
		t.Errorf("expected blocked due to transaction limit, got: %s", reason)
	}

	if reason == "" {
		t.Error("expected reason to be provided")
	}

	t.Logf("Velocity block reason: %s", reason)
}

func TestVelocityMonitor_ExceededHourlyAmountLimit(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	accountID := "acc_001"
	now := time.Now().UTC()

	// Record two large transactions that exceed hourly limit (50k)
	// Each is 30k, total 60k
	err := monitor.RecordTransaction(ctx, accountID, 30_00_000, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = monitor.RecordTransaction(ctx, accountID, 30_00_000, now.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check velocity - should be blocked
	allowed, reason := monitor.CheckVelocity(ctx, accountID)
	if allowed {
		t.Errorf("expected blocked due to amount limit, got: %s", reason)
	}

	t.Logf("Velocity block reason: %s", reason)
}

func TestVelocityMonitor_ExceededDailyAmountLimit(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	accountID := "acc_001"
	now := time.Now().UTC()

	// Record large transactions spread across the day (but within hourly limits)
	// Daily limit is 200k
	err := monitor.RecordTransaction(ctx, accountID, 100_00_000, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Record second transaction 2 hours later (different hourly period)
	err = monitor.RecordTransaction(ctx, accountID, 101_00_000, now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check velocity - should be blocked due to daily limit
	allowed, reason := monitor.CheckVelocity(ctx, accountID)
	if allowed {
		t.Errorf("expected blocked due to daily amount limit, got: %s", reason)
	}

	t.Logf("Daily limit violation: %s", reason)
}

func TestVelocityMonitor_HourlyResetAfterPeriod(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	accountID := "acc_001"
	now := time.Now().UTC()

	// Record transaction at current hour
	err := monitor.RecordTransaction(ctx, accountID, 1_00_000, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Record transaction 61 minutes later (should be in new hourly period)
	later := now.Add(61 * time.Minute)
	err = monitor.RecordTransaction(ctx, accountID, 1_00_000, later)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get the latest record
	record, err := monitor.GetRecord(ctx, accountID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After hourly reset, count should be 1 (just the new transaction)
	if record.TransactionCount != 1 {
		t.Errorf("expected transaction count 1 after reset, got %d", record.TransactionCount)
	}
}

func TestVelocityMonitor_GetRecord(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	accountID := "acc_001"
	now := time.Now().UTC()

	// Record transactions
	for i := 0; i < 3; i++ {
		err := monitor.RecordTransaction(ctx, accountID, 10_00_000, now.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Get record
	record, err := monitor.GetRecord(ctx, accountID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.TransactionCount != 3 {
		t.Errorf("expected 3 transactions, got %d", record.TransactionCount)
	}

	expectedAmount := int64(30_00_000)
	if record.TotalAmount != expectedAmount {
		t.Errorf("expected amount %.2f, got %.2f", float64(expectedAmount)/100.0, float64(record.TotalAmount)/100.0)
	}
}

func TestVelocityMonitor_Reset(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	accountID := "acc_001"
	now := time.Now().UTC()

	// Record transaction
	err := monitor.RecordTransaction(ctx, accountID, 1_00_000, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify record exists
	_, err = monitor.GetRecord(ctx, accountID)
	if err != nil {
		t.Fatalf("record should exist before reset: %v", err)
	}

	// Reset
	err = monitor.Reset(ctx, accountID)
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	// Verify record is gone
	_, err = monitor.GetRecord(ctx, accountID)
	if err == nil {
		t.Error("record should not exist after reset")
	}
}

func TestVelocityMonitor_ThreadSafety(t *testing.T) {
	monitor := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	now := time.Now().UTC()

	// Simulate concurrent transactions from multiple accounts
	done := make(chan error, 100)

	for i := 0; i < 100; i++ {
		go func(id int) {
			accountID := "acc_" + string(rune(id))
			err := monitor.RecordTransaction(ctx, accountID, int64(id*100_000), now)
			done <- err
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		err := <-done
		if err != nil {
			t.Errorf("goroutine error: %v", err)
		}
	}

	t.Log("Thread safety test passed")
}

func TestVelocityMonitor_CustomConfig(t *testing.T) {
	config := &VelocityConfig{
		MaxTransactionsPerHour: 5,
		MaxAmountPerHour:       10_00_000, // ₹10k
		MaxAmountPerDay:        20_00_000, // ₹20k
	}

	monitor := NewInMemoryVelocityMonitorWithConfig(config)
	ctx := context.Background()
	accountID := "acc_001"
	now := time.Now().UTC()

	// Record 6 transactions (should exceed custom limit of 5)
	for i := 0; i < 6; i++ {
		err := monitor.RecordTransaction(ctx, accountID, 1_00_000, now.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	allowed, reason := monitor.CheckVelocity(ctx, accountID)
	if allowed {
		t.Errorf("expected blocked with custom limit, got: %s", reason)
	}

	t.Logf("Custom config violation: %s", reason)
}
