package fx

import (
	"fmt"
	"sync"
)

// LiquidityPoolManager manages currency pools and prevents over-reservation.
// Used for netting strategies where multiple currencies are pooled together.
type LiquidityPoolManager interface {
	// GetAvailable returns the current available balance in a specific currency.
	GetAvailable(currency string) (float64, error)

	// Reserve temporarily blocks funds for settlement. Returns error if insufficient balance.
	Reserve(currency string, amount float64) error

	// Release returns reserved funds back to the pool.
	Release(currency string, amount float64) error

	// GetAllPools returns a snapshot of all managed pools.
	GetAllPools() []LiquidityPool

	// RebalanceNeeded checks if any pool is below minimum threshold (10% capacity).
	RebalanceNeeded() bool
}

// MockLiquidityPoolManager provides in-memory pool management for testing.
// Thread-safe using sync.RWMutex. In production, this connects to a real
// settlement backend (bank APIs, correspondent banking, netting services).
type MockLiquidityPoolManager struct {
	mu              sync.RWMutex
	pools           map[string]*LiquidityPool
	reserved        map[string]float64
	minThresholdPct float64 // 10% default
}

// NewMockLiquidityPoolManager creates a pool manager with phase 1 currencies.
// Seed amounts are typical institutional correspondent balances.
func NewMockLiquidityPoolManager() *MockLiquidityPoolManager {
	return &MockLiquidityPoolManager{
		pools: map[string]*LiquidityPool{
			"USD": {
				Currency:          "USD",
				AvailableBalance:  5_000_000.0, // $5M
				Provider:          "correspondent",
				SettlementCostBps: 25, // 0.25%
			},
			"EUR": {
				Currency:          "EUR",
				AvailableBalance:  2_000_000.0, // €2M
				Provider:          "correspondent",
				SettlementCostBps: 30, // 0.30%
			},
			"GBP": {
				Currency:          "GBP",
				AvailableBalance:  1_500_000.0, // £1.5M
				Provider:          "correspondent",
				SettlementCostBps: 35, // 0.35%
			},
			"INR": {
				Currency:          "INR",
				AvailableBalance:  250_000_000.0, // ₹250M
				Provider:          "pool",
				SettlementCostBps: 40, // 0.40%
			},
			"AED": {
				Currency:          "AED",
				AvailableBalance:  10_000_000.0, // AED 10M
				Provider:          "correspondent",
				SettlementCostBps: 30, // 0.30%
			},
			"SGD": {
				Currency:          "SGD",
				AvailableBalance:  3_000_000.0, // SGD 3M
				Provider:          "correspondent",
				SettlementCostBps: 25, // 0.25%
			},
		},
		reserved:        make(map[string]float64),
		minThresholdPct: 10.0,
	}
}

// GetAvailable returns the currently available balance (total - reserved).
func (m *MockLiquidityPoolManager) GetAvailable(currency string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool := m.pools[currency]
	if pool == nil {
		return 0, fmt.Errorf("unknown currency: %s", currency)
	}

	reserved := m.reserved[currency]
	available := pool.AvailableBalance - reserved
	if available < 0 {
		available = 0 // Cap at 0 to prevent reporting negative available balance
	}
	return available, nil
}

// Reserve blocks funds for settlement. Returns error if insufficient balance.
func (m *MockLiquidityPoolManager) Reserve(currency string, amount float64) error {
	if amount < 0 {
		return fmt.Errorf("cannot reserve negative amount: %f", amount)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pool := m.pools[currency]
	if pool == nil {
		return fmt.Errorf("unknown currency: %s", currency)
	}

	reserved := m.reserved[currency]
	total := reserved + amount

	if total > pool.AvailableBalance {
		return fmt.Errorf("insufficient liquidity: have %.2f %s, need %.2f %s",
			pool.AvailableBalance-reserved, currency, amount, currency)
	}

	m.reserved[currency] = total
	return nil
}

// Release returns reserved funds back to the pool.
func (m *MockLiquidityPoolManager) Release(currency string, amount float64) error {
	if amount < 0 {
		return fmt.Errorf("cannot release negative amount: %f", amount)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pool := m.pools[currency]
	if pool == nil {
		return fmt.Errorf("unknown currency: %s", currency)
	}

	reserved := m.reserved[currency]
	if amount > reserved {
		return fmt.Errorf("cannot release %.2f %s: only %.2f reserved",
			amount, currency, reserved)
	}

	m.reserved[currency] = reserved - amount
	return nil
}

// GetAllPools returns a snapshot of all managed pools with current reserved amounts.
func (m *MockLiquidityPoolManager) GetAllPools() []LiquidityPool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]LiquidityPool, 0, len(m.pools))
	for _, pool := range m.pools {
		pool := *pool // Make a copy to avoid external mutation
		result = append(result, pool)
	}
	return result
}

// RebalanceNeeded checks if any pool is below the 10% minimum threshold.
func (m *MockLiquidityPoolManager) RebalanceNeeded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pool := range m.pools {
		reserved := m.reserved[pool.Currency]
		available := pool.AvailableBalance - reserved
		usagePct := (reserved / pool.AvailableBalance) * 100.0

		// If more than 90% of capacity is reserved, rebalancing is needed
		if usagePct > (100.0 - m.minThresholdPct) {
			return true
		}

		// Also trigger if available < 10% of capacity
		if available < (pool.AvailableBalance * (m.minThresholdPct / 100.0)) {
			return true
		}
	}

	return false
}
