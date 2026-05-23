package synth

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FeedbackKind classifies a user's reaction to a recommendation/report.
type FeedbackKind string

const (
	FeedbackUp   FeedbackKind = "up"
	FeedbackDown FeedbackKind = "down"
	FeedbackEdit FeedbackKind = "edit" // user supplied a preferred answer
)

// Feedback is one captured preference signal. Aggregated traces become an
// RLAIF / DPO training set when you're ready to fine-tune.
type Feedback struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	TraceID      string                 `json:"trace_id"`
	Kind         FeedbackKind           `json:"kind"`
	Original     string                 `json:"original"`
	Preferred    string                 `json:"preferred,omitempty"` // populated when Kind=edit
	Tags         []string               `json:"tags,omitempty"`
	Metadata     map[string]any         `json:"metadata,omitempty"`
	RecordedAt   time.Time              `json:"recorded_at"`
}

// FeedbackStore is the persistence layer. Genie ships an in-memory store;
// production swaps in Postgres or any object store.
type FeedbackStore interface {
	Record(ctx context.Context, f Feedback) (Feedback, error)
	List(ctx context.Context, limit int) ([]Feedback, error)
}

// InMemoryFeedbackStore is the test/demo store.
type InMemoryFeedbackStore struct {
	mu    sync.RWMutex
	items []Feedback
}

// NewInMemoryFeedbackStore constructs the store.
func NewInMemoryFeedbackStore() *InMemoryFeedbackStore { return &InMemoryFeedbackStore{} }

// Record appends a feedback entry, generating an id + timestamp if absent.
func (s *InMemoryFeedbackStore) Record(_ context.Context, f Feedback) (Feedback, error) {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	if f.RecordedAt.IsZero() {
		f.RecordedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, f)
	return f, nil
}

// List returns the most recent entries.
func (s *InMemoryFeedbackStore) List(_ context.Context, limit int) ([]Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.items) {
		limit = len(s.items)
	}
	out := make([]Feedback, limit)
	copy(out, s.items[len(s.items)-limit:])
	return out, nil
}
