// Package bq is a pluggable "warehouse sink" for Genie observability —
// a JSONL-shaped stream of agent events that a host can dual-write into
// BigQuery, Snowflake, or any warehouse for long-horizon agent
// performance analytics.
//
// Inspired by Google ADK samples → agent-observability-bq. We don't
// pull in the google-cloud-go BigQuery client here — that would balloon
// the dependency graph. Instead this package emits a typed `Event` to a
// pluggable Sink interface; the host wires the actual BigQuery client
// (or a Snowflake stage, or a Kafka topic) to that Sink.
//
// The shape is intentionally narrow: one row per agent.handle invocation,
// plus optional roll-up rows per LLM call. Enough to power dashboards on
// p95 latency by agent, cost-per-question, and rolling success rates.
package bq

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Kind labels the warehouse row variant.
type Kind string

const (
	KindAgentHandle Kind = "agent.handle"
	KindLLMCall     Kind = "llm.call"
	KindGovernance  Kind = "governance.evaluate"
)

// Event is one warehouse row. Designed to flatten into a BigQuery
// schema with the same field names.
type Event struct {
	Kind          Kind      `json:"kind"`
	OccurredAt    time.Time `json:"occurred_at"`
	ServiceName   string    `json:"service_name"`
	TraceID       string    `json:"trace_id,omitempty"`
	SpanID        string    `json:"span_id,omitempty"`
	AgentID       string    `json:"agent_id,omitempty"`
	MessageType   string    `json:"message_type,omitempty"`
	Classification string   `json:"classification,omitempty"`
	DurationMs    int64     `json:"duration_ms"`
	Success       bool      `json:"success"`
	Error         string    `json:"error,omitempty"`
	LLMProvider   string    `json:"llm_provider,omitempty"`
	LLMModel      string    `json:"llm_model,omitempty"`
	PromptTokens  int       `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`
	CostMicros    int64     `json:"cost_micros,omitempty"`
	PolicyName    string    `json:"policy_name,omitempty"`
	PolicyDecision string   `json:"policy_decision,omitempty"`
}

// Sink writes batches of events to the warehouse. Implementations:
//   - JSONLSink: stdlib-only, writes one JSON object per line.
//   - HostBQSink: provided by the host application; wraps the real BigQuery
//     client (or Snowflake stage, or whatever).
type Sink interface {
	// Append accepts a batch of events. Implementations should be
	// non-blocking (queue + async flush) for production.
	Append(ctx context.Context, events []Event) error
}

// JSONLSink writes one Event per line to an io.Writer. Useful for local
// development, for dumping to GCS for downstream BQ load jobs, or as a
// fallback when the warehouse connector is unavailable.
type JSONLSink struct {
	mu sync.Mutex
	W  io.Writer
}

// NewJSONLSink constructs a sink against the given writer.
func NewJSONLSink(w io.Writer) *JSONLSink { return &JSONLSink{W: w} }

// Append serialises events as one JSON object per line.
func (s *JSONLSink) Append(_ context.Context, events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range events {
		if e.OccurredAt.IsZero() {
			e.OccurredAt = time.Now().UTC()
		}
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := s.W.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// Buffer is a goroutine-safe accumulator that flushes to an underlying
// Sink either when MaxBatch is reached or when Flush is called explicitly.
// Recommended at the edge so the application thread isn't blocked on
// warehouse writes.
type Buffer struct {
	mu       sync.Mutex
	queue    []Event
	MaxBatch int
	Sink     Sink
}

// NewBuffer constructs a buffer with the given batch size (defaults to 100).
func NewBuffer(s Sink, max int) *Buffer {
	if max <= 0 {
		max = 100
	}
	return &Buffer{Sink: s, MaxBatch: max}
}

// Record appends one event to the buffer. Flushes automatically if the
// buffer hits MaxBatch.
func (b *Buffer) Record(ctx context.Context, e Event) error {
	b.mu.Lock()
	b.queue = append(b.queue, e)
	if len(b.queue) < b.MaxBatch {
		b.mu.Unlock()
		return nil
	}
	batch := b.queue
	b.queue = nil
	b.mu.Unlock()
	return b.Sink.Append(ctx, batch)
}

// Flush forces any queued events to the sink.
func (b *Buffer) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.queue) == 0 {
		b.mu.Unlock()
		return nil
	}
	batch := b.queue
	b.queue = nil
	b.mu.Unlock()
	return b.Sink.Append(ctx, batch)
}
