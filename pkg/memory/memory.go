package memory

import "sync"

// KeyValueStore is a generic interface for agent memory.
//
// This is intentionally small:
// - It makes it easy to implement and test.
// - It provides a "slot" in the architecture where richer memory systems can fit.
//
// In production, you'd likely need:
// - namespaces (per agent, per tenant, per conversation)
// - typed values or structured documents
// - TTL and eviction policies
// - transactional semantics or concurrency control
type KeyValueStore interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string)
}

// InMemoryKV is a simple, thread-safe in-memory key-value store.
//
// It is useful for:
// - unit tests
// - demos
// - local experimentation
//
// It is not intended for production durability.
type InMemoryKV struct {
	mu sync.RWMutex
	m  map[string]string
}

// NewInMemoryKV constructs a new in-memory store.
//
// The returned store is safe for concurrent use.
func NewInMemoryKV() *InMemoryKV {
	return &InMemoryKV{
		m: make(map[string]string),
	}
}

// Get returns a value and a boolean indicating whether it was found.
//
// Thread-safety: read lock.
func (s *InMemoryKV) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

// Set stores a value for the given key.
//
// Thread-safety: write lock.
func (s *InMemoryKV) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// Delete removes a key.
//
// Thread-safety: write lock.
func (s *InMemoryKV) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

