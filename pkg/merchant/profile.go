package merchant

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MerchantManager defines the interface for merchant profile management.
type MerchantManager interface {
	// CreateMerchant registers a new merchant and returns the profile.
	CreateMerchant(ctx context.Context, businessName, ownerID string, businessType BusinessType) (*MerchantProfile, error)

	// GetMerchant retrieves a merchant by ID.
	GetMerchant(ctx context.Context, id string) (*MerchantProfile, error)

	// GetMerchantByOwner retrieves all merchants owned by a user.
	GetMerchantByOwner(ctx context.Context, ownerID string) ([]*MerchantProfile, error)

	// UpdateLimits updates the daily transaction limit for a merchant.
	UpdateLimits(ctx context.Context, id string, limits MerchantLimits) error

	// UpdateStatus transitions merchant status with audit trail.
	UpdateStatus(ctx context.Context, id string, newStatus MerchantStatus, reason string) error

	// Suspend suspends a merchant account.
	Suspend(ctx context.Context, id string, reason string) error

	// AssignSettlementAccount assigns a bank account for settlement.
	AssignSettlementAccount(ctx context.Context, merchantID string, accountID string) error

	// ListAll returns all merchants (for admin operations).
	ListAll(ctx context.Context) ([]*MerchantProfile, error)
}

// InMemoryMerchantManager provides thread-safe in-memory merchant storage.
type InMemoryMerchantManager struct {
	mu        sync.RWMutex
	merchants map[string]*MerchantProfile
	byOwner   map[string][]*MerchantProfile // ownerID → merchants
	nextID    int64
}

// NewInMemoryMerchantManager creates a new in-memory merchant manager.
func NewInMemoryMerchantManager() *InMemoryMerchantManager {
	return &InMemoryMerchantManager{
		merchants: make(map[string]*MerchantProfile),
		byOwner:   make(map[string][]*MerchantProfile),
	}
}

// CreateMerchant registers a new merchant and assigns default limits.
func (m *InMemoryMerchantManager) CreateMerchant(ctx context.Context, businessName, ownerID string, businessType BusinessType) (*MerchantProfile, error) {
	if businessName == "" || ownerID == "" {
		return nil, fmt.Errorf("business_name and owner_id are required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := fmt.Sprintf("merchant_%d", m.nextID)
	now := time.Now()

	profile := &MerchantProfile{
		ID:              id,
		BusinessName:    businessName,
		OwnerID:         ownerID,
		BusinessType:    businessType,
		DailyLimitPaise: 50_000_000, // ₹500k default
		Status:          StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		Metadata:        make(map[string]string),
	}

	m.merchants[id] = profile
	m.byOwner[ownerID] = append(m.byOwner[ownerID], profile)

	return profile, nil
}

// GetMerchant retrieves a merchant by ID.
func (m *InMemoryMerchantManager) GetMerchant(ctx context.Context, id string) (*MerchantProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, exists := m.merchants[id]
	if !exists {
		return nil, fmt.Errorf("merchant not found: %s", id)
	}

	return profile, nil
}

// GetMerchantByOwner retrieves all merchants owned by a user.
func (m *InMemoryMerchantManager) GetMerchantByOwner(ctx context.Context, ownerID string) ([]*MerchantProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	merchants := m.byOwner[ownerID]
	if merchants == nil {
		return []*MerchantProfile{}, nil
	}

	// Return a copy to avoid external mutation
	result := make([]*MerchantProfile, len(merchants))
	copy(result, merchants)
	return result, nil
}

// UpdateLimits updates the daily transaction limit for a merchant.
func (m *InMemoryMerchantManager) UpdateLimits(ctx context.Context, id string, limits MerchantLimits) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.merchants[id]
	if !exists {
		return fmt.Errorf("merchant not found: %s", id)
	}

	profile.DailyLimitPaise = limits.DailyP2PPaise
	profile.UpdatedAt = time.Now()

	return nil
}

// UpdateStatus transitions merchant status with audit trail.
func (m *InMemoryMerchantManager) UpdateStatus(ctx context.Context, id string, newStatus MerchantStatus, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.merchants[id]
	if !exists {
		return fmt.Errorf("merchant not found: %s", id)
	}

	// Validate state transition
	validTransition := false
	switch profile.Status {
	case StatusPending:
		if newStatus == StatusApproved || newStatus == StatusRejected {
			validTransition = true
		}
	case StatusApproved:
		if newStatus == StatusSuspended {
			validTransition = true
		}
	case StatusSuspended:
		if newStatus == StatusApproved {
			validTransition = true
		}
	}

	if !validTransition {
		return fmt.Errorf("invalid status transition: %s -> %s", profile.Status, newStatus)
	}

	profile.Status = newStatus
	if newStatus == StatusSuspended && reason != "" {
		profile.SuspensionReason = reason
	}
	profile.UpdatedAt = time.Now()

	return nil
}

// Suspend suspends a merchant account.
func (m *InMemoryMerchantManager) Suspend(ctx context.Context, id string, reason string) error {
	return m.UpdateStatus(ctx, id, StatusSuspended, reason)
}

// AssignSettlementAccount assigns a bank account for settlement.
func (m *InMemoryMerchantManager) AssignSettlementAccount(ctx context.Context, merchantID string, accountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.merchants[merchantID]
	if !exists {
		return fmt.Errorf("merchant not found: %s", merchantID)
	}

	if accountID == "" {
		return fmt.Errorf("account_id cannot be empty")
	}

	profile.SettlementAccount = accountID
	profile.UpdatedAt = time.Now()

	return nil
}

// ListAll returns all merchants (for admin operations).
func (m *InMemoryMerchantManager) ListAll(ctx context.Context) ([]*MerchantProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MerchantProfile, 0, len(m.merchants))
	for _, p := range m.merchants {
		result = append(result, p)
	}

	return result, nil
}
