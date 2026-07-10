# Genie — Architecture Reference

> **Status:** living document. Every structural claim below is anchored to a
> `file:line` in this repository and is meant to be verified, not trusted. If
> the code and this document ever disagree, **the code is authoritative** —
> open an issue or fix the doc. A `grep`-based verification harness is in
> [Appendix A](#appendix-a--how-to-verify-this-document).
>
> **Scope:** the Go service (`github.com/PratikDhanave/multi-agent-reference-architecture-go`).
> The sibling `geniepython/` and `microsoftagentframeworklearning/` trees are
> out of scope.
>
> **Audience:** engineers extending the platform, reviewers auditing it, and
> risk/compliance readers mapping it to the RBI FREE-AI report.

---

## Table of contents

1. [Context and constraints](#1-context-and-constraints)
2. [Design goals and non-goals](#2-design-goals-and-non-goals)
3. [The load-bearing decision](#3-the-load-bearing-decision-a-request-is-a-message)
4. [System context (C4 L1)](#4-system-context-c4-l1)
5. [Component decomposition (C4 L2)](#5-component-decomposition-c4-l2)
6. [The data model: Message, and the three lattices](#6-the-data-model-message-and-the-three-lattices)
7. [Request lifecycle: one question, end to end](#7-request-lifecycle-one-question-end-to-end)
8. [The canonical pipeline: fan-out and fan-in, for real](#8-the-canonical-pipeline-fan-out-and-fan-in-for-real)
9. [Concurrency and delivery semantics](#9-concurrency-and-delivery-semantics)
10. [The governance plane](#10-the-governance-plane)
11. [The autonomy-bounding stack (LLM wrappers)](#11-the-autonomy-bounding-stack-llm-wrappers)
12. [Failure model and business continuity](#12-failure-model-and-business-continuity)
13. [Security architecture](#13-security-architecture)
14. [Observability](#14-observability)
15. [Scaling and evolution: the seams](#15-scaling-and-evolution-the-seams)
16. [System invariants → enforcing code](#16-system-invariants--enforcing-code)
17. [Alternatives considered and rejected](#17-alternatives-considered-and-rejected)
18. [Known limitations](#18-known-limitations)
19. [Appendix A — how to verify this document](#appendix-a--how-to-verify-this-document)
20. [Appendix B — package map](#appendix-b--package-map)

---

## 1. Context and constraints

Genie is an AI financial assistant built as a **reference implementation** of
Microsoft's Multi-Agent Reference Architecture (MARA), aligned to the RBI
FREE-AI report (Aug 2025). It answers finance questions over a user's own
transaction data using a fleet of specialist agents, and it is engineered for a
context where the following constraints are *hard*, not aspirational:

| Constraint | Where it comes from | How it shapes the architecture |
|---|---|---|
| **Every autonomous decision must be auditable** | Regulated finance; FREE-AI accountability | One trace per request; governance as a single gate; hash-chained audit log |
| **Data residency / sovereignty** | RBI localisation | Region tag on every message; residency policy denies PII/secret egress |
| **On-prem / no mandatory external LLM** | Data-control requirement | Default provider is `mock`; Ollama is the on-prem default; cloud providers are opt-in code paths only |
| **Bounded autonomy** | Cost + safety | LLM calls wrapped in circuit/deadline/budget; reasoning loops iteration-capped |
| **Explainability on advice** | FREE-AI Rec on explainability | `ExplainabilityPolicy` requires a rationale field on named outputs |
| **Business continuity** | FREE-AI Rec 21 | Deterministic fallback agents; a CI drill (`make bcp-drill`) proves fallback fires |

The rest of this document shows the mechanisms, names the files that implement
them, and is explicit about where a mechanism is a **reference implementation**
(in-memory, single-process) versus a **production seam** (an interface you swap
for Kafka, Postgres, a KMS, a cloud LLM).

### A note on numbers (verified, not asserted)

- `agents/` contains **62 package directories** (`ls -d agents/*/ | wc -l`).
- `cmd/api/main.go` makes **60 `register(...)` calls** — **58 specialist agents**
  (via each package's `New()`) **+ 2 deterministic fallbacks** (via
  `fallback.NewFor`, at `cmd/api/main.go:208-209`).
- **3 specialist packages are implemented but not wired**: `h_supervisor`,
  `moa_recommender`, `receipt_ocr` (they are never imported in `main.go`).
- So: **61 specialist packages + 1 fallback package = 62**; **58 specialists
  are live**. These are the honest counts; re-derive them any time with the
  harness in Appendix A.

---

## 2. Design goals and non-goals

**Goals (in priority order):**

1. **Auditability over cleverness.** Every message crossing an agent boundary
   is observable, governable, and attributable to a trace. If a feature can't be
   audited, it doesn't ship in the hot path.
2. **Governance as data, not code.** Cross-cutting rules live in
   `config/ai-policy.example.yaml` and are assembled by `pkg/policy`; the risk
   team edits YAML, the system obeys. Rules are *not* scattered into agents.
3. **Bounded autonomy by construction.** An agent that reasons in a loop cannot
   run away — the LLM wrapper chain caps time, tokens, and consecutive errors.
4. **Degrade honestly.** When a primary agent fails, users get a truthful "a
   human will follow up" notice, not a fabricated answer.
5. **Swappable everything.** Bus, registry, LLM provider, key resolver, stores
   are interfaces with in-memory reference impls and production seams.

**Non-goals (explicitly out of scope for this codebase today):**

- **Not a distributed system yet.** The bus is in-process (`pkg/comm`). Multi-node
  requires swapping it (see §15). We do not claim horizontal scale today.
- **Not a guaranteed-delivery message system.** The reference bus is
  fire-and-forget with no persistence, retries, ordering, or backpressure
  (`pkg/comm/bus.go:52-54`, by design and by comment).
- **Not a PII *redaction* system.** PII handling is deny-at-the-gate plus
  optional tokenisation; there is no scrubbing of PII out of otherwise-allowed
  content.
- **Not a finished production security posture.** It ships real primitives
  (AES-256-GCM envelopes, HS256 JWT, RBAC) but with reference key management
  (env-var KEK) that you replace with a KMS in production.

Stating the non-goals is deliberate: it removes the "but it doesn't do X"
critique for every X we never set out to do.

---

## 3. The load-bearing decision: a request is a message

The single decision the whole system rests on:

> **Agents never call each other. A request becomes a `protocol.Message`
> published to a bus; agents subscribe and emit follow-up messages.**

Every agent's contract is `HandleMessage(ctx, msg, env) ([]Message, error)`
(`pkg/agent/types.go:50-59`). There is no agent-to-agent Go method call anywhere
in the hot path. This one indirection buys **four single seams** that a
function-call design cannot have:

| Seam | Because messages go through one point | File |
|---|---|---|
| **Governance** | Every message is evaluated by one composite policy before its handler runs | `pkg/orchestration/orchestrator.go:130-150` |
| **Observability** | Every publish and every handle is one span, linked by W3C traceparent carried in metadata | `pkg/comm/bus.go:108-124`, `orchestrator.go:111-123` |
| **Fallback** | A failed handler is caught at one place and rerouted to a fallback agent | `orchestrator.go:167-184` |
| **Inventory** | The live capability list is read from one registry | `pkg/web/handlers/inventory.go:30-45` |

The cost of this decision — no delivery guarantees, no built-in cycle detection,
emergent (not explicit) control flow — is discussed honestly in §9 and §18. The
decoupling *is* the architecture; the rest is consequence.

---

## 4. System context (C4 L1)

```
                        ┌──────────────────────────────────────────┐
                        │                 Genie API                 │
   End user  ──HTTP────▶│  (cmd/api) chi router + middleware +      │
   (JWT)                │   orchestrator + agent fleet + governance │
                        └───┬───────────────┬───────────────┬───────┘
                            │               │               │
                    ┌───────▼──────┐ ┌──────▼───────┐ ┌─────▼────────┐
                    │  PostgreSQL   │ │  LLM provider │ │  OTel        │
                    │ users/docs/   │ │  Mock (default)│ │  collector   │
                    │ incidents/    │ │  or Ollama     │ │  (OTLP) or   │
                    │ audit/vectors │ │  (on-prem)     │ │  stdout      │
                    └───────────────┘ └────────────────┘ └──────────────┘
```

- **Trust boundary:** the JWT-authenticated HTTP edge. Everything inside runs in
  one process today.
- **State:** PostgreSQL holds users, accounts, documents (encrypted), MCP
  tokens (encrypted), incidents, the audit log, and pgvector embeddings
  (`pkg/storage/postgres/migrations/`).
- **LLM:** default `mock` (deterministic, zero external deps); `GENIE_LLM=ollama`
  points at an on-prem Ollama. Cloud providers (`anthropic.go`, `gemini.go`,
  `openai.go` in `pkg/llm`) exist but are **not** wired into `buildLLMStack`.
- **Telemetry:** OTLP gRPC when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, else a
  stdout exporter (`pkg/observability/otel.go:138-169`).

---

## 5. Component decomposition (C4 L2)

```
   HTTP edge (pkg/web)
   ├─ router.go            chi; global mw: RequestID→Recovery→AccessLog→Trace
   ├─ mid/                 Auth (JWT), RequireRole (RBAC), RateLimit (token bucket)
   └─ handlers/            ask, documents, users, incidents, inventory, aibom, ...
                │
                │  builds protocol.Message, Await(trace_id), Publish
                ▼
   Messaging core
   ├─ pkg/protocol         Message, MessageRole, Classification  (the contract)
   ├─ pkg/agent            Agent + Environment interfaces; RiskClass; type aliases
   ├─ pkg/comm             Bus: in-memory pub/sub, goroutine-per-delivery
   ├─ pkg/busio            Correlator: sync request/reply keyed on trace_id
   └─ pkg/registry         in-memory agent map, keyed by ID
                │
                ▼
   Orchestrator (pkg/orchestration)
   └─ Start(ctx): subscribe one handler per agent → evaluate policy → handle →
                  publish outputs OR route to fallback; emit spans + metrics
                │
        ┌───────┴─────────┬──────────────────┬─────────────────────┐
        ▼                 ▼                  ▼                     ▼
   Governance         Agents            LLM stack            Cross-cutting
   pkg/governance     agents/<id>/      pkg/llm (wrappers)   pkg/crypto  (envelopes)
   pkg/policy (YAML)  58 live + 2 fb    pkg/reasoning        pkg/auth    (HS256 JWT)
                                        pkg/rag / graphrag   pkg/compliance (consent,
                                        pkg/constitution                    audit chain)
                                        pkg/eval / safety    pkg/incidents  (FREE-AI form)
                                                             pkg/observability (OTel)
                                                             pkg/storage/postgres
```

The layering rule: **the messaging core knows nothing about finance**; agents
know nothing about HTTP; governance knows nothing about specific agents (it
reads message metadata). Each arrow is a Go interface, not a concrete
dependency.

---

## 6. The data model: Message, and the three lattices

Everything on the wire is one struct (`pkg/protocol/message.go:71-97`):

```go
type Message struct {
    ID        string         `json:"id"`
    From      string         `json:"from"`
    To        string         `json:"to,omitempty"`
    Role      MessageRole    `json:"role"`
    Type      string         `json:"type"`
    Content   string         `json:"content"`
    CreatedAt time.Time      `json:"created_at"`
    Metadata  map[string]any `json:"metadata,omitempty"`
}
```

Constructed only via `NewMessage(from, to, role, msgType, content, metadata)`
(`message.go:108-119`), which stamps a UUID `ID` and a UTC `CreatedAt`.

**Three things carry the semantics:**

### 6.1 `Type` — the routing/semantic key (a plain string)

`Type` is a **plain `string`**; there are **no `MessageType` constants** in
`pkg/protocol`. Each agent declares its own `TypeIn`/`TypeOut` constants and
guards `if msg.Type != TypeIn { return nil, nil }`. The pipeline is therefore
wired *by string convention* — see the exact chain in §8. This is a deliberate
low-ceremony choice; the tradeoff (no compile-time routing check) is noted in
§18.

### 6.2 `Role` — a 6-value enum (`message.go:18-43`)

`user`, `system`, `agent`, `tool`, `observer`, `evaluator`.

### 6.3 `Metadata` — the typed-payload escape hatch (`map[string]any`)

Untyped by design. Only **three** metadata keys have constants
(`pkg/protocol/classification.go:19-26`): `classification`, `user_roles`,
`user_id`. The `region` key's constant lives in `pkg/sovereignty`
(`sovereignty.go:36`). The keys `trace_id` and `traceparent` are **bare string
literals** used across packages (e.g. `pkg/busio/correlator.go:32`,
`pkg/web/handlers/ask.go`). **`correlation_id` does not exist anywhere in the
codebase** — synchronous correlation is keyed on `trace_id` (see §7, §9). Common
domain keys in play: `csv`, `account_id`, `pii_acknowledged`.

### The classification lattice (`pkg/governance/classification.go:26-31`)

```
public (0)  <  internal (1)  <  pii (2)  <  secret (3)
```

A message's class (default `internal` when unset) must not exceed the
recipient's *ceiling*. In the wired policy the ceiling is a **hardcoded
`internal`** for every recipient — the per-recipient `AllowedTo` override map
exists in the policy struct but is **never populated from YAML** by
`BuildComposite` (`pkg/policy/policy.go:124-129`). This is a real gap between
config surface and enforced behaviour, documented here so it can't be used as a
"gotcha."

### The risk lattice (`pkg/agent/risk.go`)

```
low  <  medium  <  high        // RiskClass; agents without RiskLevel() default to low
```

`RiskOf(a)` (`risk.go:31-36`) type-asserts to `RiskAware`; agents that don't
implement `RiskLevel()` are `low`. **Important honesty point:** `RiskOf` is read
only by *reporting* surfaces — the AIBOM (`pkg/aibom`), the disclosures handler,
and the `/v1/ai-inventory` endpoint. **The orchestrator does not consult risk
class when dispatching.** The de-facto access control on high-risk actions is
`RBACPolicy`, which gates by message **`Type`**, not by risk class (see §10.3).

---

## 7. Request lifecycle: one question, end to end

The path of `POST /v1/ask` (`pkg/web/handlers/ask.go`), verified line by line:

```
Client                HTTP edge (chi)              Bus / Orchestrator / Agents        Correlator
  │  POST /v1/ask + Bearer                                                              │
  ├──────────────▶ RequestID→Recovery→AccessLog→Trace  (global mw, router.go:37-40)
  │                RateLimit(60 burst,1/s) → Auth(JWT) → handler  (per-group mw)
  │                       │
  │                       │ decode {question, document_id}; 400 if empty
  │                       │ Documents.GetByID → 404; owner check → 403
  │                       │ Encryptor.Decrypt(doc.Payload) → plaintext CSV (500 on fail)
  │                       │ traceID = "tr-<unixnano>"           (ask.go:76)
  │                       │ ch := Correlator.Await(traceID) ───────────────────────────▶ register buffered(1) chan
  │                       │ msg = NewMessage("user", "financial_supervisor",
  │                       │        RoleUser, "finance_question", question,
  │                       │        {trace_id, csv, user_id, user_roles, classification})
  │                       │ Bus.Publish(msg)  ── fire-and-forget ──▶
  │                       │                                         Orchestrator handler (per agent):
  │                       │                                          1. ExtractTraceContext(metadata)
  │                       │                                          2. span "agent.handle"
  │                       │                                          3. governance.Composite.Evaluate  ◀── THE GATE
  │                       │                                             deny → OnPolicyDeny hook, incident, return
  │                       │                                          4. agent.HandleMessage(...)
  │                       │                                          5. success → Publish each output (loop)
  │                       │                                             error   → OnAgentError hook + fallback publish
  │                       │                                         ... pipeline runs (see §8) ...
  │                       │                                         reporter emits "final_report" → to "user"
  │                       │                                                                          │
  │                       │ select {                                                                 │
  │                       │   case msg := <-ch:  ◀───────── Correlator subscriber matches trace_id ──┤ non-blocking send
  │                       │        200 {trace_id, report, disclosure_banner}
  │                       │   case <-time.After(60s): Cancel(traceID); 504
  │                       │   case <-ctx.Done():       Cancel(traceID)      (client gone)
  │                       │ }
  │  ◀── 200 / 504 ───────┤
```

Key facts (each greppable):

- **The trace id is minted in the handler** as `fmt.Sprintf("tr-%d", time.Now().UnixNano())` (`ask.go:76`). It is the correlation key for the whole request.
- **`Await` is registered *before* `Publish`** to avoid a race where the reply arrives before the waiter exists.
- **Request timeout is `GENIE_ASK_TIMEOUT`, default 60s**, set in `cmd/api/main.go:350`. (The handler's own zero-value default is 8s, but main always overrides it, so 60s is the real number.)
- **On timeout → HTTP 504 + `Cancel(traceID)`.** A late reply after `Cancel` is silently dropped by the correlator's `select { case ch <- msg: default: }` (`pkg/busio/correlator.go`).

---

## 8. The canonical pipeline: fan-out and fan-in, for real

This is the section the old doc got wrong with an illustrative snippet. Here is
the **actual** code.

### 8.1 The chain, by exact `Type` string

Each agent consumes one input Type and produces one output Type; the wiring is
the string match:

| Agent (`ID`) | Consumes `Type` | Produces `Type` | To |
|---|---|---|---|
| `ingestor` | `ingest_csv` | `raw_transactions` | `normalizer` |
| `normalizer` | `raw_transactions` | `normalized_transactions` | `enricher` |
| `enricher` | `normalized_transactions` | `enriched_transactions` | `analyzer` |
| `analyzer` | `enriched_transactions` | `analysis_result` **(×4)** | forecaster, anomaly_detector, recommender, financial_supervisor |
| `forecaster` | `analysis_result` | `forecast_result` | `financial_supervisor` |
| `anomaly_detector` | `analysis_result` | `anomalies` | `financial_supervisor` |
| `recommender` | `analysis_result` | `recommendations` | `financial_supervisor` |
| `financial_supervisor` | `finance_question`, then the 4 fan-in types | `ingest_csv` (bootstrap) / `final_report_request` (finalise) | `ingestor` / `reporter` |
| `reporter` | `final_report_request` | `final_report` | `user` |

The supervisor **bootstraps** the chain: on a `finance_question` it pulls the CSV
from `msg.Metadata["csv"]` and emits `ingest_csv` to the ingestor
(`agents/supervisor/supervisor.go:84-86`).

### 8.2 Fan-out — the analyzer emits **four** messages

`agents/analyzer/analyzer.go:52-108` (elided for aggregation):

```go
return []agent.Message{
    agent.NewMessage(ID, FanForecaster, agent.RoleAgent, TypeOut, content, msg.Metadata),
    agent.NewMessage(ID, FanAnomaly,    agent.RoleAgent, TypeOut, content, msg.Metadata),
    agent.NewMessage(ID, FanRecommend,  agent.RoleAgent, TypeOut, content, msg.Metadata),
    agent.NewMessage(ID, FanSupervisor, agent.RoleAgent, TypeOut, content, msg.Metadata),
}, nil
```

All four are `TypeOut == "analysis_result"`; the fourth goes to the supervisor so
it can cache the analyzer snapshot for the final report. (The source comment says
"three downstream specialists" — the code emits four; this doc reflects the
code.)

### 8.3 Fan-in — structural, keyed on `trace_id`, not a counter

The supervisor holds a per-trace `session` with four named slots
(`agents/supervisor/supervisor.go:41-52`):

```go
type session struct {
    Question        string
    Analysis        *analyzerSnapshot
    Forecast        json.RawMessage
    Anomalies       json.RawMessage
    Recommendations json.RawMessage
}
func (s *session) isReady() bool {
    return s.Analysis != nil && s.Forecast != nil && s.Anomalies != nil && s.Recommendations != nil
}
```

Each inbound message selects its session by `trace_id` (falling back to `msg.ID`)
and fills exactly one slot. `maybeFinalize` fires **only when all four are
non-nil**, then **deletes the session** so the reporter is triggered exactly once
(`supervisor.go:123-154`). It emits one `final_report_request` to the reporter,
which renders text and emits `final_report` to `user` — which the correlator is
waiting for.

**Why this matters for latency:** the three analysis branches
(forecast/anomaly/recommend) run concurrently as separate bus deliveries, so
end-to-end latency is `max(branch)`, not the sum. **Why it matters for
correctness:** readiness is defined by *presence of named fields*, so a duplicate
or out-of-order delivery cannot miscount — it just re-sets a slot. There is no
numeric fan-in counter to get wrong.

---

## 9. Concurrency and delivery semantics

This is where an honest reference architecture must be precise about what it does
*not* guarantee.

### 9.1 The bus (`pkg/comm/bus.go`)

```go
type Bus interface {
    Subscribe(agentID string, h Handler) (unsubscribe func())
    Publish(ctx context.Context, msg protocol.Message)
}
```

- **Delivery model:** *goroutine-per-matching-handler-per-publish*. Every
  `Publish` spawns `go h(ctx, msg)` for each handler subscribed to `msg.To`,
  plus every broadcast handler (subscribed under the empty ID)
  (`bus.go:137-150`).
- **`Publish` is non-blocking / fire-and-forget.** It holds an `RWMutex.RLock`
  only while ranging subscribers and launching goroutines; it never waits for a
  handler to finish (`bus.go:133-151`).
- **Explicit non-guarantees (stated in the source, `bus.go:52-54`):** no
  backpressure (fast publishers can overwhelm slow handlers), no ordering, no
  panic recovery inside the bus.
- **Unsubscribe** nils the handler slot rather than compacting the slice;
  `Publish` tolerates this with an `if h != nil` guard (`bus.go:72-95`).

### 9.2 Synchronous request/reply (`pkg/busio/correlator.go`)

A `Correlator` subscribes once to a terminal recipient (main wires `"user"` and
`"mcp"`, `cmd/api/main.go:309-311`) and holds `map[trace_id]chan Message`
(buffered depth 1). `Await(traceID)` registers a channel *before* the request is
published; the subscriber matches the reply by `trace_id`, **consumes the waiter
once** (deletes the map entry), and non-blocking-sends. Timeout is owned by the
**caller** (the ask handler's `select`), not the correlator — the correlator has
no timers.

### 9.3 Control-flow termination — emergent, not explicit

There is **no dispatch loop and no re-dispatch counter.** "The loop" is the
emergent effect of agents publishing outputs that the bus delivers to the next
agent. A chain terminates when:

- an agent returns zero output messages (`orchestrator.go:187` ranges an empty
  slice), or
- a message targets a `To` with no subscribers (the bus ranges an empty handler
  slice), e.g. terminal messages to `user`, consumed only by the correlator.

**There is no cycle detection and no recursion/queue-depth guard.** Two agents
that published to each other would loop unboundedly, each hop spawning a fresh
goroutine. The only backstops are the LLM-layer wrappers (§11), which bound
*LLM-backed* work but not deterministic message ping-pong. This is a real
limitation (§18), not an oversight to hide.

### 9.4 Panics are not recovered in the core

Neither the bus nor the orchestrator has a `recover()`. A panicking agent
goroutine is unrecovered. The HTTP `mid.Recovery` middleware only covers the
request goroutine, **not** the bus-spawned handler goroutines. Consequently the
fallback mechanism (§12) fires on a returned `error`, **not** on a panic.

---

## 10. The governance plane

Every message is evaluated by one composite policy **before** its handler runs
(`orchestrator.go:130-150`). This is the audit gate.

### 10.1 The interface (`pkg/governance/policy.go:42-44`)

```go
type Policy interface {
    Evaluate(ctx context.Context, msg protocol.Message) (PolicyResult, error)
}
```

`Evaluate` returns a **`PolicyResult{Decision, Reason, CheckedAt, CheckedByID}`**
plus an `error`. `Decision` is `allow`|`deny`. The `error` is reserved for
*operational* failures (bad config, a ledger that can't be reached) and is kept
distinct from a policy *deny*.

### 10.2 The composite: deny-on-first-failure (`policy.go:69-88`)

`CompositePolicy.Evaluate` iterates policies **in slice order**; the **first
deny short-circuits** and returns its reason; an operational `error` aborts
immediately. The tradeoff (one clear reason, fast; but only the first failure is
reported) is stated in the source at `policy.go:62-68`.

### 10.3 The ten policies and how they're assembled

`pkg/governance` defines **ten** concrete policies. They are assembled from YAML
by `pkg/policy`'s `BuildComposite` (`policy.go:106-148`) in **this order** — which
is also the deny-check order:

| # | Policy | Wired when | Reads | Notes |
|---|---|---|---|---|
| 1 | `MaxContentLengthPolicy` | always | `Content` | default cap 256 KiB (`max_content_length_bytes: 262144`) |
| 2 | `RequiredMetadataPolicy` | per type in `limits.required_metadata` | `Metadata[k]` | e.g. `finance_question` requires `user_id`, `trace_id` |
| 3 | `RBACPolicy` | if `governance.rbac` non-empty | `Type`, `Metadata["user_roles"]` | gates by **message Type**; unknown type ⇒ allow; `admin` bypass |
| 4 | `ClassificationPolicy` | **always** | `Metadata["classification"]`, `To` | ceiling hardcoded `internal`; `AllowedTo` never filled |
| 5 | `DataResidencyPolicy` | **always** | `Metadata["region"]`, class | forces `AllowCrossBorderForPublic=true`, ignoring YAML |
| 6 | `PIIBlockPolicy` | if `data.block_pii` | `Content` | 3 regexes; `pii_acknowledged=="true"` bypass |
| 7 | `PromptInjectionPolicy` | if `data.block_prompt_injection` | `Content` | 7 substring markers |
| 8 | `ConsentPolicy` | if a ledger + `type_to_category` | `Type`, `user_id`, ledger | `HasActive` lookup |
| 9 | `ExplainabilityPolicy` | if `explainability.applies_to` non-empty | `Type`, `Content` | requires non-empty `rationale` field |
| — | `SchemaPolicy` | **never wired** | — | implemented (`schema_policy.go`) but `BuildComposite` never constructs it |

Two honesty flags a reviewer will otherwise catch:

- **`SchemaPolicy` exists but is dead in the wired policy** — the YAML loader
  never builds it. Do not read the config as if JSON-schema validation is active.
- **`ClassificationPolicy.AllowedTo` and the YAML
  `sovereignty.allow_cross_border_for_public` field are both effectively
  ignored** by `BuildComposite` — the code hardcodes the ceiling to `internal`
  and public cross-border to `true`.

### 10.4 The exact rules

- **RBAC** (`rbac.go:28-43`): `required := RequiredRolesByType[msg.Type]`; absent
  or empty ⇒ allow (opt-in). Roles are extracted from `metadata["user_roles"]`
  in three accepted shapes (comma-string, `[]string`, `[]any`). `admin` bypasses
  when `AdminBypass`. Any-of match otherwise. In the shipped YAML, only
  `finance_question` and `portfolio_request` are gated to
  `["user","advisor","admin"]`.
- **Residency** (`sovereignty.go:33-55`): home region, on-prem, and untagged
  messages always pass. On a region mismatch: **`pii`/`secret` are always
  denied** (no override); `internal` denied; `public` denied only if cross-border
  is disabled (it isn't, as wired).
- **PII regexes** (`pii.go:15-22`): `\d{12,}` (card/account/Aadhaar-shaped), an
  email pattern, and `\+?\d{10,12}` (phone). There are **no** PAN/GSTIN-specific
  regexes — the "12+ digits" rule is the closest.
- **Prompt-injection markers** (`prompt_injection.go:19-27`): seven lowercased
  substrings including `"ignore previous instructions"` and
  `"reveal your system prompt"`.

### 10.5 Adversarial verification is a build target

`make red-team` (`cmd/red-team/main.go`) runs an 8-probe adversarial corpus
against the **YAML-built** composite and **exits non-zero on any unexpected
allow** (FREE-AI Rec 20). One honest caveat the source itself notes
(`red-team/main.go:65-66`): the residency probe is tagged `pii`+region-US, but
`ClassificationPolicy` sits *before* `DataResidencyPolicy` and denies it first —
so that probe usually never reaches the residency check. Both denials are
accepted by the test.

---

## 11. The autonomy-bounding stack (LLM wrappers)

An agent that reasons in a loop must not be able to burn unbounded time, tokens,
or a dead backend. The bound is a wrapper chain around the `Provider`, built in
`cmd/api/llmstack.go`.

### 11.1 The interface (`pkg/llm/llm.go:115-123`)

```go
type Provider interface {
    Name() string
    Region() string
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```

The method is **`Complete`** (not `Chat`/`Embed`). **Embeddings are a separate
interface, `rag.Embedder` (`pkg/rag/rag.go:44-49`), and do NOT flow through this
wrapper chain** — they are unbudgeted, uncached, and not circuit-broken.

### 11.2 The real wrapper order

On the **Ollama** path (`cmd/api/llmstack.go:83-89`), outermost → innermost —
i.e. the order a call is processed:

```
Circuit → Deadline → Budget → Cache → Cost → Ollama
```

```go
base      := llm.NewOllamaProvider(url, chatModel)
withCost  := llm.NewCostObserver(base, 0, 0)                 // local model → non-monetary
cached    := llm.NewCachedProvider(withCost, cacheTTL)       // GENIE_LLM_CACHE_TTL, def 600s
budgeted  := llm.NewBudgeted(cached, llm.NewInMemoryBudget(), budget) // GENIE_LLM_BUDGET, def 1e6
deadlined := llm.NewDeadline(budgeted, timeout)              // GENIE_LLM_TIMEOUT, def 30s
stack     := llm.NewCircuit(deadlined, threshold, 30*time.Second) // GENIE_LLM_CIRCUIT, def 5
```

> **Correction of record:** both CLAUDE.md (`Cached→Budgeted→Deadline→Circuit`)
> and the previous architecture.md (`Deadline→Circuit→Budgeted`) stated the order
> wrongly and omitted the `Cost`/OTel observer. The line above is the code.

**The default path is different.** `GENIE_LLM` defaults to `mock`, and the mock
stack is a **bare `*llm.Mock` with no wrappers at all**
(`llmstack.go:56-69`). The autonomy-bounding onion exists **only on the Ollama
path**. Any claim that "every LLM call is circuit-broken" is true only when
`GENIE_LLM=ollama`.

Two second-order consequences of the real nesting:

- **Cache sits inside Budget**, so a cache *hit* still passes through the budget
  wrapper and is counted against the daily token cap (`budget.go:83-104`). The
  cache does not shield the budget.
- **Cost/OTel sits inside Cache**, so cache hits are served before the cost
  observer and therefore emit **no** cost/latency span.

### 11.3 The wrappers (`pkg/llm/`)

| Wrapper | Behaviour | Knob / default |
|---|---|---|
| `CircuitProvider` | Breaker: opens after N consecutive failures, `ErrCircuitOpen` during cooldown, half-open probe after | `GENIE_LLM_CIRCUIT`=5; cooldown 30s (hardcoded) |
| `DeadlineProvider` | `context.WithTimeout` per call | `GENIE_LLM_TIMEOUT`=30s |
| `BudgetedProvider` | Per-principal daily token cap; `ErrBudgetExceeded`. **Principal = `req.Residency.Region`** — a documented stand-in, *not* a user id | `GENIE_LLM_BUDGET`=1e6/day; resets UTC midnight |
| `CachedProvider` | Exact-match cache; key = SHA-256 of canonical JSON(model+messages+temperature) | `GENIE_LLM_CACHE_TTL`=600s |
| `CostObserver` | OTel `llm.complete` span + token/cost/latency metrics; prices hardcoded 0 (local model) | — |

### 11.4 Reasoning loops are iteration-capped (`pkg/reasoning`)

- **ReAct** (`reasoning.go:79-135`): a `for i := 0; i < maxSteps` loop,
  `maxSteps` defaulting to **5**, each iteration one `Complete` call with
  `MaxTokens:256`. Returns an error on "max steps reached." The runaway cutoff is
  layered: the loop caps iterations; `Deadline` kills a hung call; `Circuit`
  trips after N errors (so subsequent `Complete` calls return immediately and
  ReAct exits); `Budget` returns `ErrBudgetExceeded` when the day's tokens are
  spent.
- **Reflexion** (`reflexion.go:25-79`): **not a loop** — exactly three fixed
  calls (initial → critique → refine). Bounded by construction.

### 11.5 LLM-as-judge (`pkg/constitution`)

`config/constitution.yaml` encodes the FREE-AI "7 Sutras." `Critique`
(`constitution.go:83-98`) sends the sutras + a candidate answer at
`Temperature:0` and parses a `SCORE: <0..10>` verdict (unparseable ⇒ 0). The same
sutra text feeds both the agent system prompt and the judge. (`pkg/eval` is a
record store, not a judge; `pkg/safety` is heuristic+LLM jailbreak/toxicity
detectors.)

---

## 12. Failure model and business continuity

### 12.1 Failure taxonomy — what happens in each case

| Failure | Detected at | System response |
|---|---|---|
| **Policy deny** | `orchestrator.go:138-149` | increment `governance.denials`, fire `OnPolicyDeny` hook → **incident** (`FailurePolicyDenied`, severity *low*), drop message |
| **Policy op-error** | `orchestrator.go:132-137` | record span error, log, return (no incident hook) |
| **Agent returns error** | `orchestrator.go:167-184` | increment `agent.errors`, fire `OnAgentError` hook → **incident** (`FailureAgentError`, severity *moderate*), then **route to fallback** if one is registered |
| **LLM timeout / circuit-open / budget** | `pkg/llm` wrappers | surfaces as a `HandleMessage` error ⇒ same path as "agent returns error" ⇒ fallback |
| **Agent panic** | *not recovered* | crashes the handler goroutine; **does not** trigger fallback (§9.4) |
| **Request timeout** | `ask.go` `select` | `Cancel(trace_id)` + HTTP 504 |

### 12.2 The fallback mechanism — precisely

Two pieces cooperate:

1. **A fallback agent registered like any other** via `fallback.NewFor(primary)`
   (`agents/fallback/fallback.go`), whose ID is `"<primary>_fallback"` and which
   responds to *any* message with a deterministic notice:

   > "The `<primary>` agent could not complete your request. A human reviewer
   > will follow up. Reference id: `<msg.ID>`."

   No LLM, no network. Emits Type `fallback_notice` to `user`.

2. **A routing map** on the orchestrator, set with
   `SetFallback(originalID, fallbackID string)` — **two strings**
   (`orchestrator.go:55-65`). On an agent error, the orchestrator publishes a
   `fallback_request` message to the mapped fallback ID (`orchestrator.go:178-181`).

> **Correction of record:** the old doc showed
> `orchestrator.SetFallback("portfolio_advisor", deterministic.PortfolioFallback{})`.
> The method exists but that call is fictional — the signature takes **two
> strings**, and there is no `deterministic` package or `PortfolioFallback` type.
> Real wiring: `cmd/api/main.go:208-209` + `:285-286`.

**Honest scope of the fallback:** only `portfolio_advisor` and `recommender`
have fallbacks wired; the notice is a human-review message, **not** a
recomputed answer; and it fires on a returned error, **not** on a panic or a raw
timeout that never becomes an error.

### 12.3 Business continuity is a CI target

`make bcp-drill` forces a `portfolio_advisor` failure and asserts the fallback
fires (FREE-AI Rec 21). Continuity is a test, not a promise.

---

## 13. Security architecture

### 13.1 Authentication — hand-rolled HS256 JWT (`pkg/auth`)

No third-party JWT library. `Issue` builds `Claims{Subject, Email, Roles, iat,
exp, iss, aud}` and signs with `hmac.New(sha256.New, secret)` (`jwt.go:129-133`).
`Verify` (`jwt.go:59-102`) **rejects any `alg != "HS256"`**, does a constant-time
`hmac.Equal`, and checks `exp`/`iss`/`aud`. Secret from `GENIE_JWT_SECRET`; TTL
60 min; issuer/audience `"genie-api"` (`cmd/api/main.go:157-158`). Roles:
`user`, `advisor`, `admin`.

### 13.2 Authorization — two layers

- **Route-level RBAC** (`mid.RequireRole`, `pkg/web/mid/auth.go:63-80`):
  OR-semantics (any one listed role passes); must run *after* `mid.Auth`. Admin
  routes: `/v1/incidents` (GET), `/v1/ai-inventory`, `/v1/aibom`,
  `/debug/pprof/*`.
- **Message-level RBAC** (`RBACPolicy`, §10.4): gates by message `Type` using
  `metadata["user_roles"]`, which the ask handler copies from the JWT claims
  (`ask.go`).

The roles→metadata→policy path: `mid.Auth` verifies the token and stores claims →
the ask handler stringifies `claims.Roles` into `metadata["user_roles"]` →
`RBACPolicy.Evaluate` reads them. That chain is the *actual* access control on
sensitive message types.

### 13.3 Encryption at rest — AES-256-GCM envelopes (`pkg/crypto`)

```go
type EncryptedPayload struct {
    KEKID, WrappedDEK, Nonce, Ciphertext, Algorithm string  // all base64, alg = "AES-256-GCM"
}
```

`Encrypt` (`envelope.go:68-100`) generates a fresh 32-byte **DEK** per payload,
seals the plaintext with AES-GCM, then wraps the DEK with the **KEK**. `Decrypt`
(`:102-135`) **rejects any algorithm other than `AES-256-GCM`**. The reference
`EnvKeyResolver` loads the KEK from `GENIE_KEK_BASE64`, requiring it to
base64-decode to **exactly 32 bytes** (default KEK id `local-env-v1`). A
`KMSKeyResolver` + `KMSClient` interface is the production seam
(`resolver.go:89-114`) — Wrap/Unwrap delegate to a KMS; unimplemented by default.

**What is actually encrypted:** only `documents.payload` and `mcp_tokens.payload`
(both JSONB envelopes). `users`, `accounts`, `incidents`, `audit_log`, and
`rag_embeddings` are plaintext at rest. Crucially, **encryption happens in the
HTTP handlers, not the Postgres repos** — `handlers/documents.go` encrypts before
insert, `handlers/ask.go` decrypts after select; the repos only
marshal/unmarshal the envelope. Plaintext exists only in memory during those
handler calls.

### 13.4 Consent and audit (`pkg/compliance`)

- **Consent ledger:** `Grant/Revoke/HasActive` over four categories
  (`transactions`, `portfolio`, `recommendations`, `third_party_share`).
  `ConsentPolicy` denies a mapped message type when no active consent exists.
- **Audit log:** a **SHA-256 hash chain** — each `AuditEntry.RowHash =
  sha256(prev || canonical-json(row))`, and `Verify` walks from `"genesis"` and
  fails on any mismatch (`audit.go:77-105`). Tamper-evidence without encryption.

### 13.5 Privacy primitives (`pkg/privacy`)

An HMAC-SHA256 `Tokeniser` (`tok_<hex>`) for deterministic pseudonymisation and
Laplace/Gaussian differential-privacy helpers. **There is no PII redaction of
message content** — PII control is deny-at-the-gate (§10.4) plus optional
tokenisation a caller must invoke.

---

## 14. Observability

**One trace per request.** The W3C `traceparent` rides in `msg.Metadata`;
`bus.Publish` injects it (`InjectTraceContext`, `bus.go:124`) and the
orchestrator extracts it (`ExtractTraceContext`, `orchestrator.go:111`) to
re-parent the handler span across the goroutine boundary. The propagator is the
composite `TraceContext{} + Baggage{}` (`otel.go:129-132`).

**Span tree per question:**

```
http POST /v1/ask                     (SpanKindServer, pkg/web)
└─ bus.publish                        (SpanKindProducer, pkg/comm)
   └─ agent.handle <supervisor>       (SpanKindConsumer, pkg/orchestration)
      └─ governance.evaluate
   └─ agent.handle <ingestor> … <analyzer> …   (siblings, re-parented via traceparent)
      └─ llm.complete                 (CostObserver, on the Ollama path)
```

Note a subtlety a reviewer might flag: `bus.publish`'s span ends
(`defer span.End()`) *before* the handler goroutines finish; child spans are
linked through the extracted traceparent in metadata, not through a live parent
span. That's intentional for a fire-and-forget bus.

**Metrics** (`pkg/observability/metrics.go`): `bus.messages_published`,
`agent.messages_handled`, `governance.denials`, `agent.errors`,
`agent.handle_duration_ms` (histogram). **Exporters:** OTLP gRPC when
`OTEL_EXPORTER_OTLP_ENDPOINT` is set, else stdout.

**Live inventory, never stale:** `GET /v1/ai-inventory` reads the registry at
request time (`handlers/inventory.go`), reporting each agent's risk class via
`RiskOf`. The AIBOM endpoint (`/v1/aibom`) and disclosures (`/v1/disclosures`)
are the other read surfaces for the same registry data.

**Two honesty flags:** (1) `pkg/observability/bq` (a BigQuery/warehouse `Sink`
with an `Event` row schema and a `JSONLSink` reference impl) is **implemented but
not wired** — there is no call site, so there is no live warehouse export path.
(2) All OTel tracer/meter scope names are hard-coded `github.com/c2siorg/genie/…`,
stale relative to the actual module path `github.com/PratikDhanave/…`.

---

## 15. Scaling and evolution: the seams

The system is single-process today. Every scaling axis is an interface swap, not
a rewrite:

| Axis | Today (reference impl) | Production seam | What breaks / must change |
|---|---|---|---|
| **Messaging** | `comm.InMemoryBus` (fire-and-forget, in-proc) | `Bus` interface → Kafka / NATS / Redis Streams | Cross-process trace propagation already works (traceparent in metadata); you gain persistence/ordering/backpressure the in-mem bus lacks |
| **Sync reply** | `busio.Correlator` (in-proc map by trace_id) | Reply-to topic + correlation on trace_id | Correlator becomes a per-node consumer of a reply topic |
| **Registry** | `registry.InMemoryRegistry` (map) | `Registry` interface → shared store / service discovery | Orchestrator snapshots `List()` at `Start`; a dynamic registry needs re-subscription support |
| **LLM** | Mock / Ollama, wrapped | `Provider` interface → cloud providers already coded (`anthropic.go`, `gemini.go`, `openai.go`) but unwired; `router.go`/`shadow.go` exist for routing/shadowing | Wire them into `buildLLMStack`; mind residency (`Region()`) |
| **Budget** | `InMemoryBudget` (per-process, resets UTC midnight) | Shared ledger (Redis) keyed on a real principal | Fix the principal: today it's `req.Residency.Region`, a stand-in, not a user id |
| **Key mgmt** | `EnvKeyResolver` (32-byte env KEK) | `KMSKeyResolver` + `KMSClient` (interface present) | Implement the KMS client; rotate KEK ids |
| **Warehouse** | none wired | `observability/bq.Sink` (interface + JSONL ref impl present) | Provide a BigQuery/Snowflake `Sink` and a call site |

**Back-of-envelope for the single process:** because the bus is
goroutine-per-delivery with no pool, throughput is bounded by goroutine
scheduling and per-agent CPU, and there is **no backpressure** — a load spike
creates unbounded goroutines. Treat the current process as a
*correctness/reference* target, and move to a real broker before load-testing for
scale.

---

## 16. System invariants → enforcing code

The table a reviewer should attack — each row is a property and the exact code
that enforces it. If the code doesn't back the claim, the claim is a bug.

| # | Invariant | Enforced by |
|---|---|---|
| I1 | No message reaches an agent without passing the composite policy | `orchestrator.go:130-150` (Evaluate before HandleMessage) |
| I2 | The first policy deny short-circuits with a single reason | `pkg/governance/policy.go:69-88` |
| I3 | Every publish + handle is one span, linked by traceparent in metadata | `bus.go:108-124`, `orchestrator.go:111-123` |
| I4 | An agent error is caught centrally and can reroute to a fallback | `orchestrator.go:167-184` |
| I5 | A message class may not exceed its recipient ceiling | `pkg/governance/classification.go:35-42` |
| I6 | PII/secret classified data cannot leave the home region | `pkg/governance/sovereignty.go:40-46` |
| I7 | High-classification data is encrypted at rest with per-payload DEKs | `pkg/crypto/envelope.go:68-100` (documents, mcp_tokens) |
| I8 | JWT verification rejects non-HS256 and forged signatures | `pkg/auth/jwt.go:59-102` |
| I9 | The audit log is tamper-evident (hash chain) | `pkg/compliance/audit.go:77-105` |
| I10 | An LLM call cannot exceed the per-call deadline (Ollama path) | `pkg/llm/deadline.go` |
| I11 | Repeated LLM failures stop hammering the backend (Ollama path) | `pkg/llm/circuit.go` |
| I12 | A reasoning loop cannot iterate unbounded | `pkg/reasoning/reasoning.go:79-135` (maxSteps) |
| I13 | The published capability inventory is read live from the registry | `pkg/web/handlers/inventory.go:30-45` |
| I14 | The governance policy is adversarially tested in CI | `cmd/red-team/main.go` (`make red-team`) |
| I15 | Fan-in fires exactly once per trace and cannot miscount | `agents/supervisor/supervisor.go:123-154` (session delete on finalise) |

**Invariants deliberately *not* claimed** (so no one can claim we implied them):
delivery guarantees, message ordering, panic isolation, cycle prevention,
risk-class-based dispatch gating, and encryption of `users`/`incidents`/`audit`
tables.

---

## 17. Alternatives considered and rejected

| Alternative | Why rejected for this system |
|---|---|
| **Direct agent-to-agent Go calls** | Kills all four seams of §3: no single audit point, governance must be threaded into every caller, tracing is manual, no fallback interception. The whole thesis is that the indirection pays for itself. |
| **A third-party agent framework (LangGraph/AutoGen-style)** | Would hide the governance gate and the trace seam inside someone else's control loop; regulated-finance auditability needs the ~200-line orchestrator to be *ours* and readable. |
| **A god-object orchestrator that calls agents in a fixed sequence** | Couples the control flow to the agent set; adding an agent means editing the orchestrator. The bus + Type-string convention lets the pipeline self-assemble. |
| **Synchronous, blocking bus (call-and-wait)** | Would serialise the analyzer's fan-out and make latency `sum(stages)` instead of `max(branch)`. The async bus + structural fan-in (§8.3) is what buys parallelism. |
| **Numeric fan-in counter in the supervisor** | Fragile under duplicate/out-of-order delivery. Named-field readiness (I15) is idempotent. |
| **Risk-class-based dispatch gating** | Considered and *not* built — it would duplicate what `RBACPolicy` already does by message Type, and risk class is genuinely only needed by the reporting surfaces (inventory/AIBOM/disclosures). Documented as a non-invariant rather than half-implemented. |
| **Encrypt every table** | Rejected in favour of encrypting only sensitive payloads (documents, MCP tokens) and making the audit log tamper-evident via hashing — encryption of an append-only hash chain adds cost without adding integrity. |

---

## 18. Known limitations

Named up front so they inform, not ambush:

1. **In-process bus, no delivery guarantees.** No persistence, retries,
   ordering, or backpressure; a load spike spawns unbounded goroutines
   (`pkg/comm/bus.go:52-54`).
2. **No cycle / recursion-depth guard.** Mutually-publishing agents would loop
   forever; only LLM-backed work is bounded (§9.3).
3. **Panics are not isolated.** A panicking agent goroutine is unrecovered and
   is not caught by the HTTP recovery middleware (§9.4). Fallback does not fire
   on panic.
4. **Config surface exceeds enforcement in two spots.** `SchemaPolicy` is never
   wired; `ClassificationPolicy.AllowedTo` and
   `sovereignty.allow_cross_border_for_public` are ignored by `BuildComposite`
   (§10.3).
5. **The autonomy onion is Ollama-only.** The default `mock` provider is
   unwrapped (§11.2); circuit/deadline/budget do not apply to it.
6. **Budget principal is a stand-in.** `BudgetedProvider` keys on
   `req.Residency.Region`, not a user id, so the daily cap is effectively
   per-region (§11.3).
7. **Embeddings bypass the wrapper chain** — unbudgeted, uncached, not
   circuit-broken (§11.1).
8. **Risk class is reporting-only**, not a dispatch gate; the CLAUDE.md/main.go
   comment implying a "risk ceiling at dispatch" overstates the code (§6.3,
   §10). *(This doc is authoritative; those comments should be corrected.)*
9. **Rate-limit keying quirk.** For authenticated `/v1` routes the limiter runs
   before `mid.Auth` populates claims, so it keys on `RemoteAddr`, not the JWT
   subject.
10. **Unwired seams:** `observability/bq` (no warehouse export), cloud LLM
    providers, `KMSKeyResolver`, and three specialist agent packages
    (`h_supervisor`, `moa_recommender`, `receipt_ocr`).
11. **Stale instrumentation scope names** hard-coded to `c2siorg/genie` vs the
    real module path (§14).

---

## Appendix A — how to verify this document

Run these from the repo root. Each maps to a claim above; the point is that you
can refute or confirm this doc in minutes.

```bash
# §1 counts: 62 agent dirs; 60 register calls; 2 fallbacks
ls -d agents/*/ | wc -l
grep -c 'register(' cmd/api/main.go
grep -c 'fallback.NewFor' cmd/api/main.go

# §1 the 3 unwired specialists
for a in h_supervisor moa_recommender receipt_ocr; do grep -q "$a" cmd/api/main.go || echo "unwired: $a"; done

# §6 Message struct, 6 roles, no MessageType constants
sed -n '71,97p' pkg/protocol/message.go
grep -n 'Role.*MessageRole = ' pkg/protocol/message.go
grep -rn 'MessageType' pkg/protocol/            # expect: no results
grep -rn 'correlation_id' pkg/ cmd/ agents/     # expect: no results (correlation is trace_id)

# §8 analyzer fans out to FOUR; supervisor fan-in is structural
grep -n 'agent.NewMessage(ID,' agents/analyzer/analyzer.go   # 4 lines
sed -n '41,52p' agents/supervisor/supervisor.go              # session + isReady

# §10 governance: interface, deny-on-first-failure, assembly order, SchemaPolicy unwired
sed -n '42,44p' pkg/governance/policy.go
sed -n '69,88p' pkg/governance/policy.go
sed -n '106,148p' pkg/policy/policy.go
grep -rn 'SchemaPolicy' pkg/policy/             # expect: no construction in BuildComposite

# §11 the REAL wrapper order (Circuit→Deadline→Budget→Cache→Cost→Ollama) and bare mock
sed -n '83,89p' cmd/api/llmstack.go
sed -n '56,69p' cmd/api/llmstack.go             # mock stack is bare

# §12 fallback signature is two strings; only 2 wired
grep -n 'func .*SetFallback' pkg/orchestration/orchestrator.go
grep -n 'SetFallback(' cmd/api/main.go

# §13 crypto AES-256-GCM, 32-byte KEK; HS256 JWT
grep -n 'AES-256-GCM' pkg/crypto/envelope.go
grep -n 'HS256' pkg/auth/jwt.go
grep -n 'payload' pkg/storage/postgres/migrations/*.sql   # which tables carry envelopes

# §14 bq sink is unwired; stale tracer names
grep -rn 'observability/bq' cmd/ pkg/ agents/ | grep -v '/bq/'   # expect: no call sites
grep -rn 'c2siorg/genie' pkg/ | head
```

If any of these disagree with the prose, the prose is wrong — fix it.

---

## Appendix B — package map

**Messaging core:** `pkg/protocol` (Message, Classification), `pkg/agent`
(Agent/Environment interfaces, RiskClass), `pkg/comm` (bus), `pkg/busio`
(correlator), `pkg/registry` (agent map).

**Control:** `pkg/orchestration` (dispatch), `pkg/governance` (10 policies),
`pkg/policy` (YAML → composite), `pkg/incidents` (FREE-AI incident form),
`pkg/sovereignty` (region).

**Reasoning/LLM:** `pkg/llm` (provider + wrappers), `pkg/reasoning` (ReAct,
Reflexion), `pkg/constitution` (LLM-as-judge), `pkg/eval`, `pkg/safety`,
`pkg/rag`, `pkg/graphrag`.

**Security/data:** `pkg/auth` (HS256 JWT), `pkg/identity`, `pkg/crypto`
(envelopes), `pkg/privacy` (tokeniser, DP), `pkg/compliance` (consent, audit
chain), `pkg/storage/postgres` (migrations + repos).

**Edge/ops:** `pkg/web` (chi router, `mid/`, `handlers/`), `pkg/observability`
(OTel, metrics, unwired `bq`), `pkg/aibom` (AI bill of materials), `pkg/mcp`,
`pkg/a2a`.

**Agents:** `agents/<id>/<id>.go` — 61 specialist packages (+ `agents/fallback`);
58 specialists wired in `cmd/api/main.go`.

---

## Where to go next

- [agents/README.md](agents/README.md) — the agent contract and how to add one
- [api.md](api.md) — every HTTP endpoint with sample curl
- [protocols.md](protocols.md) — message types and payload shapes
- [operations.md](operations.md) — running the stack and required env
- [free-ai-mapping.md](free-ai-mapping.md) — every FREE-AI recommendation → file path
