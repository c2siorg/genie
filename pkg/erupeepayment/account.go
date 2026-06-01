package erupeepayment

import (
	"fmt"
	"sync"
	"time"
)

// AccountManager defines operations on e-Rupee accounts.
type AccountManager interface {
	// GetAccount retrieves an account by ID; returns nil if not found.
	GetAccount(id string) *Account
	// CreateAccount creates a new account for a holder.
	CreateAccount(holderID string, acctType AccountType) (*Account, error)
	// UpdateBalance adjusts the account balance by delta (can be negative).
	// Returns error if the resulting balance would be negative.
	UpdateBalance(id string, delta int64) error
	// UpdateStatus changes the account status.
	UpdateStatus(id string, status AccountStatus) error
}

// InMemoryAccountManager is a thread-safe, map-based account store.
type InMemoryAccountManager struct {
	mu       sync.RWMutex
	accounts map[string]*Account
	idGen    func() string // injectable ID generator for testing
}

// NewInMemoryAccountManager creates a new in-memory account manager.
// idGen is used to generate account IDs; if nil, a simple counter is used.
func NewInMemoryAccountManager(idGen func() string) *InMemoryAccountManager {
	if idGen == nil {
		counter := 0
		idGen = func() string {
			counter++
			return fmt.Sprintf("acct_%d", counter)
		}
	}
	return &InMemoryAccountManager{
		accounts: make(map[string]*Account),
		idGen:    idGen,
	}
}

// GetAccount retrieves an account by ID.
func (m *InMemoryAccountManager) GetAccount(id string) *Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if acct, ok := m.accounts[id]; ok {
		// Return a shallow copy to prevent external mutations
		cpy := *acct
		return &cpy
	}
	return nil
}

// CreateAccount creates a new account with zero balance and active status.
func (m *InMemoryAccountManager) CreateAccount(holderID string, acctType AccountType) (*Account, error) {
	if holderID == "" {
		return nil, fmt.Errorf("holder_id cannot be empty")
	}
	if acctType != TypePersonal && acctType != TypeMerchant {
		return nil, fmt.Errorf("invalid account type: %s", acctType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	acct := &Account{
		ID:           m.idGen(),
		HolderID:     holderID,
		BalancePaise: 0,
		Type:         acctType,
		Status:       StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.accounts[acct.ID] = acct
	return acct, nil
}

// UpdateBalance adjusts the account balance. Rejects if balance would go negative.
func (m *InMemoryAccountManager) UpdateBalance(id string, delta int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acct, ok := m.accounts[id]
	if !ok {
		return fmt.Errorf("account not found: %s", id)
	}

	newBalance := acct.BalancePaise + delta
	if newBalance < 0 {
		return fmt.Errorf("insufficient balance: current=%d, delta=%d", acct.BalancePaise, delta)
	}

	acct.BalancePaise = newBalance
	acct.UpdatedAt = time.Now()
	return nil
}

// UpdateStatus changes the account status.
func (m *InMemoryAccountManager) UpdateStatus(id string, status AccountStatus) error {
	if status != StatusActive && status != StatusFrozen && status != StatusClosed {
		return fmt.Errorf("invalid account status: %s", status)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	acct, ok := m.accounts[id]
	if !ok {
		return fmt.Errorf("account not found: %s", id)
	}

	acct.Status = status
	acct.UpdatedAt = time.Now()
	return nil
}
