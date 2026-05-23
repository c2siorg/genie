package llm

import (
	"context"
	"time"
)

// DeadlineProvider applies a per-call deadline to Inner.Complete. Useful as
// a defense-in-depth layer on top of HTTP timeouts.
type DeadlineProvider struct {
	Inner   Provider
	Timeout time.Duration
}

// NewDeadline wraps with a per-call timeout (default 30s).
func NewDeadline(inner Provider, timeout time.Duration) *DeadlineProvider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &DeadlineProvider{Inner: inner, Timeout: timeout}
}

func (d *DeadlineProvider) Name() string   { return d.Inner.Name() + "+deadline" }
func (d *DeadlineProvider) Region() string { return d.Inner.Region() }

func (d *DeadlineProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	c, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	return d.Inner.Complete(c, req)
}
