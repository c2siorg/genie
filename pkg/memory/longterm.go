// longterm.go — third memory tier: consolidated facts that survive across
// sessions.
//
// The existing tiers in this package:
//   - SemanticMemory — per-user vector store of arbitrary text.
//   - EpisodicMemory — rolling buffer per session with rollup summary.
//
// What was missing: a place to put *durable facts* about a user that the
// system has decided are stable enough to act on across sessions.
// Examples: "monthly net inflow ≈ ₹1.2L", "primary bank: HDFC", "risk
// appetite: moderate", "has 2 dependents".
//
// Pattern adopted from Google ADK samples → memory-bank. Tiered memory is
// table stakes for any agent system that wants to feel like it *knows* the
// user rather than re-deriving everything every turn.
//
// Design choices:
//   - Append-only. Updates are a new Fact superseding the old one. Keeps
//     audit clean for compliance review.
//   - Each fact carries a Confidence and a Source so the consumer can decide
//     whether to surface it ("I think you bank with HDFC — is that right?")
//     vs assume it ("Your HDFC statement…").
//   - Consolidation is offline. A separate worker reads episodic + semantic
//     memory periodically and proposes facts via the optional Consolidator
//     interface. The bank's risk team approves the consolidator's rule set.
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Fact is one consolidated, durable statement about a user.
type Fact struct {
	Key        string    // canonical key (e.g. "primary_bank", "risk_appetite")
	Value      string    // human-readable value (e.g. "HDFC", "moderate")
	Confidence float64   // 0..1 — the consolidator's confidence in this fact
	Source     string    // free-text provenance, e.g. "consolidated from 7 statements"
	RecordedAt time.Time // when this version was written
	SupersededAt *time.Time // nil if current; set when a later fact for the same key takes over
}

// LongTermMemory is the append-only fact store, partitioned per user.
type LongTermMemory struct {
	mu     sync.RWMutex
	facts  map[string][]Fact // userID -> ordered Facts (oldest first)
}

// NewLongTermMemory constructs an empty store.
func NewLongTermMemory() *LongTermMemory {
	return &LongTermMemory{facts: map[string][]Fact{}}
}

// Record appends a fact. If a current fact with the same Key already exists
// for the user, it is marked Superseded.
func (m *LongTermMemory) Record(userID string, f Fact) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f.RecordedAt.IsZero() {
		f.RecordedAt = time.Now().UTC()
	}
	facts := m.facts[userID]
	now := f.RecordedAt
	for i := range facts {
		if facts[i].SupersededAt == nil && facts[i].Key == f.Key {
			t := now
			facts[i].SupersededAt = &t
		}
	}
	m.facts[userID] = append(facts, f)
}

// Current returns the active fact for a key, or false if none.
func (m *LongTermMemory) Current(userID, key string) (Fact, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := len(m.facts[userID]) - 1; i >= 0; i-- {
		f := m.facts[userID][i]
		if f.Key == key && f.SupersededAt == nil {
			return f, true
		}
	}
	return Fact{}, false
}

// CurrentAll returns all active facts for the user, ordered by RecordedAt asc.
func (m *LongTermMemory) CurrentAll(userID string) []Fact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Fact{}
	for _, f := range m.facts[userID] {
		if f.SupersededAt == nil {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RecordedAt.Before(out[j].RecordedAt) })
	return out
}

// History returns every recorded fact for a key (current + superseded), oldest
// first. Useful for audit ("what did we think the user's primary bank was on
// May 14?").
func (m *LongTermMemory) History(userID, key string) []Fact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Fact{}
	for _, f := range m.facts[userID] {
		if f.Key == key {
			out = append(out, f)
		}
	}
	return out
}

// SearchValue does a substring contains-search across active facts.
// Cheap and predictable; for fuzzy search wire SemanticMemory.
func (m *LongTermMemory) SearchValue(userID, query string) []Fact {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Fact{}
	q := strings.ToLower(query)
	for _, f := range m.facts[userID] {
		if f.SupersededAt != nil {
			continue
		}
		if strings.Contains(strings.ToLower(f.Value), q) || strings.Contains(strings.ToLower(f.Key), q) {
			out = append(out, f)
		}
	}
	return out
}

// Consolidator is the (optional) offline worker shape — reads short-term
// memory and proposes new facts. Hosts can implement this with rules,
// statistics, or an LLM. The reference implementation ships none, because
// the policy team should own the rules.
type Consolidator interface {
	Consolidate(ctx context.Context, userID string) ([]Fact, error)
}

// Apply runs a consolidator and records its proposals. Returns the number
// of facts written.
func (m *LongTermMemory) Apply(ctx context.Context, userID string, c Consolidator) (int, error) {
	facts, err := c.Consolidate(ctx, userID)
	if err != nil {
		return 0, err
	}
	for _, f := range facts {
		m.Record(userID, f)
	}
	return len(facts), nil
}
