package eval

import (
	"sync"
	"time"
)

// InteractionRecord captures a simplified trace of an interaction for later evaluation.
//
// An "interaction" can mean many things:
// - one end-to-end user request handled by multiple agents
// - a single sub-task inside a larger workflow
// - a unit test scenario
//
// The goal is to record enough to compare runs over time.
type InteractionRecord struct {
	ID        string            `json:"id"`
	Scenario  string            `json:"scenario"`
	Success   bool              `json:"success"`
	Metrics   map[string]float64`json:"metrics,omitempty"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
	StartedAt time.Time         `json:"started_at"`
	EndedAt   time.Time         `json:"ended_at"`
}

// Store persists evaluation records.
//
// In production, this might write to:
// - a database
// - blob storage
// - an observability pipeline
// - a benchmark dashboard
type Store interface {
	Save(record InteractionRecord) error
	List() []InteractionRecord
}

// InMemoryStore is a trivial in-memory implementation of Store.
//
// This is meant for demos/tests; it does not persist across process restarts.
type InMemoryStore struct {
	mu      sync.RWMutex
	records []InteractionRecord
}

// NewInMemoryStore constructs a new in-memory evaluation store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

// Save appends a record.
//
// Thread-safety: write lock.
func (s *InMemoryStore) Save(r InteractionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	return nil
}

// List returns all stored records.
//
// The returned slice is a copy; callers can mutate it safely.
func (s *InMemoryStore) List() []InteractionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InteractionRecord, len(s.records))
	copy(out, s.records)
	return out
}

