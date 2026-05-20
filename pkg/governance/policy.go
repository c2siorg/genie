package governance

import (
	"context"
	"errors"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// Decision describes the outcome of a policy check.
//
// A binary decision is intentionally simple for a reference implementation.
// Production platforms often extend this to include:
// - "allow with modifications" (e.g., redact sensitive data)
// - "allow but require approval" (human-in-the-loop)
// - structured actions (log, alert, throttle, reroute)
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// PolicyResult captures the decision plus optional explanation.
//
// Governance must be explainable. The Reason field provides a human-readable
// rationale suitable for logs and audits.
type PolicyResult struct {
	Decision    Decision
	Reason      string
	CheckedAt   time.Time
	CheckedByID string
}

// Policy is the interface for governance policies applied to messages.
//
// Policies are designed to be:
// - composable (many small policies are easier to manage than one huge policy)
// - deterministic where possible (predictability helps debugging and testing)
// - fast (they run in the critical path before every agent invocation)
type Policy interface {
	Evaluate(ctx context.Context, msg protocol.Message) (PolicyResult, error)
}

// CompositePolicy runs multiple policies and denies if any denies.
//
// This pattern lets you build a policy stack such as:
// - input size limits
// - sender allowlists
// - tool authorization checks
// - content filters
type CompositePolicy struct {
	policies []Policy
}

// NewComposite constructs a composite policy.
func NewComposite(policies ...Policy) *CompositePolicy {
	return &CompositePolicy{policies: policies}
}

// Evaluate runs all policies in order.
//
// Deny-on-first-failure is a common default because it:
// - keeps the execution fast
// - yields a single clear reason
//
// If you want "collect all reasons", you can implement an alternative composite.
func (c *CompositePolicy) Evaluate(ctx context.Context, msg protocol.Message) (PolicyResult, error) {
	now := time.Now().UTC()
	for _, p := range c.policies {
		res, err := p.Evaluate(ctx, msg)
		if err != nil {
			return PolicyResult{}, err
		}
		if res.Decision == DecisionDeny {
			if res.CheckedAt.IsZero() {
				res.CheckedAt = now
			}
			return res, nil
		}
	}
	return PolicyResult{
		Decision:  DecisionAllow,
		Reason:    "all policies allow",
		CheckedAt: now,
	}, nil
}

// MaxContentLengthPolicy denies messages whose content exceeds a limit.
//
// Why it exists:
// - protects the system from runaway payload sizes
// - is an easy example of a governance rule
//
// In LLM-backed systems, message size limits also help bound prompt costs.
type MaxContentLengthPolicy struct {
	Max int
}

// Evaluate applies the content length rule.
//
// Note: We check msg.Content length only. A production policy may consider the
// serialized size of the entire message (including Metadata) and apply limits
// per-role or per-message type.
func (p MaxContentLengthPolicy) Evaluate(_ context.Context, msg protocol.Message) (PolicyResult, error) {
	if p.Max <= 0 {
		return PolicyResult{}, errors.New("invalid max content length")
	}
	if len(msg.Content) > p.Max {
		return PolicyResult{
			Decision:  DecisionDeny,
			Reason:    "content too long",
			CheckedAt: time.Now().UTC(),
		}, nil
	}
	return PolicyResult{
		Decision:  DecisionAllow,
		Reason:    "within content length limit",
		CheckedAt: time.Now().UTC(),
	}, nil
}

