package busio

import (
	"context"
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// EventTap is a broadcast subscriber that streams every message it sees to
// handlers keyed by trace_id. Used by the SSE endpoint to surface live
// agent.handle events to HTTP clients.
//
// One EventTap is constructed per process and shared across requests; HTTP
// handlers register a per-request channel via Subscribe(traceID).
type EventTap struct {
	mu      sync.Mutex
	streams map[string]chan protocol.Message
}

// NewEventTap subscribes the tap to bus broadcasts.
func NewEventTap(bus comm.Bus) *EventTap {
	t := &EventTap{streams: map[string]chan protocol.Message{}}
	bus.Subscribe("", func(_ context.Context, msg protocol.Message) {
		traceID, _ := msg.Metadata["trace_id"].(string)
		if traceID == "" {
			return
		}
		t.mu.Lock()
		ch := t.streams[traceID]
		t.mu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
				// Don't block the bus on a slow client.
			}
		}
	})
	return t
}

// Subscribe returns a channel that receives every bus message tagged with
// traceID. Buffered so a slow consumer doesn't block the bus.
func (t *EventTap) Subscribe(traceID string) chan protocol.Message {
	ch := make(chan protocol.Message, 32)
	t.mu.Lock()
	t.streams[traceID] = ch
	t.mu.Unlock()
	return ch
}

// Unsubscribe removes a trace's stream and closes the channel.
func (t *EventTap) Unsubscribe(traceID string) {
	t.mu.Lock()
	if ch, ok := t.streams[traceID]; ok {
		close(ch)
		delete(t.streams, traceID)
	}
	t.mu.Unlock()
}
