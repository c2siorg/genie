package comm

import (
	"context"
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Handler receives messages published on the bus.
//
// In more advanced systems this is often a method on a subscriber object that
// can maintain state and expose lifecycle hooks. A simple function keeps the
// demo compact and readable.
type Handler func(ctx context.Context, msg protocol.Message)

// Bus is a simple pub/sub message bus for agents.
//
// Conceptually:
// - Subscribe defines "who is interested in what?"
// - Publish defines "a new event occurred; deliver it"
//
// This interface is intentionally small so you can later swap an in-memory bus
// with a distributed transport without touching agent code.
type Bus interface {
	// Subscribe registers a handler for messages addressed to a specific agentID.
	//
	// The returned function removes the subscription. (In long-running systems
	// you need this for shutdown/draining and dynamic membership.)
	Subscribe(agentID string, h Handler) (unsubscribe func())

	// Publish routes the message to matching subscribers.
	Publish(ctx context.Context, msg protocol.Message)
}

// InMemoryBus is a basic in-memory implementation of Bus.
//
// Routing rules used here:
// - If msg.To is non-empty: deliver to subscribers of that agentID.
// - Always deliver to broadcast subscribers (agentID == "").
//
// Concurrency model:
// - Subscription lists are guarded by an RWMutex.
// - Handler invocation happens in separate goroutines to avoid blocking Publish.
//
// Important caveats (by design for demo simplicity):
// - No backpressure: fast publishers can overwhelm slow handlers.
// - No ordering guarantees: goroutine scheduling may reorder delivery.
// - No retries: handler panics/errors are not managed here.
type InMemoryBus struct {
	mu        sync.RWMutex
	subs      map[string][]Handler
	broadcast []Handler
}

// NewInMemoryBus constructs a new in-memory bus.
//
// The returned bus is safe for concurrent use.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		subs: make(map[string][]Handler),
	}
}

// Subscribe registers a handler for messages addressed to a specific agent ID.
// If agentID is empty, the handler receives all messages (broadcast).
func (b *InMemoryBus) Subscribe(agentID string, h Handler) (unsubscribe func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if agentID == "" {
		b.broadcast = append(b.broadcast, h)
		idx := len(b.broadcast) - 1
		return func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if idx >= 0 && idx < len(b.broadcast) {
				b.broadcast[idx] = nil
			}
		}
	}
	b.subs[agentID] = append(b.subs[agentID], h)
	idx := len(b.subs[agentID]) - 1
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx >= 0 && idx < len(b.subs[agentID]) {
			b.subs[agentID][idx] = nil
		}
	}
}

// Publish sends a message to subscribed handlers.
//
// Message delivery is asynchronous (goroutines). This keeps the demo simple
// and avoids accidental deadlocks (e.g., a handler that publishes another
// message while holding locks).
//
// In a production bus you'd typically:
// - capture panics in handlers
// - propagate context deadlines/cancellation
// - implement buffering/backpressure strategies
func (b *InMemoryBus) Publish(ctx context.Context, msg protocol.Message) {
	tracer := otel.Tracer("github.com/c2siorg/genie/pkg/comm")
	ctx, span := tracer.Start(ctx, "bus.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("genie.msg.id", msg.ID),
			attribute.String("genie.msg.from", msg.From),
			attribute.String("genie.msg.to", msg.To),
			attribute.String("genie.msg.type", msg.Type),
			attribute.String("genie.msg.role", string(msg.Role)),
		),
	)
	defer span.End()

	if msg.Metadata == nil {
		msg.Metadata = map[string]any{}
	}
	observability.InjectTraceContext(ctx, msg.Metadata)

	if pm := observability.Metrics(); pm != nil && pm.MessagesPublished != nil {
		pm.MessagesPublished.Add(ctx, 1, metric.WithAttributes(
			attribute.String("msg.to", msg.To),
			attribute.String("msg.type", msg.Type),
		))
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Targeted subscribers.
	if msg.To != "" {
		for _, h := range b.subs[msg.To] {
			if h != nil {
				go h(ctx, msg)
			}
		}
	}

	// Broadcast subscribers see all messages.
	for _, h := range b.broadcast {
		if h != nil {
			go h(ctx, msg)
		}
	}
}


var _ Bus = (*InMemoryBus)(nil)

