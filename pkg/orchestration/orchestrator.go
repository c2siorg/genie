package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Orchestrator coordinates multiple agents, applying governance and routing messages via a bus.
//
// This type represents the "orchestrator" building block from the reference
// architecture.
//
// Key relationships:
// - Registry: provides the set of agents and their IDs/capabilities.
// - Bus: transports protocol messages between components.
// - Policy: enforces governance rules before work is performed.
// - Environment: provides shared utilities (logging, time, and future services).
//
// Importantly, Orchestrator does NOT contain agent-specific logic; it is a
// generic coordinator. This keeps the platform stable even as agents change.
type Orchestrator struct {
	reg       registry.Registry
	bus       comm.Bus
	policy    governance.Policy
	env       agent.Environment
	hooks     Hooks
	fallbacks map[string]string // agent id -> fallback agent id
}

// Hooks lets cmd-level code observe orchestration events without taking a
// dependency on incident / audit packages from inside pkg/orchestration.
type Hooks struct {
	OnPolicyDeny  func(ctx context.Context, msg agent.Message, reason string)
	OnAgentError  func(ctx context.Context, agentID string, msg agent.Message, err error)
}

// WithHooks installs the orchestrator hooks. Idempotent.
func (o *Orchestrator) WithHooks(h Hooks) *Orchestrator { o.hooks = h; return o }

// SetFallback declares that messages bound to original that fail under
// HandleMessage should be republished to fallbackID. Returns the orchestrator
// for chaining. Use the empty string to remove a mapping.
func (o *Orchestrator) SetFallback(originalID, fallbackID string) *Orchestrator {
	if o.fallbacks == nil {
		o.fallbacks = map[string]string{}
	}
	if fallbackID == "" {
		delete(o.fallbacks, originalID)
		return o
	}
	o.fallbacks[originalID] = fallbackID
	return o
}

// NewOrchestrator constructs a new orchestrator.
//
// Dependency injection is used so you can easily swap implementations:
// - a distributed registry vs in-memory
// - Kafka/NATS bus vs in-memory
// - a more advanced governance engine
// - a richer environment implementation
func NewOrchestrator(reg registry.Registry, bus comm.Bus, policy governance.Policy, env agent.Environment) *Orchestrator {
	return &Orchestrator{
		reg:    reg,
		bus:    bus,
		policy: policy,
		env:    env,
	}
}

// Start wiring: subscribe each registered agent to the bus.
//
// What Start does (step-by-step):
//
//  1) Snapshot the current agent list from the registry.
//  2) For each agent:
//      - Subscribe the bus on the agent's ID.
//      - When a message arrives:
//          a) Evaluate governance policy (allow/deny).
//          b) Invoke Agent.HandleMessage(...) with the message.
//          c) Publish each returned message back onto the bus.
//
// Why this design:
//
// - The orchestrator is the single place where governance is enforced.
// - Agents do not call each other directly; they communicate by emitting messages.
// - Observability becomes straightforward: all interactions are messages.
func (o *Orchestrator) Start(ctx context.Context) {
	tracer := otel.Tracer("github.com/c2siorg/genie/pkg/orchestration")
	pm := observability.Metrics()

	for _, a := range o.reg.List(ctx) {
		agentID := a.ID()
		ag := a
		o.bus.Subscribe(agentID, func(c context.Context, msg agent.Message) {
			// Re-attach the publisher's trace context so this span is a child
			// of the bus.publish span, even though the handler runs in its
			// own goroutine.
			c = observability.ExtractTraceContext(c, msg.Metadata)

			c, span := tracer.Start(c, "agent.handle",
				trace.WithSpanKind(trace.SpanKindConsumer),
				trace.WithAttributes(
					attribute.String("genie.agent.id", agentID),
					attribute.String("genie.agent.name", ag.Name()),
					attribute.String("genie.msg.id", msg.ID),
					attribute.String("genie.msg.from", msg.From),
					attribute.String("genie.msg.type", msg.Type),
				),
			)
			defer span.End()

			handleAttrs := []attribute.KeyValue{
				attribute.String("agent.id", agentID),
				attribute.String("msg.type", msg.Type),
			}

			if o.policy != nil {
				res, err := o.runPolicy(c, tracer, msg)
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, "policy error")
					o.env.Logf("policy error for msg %s: %v", msg.ID, err)
					return
				}
				if res.Decision == governance.DecisionDeny {
					span.SetAttributes(attribute.String("genie.policy.reason", res.Reason))
					span.SetStatus(codes.Error, "denied by policy")
					if pm != nil && pm.PolicyDenials != nil {
						pm.PolicyDenials.Add(c, 1, metric.WithAttributes(handleAttrs...))
					}
					if o.hooks.OnPolicyDeny != nil {
						o.hooks.OnPolicyDeny(c, msg, res.Reason)
					}
					o.env.Logf("message %s denied by policy: %s", msg.ID, res.Reason)
					return
				}
			}

			o.env.Logf("agent %s handling message %s from %s", agentID, msg.ID, msg.From)

			start := time.Now()
			out, err := ag.HandleMessage(c, msg, o.env)
			elapsedMs := float64(time.Since(start).Microseconds()) / 1000.0

			if pm != nil {
				if pm.HandleDuration != nil {
					pm.HandleDuration.Record(c, elapsedMs, metric.WithAttributes(handleAttrs...))
				}
				if pm.MessagesHandled != nil {
					pm.MessagesHandled.Add(c, 1, metric.WithAttributes(handleAttrs...))
				}
			}

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "agent error")
				if pm != nil && pm.AgentErrors != nil {
					pm.AgentErrors.Add(c, 1, metric.WithAttributes(handleAttrs...))
				}
				if o.hooks.OnAgentError != nil {
					o.hooks.OnAgentError(c, agentID, msg, err)
				}
				o.env.Logf("agent %s error: %v", agentID, err)
				// Route to fallback agent if one is registered for this id.
				if fb, ok := o.fallbacks[agentID]; ok && fb != "" {
					o.env.Logf("dispatching to fallback %s", fb)
					fbMsg := agent.NewMessage(agentID, fb, agent.RoleAgent, "fallback_request", msg.Content, msg.Metadata)
					o.bus.Publish(c, fbMsg)
				}
				return
			}

			span.SetAttributes(attribute.Int("genie.agent.outputs", len(out)))
			for _, m := range out {
				o.bus.Publish(c, m)
			}
		})
	}
}

func (o *Orchestrator) runPolicy(ctx context.Context, tracer trace.Tracer, msg agent.Message) (governance.PolicyResult, error) {
	ctx, span := tracer.Start(ctx, "governance.evaluate",
		trace.WithAttributes(
			attribute.String("genie.msg.id", msg.ID),
			attribute.String("genie.msg.type", msg.Type),
		),
	)
	defer span.End()

	res, err := o.policy.Evaluate(ctx, msg)
	if err != nil {
		return res, err
	}
	span.SetAttributes(
		attribute.String("genie.policy.decision", string(res.Decision)),
		attribute.String("genie.policy.reason", res.Reason),
	)
	return res, nil
}

// SimpleEnvironment is a basic implementation of agent.Environment using observability primitives.
//
// Environment exists so agents do not depend on concrete logging/time packages.
// For example, you can replace Logger with a structured logger, or wire a trace
// context into Logf.
type SimpleEnvironment struct {
	Logger observability.Logger
	Clock  observability.Clock
}

// Now returns the current time.
//
// Orchestrators and agents use time for timestamps, timeouts, and ordering hints.
// The Clock abstraction makes this deterministic in tests.
func (e *SimpleEnvironment) Now() time.Time {
	if e.Clock == nil {
		return time.Now().UTC()
	}
	return e.Clock.Now()
}

// Logf logs a formatted message.
//
// Logging is intentionally "printf-style" for simplicity. In production, you'd
// typically use structured logging with fields like trace_id, agent_id, msg_id,
// and message_type.
func (e *SimpleEnvironment) Logf(format string, args ...any) {
	if e.Logger == nil {
		fmt.Printf(format+"\n", args...)
		return
	}
	e.Logger.Printf(format, args...)
}

var _ agent.Environment = (*SimpleEnvironment)(nil)

