package hitl

import (
	"fmt"
	"sync"
	"time"
)

// ─── InMemoryStore ────────────────────────────────────────────────────────

// pendingApproval holds a request and the channel on which the decision
// will arrive.
type pendingApproval struct {
	request  ApprovalRequest
	decision chan ApprovalDecision
}

// InMemoryStore persists pending approval requests in memory. It is safe for
// concurrent use. For production multi-replica deployments, swap this for a
// Postgres-backed store that serialises requests to a table.
type InMemoryStore struct {
	mu      sync.Mutex
	pending map[string]*pendingApproval
}

// NewInMemoryStore creates an empty InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{pending: make(map[string]*pendingApproval)}
}

// Create registers a new approval request and returns a receive-only channel
// that will receive exactly one ApprovalDecision when Decide is called.
// The caller is responsible for supplying a unique req.ID.
func (s *InMemoryStore) Create(req ApprovalRequest) (<-chan ApprovalDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pending[req.ID]; exists {
		return nil, fmt.Errorf("hitl: duplicate request id %q", req.ID)
	}
	ch := make(chan ApprovalDecision, 1)
	s.pending[req.ID] = &pendingApproval{request: req, decision: ch}
	return ch, nil
}

// Decide records a decision for the given request ID and unblocks the waiting
// AsyncApprover. Returns an error if the ID is not found.
func (s *InMemoryStore) Decide(decision ApprovalDecision) error {
	s.mu.Lock()
	p, ok := s.pending[decision.RequestID]
	if ok {
		delete(s.pending, decision.RequestID)
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("hitl: no pending approval %q", decision.RequestID)
	}
	decision.DecidedAt = time.Now().UTC()
	p.decision <- decision
	return nil
}

// List returns a snapshot of all pending approval requests.
func (s *InMemoryStore) List() []ApprovalRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	reqs := make([]ApprovalRequest, 0, len(s.pending))
	for _, p := range s.pending {
		reqs = append(reqs, p.request)
	}
	return reqs
}

// Get returns the pending request with the given ID, or false if not found.
func (s *InMemoryStore) Get(id string) (ApprovalRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.pending[id]; ok {
		return p.request, true
	}
	return ApprovalRequest{}, false
}

// Cancel removes a pending request without delivering a decision. Use this to
// clean up expired requests. Callers blocked on the channel will be unblocked
// when their context expires.
func (s *InMemoryStore) Cancel(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
}
