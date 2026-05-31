package hitl

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// AsyncApprover implements HITL for HTTP / serverless environments where the
// agent cannot block an OS thread indefinitely.
//
// Flow:
//  1. RequestApproval stores the request in the InMemoryStore and gets a channel.
//  2. It notifies all registered Notifiers (email, Slack, …) about the pending request.
//  3. It blocks on the channel (or ctx cancellation).
//  4. An HTTP handler (HITLHandler) receives the human's decision and calls
//     store.Decide, which sends on the channel and unblocks this method.
//
// The blocking here is goroutine-level only, not OS-thread-level, so Go's
// scheduler handles thousands of concurrent pending approvals efficiently.
type AsyncApprover struct {
	store     *InMemoryStore
	notifiers []Notifier
	// DefaultTTL is added to time.Now() to set ExpiresAt when it is zero.
	// Zero means no deadline.
	DefaultTTL time.Duration
}

// NewAsyncApprover creates an AsyncApprover backed by store.
// Notifiers are optional; add them with AddNotifier.
func NewAsyncApprover(store *InMemoryStore) *AsyncApprover {
	return &AsyncApprover{store: store}
}

// AddNotifier appends a Notifier that is called whenever a new approval is requested.
func (a *AsyncApprover) AddNotifier(n Notifier) {
	a.notifiers = append(a.notifiers, n)
}

// RequestApproval satisfies the Approver interface.
func (a *AsyncApprover) RequestApproval(ctx context.Context, req ApprovalRequest) (bool, error) {
	if req.ID == "" {
		req.ID = newID()
	}
	req.CreatedAt = time.Now().UTC()
	if req.ExpiresAt.IsZero() && a.DefaultTTL > 0 {
		req.ExpiresAt = req.CreatedAt.Add(a.DefaultTTL)
	}

	ch, err := a.store.Create(req)
	if err != nil {
		return false, err
	}

	// Notify all channels (best-effort — errors are ignored so a broken
	// notification path never blocks the agent).
	for _, n := range a.notifiers {
		_ = n.Notify(ctx, req)
	}

	// Apply deadline from ExpiresAt if it is in the future.
	waitCtx := ctx
	var cancel context.CancelFunc
	if !req.ExpiresAt.IsZero() {
		waitCtx, cancel = context.WithDeadline(ctx, req.ExpiresAt)
		defer cancel()
	}

	select {
	case <-waitCtx.Done():
		a.store.Cancel(req.ID)
		return false, waitCtx.Err()
	case decision := <-ch:
		return decision.Approved, nil
	}
}

// Store returns the underlying InMemoryStore so HTTP handlers can call Decide.
func (a *AsyncApprover) Store() *InMemoryStore { return a.store }

// ─── Notifier ─────────────────────────────────────────────────────────────

// Notifier sends a notification when an approval request is created.
// Implementations must be non-blocking (fire and forget or use goroutines).
type Notifier interface {
	Notify(ctx context.Context, req ApprovalRequest) error
}

// NotifierFunc lets a plain function satisfy Notifier.
type NotifierFunc func(ctx context.Context, req ApprovalRequest) error

// Notify implements Notifier.
func (f NotifierFunc) Notify(ctx context.Context, req ApprovalRequest) error {
	return f(ctx, req)
}

// ─── helpers ──────────────────────────────────────────────────────────────

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "hitl-" + hex.EncodeToString(b)
}
