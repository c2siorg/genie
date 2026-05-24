# pkg/observability/bq — Warehouse sink

> **Where:** `pkg/observability/bq/bq.go` · **Lines of code:** ~140 · **Tests:** 5
> **Inspired by:** Google ADK `agent-observability-bq`

---

## Overview

A pluggable "warehouse sink" for Genie observability — a JSONL-shaped
stream of agent events that a host can dual-write into BigQuery,
Snowflake, or any warehouse for long-horizon agent performance
analytics.

Genie already ships OTel traces and metrics. Tempo / Prometheus are
great for "what happened in the last 30 min." But for "what was the p99
latency of `recommender` over the last 90 days, segmented by user
roles?" you need a warehouse.

We **do not** pull in the `google-cloud-go` BigQuery client here —
that would balloon the dependency graph. Instead the package emits a
typed `Event` to a pluggable `Sink` interface; the host wires the
actual BigQuery client (or a Snowflake stage, or a Kafka topic) to
that Sink.

The shape is intentionally narrow: one row per `agent.handle`
invocation, plus optional roll-up rows per LLM call and per
governance evaluation. Enough to power dashboards on:

- p95 latency by agent
- cost-per-question (sum of LLM tokens × cost)
- governance-denial rate by policy name
- agent error rates

---

## Surface

```go
type Kind string
const (
    KindAgentHandle Kind = "agent.handle"
    KindLLMCall     Kind = "llm.call"
    KindGovernance  Kind = "governance.evaluate"
)

type Event struct {
    Kind             Kind
    OccurredAt       time.Time
    ServiceName      string
    TraceID          string
    SpanID           string
    AgentID          string
    MessageType      string
    Classification   string
    DurationMs       int64
    Success          bool
    Error            string
    LLMProvider      string
    LLMModel         string
    PromptTokens     int
    CompletionTokens int
    CostMicros       int64
    PolicyName       string
    PolicyDecision   string
}

type Sink interface {
    Append(ctx context.Context, events []Event) error
}

type JSONLSink struct { ... }    // stdlib-only reference impl
func NewJSONLSink(w io.Writer) *JSONLSink

type Buffer struct { ... }       // batching wrapper
func NewBuffer(s Sink, max int) *Buffer
func (b *Buffer) Record(ctx, Event) error
func (b *Buffer) Flush(ctx) error
```

---

## Schema design

The `Event` struct is **flat** so it maps 1:1 to a BigQuery / Snowflake
table column-per-field. No nested objects, no arrays — flat is friendly
to SQL.

Suggested DDL for BigQuery:

```sql
CREATE TABLE `project.dataset.genie_events` (
  kind STRING,
  occurred_at TIMESTAMP,
  service_name STRING,
  trace_id STRING,
  span_id STRING,
  agent_id STRING,
  message_type STRING,
  classification STRING,
  duration_ms INT64,
  success BOOL,
  error STRING,
  llm_provider STRING,
  llm_model STRING,
  prompt_tokens INT64,
  completion_tokens INT64,
  cost_micros INT64,
  policy_name STRING,
  policy_decision STRING
)
PARTITION BY DATE(occurred_at)
CLUSTER BY agent_id, kind;
```

Partition on day, cluster on (agent_id, kind) so dashboards
filter cheaply.

---

## Recommended wiring

```go
// At process startup.
file, _ := os.Create("agent-events.jsonl")
sink := bq.NewJSONLSink(file)
buffer := bq.NewBuffer(sink, 100) // flush every 100 events

// In your orchestrator's agent.handle wrapper:
start := time.Now()
out, err := agent.HandleMessage(ctx, msg, env)
_ = buffer.Record(ctx, bq.Event{
    Kind:           bq.KindAgentHandle,
    AgentID:        agent.ID(),
    MessageType:    msg.Type,
    Classification: getMetadata(msg, "classification"),
    DurationMs:     time.Since(start).Milliseconds(),
    Success:        err == nil,
})

// At shutdown.
_ = buffer.Flush(ctx)
```

A cron loads the JSONL into BigQuery hourly via a `bq load` job, or you
implement a `Sink` that streams via the BigQuery streaming API.

---

## What it does NOT do

- **No direct BigQuery write.** Host concern; keep the dependency out of Genie.
- **No retry / backoff.** Sinks should implement their own retry policy.
- **No PII redaction.** If `MessageType` or `Error` text might carry PII, scrub it before calling `Record`.

---

## FREE-AI alignment

- **Rec 24 (Audit Framework)** — the warehouse is the long-horizon audit substrate; OTel is for ops-time, the warehouse is for quarterly review.
- **Rec 23 (AI Inventory)** — the inventory + warehouse together let auditors compute "what % of `recommender` decisions in Q1 were medium-grade or higher?" with a SQL query.

---

## Anti-patterns

1. **Logging the full message payload** in `Error`. The payload may carry PII. Log the *type* and *length* only; refer to OTel for the full trace.
2. **Skipping the buffer.** Writing one event at a time to BigQuery streaming is expensive. Batch.
3. **Treating absence of an Event row as "didn't happen."** The buffer can drop on shutdown if `Flush` isn't called. Use the OTel trace as the source of truth; the warehouse is a *projection*.

---

## Testing

`pkg/observability/bq/bq_test.go` covers:

- JSONLSink writes one line per event
- JSONLSink auto-sets `OccurredAt` when zero
- Buffer auto-flushes at `MaxBatch`
- Buffer manual flush
- Empty flush is a no-op

Run:

```bash
go test ./pkg/observability/bq/ -v
```

---

## References

- [BigQuery streaming inserts](https://cloud.google.com/bigquery/docs/streaming-data-into-bigquery)
- [Snowflake Snowpipe](https://docs.snowflake.com/en/user-guide/data-load-snowpipe-intro)
- [OpenInference semantic conventions](https://github.com/Arize-ai/openinference) — for the OTel-side LLM attribute names
