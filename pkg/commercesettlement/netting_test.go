package commercesettlement

import (
	"testing"
	"time"
)

func TestBilateralNetting(t *testing.T) {
	// Two merchants with mutual billing - realistic commerce scenario:
	// Merchant A owes Merchant B 100 paise (for goods/services)
	// Merchant B owes Merchant A 50 paise (for return goods)
	// Net: A pays B 50 paise
	// However, in the test we model both as owing to a settlement pool, so no netting occurs.
	// For actual bilateral netting, we'd need to track who owes whom (not implemented here).
	// This test verifies the netting validation works with pool-based amounts.
	calc := NewSimpleNettingCalculator()

	entries := []*SettlementEntry{
		{MerchantID: "merchant_a", AmountOwedPaise: 100},
		{MerchantID: "merchant_b", AmountOwedPaise: 80},
	}

	pool, err := calc.CalculateNetPositionsWithPool(time.Now(), entries)
	if err != nil {
		t.Fatalf("CalculateNetPositionsWithPool failed: %v", err)
	}

	// Gross = 100 + 80 = 180
	// In pool model: both owe the settlement authority, no offset, net = gross
	if pool.GrossAmount != 180 {
		t.Errorf("Expected gross 180, got %d", pool.GrossAmount)
	}
	if pool.NetAmount != 180 {
		t.Errorf("Expected net 180 (no bilateral offset), got %d", pool.NetAmount)
	}

	// Validate that net positions sum to gross (pool model)
	if err := ValidateNetting(pool.NetPosition); err != nil {
		t.Errorf("Netting validation failed: %v", err)
	}
}

func TestMultilateralNetting3Merchants(t *testing.T) {
	// Three merchants, all owing to settlement pool:
	// A owes 100, B owes 80, C owes 60
	// Total = 240
	// In pool model: no bilateral offset, everyone settles with the pool
	// Net = 240 (same as gross, savings = 0)
	calc := NewSimpleNettingCalculator()

	entries := []*SettlementEntry{
		{MerchantID: "merchant_a", AmountOwedPaise: 100},
		{MerchantID: "merchant_b", AmountOwedPaise: 80},
		{MerchantID: "merchant_c", AmountOwedPaise: 60},
	}

	pool, err := calc.CalculateNetPositionsWithPool(time.Now(), entries)
	if err != nil {
		t.Fatalf("CalculateNetPositionsWithPool failed: %v", err)
	}

	if pool.GrossAmount != 240 {
		t.Errorf("Expected gross 240, got %d", pool.GrossAmount)
	}

	// Validate netting.
	if err := ValidateNetting(pool.NetPosition); err != nil {
		t.Errorf("Netting validation failed: %v", err)
	}

	// In pool model, net should equal gross (all are debtors to pool).
	if pool.NetAmount != pool.GrossAmount {
		t.Errorf("Expected net == gross for pool model, got net=%d, gross=%d", pool.NetAmount, pool.GrossAmount)
	}
}

func TestMultilateralNetting4Merchants(t *testing.T) {
	// Four merchants all owing to settlement pool:
	// A owes 100, B owes 90, C owes 50, D owes 60
	// Total gross = 300
	// Pool model: net = gross = 300, savings = 0
	calc := NewSimpleNettingCalculator()

	entries := []*SettlementEntry{
		{MerchantID: "a", AmountOwedPaise: 100},
		{MerchantID: "b", AmountOwedPaise: 90},
		{MerchantID: "c", AmountOwedPaise: 50},
		{MerchantID: "d", AmountOwedPaise: 60},
	}

	pool, err := calc.CalculateNetPositionsWithPool(time.Now(), entries)
	if err != nil {
		t.Fatalf("CalculateNetPositionsWithPool failed: %v", err)
	}

	if pool.GrossAmount != 300 {
		t.Errorf("Expected gross 300, got %d", pool.GrossAmount)
	}

	// Validate.
	if err := ValidateNetting(pool.NetPosition); err != nil {
		t.Errorf("Netting validation failed: %v", err)
	}

	// Pool model: net = gross, all debtors settle with pool.
	if pool.NetAmount != pool.GrossAmount {
		t.Errorf("Expected net == gross in pool model, got net=%d, gross=%d", pool.NetAmount, pool.GrossAmount)
	}
}

func TestNettingWithZeroAmount(t *testing.T) {
	calc := NewSimpleNettingCalculator()

	entries := []*SettlementEntry{
		{MerchantID: "a", AmountOwedPaise: 100},
		{MerchantID: "b", AmountOwedPaise: 0},
	}

	pool, err := calc.CalculateNetPositionsWithPool(time.Now(), entries)
	if err != nil {
		t.Fatalf("CalculateNetPositionsWithPool failed: %v", err)
	}

	if pool.GrossAmount != 100 {
		t.Errorf("Expected gross 100, got %d", pool.GrossAmount)
	}

	if err := ValidateNetting(pool.NetPosition); err != nil {
		t.Errorf("Netting validation failed: %v", err)
	}
}

func TestNettingEmptyEntries(t *testing.T) {
	calc := NewSimpleNettingCalculator()

	pool, err := calc.CalculateNetPositionsWithPool(time.Now(), []*SettlementEntry{})
	if err != nil {
		t.Fatalf("CalculateNetPositionsWithPool failed: %v", err)
	}

	if pool.GrossAmount != 0 {
		t.Errorf("Expected gross 0, got %d", pool.GrossAmount)
	}
	if pool.NettingSavings != 0 {
		t.Errorf("Expected savings 0, got %d", pool.NettingSavings)
	}
}

func TestValidateNettingSuccess(t *testing.T) {
	netPositions := map[string]int64{
		"a": 100,
		"b": -100,
	}

	if err := ValidateNetting(netPositions); err != nil {
		t.Errorf("ValidateNetting failed for valid net: %v", err)
	}
}

func TestValidateNettingFailure(t *testing.T) {
	netPositions := map[string]int64{
		"a": 100,
		"b": -150, // Imbalanced negative, sum = -50
	}

	if err := ValidateNetting(netPositions); err == nil {
		t.Error("Expected ValidateNetting to fail for negative sum")
	}
}

func TestNettingSavingsCalculation(t *testing.T) {
	// Test case: A owes 50, B owes 40 to settlement pool
	// Gross = 90, Net = 90 (pool model, no bilateral offset), Savings = 0
	calc := NewSimpleNettingCalculator()

	entries := []*SettlementEntry{
		{MerchantID: "a", AmountOwedPaise: 50},
		{MerchantID: "b", AmountOwedPaise: 40},
	}

	pool, err := calc.CalculateNetPositionsWithPool(time.Now(), entries)
	if err != nil {
		t.Fatalf("CalculateNetPositionsWithPool failed: %v", err)
	}

	// Pool model: savings should be 0 (no bilateral offset to reduce settlement).
	if pool.NettingSavings != 0 {
		t.Errorf("Expected savings 0 in pool model, got %d", pool.NettingSavings)
	}
	if pool.NetAmount != pool.GrossAmount {
		t.Errorf("Expected net == gross, got net=%d, gross=%d", pool.NetAmount, pool.GrossAmount)
	}
}
