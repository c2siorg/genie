package llm

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the breaker is tripped.
var ErrCircuitOpen = errors.New("llm circuit breaker open")

// CircuitState describes the breaker's current mode.
type CircuitState int

const (
	CircuitClosed CircuitState = iota // normal: all calls go through
	CircuitOpen                       // tripped: short-circuit with ErrCircuitOpen
	CircuitHalfOpen                   // recovery: allow one probe
)

// CircuitProvider wraps any Provider with a textbook circuit breaker. When
// consecutive failures exceed Threshold, it opens for CoolDown; the next
// call after CoolDown probes the provider.
type CircuitProvider struct {
	Inner     Provider
	Threshold int
	CoolDown  time.Duration

	mu          sync.Mutex
	state       CircuitState
	failures    int
	openedAt    time.Time
}

// NewCircuit constructs a breaker. threshold defaults to 5; cooldown 30s.
func NewCircuit(inner Provider, threshold int, cooldown time.Duration) *CircuitProvider {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitProvider{Inner: inner, Threshold: threshold, CoolDown: cooldown}
}

func (c *CircuitProvider) Name() string   { return c.Inner.Name() + "+circuit" }
func (c *CircuitProvider) Region() string { return c.Inner.Region() }

// State returns the current breaker state (test/observability).
func (c *CircuitProvider) State() CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func (c *CircuitProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	c.mu.Lock()
	if c.state == CircuitOpen {
		if time.Since(c.openedAt) >= c.CoolDown {
			c.state = CircuitHalfOpen
		} else {
			c.mu.Unlock()
			return CompletionResponse{}, ErrCircuitOpen
		}
	}
	c.mu.Unlock()

	resp, err := c.Inner.Complete(ctx, req)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.failures++
		if c.failures >= c.Threshold {
			c.state = CircuitOpen
			c.openedAt = time.Now()
		}
		return resp, err
	}
	// success → reset
	c.failures = 0
	c.state = CircuitClosed
	return resp, nil
}
