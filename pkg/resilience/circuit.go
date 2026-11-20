// Package resilience provides transport-agnostic reliability primitives shared
// across the platform. Its first member is a general-purpose circuit breaker.
//
// Why this exists: pkg/llm/circuit.go already implements a textbook breaker, but
// it is welded to the llm.Provider interface — NewCircuit takes a Provider and
// the only guarded method is Complete(CompletionRequest) (CompletionResponse).
// It cannot wrap an arbitrary HTTP call. The agent-decoupling work needs a
// breaker around HTTP calls to remote agent pods (in HTTPRegistryAgent and the
// agent HTTPClient), so this package generalizes the same state machine to any
// func(context.Context) error. The state-transition semantics here are
// deliberately identical to pkg/llm/circuit.go so behavior is consistent.
package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned by Do (and signalled by Allow returning false) when the
// breaker is tripped and short-circuiting calls.
var ErrOpen = errors.New("circuit breaker open")

// State describes the breaker's current mode.
type State int

const (
	StateClosed   State = iota // normal: all calls go through
	StateOpen                  // tripped: short-circuit without calling through
	StateHalfOpen              // recovery: allow a probe after cooldown
)

// String renders the state for logs and metrics.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Circuit is a concurrency-safe circuit breaker over an arbitrary operation.
// When consecutive failures reach Threshold it opens for CoolDown; the first
// call after CoolDown elapses is allowed through as a probe (half-open). A
// success resets to closed; a failure re-opens.
type Circuit struct {
	threshold int
	cooldown  time.Duration
	now       func() time.Time // injectable clock; defaults to time.Now

	mu       sync.Mutex
	state    State
	failures int
	openedAt time.Time
}

// New constructs a breaker. threshold defaults to 5; cooldown defaults to 30s.
func New(threshold int, cooldown time.Duration) *Circuit {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &Circuit{
		threshold: threshold,
		cooldown:  cooldown,
		now:       time.Now,
	}
}

// State returns the current breaker state. Note: when open and the cooldown has
// elapsed, the state still reads Open until the next Allow/Do call promotes it
// to half-open — matching the lazy transition in pkg/llm/circuit.go.
func (c *Circuit) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// IsOpen reports whether the breaker is currently short-circuiting. It accounts
// for an elapsed cooldown: an open breaker whose cooldown has passed is treated
// as not-open (a probe will be allowed), so callers using IsOpen as a gate do
// not get stuck after recovery.
func (c *Circuit) IsOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateOpen && c.now().Sub(c.openedAt) < c.cooldown {
		return true
	}
	return false
}

// Allow reports whether a call may proceed now, lazily promoting Open→HalfOpen
// once the cooldown has elapsed. Callers that drive the breaker manually use
// Allow + RecordSuccess/RecordFailure; most callers should prefer Do.
func (c *Circuit) Allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateOpen {
		if c.now().Sub(c.openedAt) >= c.cooldown {
			c.state = StateHalfOpen
			return true
		}
		return false
	}
	return true
}

// RecordSuccess clears the failure count and closes the breaker.
func (c *Circuit) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = 0
	c.state = StateClosed
}

// RecordFailure increments the failure count and opens the breaker once the
// threshold is reached (also re-opens on a failed half-open probe).
func (c *Circuit) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	if c.failures >= c.threshold {
		c.state = StateOpen
		c.openedAt = c.now()
	}
}

// Do runs fn through the breaker. If the breaker is open (cooldown not elapsed)
// it returns ErrOpen without invoking fn. Otherwise it invokes fn and records
// the outcome, returning fn's error unchanged on failure.
func (c *Circuit) Do(ctx context.Context, fn func(context.Context) error) error {
	if !c.Allow() {
		return ErrOpen
	}
	if err := fn(ctx); err != nil {
		c.RecordFailure()
		return err
	}
	c.RecordSuccess()
	return nil
}
