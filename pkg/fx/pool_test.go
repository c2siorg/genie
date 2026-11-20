package fx

import (
	"testing"
)

func TestMockLiquidityPoolManagerGetAvailable(t *testing.T) {
	manager := NewMockLiquidityPoolManager()

	tests := []struct {
		name        string
		currency    string
		shouldErr   bool
		minExpected float64
	}{
		{"USD available", "USD", false, 0},
		{"EUR available", "EUR", false, 0},
		{"INR available", "INR", false, 0},
		{"Unknown currency", "XYZ", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			available, err := manager.GetAvailable(tt.currency)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if available < tt.minExpected {
					t.Errorf("available balance %.2f should be >= %.2f", available, tt.minExpected)
				}
			}
		})
	}
}

func TestMockLiquidityPoolManagerReserveRelease(t *testing.T) {
	manager := NewMockLiquidityPoolManager()

	// Get initial available balance
	initialUSD, _ := manager.GetAvailable("USD")

	// Reserve some funds
	reserveAmount := 100_000.0
	err := manager.Reserve("USD", reserveAmount)
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	// Check available balance decreased
	afterReserve, _ := manager.GetAvailable("USD")
	if afterReserve != initialUSD-reserveAmount {
		t.Errorf("expected %.2f available, got %.2f", initialUSD-reserveAmount, afterReserve)
	}

	// Release the funds
	err = manager.Release("USD", reserveAmount)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	// Check available balance restored
	afterRelease, _ := manager.GetAvailable("USD")
	if afterRelease != initialUSD {
		t.Errorf("expected %.2f available, got %.2f", initialUSD, afterRelease)
	}
}

func TestMockLiquidityPoolManagerInsufficientFunds(t *testing.T) {
	manager := NewMockLiquidityPoolManager()

	// Try to reserve more than available
	initialUSD, _ := manager.GetAvailable("USD")
	tooMuch := initialUSD + 1_000_000.0

	err := manager.Reserve("USD", tooMuch)
	if err == nil {
		t.Errorf("expected error when reserving more than available")
	}
}

func TestMockLiquidityPoolManagerReleaseMoreThanReserved(t *testing.T) {
	manager := NewMockLiquidityPoolManager()

	// Reserve
	err := manager.Reserve("EUR", 500_000.0)
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	// Try to release more than reserved
	err = manager.Release("EUR", 1_000_000.0)
	if err == nil {
		t.Errorf("expected error when releasing more than reserved")
	}
}

func TestMockLiquidityPoolManagerGetAllPools(t *testing.T) {
	manager := NewMockLiquidityPoolManager()

	pools := manager.GetAllPools()
	if len(pools) == 0 {
		t.Errorf("expected at least one pool")
	}

	// Verify all phase 1 currencies are present
	found := make(map[string]bool)
	for _, pool := range pools {
		found[pool.Currency] = true
	}

	expected := map[string]bool{
		"USD": true,
		"EUR": true,
		"GBP": true,
		"INR": true,
		"AED": true,
		"SGD": true,
	}

	for curr := range expected {
		if !found[curr] {
			t.Errorf("missing currency in pools: %s", curr)
		}
	}
}

func TestMockLiquidityPoolManagerRebalanceNeeded(t *testing.T) {
	manager := NewMockLiquidityPoolManager()

	// Initially should not need rebalancing
	if manager.RebalanceNeeded() {
		t.Errorf("fresh pools should not need rebalancing")
	}

	// Reserve 91% of USD (above 90% threshold)
	initialUSD, _ := manager.GetAvailable("USD")
	reserveAmount := (initialUSD * 91) / 100

	err := manager.Reserve("USD", reserveAmount)
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	// Now should need rebalancing
	if !manager.RebalanceNeeded() {
		t.Errorf("should need rebalancing when more than 90%% reserved")
	}

	// Release and verify back to normal
	err = manager.Release("USD", reserveAmount)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	if manager.RebalanceNeeded() {
		t.Errorf("should not need rebalancing after release")
	}
}

func TestMockLiquidityPoolManagerNegativeOperations(t *testing.T) {
	manager := NewMockLiquidityPoolManager()

	// Try negative reserve
	err := manager.Reserve("USD", -100.0)
	if err == nil {
		t.Errorf("expected error on negative reserve")
	}

	// Try negative release
	err = manager.Release("USD", -100.0)
	if err == nil {
		t.Errorf("expected error on negative release")
	}
}
