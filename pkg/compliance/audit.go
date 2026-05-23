package compliance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// AuditEntry is one row in the audit log. Each row references the prior
// row's hash, building a chain — clients can verify integrity by walking
// from genesis to head and recomputing hashes.
//
// The RBI cyber-resilience framework expects "logs that cannot be tampered
// with without detection." A hash chain is the cheapest way to give that
// guarantee without needing a write-once medium.
type AuditEntry struct {
	Seq        int64                  `json:"seq"`
	OccurredAt time.Time              `json:"occurred_at"`
	Actor      string                 `json:"actor"`   // user id, agent id, "system"
	Action     string                 `json:"action"`  // e.g. "consent.grant"
	Target     string                 `json:"target"`
	Details    map[string]any         `json:"details,omitempty"`
	PrevHash   string                 `json:"prev_hash"` // hex
	RowHash    string                 `json:"row_hash"`  // hex
}

// AuditLog is an append-only writer with hash chaining.
type AuditLog interface {
	Append(ctx context.Context, actor, action, target string, details map[string]any) (AuditEntry, error)
	List(ctx context.Context) ([]AuditEntry, error)
	Verify(ctx context.Context) error
}

// InMemoryAuditLog is the test/demo implementation.
type InMemoryAuditLog struct {
	mu      sync.RWMutex
	entries []AuditEntry
	next    int64
}

func NewInMemoryAuditLog() *InMemoryAuditLog { return &InMemoryAuditLog{} }

func (a *InMemoryAuditLog) Append(_ context.Context, actor, action, target string, details map[string]any) (AuditEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	prev := "genesis"
	if len(a.entries) > 0 {
		prev = a.entries[len(a.entries)-1].RowHash
	}
	a.next++
	e := AuditEntry{
		Seq:        a.next,
		OccurredAt: time.Now().UTC(),
		Actor:      actor,
		Action:     action,
		Target:     target,
		Details:    details,
		PrevHash:   prev,
	}
	e.RowHash = rowHash(prev, e)
	a.entries = append(a.entries, e)
	return e, nil
}

func (a *InMemoryAuditLog) List(_ context.Context) ([]AuditEntry, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]AuditEntry, len(a.entries))
	copy(out, a.entries)
	return out, nil
}

// Verify walks the chain and fails if any row's hash is inconsistent.
func (a *InMemoryAuditLog) Verify(_ context.Context) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	prev := "genesis"
	for _, e := range a.entries {
		if e.PrevHash != prev {
			return fmt.Errorf("audit: seq %d prev_hash mismatch", e.Seq)
		}
		want := rowHash(prev, e)
		if want != e.RowHash {
			return fmt.Errorf("audit: seq %d row_hash tampered", e.Seq)
		}
		prev = e.RowHash
	}
	return nil
}

// rowHash computes sha256(prev || canonical-json(row)) and returns the hex.
func rowHash(prev string, e AuditEntry) string {
	// Zero RowHash before hashing so the hash is over content only.
	e.RowHash = ""
	body, _ := json.Marshal(struct {
		Prev string     `json:"prev"`
		Row  AuditEntry `json:"row"`
	}{Prev: prev, Row: e})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
