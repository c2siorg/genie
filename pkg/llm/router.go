package llm

import (
	"context"
	"errors"
)

// Router selects a Provider based on the request. The simplest implementation
// is FixedRouter; ChainRouter chains a cheap provider with a fallback.
type Router interface {
	Pick(req CompletionRequest) Provider
}

// FixedRouter always returns Provider.
type FixedRouter struct{ Provider Provider }

// Pick returns the configured provider.
func (r FixedRouter) Pick(_ CompletionRequest) Provider { return r.Provider }

// ChainRouter returns Primary; falls back to Secondary if a Complete call
// errors. Used via ChainProvider below.
type ChainRouter struct {
	Primary, Secondary Provider
}

// Pick — unused; ChainProvider drives the chain semantics.
func (r ChainRouter) Pick(_ CompletionRequest) Provider { return r.Primary }

// ChainProvider implements Provider. Calls Primary; on error falls back to
// Secondary. Useful for cheap→expensive escalation: try Ollama locally,
// escalate to a hosted model on failure.
type ChainProvider struct {
	Primary, Secondary Provider
}

// NewChain returns the chain provider.
func NewChain(primary, secondary Provider) *ChainProvider {
	return &ChainProvider{Primary: primary, Secondary: secondary}
}

// Name reports both names so audit logs are explicit.
func (c *ChainProvider) Name() string {
	if c.Secondary == nil {
		return c.Primary.Name()
	}
	return c.Primary.Name() + "->" + c.Secondary.Name()
}

// Region returns the primary's region.
func (c *ChainProvider) Region() string { return c.Primary.Region() }

// Complete tries Primary then Secondary.
func (c *ChainProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	resp, err := c.Primary.Complete(ctx, req)
	if err == nil {
		return resp, nil
	}
	if c.Secondary == nil {
		return resp, err
	}
	return c.Secondary.Complete(ctx, req)
}

// ErrAllProvidersFailed is returned when every provider in a multi-chain
// fails. Useful when wrapping more than two providers.
var ErrAllProvidersFailed = errors.New("all providers failed")
