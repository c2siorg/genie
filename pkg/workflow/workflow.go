// Package workflow implements an explicit DAG runtime over Genie agents,
// with Saga-style compensations and human-in-the-loop checkpoints.
//
// Genie's bus is implicit — agents route messages by name and the
// orchestrator wires the graph at runtime. For multi-step flows that need
// rollback (transferring money, granting consent, calling third parties)
// the implicit shape is hard to reason about. Workflow makes the graph
// declarative so it can be inspected, replayed, and rolled back.
//
// Three guarantees:
//
//   - Steps run in topological order.
//   - On failure, all already-completed steps with compensations run them
//     in reverse — classic Saga.
//   - Steps marked RequireApproval pause until ApproveStep is called.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Step is one node in the DAG.
type Step struct {
	ID              string
	DependsOn       []string
	Run             func(ctx context.Context, state State) error
	Compensate      func(ctx context.Context, state State) error // optional rollback
	RequireApproval bool                                          // pause until ApproveStep is called
}

// State is the rolling key/value bag a step can read from + write to.
type State map[string]any

// Get returns the typed value if present.
func (s State) Get(key string) (any, bool) { v, ok := s[key]; return v, ok }

// Set writes a key.
func (s State) Set(key string, v any) { s[key] = v }

// EventKind labels each entry in the event-sourced log.
type EventKind string

const (
	EventStarted    EventKind = "started"
	EventCompleted  EventKind = "completed"
	EventFailed     EventKind = "failed"
	EventAwaiting   EventKind = "awaiting_approval"
	EventApproved   EventKind = "approved"
	EventCompensated EventKind = "compensated"
)

// Event records one transition.
type Event struct {
	StepID     string         `json:"step_id"`
	Kind       EventKind      `json:"kind"`
	OccurredAt time.Time      `json:"occurred_at"`
	Detail     string         `json:"detail,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// EventSink is the append-only log. InMemorySink ships for tests; production
// implementations write to Postgres or Kafka.
type EventSink interface {
	Append(Event) error
	Events() []Event
}

// InMemorySink is the demo sink.
type InMemorySink struct {
	mu     sync.RWMutex
	events []Event
}

// NewInMemorySink builds an empty sink.
func NewInMemorySink() *InMemorySink { return &InMemorySink{} }

// Append records the event.
func (s *InMemorySink) Append(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	s.events = append(s.events, e)
	return nil
}

// Events returns a copy.
func (s *InMemorySink) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// Workflow is a runnable DAG.
type Workflow struct {
	Steps map[string]Step
	Sink  EventSink

	mu        sync.Mutex
	approvals map[string]chan struct{}
}

// New builds an empty workflow.
func New(sink EventSink) *Workflow {
	if sink == nil {
		sink = NewInMemorySink()
	}
	return &Workflow{Steps: map[string]Step{}, Sink: sink, approvals: map[string]chan struct{}{}}
}

// Add registers a step. Returns the workflow for chaining.
func (w *Workflow) Add(s Step) *Workflow {
	if s.ID == "" {
		panic("workflow: step id required")
	}
	w.Steps[s.ID] = s
	return w
}

// ApproveStep unblocks a step waiting on human approval. Returns false if
// the step isn't currently waiting.
func (w *Workflow) ApproveStep(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch, ok := w.approvals[id]
	if !ok {
		return false
	}
	delete(w.approvals, id)
	close(ch)
	return true
}

// Run executes the DAG. Returns the first error encountered after running
// compensations for already-completed steps.
func (w *Workflow) Run(ctx context.Context, state State) error {
	order, err := w.topoSort()
	if err != nil {
		return err
	}
	done := map[string]bool{}
	for _, id := range order {
		step := w.Steps[id]
		// Check dependencies were met (topo sort should guarantee this, but
		// be defensive in case a step was added mid-run).
		for _, d := range step.DependsOn {
			if !done[d] {
				return fmt.Errorf("workflow: step %q missing dependency %q", id, d)
			}
		}

		if step.RequireApproval {
			ch := make(chan struct{})
			w.mu.Lock()
			w.approvals[id] = ch
			w.mu.Unlock()
			_ = w.Sink.Append(Event{StepID: id, Kind: EventAwaiting})
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ch:
				_ = w.Sink.Append(Event{StepID: id, Kind: EventApproved})
			}
		}

		_ = w.Sink.Append(Event{StepID: id, Kind: EventStarted})
		if err := step.Run(ctx, state); err != nil {
			_ = w.Sink.Append(Event{StepID: id, Kind: EventFailed, Detail: err.Error()})
			w.compensate(ctx, order, done, state, id)
			return fmt.Errorf("workflow: step %q failed: %w", id, err)
		}
		_ = w.Sink.Append(Event{StepID: id, Kind: EventCompleted})
		done[id] = true
	}
	return nil
}

// compensate runs Compensate on every already-completed step in reverse order.
func (w *Workflow) compensate(ctx context.Context, order []string, done map[string]bool, state State, _ string) {
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		if !done[id] {
			continue
		}
		step := w.Steps[id]
		if step.Compensate == nil {
			continue
		}
		if err := step.Compensate(ctx, state); err != nil {
			_ = w.Sink.Append(Event{StepID: id, Kind: EventFailed, Detail: "compensation failed: " + err.Error()})
			continue
		}
		_ = w.Sink.Append(Event{StepID: id, Kind: EventCompensated})
	}
}

// topoSort returns step IDs in dependency order. Errors on cycles.
func (w *Workflow) topoSort() ([]string, error) {
	indeg := map[string]int{}
	for id := range w.Steps {
		indeg[id] = 0
	}
	for _, s := range w.Steps {
		for _, d := range s.DependsOn {
			if _, ok := w.Steps[d]; !ok {
				return nil, fmt.Errorf("workflow: step %q depends on unknown %q", s.ID, d)
			}
			indeg[s.ID]++
		}
	}
	queue := make([]string, 0, len(w.Steps))
	for id, n := range indeg {
		if n == 0 {
			queue = append(queue, id)
		}
	}
	var order []string
	for len(queue) > 0 {
		// Pop one with smallest id for deterministic order.
		min := 0
		for i := 1; i < len(queue); i++ {
			if queue[i] < queue[min] {
				min = i
			}
		}
		id := queue[min]
		queue = append(queue[:min], queue[min+1:]...)
		order = append(order, id)
		for _, s := range w.Steps {
			for _, d := range s.DependsOn {
				if d == id {
					indeg[s.ID]--
					if indeg[s.ID] == 0 {
						queue = append(queue, s.ID)
					}
				}
			}
		}
	}
	if len(order) != len(w.Steps) {
		return nil, errors.New("workflow: cycle detected")
	}
	return order, nil
}
