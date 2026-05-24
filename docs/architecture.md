# Genie Architecture — Deep Dive

> Companion to the architecture section of the root README. Goes one
> level deeper into the protocol, the bus, the orchestrator, and the
> seven load-bearing pieces that make multi-agent architecture work in
> production.

---

## The mental model

A user types a question. Some milliseconds later they get an answer.
What happens in between, in Genie's world:

```
User
 │
 │ HTTP POST /v1/ask + JWT
 ▼
chi router → middleware (auth, RBAC, OTel, rate-limit, recovery)
 │
 │ msg = NewMessage(role=user, type="finance_question", ...)
 ▼
Orchestrator.dispatch(msg)
 │ governance.Composite.Evaluate(msg)   ← THE BIG GATE
 │   – length, metadata, RBAC, classification ceiling, residency,
 │     consent, explainability, PII regex, prompt-injection, schema
 │
 │ if any deny → record incident, drop msg, return error
 │ else → bus.Publish(ctx, msg)
 ▼
in-memory pub/sub bus
 │ subscribers: supervisor (then ingestor, normalizer, ... in turn)
 ▼
Agent.HandleMessage(ctx, msg, env)
 │ – pure deterministic logic where possible
 │ – LLM call when needed (wrapped in cost/cache/circuit/budget)
 │ – produces 0..N follow-up messages
 ▼
Each new message → back to Orchestrator.dispatch (loop)
 │
 ▼
… eventually `reporter` emits the final report
 │
 ▼
busio.Correlator wakes the original HTTP handler
 │
 ▼
HTTP response → user
```

Every arrow is observable. The W3C `traceparent` header rides in
`msg.Metadata`; every agent re-extracts it; the OTel collector sees one
distributed trace per user question that spans HTTP → governance → bus
→ every agent → every LLM call.

---

## The seven pieces (deep)

### 1. Protocol (`pkg/protocol`)

```go
type Message struct {
    ID        string
    From      string
    To        string
    Role      MessageRole // user|system|agent|tool|observer|evaluator
    Type      string
    Content   string
    CreatedAt time.Time
    Metadata  map[string]any
}
```

**What lives in `Metadata`:**

- `trace_id` / `traceparent` — W3C distributed trace context
- `user_id` / `account_id` — authn identity
- `user_roles` — authz identity (consumed by `governance.RBACPolicy`)
- `classification` — `public` | `internal` | `pii` | `secret`
- `region` — `in` | `us` | `on-prem`
- `correlation_id` — for fan-out fan-in via `busio.Correlator`
- domain-specific extras (csv body, document_id, ...)

The `Type` field carries 90 % of the routing semantics. Examples in
play across the platform: `finance_question`, `kyc_request`,
`portfolio_request`, `ingest_csv`, `raw_transactions`,
`normalized_transactions`, `enriched_transactions`, `analysis_result`,
`forecast_result`, `anomalies`, `recommendations`, `final_report`,
`mpc_signal`, `claim_request`, `payment_request`, ...

### 2. Registry (`pkg/registry`)

```go
type Registry interface {
    Register(ctx, agent.Agent) error
    Get(ctx, id string) (agent.Agent, bool)
    List(ctx) []agent.Agent
}
```

Reference impl: `registry.NewInMemory()`. The orchestrator subscribes
agents to message types based on what they declare. The
`GET /v1/ai-inventory` endpoint reads from this — so the inventory is
always live, never a separate document that goes stale (FREE-AI Rec 23).

### 3. Bus (`pkg/comm`)

```go
type Bus interface {
    Publish(ctx, msg agent.Message) error
    Subscribe(toID string, handler func(ctx, msg agent.Message))
}
```

Reference impl: `comm.NewInMemoryBus()` — a goroutine-per-subscriber
fan-out with OTel spans around `publish` and `dispatch`. Swap for
Kafka / NATS / Redis Streams when you outgrow one process.

A subtle but important behaviour: `Publish` is fire-and-forget.
Synchronous request/response is built on top via
`busio.Correlator` which matches inbound replies to a pending request
by `correlation_id`. This is how `POST /v1/ask` waits for `final_report`
without spinning a goroutine in the handler.

### 4. Orchestrator (`pkg/orchestration`)

```go
type Orchestrator struct {
    Registry registry.Registry
    Bus      comm.Bus
    Policy   governance.Policy
    Env      agent.Environment
}
func (o *Orchestrator) Start(ctx)
```

Responsibilities (in order, every message):

1. **Extract trace context** from `msg.Metadata` → continue the parent trace.
2. **Evaluate policy** — `policy.Evaluate(ctx, msg)`; on deny → record incident, drop, return.
3. **Look up handler** — `registry.Get(ctx, msg.To)` (or fan-out if To = "broadcast").
4. **Enforce risk ceiling** — if handler is `RiskHigh` and `msg.Metadata.user_roles` lacks `advisor`/`admin`, deny.
5. **Start span** `agent.handle <id>`.
6. **Invoke** `agent.HandleMessage(ctx, msg, env)`.
7. **Publish** the agent's output messages back to the bus (loop).
8. **Record** errors as incidents; if `RiskHigh` errors, also fire the fallback.

The orchestrator is ~300 lines of Go. Everything above is in those
lines. Read them before you build something custom.

### 5. Governance as middleware (`pkg/governance`)

Every message passes through a **composite policy** *before* any agent
runs. The composite is "deny on first failure" by default. The shipped
policies:

| Policy | What it does |
|---|---|
| `MaxContentLengthPolicy` | Hard limit on `msg.Content` size |
| `RequiredMetadataPolicy` | Asserts required metadata keys per message type |
| `RBACPolicy` | Maps message type → required role |
| `ClassificationPolicy` | Recipient may not receive a higher-classification message than its ceiling |
| `DataResidencyPolicy` | PII/Secret may not leave home region except to `on-prem` provider |
| `ConsentPolicy` | Checks `ConsentLedger` for active consent on the relevant data category |
| `ExplainabilityPolicy` | Requires `rationale` on output of named agents (e.g. recommender) |
| `PIIBlockPolicy` | Regex for card numbers, full Aadhaar, emails, phones |
| `PromptInjectionPolicy` | Regex for known injection markers ("ignore previous instructions" family) |
| `SchemaPolicy` | JSON-Schema validation on payloads where a schema is registered |

The composite is loaded from `config/ai-policy.example.yaml` — the
board-approved YAML. Risk team edits a file; the system obeys it.

### 6. Per-agent risk class (`pkg/agent/risk.go`)

```go
type RiskClass string
const (
    RiskLow    RiskClass = "low"
    RiskMedium RiskClass = "medium"
    RiskHigh   RiskClass = "high"
)
type RiskAware interface { RiskLevel() RiskClass }
func RiskOf(a Agent) RiskClass
```

The orchestrator enforces ceilings: `RiskHigh` agents (AML monitor,
VaR calculator, KYC orchestrator, payment orchestrator) cannot execute
on a message that lacks `advisor` or `admin` role on
`metadata.user_roles`.

Autonomous reasoning loops (ReAct, Reflexion in `pkg/reasoning`) are
bounded by three wrappers on the LLM provider:

- `DeadlineProvider` — per-call timeout
- `CircuitProvider` — breaker after N consecutive errors
- `BudgetedProvider` — daily per-principal token cap

So a high-risk agent that ReActs into an infinite loop can't burn
through your LLM budget — the wrapper cuts it off.

### 7. Fallback agents (`agents/fallback`)

```go
orchestrator.SetFallback("portfolio_advisor", deterministic.PortfolioFallback{})
```

When the primary times out, circuit-breaks, or panics, the orchestrator
dispatches the fallback. Fallbacks are deterministic — no LLM, no
network. Users get a degraded but truthful answer; on-call gets paged;
the system stays up.

`make bcp-drill` forces a `portfolio_advisor` failure and verifies the
fallback fires. Business continuity is a CI test (FREE-AI Rec 21).

---

## How fan-out / fan-in works (canonical pipeline)

The `analyzer` agent does parallel fan-out:

```go
func (a *Analyzer) HandleMessage(ctx, msg, env) ([]Message, error) {
    analysis := compute(msg)
    out := []Message{
        newMsg(a.ID, "forecaster",       "analysis_result", analysis),
        newMsg(a.ID, "anomaly_detector", "analysis_result", analysis),
        newMsg(a.ID, "recommender",      "analysis_result", analysis),
        newMsg(a.ID, "supervisor",       "analysis_snapshot", analysis),
    }
    return out, nil
}
```

The bus delivers all four in parallel. Each downstream agent emits its
result to `supervisor`. The supervisor counts 4 fan-ins by
`correlation_id`. When all 4 arrive, it dispatches the final
`reporter`. Total latency = max(stages), not sum.

`busio.Correlator` does the same job for sync HTTP requests — the
`/v1/ask` handler waits on a channel; the bus delivers the final
report; the channel wakes the handler.

---

## Why "messages, not function calls" matters

If two agents call each other directly via Go method calls:

- The compliance team's question "what does agent X do with PII?" has
  no answer except "depends on every caller."
- Adding governance means editing every caller — easy to miss one.
- Tracing means manually weaving context through every call site.
- Failure modes are all-or-nothing — no fallback seam.

Messages on a bus solve all four:

- One audit surface for governance.
- One observability surface for tracing.
- One swap-out seam for fallbacks.
- One inventory for capabilities (FREE-AI Rec 23).

The decoupling is the architecture.

---

## What this buys you concretely

| Property | Mechanism |
|---|---|
| Parallelism | Fan-out via bus → max(stages) latency, not sum |
| Surgical caching | Per-agent cache policy via the LLM wrapper chain |
| Differential observability | Per-agent OTel attributes; warehouse via `pkg/observability/bq` |
| Composable safety | Drop a policy into the composite; applies everywhere |
| Surgical upgrades | Swap one agent's LLM; the other 39 unaffected |
| Honest failure stories | Fallback agents return a degraded but truthful answer |
| Live inventory | `GET /v1/ai-inventory` reads from registry; never goes stale |

---

## Where to go next

- [agents/README.md](agents/README.md) — the agent contract and how to add one
- [packages/README.md](packages/README.md) — the seven platform packages
- [api.md](api.md) — every HTTP endpoint with sample curl
- [operations.md](operations.md) — running the stack
- [free-ai-mapping.md](free-ai-mapping.md) — every Rec → file path
