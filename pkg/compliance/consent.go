// Package compliance carries the RBI / DPDP-aligned safety primitives:
// explicit consent ledger, append-only tamper-evident audit log, and
// hooks for explainability on recommender output.
//
// The interfaces are storage-agnostic. Genie ships an in-memory impl for
// tests; a Postgres-backed impl lives in pkg/storage/postgres.
package compliance

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConsentCategory labels what the user is consenting to.
// Use short identifiers — they end up in audit logs and UIs.
type ConsentCategory string

const (
	CategoryTransactions ConsentCategory = "transactions"
	CategoryPortfolio    ConsentCategory = "portfolio"
	CategoryRecommend    ConsentCategory = "recommendations"
	CategoryThirdParty   ConsentCategory = "third_party_share"
)

// Consent represents a single user consent record.
type Consent struct {
	ID        string
	UserID    string
	Category  ConsentCategory
	Purpose   string
	Granted   bool
	GrantedAt time.Time
	RevokedAt *time.Time
}

// ErrConsentNotFound is returned when a (user, category) lookup misses.
var ErrConsentNotFound = errors.New("consent record not found")

// Ledger persists user consents.
type Ledger interface {
	Grant(ctx context.Context, userID string, category ConsentCategory, purpose string) (Consent, error)
	Revoke(ctx context.Context, userID string, category ConsentCategory) error
	HasActive(ctx context.Context, userID string, category ConsentCategory) (bool, error)
}

// InMemoryLedger is the test/demo Ledger.
type InMemoryLedger struct {
	mu      sync.RWMutex
	records []Consent
}

func NewInMemoryLedger() *InMemoryLedger { return &InMemoryLedger{} }

func (l *InMemoryLedger) Grant(_ context.Context, userID string, category ConsentCategory, purpose string) (Consent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := Consent{
		ID:        uuid.NewString(),
		UserID:    userID,
		Category:  category,
		Purpose:   purpose,
		Granted:   true,
		GrantedAt: time.Now().UTC(),
	}
	l.records = append(l.records, c)
	return c, nil
}

func (l *InMemoryLedger) Revoke(_ context.Context, userID string, category ConsentCategory) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UTC()
	for i := range l.records {
		c := &l.records[i]
		if c.UserID == userID && c.Category == category && c.Granted {
			c.Granted = false
			c.RevokedAt = &now
		}
	}
	return nil
}

func (l *InMemoryLedger) HasActive(_ context.Context, userID string, category ConsentCategory) (bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, c := range l.records {
		if c.UserID == userID && c.Category == category && c.Granted {
			return true, nil
		}
	}
	return false, nil
}
