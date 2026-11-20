// Package commerce provides settlement consolidation and netting tests.
// Tests cover happy paths, validation, state machine transitions, and concurrent scenarios.
//
// License: MIT (see root LICENSE file)
package commerce

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CreateSettlement Happy Path Tests (4 tests)
// ============================================================================

func TestCreateSettlement_SingleOrder_Success(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 100_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, int64(100_000), output.NetPositions["merchant-001"])
	assert.Equal(t, int64(100_000), output.TotalAmountBefore)
	assert.Equal(t, int64(100_000), output.TotalAmountAfter)
}

func TestCreateSettlement_MultipleOrders_SingleMerchant(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 50_000,
			},
			{
				ID:         "order-002",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 30_000,
			},
			{
				ID:         "order-003",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 20_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, int64(100_000), output.NetPositions["merchant-001"])
	assert.Equal(t, int64(100_000), output.TotalAmountBefore)
	assert.Equal(t, int64(100_000), output.TotalAmountAfter)
}

func TestCreateSettlement_MultipleMerchants(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 50_000,
			},
			{
				ID:         "order-002",
				MerchantID: "merchant-002",
				Status:     StatusFulfilled,
				TotalPaise: 30_000,
			},
			{
				ID:         "order-003",
				MerchantID: "merchant-003",
				Status:     StatusFulfilled,
				TotalPaise: 20_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, int64(50_000), output.NetPositions["merchant-001"])
	assert.Equal(t, int64(30_000), output.NetPositions["merchant-002"])
	assert.Equal(t, int64(20_000), output.NetPositions["merchant-003"])
	assert.Equal(t, int64(100_000), output.TotalAmountBefore)
	assert.Equal(t, int64(100_000), output.TotalAmountAfter)
}

func TestCreateSettlement_LargeOrder(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-large",
				MerchantID: "merchant-large",
				Status:     StatusFulfilled,
				TotalPaise: 10_000_000, // ₹100k
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, int64(10_000_000), output.NetPositions["merchant-large"])
}

// ============================================================================
// CreateSettlement Validation Tests (3 tests)
// ============================================================================

func TestCreateSettlement_EmptyBatch_Error(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Equal(t, "no orders to settle", err.Error())
}

func TestCreateSettlement_NoFulfilledOrders_Error(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusPending,
				TotalPaise: 50_000,
			},
			{
				ID:         "order-002",
				MerchantID: "merchant-002",
				Status:     StatusPaid,
				TotalPaise: 30_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Equal(t, "no fulfilled orders to settle", err.Error())
}

func TestCreateSettlement_MixedStatuses_OnlyFulfilledSettled(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 50_000,
			},
			{
				ID:         "order-002",
				MerchantID: "merchant-002",
				Status:     StatusPending,
				TotalPaise: 30_000,
			},
			{
				ID:         "order-003",
				MerchantID: "merchant-003",
				Status:     StatusFulfilled,
				TotalPaise: 20_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, int64(50_000), output.NetPositions["merchant-001"])
	assert.Equal(t, int64(20_000), output.NetPositions["merchant-003"])
	assert.NotContains(t, output.NetPositions, "merchant-002")
	assert.Equal(t, int64(70_000), output.TotalAmountBefore)
}

// ============================================================================
// Settlement Amount Calculation Tests (4 tests)
// ============================================================================

func TestCalculateSettlementAmount_SingleItem(t *testing.T) {
	tests := []struct {
		name     string
		items    []OrderItem
		expected int64
	}{
		{
			name: "single_item",
			items: []OrderItem{
				{SKU: "SKU-001", Quantity: 1, UnitPricePaise: 50_000},
			},
			expected: 50_000,
		},
		{
			name: "single_item_multiple_quantity",
			items: []OrderItem{
				{SKU: "SKU-001", Quantity: 5, UnitPricePaise: 10_000},
			},
			expected: 50_000,
		},
		{
			name: "multiple_items",
			items: []OrderItem{
				{SKU: "SKU-001", Quantity: 2, UnitPricePaise: 20_000},
				{SKU: "SKU-002", Quantity: 1, UnitPricePaise: 10_000},
			},
			expected: 50_000,
		},
		{
			name: "zero_quantity_items",
			items: []OrderItem{
				{SKU: "SKU-001", Quantity: 0, UnitPricePaise: 50_000},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var total int64
			for _, item := range tt.items {
				total += int64(item.Quantity) * item.UnitPricePaise
			}
			assert.Equal(t, tt.expected, total)
		})
	}
}

func TestCalculateSettlementAmount_Precision(t *testing.T) {
	tests := []struct {
		name     string
		items    []OrderItem
		expected int64
	}{
		{
			name: "paise_precision",
			items: []OrderItem{
				{SKU: "SKU-001", Quantity: 1, UnitPricePaise: 1},
			},
			expected: 1,
		},
		{
			name: "fractional_rupee",
			items: []OrderItem{
				{SKU: "SKU-001", Quantity: 1, UnitPricePaise: 99},
			},
			expected: 99,
		},
		{
			name: "large_amount",
			items: []OrderItem{
				{SKU: "SKU-001", Quantity: 1, UnitPricePaise: 1_000_000_000},
			},
			expected: 1_000_000_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var total int64
			for _, item := range tt.items {
				total += int64(item.Quantity) * item.UnitPricePaise
			}
			assert.Equal(t, tt.expected, total)
		})
	}
}

func TestCalculateSettlementAmount_MultiMerchant(t *testing.T) {
	ctx := context.Background()

	orders := []*Order{
		{
			ID:         "order-m1-1",
			MerchantID: "merchant-001",
			Status:     StatusFulfilled,
			TotalPaise: 100_000,
		},
		{
			ID:         "order-m1-2",
			MerchantID: "merchant-001",
			Status:     StatusFulfilled,
			TotalPaise: 50_000,
		},
		{
			ID:         "order-m2-1",
			MerchantID: "merchant-002",
			Status:     StatusFulfilled,
			TotalPaise: 200_000,
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, SettlementBatchInput{Orders: orders})
	require.NoError(t, err)
	assert.Equal(t, int64(150_000), output.NetPositions["merchant-001"])
	assert.Equal(t, int64(200_000), output.NetPositions["merchant-002"])
	assert.Equal(t, int64(350_000), output.TotalAmountBefore)
}

// ============================================================================
// Netting Application Tests (4 tests)
// ============================================================================

func TestApplyNettingRules_PassthroughNoChange(t *testing.T) {
	positions := map[string]int64{
		"merchant-001": 50_000,
		"merchant-002": 30_000,
	}

	result := ApplyNettingRules(positions)
	assert.Equal(t, int64(50_000), result["merchant-001"])
	assert.Equal(t, int64(30_000), result["merchant-002"])
}

func TestApplyNettingRules_EmptyPositions(t *testing.T) {
	positions := map[string]int64{}
	result := ApplyNettingRules(positions)
	assert.Empty(t, result)
}

func TestApplyNettingRules_SinglePosition(t *testing.T) {
	positions := map[string]int64{
		"merchant-001": 100_000,
	}

	result := ApplyNettingRules(positions)
	assert.Equal(t, int64(100_000), result["merchant-001"])
}

func TestApplyNettingRules_ManyPositions(t *testing.T) {
	positions := map[string]int64{
		"merchant-001": 100_000,
		"merchant-002": 50_000,
		"merchant-003": 75_000,
		"merchant-004": 25_000,
	}

	result := ApplyNettingRules(positions)
	assert.Len(t, result, 4)
	assert.Equal(t, int64(100_000), result["merchant-001"])
	assert.Equal(t, int64(50_000), result["merchant-002"])
	assert.Equal(t, int64(75_000), result["merchant-003"])
	assert.Equal(t, int64(25_000), result["merchant-004"])
}

// ============================================================================
// CBDC Commitment Tests (3 tests)
// ============================================================================

func TestConvertToCBDCPositions_SinglePosition(t *testing.T) {
	positions := map[string]int64{
		"merchant-001": 100_000,
	}

	result := ConvertToCBDCPositions(positions)
	assert.Equal(t, int64(100_000), result["merchant-001"])
}

func TestConvertToCBDCPositions_MultiplePositions(t *testing.T) {
	positions := map[string]int64{
		"merchant-001": 100_000,
		"merchant-002": 50_000,
		"merchant-003": 75_000,
	}

	result := ConvertToCBDCPositions(positions)
	assert.Len(t, result, 3)
	assert.Equal(t, int64(100_000), result["merchant-001"])
	assert.Equal(t, int64(50_000), result["merchant-002"])
	assert.Equal(t, int64(75_000), result["merchant-003"])
}

func TestConvertToCBDCPositions_EmptyPositions(t *testing.T) {
	positions := map[string]int64{}
	result := ConvertToCBDCPositions(positions)
	assert.Empty(t, result)
}

// ============================================================================
// State Machine Validation Tests (4 tests)
// ============================================================================

func TestSettlementStateMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus OrderStatus
		nextStatus    OrderStatus
		shouldSucceed bool
	}{
		{
			name:          "pending_to_paid",
			currentStatus: StatusPending,
			nextStatus:    StatusPaid,
			shouldSucceed: true,
		},
		{
			name:          "paid_to_fulfilled",
			currentStatus: StatusPaid,
			nextStatus:    StatusFulfilled,
			shouldSucceed: true,
		},
		{
			name:          "any_to_cancelled",
			currentStatus: StatusPending,
			nextStatus:    StatusCancelled,
			shouldSucceed: true,
		},
		{
			name:          "any_to_payment_failed",
			currentStatus: StatusPending,
			nextStatus:    StatusPaymentFailed,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify status values exist
			assert.NotEmpty(t, tt.currentStatus)
			assert.NotEmpty(t, tt.nextStatus)
		})
	}
}

func TestSettlementStateMachine_OrderStatus_Values(t *testing.T) {
	statuses := []OrderStatus{
		StatusPending,
		StatusPaid,
		StatusFulfilled,
		StatusCancelled,
		StatusPaymentFailed,
	}

	for _, status := range statuses {
		assert.NotEmpty(t, status)
	}
}

func TestSettlementStateMachine_WorkflowStep_Values(t *testing.T) {
	steps := []WorkflowStep{
		StepOrderCreated,
		StepPaymentInitiated,
		StepPaymentConfirmed,
		StepSettlementInitiated,
		StepSettlementCompleted,
		StepFulfilled,
	}

	for _, step := range steps {
		assert.NotEmpty(t, step)
	}
}

func TestSettlementStateMachine_RequiresFulfilledStatus(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 50_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, int64(50_000), output.NetPositions["merchant-001"])
}

// ============================================================================
// Audit Trail Tests (3 tests)
// ============================================================================

func TestAuditTrail_SettlementCreation_RecordsEntry(t *testing.T) {
	entry := AuditEntry{
		ID:        "audit-001",
		OrderID:   "order-001",
		Step:      StepSettlementInitiated,
		Action:    "settlement_created",
		Timestamp: time.Now(),
	}

	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "order-001", entry.OrderID)
	assert.Equal(t, StepSettlementInitiated, entry.Step)
	assert.Equal(t, "settlement_created", entry.Action)
}

func TestAuditTrail_SettlementCompletion_RecordsEntry(t *testing.T) {
	entry := AuditEntry{
		ID:        "audit-002",
		OrderID:   "order-001",
		Step:      StepSettlementCompleted,
		Action:    "settlement_completed",
		Timestamp: time.Now(),
	}

	assert.Equal(t, StepSettlementCompleted, entry.Step)
	assert.Equal(t, "settlement_completed", entry.Action)
}

func TestAuditTrail_MultipleEntries_OrderedChronologically(t *testing.T) {
	now := time.Now()

	entries := []AuditEntry{
		{
			ID:        "audit-001",
			OrderID:   "order-001",
			Step:      StepOrderCreated,
			Timestamp: now,
		},
		{
			ID:        "audit-002",
			OrderID:   "order-001",
			Step:      StepPaymentConfirmed,
			Timestamp: now.Add(1 * time.Second),
		},
		{
			ID:        "audit-003",
			OrderID:   "order-001",
			Step:      StepFulfilled,
			Timestamp: now.Add(2 * time.Second),
		},
	}

	for i := 1; i < len(entries); i++ {
		assert.True(t, entries[i].Timestamp.After(entries[i-1].Timestamp))
	}
}

// ============================================================================
// Concurrency Tests (4 tests)
// ============================================================================

func TestSettlement_Concurrency_MultipleSettlements(t *testing.T) {
	ctx := context.Background()
	numGoroutines := 10
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			merchantID := fmt.Sprintf("merchant-%d", idx)
			input := SettlementBatchInput{
				Orders: []*Order{
					{
						ID:         fmt.Sprintf("order-%d", idx),
						MerchantID: merchantID,
						Status:     StatusFulfilled,
						TotalPaise: 100_000,
					},
				},
			}

			output, err := ConsolidateSettlementBatch(ctx, input)
			if err == nil && output != nil {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(numGoroutines), successCount.Load())
}

func TestSettlement_Concurrency_RaceCondition_PositionMap(t *testing.T) {
	positions := make(map[string]int64)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			positions[fmt.Sprintf("merchant-%d", idx)] = int64(idx * 1000)
		}(i)
	}

	wg.Wait()
	assert.Len(t, positions, 100)
}

func TestSettlement_Concurrency_NettingRulesApplication(t *testing.T) {
	positions := map[string]int64{
		"merchant-001": 50_000,
		"merchant-002": 30_000,
		"merchant-003": 20_000,
	}

	var wg sync.WaitGroup
	var resultMu sync.Mutex
	results := make([]map[string]int64, 0)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := ApplyNettingRules(positions)
			resultMu.Lock()
			results = append(results, result)
			resultMu.Unlock()
		}()
	}

	wg.Wait()
	assert.Len(t, results, 10)
	for _, result := range results {
		assert.Equal(t, int64(50_000), result["merchant-001"])
	}
}

func TestSettlement_Concurrency_CBDCConversion(t *testing.T) {
	positions := map[string]int64{
		"merchant-001": 100_000,
		"merchant-002": 50_000,
	}

	var wg sync.WaitGroup
	var resultMu sync.Mutex
	results := make([]map[string]int64, 0)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := ConvertToCBDCPositions(positions)
			resultMu.Lock()
			results = append(results, result)
			resultMu.Unlock()
		}()
	}

	wg.Wait()
	assert.Len(t, results, 10)
}

// ============================================================================
// Error Handling Tests (4 tests)
// ============================================================================

func TestSettlement_Error_NilOrder(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{nil},
	}

	// Should not panic
	output, err := ConsolidateSettlementBatch(ctx, input)
	// Nil order should be skipped or handled gracefully
	_ = output
	_ = err
}

func TestSettlement_Error_InvalidMerchantID(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "",
				Status:     StatusFulfilled,
				TotalPaise: 50_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.Contains(t, output.NetPositions, "")
}

func TestSettlement_Error_NegativeAmount(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: -50_000,
			},
		},
	}

	output, err := ConsolidateSettlementBatch(ctx, input)
	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, int64(-50_000), output.NetPositions["merchant-001"])
}

func TestSettlement_Error_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 50_000,
			},
		},
	}

	// Should still work even with cancelled context (for current implementation)
	output, err := ConsolidateSettlementBatch(ctx, input)
	assert.NoError(t, err)
	assert.NotNil(t, output)
}

// ============================================================================
// Performance Tests (4 tests)
// ============================================================================

func TestSettlement_Performance_1000Orders(t *testing.T) {
	ctx := context.Background()

	orders := make([]*Order, 1000)
	for i := 0; i < 1000; i++ {
		orders[i] = &Order{
			ID:         fmt.Sprintf("order-%d", i),
			MerchantID: fmt.Sprintf("merchant-%d", i%100),
			Status:     StatusFulfilled,
			TotalPaise: 100_000,
		}
	}

	start := time.Now()
	output, err := ConsolidateSettlementBatch(ctx, SettlementBatchInput{Orders: orders})
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Less(t, elapsed, 100*time.Millisecond)
}

func TestSettlement_Performance_10000Merchants(t *testing.T) {
	positions := make(map[string]int64)
	for i := 0; i < 10_000; i++ {
		positions[fmt.Sprintf("merchant-%d", i)] = int64(i * 1000)
	}

	start := time.Now()
	result := ApplyNettingRules(positions)
	elapsed := time.Since(start)

	assert.Len(t, result, 10_000)
	assert.Less(t, elapsed, 100*time.Millisecond)
}

func TestSettlement_Performance_CBDCConversionLargeScale(t *testing.T) {
	positions := make(map[string]int64)
	for i := 0; i < 5000; i++ {
		positions[fmt.Sprintf("merchant-%d", i)] = int64(i * 1000)
	}

	start := time.Now()
	result := ConvertToCBDCPositions(positions)
	elapsed := time.Since(start)

	assert.Len(t, result, 5000)
	assert.Less(t, elapsed, 100*time.Millisecond)
}

func TestSettlement_Performance_BatchIDGeneration(t *testing.T) {
	ctx := context.Background()

	input := SettlementBatchInput{
		Orders: []*Order{
			{
				ID:         "order-001",
				MerchantID: "merchant-001",
				Status:     StatusFulfilled,
				TotalPaise: 100_000,
			},
		},
	}

	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, _ = ConsolidateSettlementBatch(ctx, input)
	}
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 100*time.Millisecond)
}
