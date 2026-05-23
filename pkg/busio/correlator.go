// Package busio bridges async multi-agent responses with synchronous callers
// (HTTP handlers, RPC, etc.).
package busio

import (
	"context"
	"errors"
	"sync"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// Correlator subscribes to "user" responses and lets HTTP handlers wait for
// the message produced by a given trace id. It is the bridge between async
// agent work and synchronous HTTP responses.
//
// Trade-off: holding HTTP requests open while the bus runs the pipeline is
// fine for a demo. For long-running flows replace with a callback URL or a
// status endpoint that polls a persistence layer.
type Correlator struct {
	mu      sync.Mutex
	waiters map[string]chan protocol.Message
}

// NewCorrelator constructs a correlator subscribed to messages addressed to
// the recipient id. Most callers pass "user" so the supervisor's final
// reports terminate here.
func NewCorrelator(bus comm.Bus, recipient string) *Correlator {
	c := &Correlator{waiters: map[string]chan protocol.Message{}}
	bus.Subscribe(recipient, func(_ context.Context, msg protocol.Message) {
		traceID, _ := msg.Metadata["trace_id"].(string)
		if traceID == "" {
			return
		}
		c.mu.Lock()
		ch, ok := c.waiters[traceID]
		if ok {
			delete(c.waiters, traceID)
		}
		c.mu.Unlock()
		if ok {
			select {
			case ch <- msg:
			default:
			}
		}
	})
	return c
}

// Await registers a waiter and returns a channel that delivers the first
// matching message. The caller MUST select on ctx.Done() to avoid leaks.
func (c *Correlator) Await(traceID string) <-chan protocol.Message {
	ch := make(chan protocol.Message, 1)
	c.mu.Lock()
	c.waiters[traceID] = ch
	c.mu.Unlock()
	return ch
}

// Cancel removes a pending waiter (e.g. on context cancellation).
func (c *Correlator) Cancel(traceID string) {
	c.mu.Lock()
	delete(c.waiters, traceID)
	c.mu.Unlock()
}

// ErrTimeout is returned by handlers that gave up before the bus produced a
// response.
var ErrTimeout = errors.New("timed out waiting for bus response")
