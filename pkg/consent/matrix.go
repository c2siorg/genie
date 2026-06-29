package consent

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryConsentMatrix is a thread-safe, in-memory implementation of ConsentMatrix.
//
// It stores consents in two indices:
//   - byUserResource: keyed by "userID:resourceType", optimized for Grant/Revoke/Check
//   - byUser: keyed by userID, optimized for List operations
//
// All reads and writes are protected by a single RWMutex. For very high-concurrency
// scenarios, consider replacing this with a more sophisticated implementation
// (e.g. sharded locks, radix tree indices).
type InMemoryConsentMatrix struct {
	mu             sync.RWMutex
	byUserResource map[string]*ConsentRecord // key: "userID:resourceType"
	byUser         map[string][]*ConsentRecord
	byResourceType map[string][]*ConsentRecord
}

// NewInMemoryConsentMatrix creates a new, empty in-memory consent matrix.
func NewInMemoryConsentMatrix() *InMemoryConsentMatrix {
	return &InMemoryConsentMatrix{
		byUserResource: make(map[string]*ConsentRecord),
		byUser:         make(map[string][]*ConsentRecord),
		byResourceType: make(map[string][]*ConsentRecord),
	}
}

// Grant creates a new consent record or updates an existing one.
// If a consent for (userID, resourceType) already exists, it is replaced.
// TTL of 0 means the grant never expires.
func (m *InMemoryConsentMatrix) Grant(userID, resourceType string, perms PermissionLevel, ttl time.Duration, reason, grantedBy string) (string, error) {
	if userID == "" || resourceType == "" {
		return "", fmt.Errorf("%w: userID and resourceType cannot be empty", ErrInvalidInput)
	}

	if perms == PermNone {
		return "", fmt.Errorf("%w: cannot grant PermNone", ErrInvalidInput)
	}

	now := time.Now().UTC()
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = now.Add(ttl)
	}

	record := &ConsentRecord{
		ID:           uuid.New().String(),
		UserID:       userID,
		ResourceType: resourceType,
		Permissions:  perms,
		ExpiresAt:    expiresAt,
		Reason:       reason,
		GrantedBy:    grantedBy,
		CreatedAt:    now,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.makeKey(userID, resourceType)

	// If a previous record exists, we'll replace it but first check if we need to
	// clean up the byUser index.
	if old, exists := m.byUserResource[key]; exists {
		// Remove old record from byUser index
		m.removeFromUserList(userID, old)
		m.removeFromResourceTypeList(resourceType, old)
	}

	// Add new record
	m.byUserResource[key] = record

	// Update byUser index
	if _, ok := m.byUser[userID]; !ok {
		m.byUser[userID] = []*ConsentRecord{}
	}
	m.byUser[userID] = append(m.byUser[userID], record)

	// Update byResourceType index
	if _, ok := m.byResourceType[resourceType]; !ok {
		m.byResourceType[resourceType] = []*ConsentRecord{}
	}
	m.byResourceType[resourceType] = append(m.byResourceType[resourceType], record)

	return record.ID, nil
}

// Revoke removes a consent grant by (userID, resourceType).
// Returns ErrNotFound if no matching grant exists.
func (m *InMemoryConsentMatrix) Revoke(userID, resourceType string) error {
	if userID == "" || resourceType == "" {
		return fmt.Errorf("%w: userID and resourceType cannot be empty", ErrInvalidInput)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.makeKey(userID, resourceType)
	record, exists := m.byUserResource[key]
	if !exists {
		return ErrNotFound
	}

	// Remove from primary index
	delete(m.byUserResource, key)

	// Remove from byUser index
	m.removeFromUserList(userID, record)

	// Remove from byResourceType index
	m.removeFromResourceTypeList(resourceType, record)

	return nil
}

// Check evaluates whether a grant exists, is not expired, and covers the given action.
// Returns (allowed, permissionLevel, error).
func (m *InMemoryConsentMatrix) Check(userID, resourceType string, action ActionType) (bool, PermissionLevel, error) {
	if userID == "" || resourceType == "" {
		return false, PermNone, fmt.Errorf("%w: userID and resourceType cannot be empty", ErrInvalidInput)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.makeKey(userID, resourceType)
	record, exists := m.byUserResource[key]
	if !exists {
		return false, PermNone, nil
	}

	// Check expiration
	if record.IsExpired() {
		return false, PermNone, nil
	}

	// Check if permission covers the action
	if record.AllowsAction(action) {
		return true, record.Permissions, nil
	}

	return false, record.Permissions, nil
}

// List returns all consent records for a given userID, including expired ones.
// Returns an empty slice if the user has no grants.
func (m *InMemoryConsentMatrix) List(userID string) ([]ConsentRecord, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: userID cannot be empty", ErrInvalidInput)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.byUser[userID]
	if records == nil {
		return []ConsentRecord{}, nil
	}

	// Return a copy to avoid external mutation of our internal state
	result := make([]ConsentRecord, len(records))
	for i, r := range records {
		result[i] = *r
	}
	return result, nil
}

// ListByResource returns all consent records for a given resourceType.
// Returns an empty slice if no grants exist for the resource.
func (m *InMemoryConsentMatrix) ListByResource(resourceType string) ([]ConsentRecord, error) {
	if resourceType == "" {
		return nil, fmt.Errorf("%w: resourceType cannot be empty", ErrInvalidInput)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.byResourceType[resourceType]
	if records == nil {
		return []ConsentRecord{}, nil
	}

	// Return a copy to avoid external mutation of our internal state
	result := make([]ConsentRecord, len(records))
	for i, r := range records {
		result[i] = *r
	}
	return result, nil
}

// ---- Internal helpers ----

func (m *InMemoryConsentMatrix) makeKey(userID, resourceType string) string {
	return userID + ":" + resourceType
}

// removeFromUserList removes a specific record from the user's consent list.
// Assumes the mutex is already held.
func (m *InMemoryConsentMatrix) removeFromUserList(userID string, record *ConsentRecord) {
	records, ok := m.byUser[userID]
	if !ok {
		return
	}

	for i, r := range records {
		if r.ID == record.ID {
			// Remove by shifting
			m.byUser[userID] = append(records[:i], records[i+1:]...)
			if len(m.byUser[userID]) == 0 {
				delete(m.byUser, userID)
			}
			return
		}
	}
}

// removeFromResourceTypeList removes a specific record from the resourceType list.
// Assumes the mutex is already held.
func (m *InMemoryConsentMatrix) removeFromResourceTypeList(resourceType string, record *ConsentRecord) {
	records, ok := m.byResourceType[resourceType]
	if !ok {
		return
	}

	for i, r := range records {
		if r.ID == record.ID {
			// Remove by shifting
			m.byResourceType[resourceType] = append(records[:i], records[i+1:]...)
			if len(m.byResourceType[resourceType]) == 0 {
				delete(m.byResourceType, resourceType)
			}
			return
		}
	}
}
