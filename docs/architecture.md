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
>
> **Runtime note — read this first.** The flagship HTTP edge (`cmd/api`) no
> longer runs on a message bus. A recent cutover replaced the bus + orchestrator
> + correlator with a **governed agent-framework pipeline** (`pkg/afg`,
> `afg.QAService`). The in-memory message bus still exists — it is the engine of
> the **CLI demo** (`cmd/genie`) and is preserved whole on the
> `legacy/bus-architecture` branch (tag `ec27f6b`) — but it is **not** in the
> production `/v1/ask` path. §7–§9 describe the live pipeline; §15 documents the
> legacy bus honestly as history/CLI.

---

## Table of contents

1. [Context and constraints](#1-context-and-constraints)
2. [Design goals and non-goals](#2-design-goals-and-non-goals)
3. [The load-bearing decision](#3-the-load-bearing-decision-governance-survives-as-a-single-construction-door)
4. [System context (C4 L1)](#4-system-context-c4-l1)
5. [Component decomposition (C4 L2)](#5-component-decomposition-c4-l2)
6. [The data model: Message, and the lattices](#6-the-data-model-message-and-the-lattices)
7. [Request lifecycle: one question, end to end](#7-request-lifecycle-one-question-end-to-end)
8. [The governed pipeline: nine stages, one gate each](#8-the-governed-pipeline-nine-stages-one-gate-each)
9. [Execution and error semantics](#9-execution-and-error-semantics)
10. [The governance plane](#10-the-governance-plane)
11. [The autonomy-bounding stack (LLM wrappers)](#11-the-autonomy-bounding-stack-llm-wrappers)
12. [Failure model and business continuity](#12-failure-model-and-business-continuity)
13. [Security architecture](#13-security-architecture)
14. [Observability](#14-observability)
15. [The legacy bus architecture (CLI + preserved branch)](#15-the-legacy-bus-architecture-cli--preserved-branch)
16. [Scaling and evolution: the seams](#16-scaling-and-evolution-the-seams)
17. [System invariants → enforcing code](#17-system-invariants--enforcing-code)
18. [Alternatives considered and rejected](#18-alternatives-considered-and-rejected)
19. [Known limitations](#19-known-limitations)
20. [Appendix A — how to verify this document](#appendix-a--how-to-verify-this-document)
21. [Appendix B — package map](#appendix-b--package-map)

---

## 1. Context and constraints

Genie is an AI financial assistant built as a **reference implementation** of
Microsoft's Multi-Agent Reference Architecture (MARA), aligned to the RBI
FREE-AI report (Aug 2025). It answers finance questions over a user's own
transaction data using a fleet of specialist agents, and it is engineered for a
context where the following constraints are *hard*, not aspirational:

| Constraint | Where it comes from | How it shapes the architecture |
|---|---|---|
| **Every autonomous decision must be auditable** | Regulated finance; FREE-AI accountability | One trace per request; governance as a gate on every stage; hash-chained audit log |
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
  `fallback.NewFor`, at `cmd/api/main.go:204-205`).
- **3 specialist packages are implemented but not wired**: `h_supervisor`,
  `moa_recommender`, `receipt_ocr` (they are never imported in `main.go`).
- So: **61 specialist packages + 1 fallback package = 62**; **58 specialists
  are live**. These are the honest counts; re-derive them any time with the
  harness in Appendix A.
- **Important scope point:** those 58 agents are registered into
  `registry.NewInMemory()` (`cmd/api/main.go:159`), which after the cutover is
  **only** the AI-inventory / AIBOM / disclosures source of truth — it is **not**
  a runtime dispatcher. The `/v1/ask` hot path is the fixed 9-stage
  `afg.QAService` pipeline (§7–§8), which drives just nine of those agents.

---

## 2. Design goals and non-goals

**Goals (in priority order):**

1. **Auditability over cleverness.** Every stage in the question pipeline is
   governed, observable, and attributable to a trace. If a feature can't be
   audited, it doesn't ship in the hot path.
2. **Governance as data, not code.** Cross-cutting rules live in
   `config/ai-policy.example.yaml` and are assembled by `pkg/policy`; the risk
   team edits YAML, the system obeys. Rules are *not* scattered into agents.
3. **Bounded autonomy by construction.** An agent that reasons in a loop cannot
   run away — the LLM wrapper chain (legacy `pkg/llm` and framework
   `GuardMiddleware`) caps time, tokens, and consecutive errors.
4. **Degrade honestly.** When a primary agent fails, users get a truthful "a
   human will follow up" notice, not a fabricated answer.
5. **Swappable everything.** Registry, LLM provider, key resolver, stores are
   interfaces with in-memory reference impls and production seams.

**Non-goals (explicitly out of scope for this codebase today):**

- **Not a distributed system yet.** The production pipeline is a single
  in-process call chain; the legacy bus (`pkg/comm`) is also in-process.
  Multi-node requires new infrastructure (see §16). We do not claim horizontal
  scale today.
- **Not a streaming-per-hop API.** After the cutover the pipeline runs as one
  inline call, so the SSE edge collapses per-hop progress into a single event
  (§14). Restoring per-hop progress is a tracked follow-up, not a shipped
  feature.
- **Not a PII *redaction* system.** PII handling is deny-at-the-gate plus
  optional tokenisation; there is no scrubbing of PII out of otherwise-allowed
  content.
- **Not a finished production security posture.** It ships real primitives
  (AES-256-GCM envelopes, HS256 JWT, RBAC) but with reference key management
  (env-var KEK) that you replace with a KMS in production.

Stating the non-goals is deliberate: it removes the "but it doesn't do X"
critique for every X we never set out to do.

---

## 3. The load-bearing decision: governance survives as a single construction door

The original architecture rested on one decision: *a request is a
`protocol.Message` on a bus, and every message crosses a single governance
chokepoint before any handler runs.* The agent-framework cutover kept the
**guarantee** and changed the **mechanism**.

The Microsoft agent framework has **no single chokepoint** — middleware is
attached per-agent, not to a shared bus. So `pkg/afg` preserves the guarantee
*structurally* (`pkg/afg/factory.go:1-15`, package doc):

> **Every framework agent is built through exactly one of two sanctioned
> constructors — `NewGovernedDeterministic` or `NewGovernedOllama`
> (`pkg/afg/factory.go:96-105`, `:111-129`) — and both inject `GovMiddleware`
> as the outermost middleware. No message reaches a model or a tool without
> passing the gate.**

This is enforced, not asserted. `TestSingleConstructionDoor`
(`pkg/afg/singledoor_test.go:18`) walks every `.go` file that imports the
framework and **fails CI** if any file outside `pkg/afg` calls a raw framework
constructor (`agent.New(`, `*provider.New*Agent(`). The gate cannot be smuggled
around because there is only one door to build an agent, and the door welds the
gate on.

`GovMiddleware.Run` (`pkg/afg/factory.go:59-78`) runs **before** the provider:
it evaluates the composite policy and, on a deny, returns an error-yielding
stream **without calling `next`** — so no model or tool call happens on a
rejected message. That reconstructs the bus's single chokepoint, per agent.

| Seam | How it survived the cutover | File |
|---|---|---|
| **Governance** | `GovMiddleware`, outermost on every governed agent, enforced by the single-door test | `pkg/afg/factory.go:59-78`, `singledoor_test.go:18` |
| **Bounded autonomy** | `GuardMiddleware` (circuit/deadline/budget/cache) just inside the gate on LLM agents | `pkg/afg/llmguard.go:37`, `factory.go:123` |
| **Inventory** | The live capability list is still read from one registry | `pkg/web/handlers/inventory.go` |
| **Fallback** | Retained at the framework catalog edge (`RunWithFallback`); the QA pipeline aborts on deny with no rescue | `pkg/afg/httpedge.go`, `qa_service.go:272-324` |

What was **lost** relative to the bus is honestly enumerated in §12 and §19: the
orchestrator's incident-on-deny / incident-on-error hooks, the auditor's
eval-on-every-message subscription, and MCP bus-tools are **deferred** (tracked
follow-ups noted in-source at `cmd/api/main.go:286-290`), and per-hop SSE
streaming is collapsed to one progress event.

---

## 4. System context (C4 L1)

```
                        ┌──────────────────────────────────────────┐
                        │                 Genie API                 │
   End user  ──HTTP────▶│  (cmd/api) chi router + middleware +      │
   (JWT)                │   afg.QAService pipeline + governance      │
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
- **LLM:** the `/v1/ask` question pipeline is **deterministic** — it drives
  no-LLM framework stages (§8). LLM enters `cmd/api` only through the RAG
  embedder, the educator's citations, and the auditor's constitutional judge,
  all built from `buildLLMStack` (`cmd/api/llmstack.go`). Default `mock`
  (deterministic, zero external deps); `GENIE_LLM=ollama` points at an on-prem
  Ollama. Cloud providers exist in `pkg/llm` but are **not** wired into
  `buildLLMStack`.
- **Telemetry:** OTLP gRPC when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, else a
  stdout exporter.

A second, Postgres-free edge — `cmd/af-serve` — serves the **full 58-agent
governed catalog** through the same framework construction door and the same
`afg.NewHandler` HTTP edge (`cmd/af-serve/main.go:39,53`). It is a demo of the
catalog, not the flagship `/v1/ask` product path.

---

## 5. Component decomposition (C4 L2)

```
   HTTP edge (pkg/web)
   ├─ router.go            chi; global mw: RequestID→Recovery→AccessLog→Trace
   ├─ mid/                 Auth (JWT), RequireRole (RBAC), RateLimit (token bucket)
   └─ handlers/            ask, ask_stream, chat_ws, documents, users, inventory, ...
                │
                │  builds gov protocol.Message, calls QA.Answer(ctx, csv, question)
                ▼
   Governed pipeline (pkg/afg)
   ├─ factory.go           GovMiddleware (the gate) + the two construction doors
   ├─ llmguard.go          GuardMiddleware (circuit/deadline/budget/cache) for LLM agents
   ├─ qa_service.go        QAService: 9 fixed governed stages, run inline
   └─ (each stage DRIVES the real legacy agents/<id>.HandleMessage)
                │
        ┌───────┴─────────┬──────────────────┬─────────────────────┐
        ▼                 ▼                  ▼                     ▼
   Governance         Legacy agents     LLM stack            Cross-cutting
   pkg/governance     agents/<id>/      pkg/llm (wrappers)   pkg/crypto  (envelopes)
   pkg/policy (YAML)  (library deps)    pkg/reasoning        pkg/auth    (HS256 JWT)
                      driven by afg      pkg/rag / graphrag   pkg/compliance (consent,
                                        pkg/constitution                    audit chain)
                                        pkg/eval / safety    pkg/incidents  (FREE-AI form)
   pkg/registry ── static AI-inventory / AIBOM / disclosures source (NOT a dispatcher)
```

The layering rule holds: **the framework layer knows nothing about finance**
(it contributes governance and sequencing); the **legacy agents know nothing
about HTTP or the framework** (`afg` drives their `HandleMessage` under a no-op
environment); **governance knows nothing about specific agents** (it reads
message metadata). Each arrow is a Go interface or a construction door, not a
concrete dependency.

The reason each stage drives the *real* legacy agent rather than reimplementing
it: the forecaster/anomaly/recommender/reporter output is embedded verbatim in
the final report, so re-derivation would risk silent drift. Driving the real
`HandleMessage` makes byte-parity with the legacy oracle **structural, not
coincidental** — asserted by the oracle parity test
(`pkg/afg/qa_service_test.go:3`).

---

## 6. The data model: Message, and the lattices

`protocol.Message` remains the governance contract and the legacy agent wire
format (`pkg/protocol/message.go:71-97`):

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
(`message.go:108-119`), which stamps a UUID `ID` and a UTC `CreatedAt`. In the
new edge, the HTTP handler builds a **governance-relevant** `Message` (Type +
Metadata, empty Content) and attaches it to the context with
`afg.WithGovMessage` (`pkg/web/handlers/ask.go:93-107`); `GovMiddleware` reads
it out at each stage (`pkg/afg/factory.go:42-57`).

**Three things carry the semantics:**

### 6.1 `Type` — the routing/semantic key (a plain string)

`Type` is a **plain `string`**; there are **no `MessageType` constants** in
`pkg/protocol`. Each legacy agent declares its own `TypeIn`/`TypeOut` constants
and guards `if msg.Type != TypeIn { return nil, nil }`. The QA pipeline wires
those Types together explicitly, in Go, when it drives each stage — see the
exact chain in §8. In governance, `Type` is the RBAC/classification key: the
`/v1/ask` edge sets it to `finance_question`
(`pkg/web/handlers/ask.go:22,93-94`).

### 6.2 `Role` — a 6-value enum (`message.go:18-43`)

`user`, `system`, `agent`, `tool`, `observer`, `evaluator`.

### 6.3 `Metadata` — the typed-payload escape hatch (`map[string]any`)

Untyped by design. Only **three** metadata keys have constants
(`pkg/protocol/classification.go:19-26`): `classification`, `user_roles`,
`user_id`. The `region` key's constant lives in `pkg/sovereignty`. The key
`trace_id` is a bare string literal (the `/v1/ask` handler mints
`tr-<unixnano>` at `ask.go:82`). **`correlation_id` does not exist anywhere in
the codebase.** Common domain keys in play: `csv`, `account_id`,
`pii_acknowledged`.

### The classification lattice (`pkg/governance/classification.go:26-31`)

```
public (0)  <  internal (1)  <  pii (2)  <  secret (3)
```

A message's class (default `internal` when unset) must not exceed the
recipient's *ceiling*. In the wired policy the ceiling is a **hardcoded
`internal`** for every recipient — the per-recipient `AllowedTo` override map
exists in the policy struct but is **never populated from YAML** by
`BuildComposite` (`pkg/policy/policy.go:107`). This is a real gap between config
surface and enforced behaviour, documented here so it can't be used as a
"gotcha."

### The risk lattice (`pkg/agent/risk.go`)

```
low  <  medium  <  high        // RiskClass; agents without RiskLevel() default to low
```

`RiskOf(a)` (`risk.go:31-36`) type-asserts to `RiskAware`; agents that don't
implement `RiskLevel()` are `low`. **Important honesty point:** `RiskOf` is read
only by *reporting* surfaces — the AIBOM (`pkg/aibom`), the disclosures handler,
and the `/v1/ai-inventory` endpoint. **Nothing dispatches on risk class.** The
de-facto access control on high-risk actions is `RBACPolicy`, which gates by
message **`Type`**, not by risk class (see §10.3).

---

## 7. Request lifecycle: one question, end to end

The path of `POST /v1/ask` (`pkg/web/handlers/ask.go:51`), verified line by
line. There is **no bus, no correlator, no async wait** — the pipeline runs
inline and returns.

```
Client                HTTP edge (chi)                     afg.QAService (in-process)
  │  POST /v1/ask + Bearer
  ├──────────────▶ RequestID→Recovery→AccessLog→Trace  (global mw, router.go:38-41)
  │                RateLimit(60 burst,1/s) → Auth(JWT) → handler  (router.go:60-101)
  │                       │
  │                       │ decode {question, document_id}; 400 if empty      (ask.go:57-65)
  │                       │ Documents.GetByID → 404; owner check → 403          (ask.go:67-75)
  │                       │ Encryptor.Decrypt(doc.Payload) → plaintext CSV (500) (ask.go:76)
  │                       │ traceID = "tr-<unixnano>"                            (ask.go:82)
  │                       │ gov = protocol.Message{Type:"finance_question",
  │                       │        Metadata:{trace_id,user_id,user_roles,classification}} (ask.go:93-101)
  │                       │ ctx = WithGovMessage(ctx, gov); WithTimeout(60s)      (ask.go:103-108)
  │                       │ report, err := QA.Answer(ctx, csv, question) ─────────▶ 9 governed stages,
  │                       │                                                          gate at EACH (§8)
  │                       │                                                          reporter renders text
  │                       │ ◀──────────────────────── report ───────────────────────┘
  │                       │ switch on err:
  │                       │   *DeniedError            → 403 (no fallback)          (ask.go:112-116)
  │                       │   context.DeadlineExceeded → 504                        (ask.go:117-120)
  │                       │   other                    → 502 pipeline error         (ask.go:121-122)
  │                       │ else 200 {trace_id, report, ai_disclosure}             (ask.go:125-129)
  │  ◀── 200 / 403 / 502 / 504 ───┤
```

Key facts (each greppable):

- **The trace id is minted in the handler** as
  `fmt.Sprintf("tr-%d", time.Now().UnixNano())` (`ask.go:82`). It rides in the
  governance metadata; there is no correlator keyed on it anymore.
- **Governance runs at every stage, not once.** The gate is
  `GovMiddleware`, injected as the outermost middleware on all nine stages by
  `NewQAService` → `NewGovernedDeterministic` (`qa_service.go:227-240`,
  `factory.go:96-105`).
- **A governance deny aborts the whole run.** `GovMiddleware` yields a
  `*afg.DeniedError` (`factory.go:74-76,84-91`), which `QAService.Answer`
  propagates unwrapped (`qa_service.go:272-324`); the handler maps it to **403,
  with no fallback rescue** (`ask.go:112-116`).
- **Request timeout is `GENIE_ASK_TIMEOUT`, default 60s**, set in
  `cmd/api/main.go:313`. (The handler's own zero-value default is 8s, but main
  always overrides it.) On timeout the context's `DeadlineExceeded` surfaces as
  HTTP 504 (`ask.go:117-120`).

Two sibling edges share the same pipeline and gate:

- **`POST /v1/ask/stream`** (`pkg/web/handlers/ask_stream.go`) — SSE. It emits
  `ai_disclosure`, a `trace` id, **one** `agent.handle` progress event, then the
  `report` event. Because `QAService.Answer` is one inline call, per-hop
  progress is collapsed into that single event (§14).
- **`GET /v1/chat/ws`** (`pkg/web/handlers/chat_ws.go`) — WebSocket, same
  `QA.Answer` call per request, same single `pipeline_running` progress frame.

---

## 8. The governed pipeline: nine stages, one gate each

`afg.QAService` reproduces Genie's legacy supervisor question-pipeline as a
**fixed, in-process sequence of nine governed framework stages**
(`pkg/afg/qa_service.go`). The struct holds exactly those nine agents
(`qa_service.go:211-223`) and `NewQAService` builds each one through the single
construction door under the same gate (`qa_service.go:227-240`):

```
ingestor → normalizer → enricher → analyzer
        → {forecaster, anomaly, recommender}  (fan-out, run sequentially here)
        → supervisor → reporter
```

### 8.1 The chain, stage by stage

Each stage's `RunFunc` drives the real legacy agent's `HandleMessage` under
Genie's own no-op `Environment` (`qaEnv`, `qa_service.go:42-47`), then asserts
the expected number of follow-up messages:

| Governed stage | Legacy agent driven | Input → output |
|---|---|---|
| `qa_ingestor` | `agents/ingestor` | CSV → `raw_transactions` (`qa_service.go:62-69`) |
| `qa_normalizer` | `agents/normalizer` | raw → `normalized_transactions` (`:71-78`) |
| `qa_enricher` | `agents/enricher` | normalized → `enriched_transactions` (`:80-87`) |
| `qa_analyzer` | `agents/analyzer` | enriched → `analysis_result` (**fans out ×4; all identical, verified**) (`:94-110`) |
| `qa_forecaster` | `agents/forecaster` | analysis → `forecast_result` (`:112-119`) |
| `qa_anomaly` | `agents/anomaly` | analysis → `anomalies` (`:121-128`) |
| `qa_recommender` | `agents/recommender` | analysis → `recommendations` (`:130-137`) |
| `qa_supervisor` | `agents/supervisor` | replays the four fan-in Types into the real session logic (`:159-195`) |
| `qa_reporter` | `agents/reporter` | bundle → final human-readable report (`:197-204`) |

`Answer` (`qa_service.go:272-324`) threads these calls: ingest → normalize →
enrich → analyze, then the forecaster/anomaly/recommender trio over the same
analyzer output, then it marshals a `qaSupervisorPacket`
(`qa_service.go:147-153`) and hands it to the supervisor stage, which
**replays the exact HandleMessage sequence** (`finance_question`,
`analysis_result`, `forecast_result`, `anomalies`, `recommendations`) that the
bus would have delivered over time — reusing the legacy session/bundle-assembly
logic unchanged (`qa_service.go:170-194`). The reporter renders the final text.

### 8.2 Fan-out is real but runs sequentially here

The legacy analyzer still fans out four byte-identical copies of its result
(`agents/analyzer/analyzer.go:104-107`) — the QA analyzer stage takes one and
**defensively confirms all four are identical** before proceeding
(`qa_service.go:100-109`). But where the *bus* delivered the
forecaster/anomaly/recommender branches concurrently (latency `max(branch)`),
`QAService.Answer` runs them **sequentially** (latency `sum(stages)`). This is a
deliberate trade for a simpler, data-race-free single-shot driver, and it is
**content-neutral**: forecaster/anomaly/recommender are each pure functions of
the same analyzer output and never interact, so the assembled bundle is
identical either way (`qa_service.go:256-271`, comment).

### 8.3 Byte-parity is a test, not a hope

Because each governed stage drives the *real* legacy agent, the report is
byte-identical to what the old bus pipeline produced for the same inputs. The
**oracle parity test** (`pkg/afg/qa_service_test.go:3`) drives the legacy agents
directly and asserts the two reports match. If a legacy agent's logic changes,
both paths change together; drift is structurally impossible.

---

## 9. Execution and error semantics

The `/v1/ask` pipeline is an **inline, sequential, single-goroutine** call
chain. That removes an entire class of the bus's non-guarantees (no delivery
races, no ordering hazard, no unbounded goroutine fan-out) at the cost of
`sum(stages)` latency. What remains worth stating precisely:

### 9.1 One stage at a time, fail-fast

`Answer` (`qa_service.go:272-324`) calls each governed stage via `qaCollect`
(`qa_service.go:248-254`) and **returns on the first error**, so no downstream
stage runs after a failure. `qaCollect` uses `ResponseStream.Collect`, which
propagates the yielded error value **unchanged** — so a `*DeniedError` raised by
`GovMiddleware` is the very same value the caller can `errors.As` for
(`qa_service.go:242-254`, comment).

### 9.2 Governance deny vs. execution error

`GovMiddleware` distinguishes two failure kinds (`factory.go:70-76`):

- a policy **operational error** (bad config, unreachable ledger) is wrapped as
  `"governance evaluation error: …"`;
- a policy **deny** yields a `*DeniedError{Type, Reason}` (`factory.go:84-91`).

The edge treats them differently (`ask.go:112-122`): a `*DeniedError` → **403,
no fallback** (a denied message must not be answered by a fallback, only
recorded); a `DeadlineExceeded` → 504; anything else → 502. This is the
deliberate "a policy rejection is not an outage" distinction the bus made too.

### 9.3 Per-call bounding on LLM stages

The nine QA stages are all **deterministic** (`NewGovernedDeterministic`, no LLM,
no network), so there is nothing to time-box beyond the request context. LLM
agents built via `NewGovernedOllama` get `GuardMiddleware` **inside** the gate
(§11), which imposes circuit/deadline/budget/cache. `GuardMiddleware` runs
`next` in its own goroutine so a provider that ignores context cancellation is
still cut off at the deadline — an honestly-noted goroutine-leak risk if the
provider never returns and never respects ctx (`pkg/afg/llmguard.go:155-207`).

### 9.4 Panics

Neither the framework RunFunc path nor `QAService.Answer` installs a
`recover()`. A panicking stage propagates up the goroutine that serves the HTTP
request, where `mid.Recovery` (`router.go:39`) **does** catch it and return
500 — unlike the legacy bus, whose handler goroutines ran *outside* the request
goroutine and were unrecovered (§15). So on the new edge a stage panic is a
clean 500, not a leaked crash.

---

## 10. The governance plane

Every stage of the pipeline is evaluated by one composite policy **before** its
RunFunc executes, via `GovMiddleware` as the outermost middleware
(`pkg/afg/factory.go:59-78`). This is the audit gate — reconstructed per-agent
after the bus's single chokepoint went away, and kept unbypassable by the
single-construction-door test (§3).

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

`CompositePolicy.Evaluate` (`policy.go:69`) iterates policies **in slice order**;
the **first deny short-circuits** and returns its reason; an operational `error`
aborts immediately. The tradeoff (one clear reason, fast; but only the first
failure is reported) is stated in the source. The composite is assembled once at
startup by `BuildComposite` (`pkg/policy/policy.go:107`) and handed to
`afg.NewQAService(composite)` (`cmd/api/main.go:175,291`).

### 10.3 The ten policies and how they're assembled

`pkg/governance` defines **ten** concrete policies. `BuildComposite`
(`pkg/policy/policy.go:107`) assembles them from YAML in **this order** — which
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

> **How the gate sees each stage.** On `/v1/ask`, the handler sets a governance
> `Message` with `Type=finance_question` and **empty Content**, so each stage's
> gate falls back to evaluating **that stage's own payload** — the CSV, the
> enriched transactions, and so on — via `govMessageFrom`'s fallback-content
> path (`factory.go:46-57`, `ask.go:89-101`). Type + roles drive the
> RBAC/classification checks uniformly across all nine stages; the content-based
> checks (length, PII, prompt-injection) see each stage's real intermediate
> data. This reproduces the legacy bus's per-hop content evaluation.

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

`make red-team` (`cmd/red-team/main.go`) runs an adversarial corpus against the
**YAML-built** composite and **exits non-zero on any unexpected allow** (FREE-AI
Rec 20). One honest caveat the source itself notes: the residency probe is
tagged `pii`+region-US, but `ClassificationPolicy` sits *before*
`DataResidencyPolicy` and denies it first — so that probe usually never reaches
the residency check. Both denials are accepted by the test.

---

## 11. The autonomy-bounding stack (LLM wrappers)

An agent that reasons in a loop must not be able to burn unbounded time, tokens,
or a dead backend. After the cutover there are **two** bounding mechanisms, for
two different call surfaces.

### 11.1 Framework LLM agents — `GuardMiddleware` (`pkg/afg/llmguard.go`)

LLM-backed framework agents are built via `NewGovernedOllama`
(`factory.go:111-129`), which composes middleware as **`{GovMiddleware, GuardMiddleware}`**
— gate outermost (denials short-circuit before any spend), guard just inside it.
`GuardMiddleware` (`llmguard.go:37`) re-hosts the legacy wrapper chain onto the
framework RunFunc path. Its `Run` order (all within the one middleware,
`llmguard.go:130-207`) is:

```
circuit-open check → cache lookup (hit ⇒ return, next NOT called)
                   → budget check → deadline → next
```

with, on success, cache-store + circuit-reset + budget-add, and on error a
circuit-failure tick. `DefaultGuardConfig` (`llmguard.go:92-100`) reads the
**same env vars** as the legacy stack, so both paths are bounded identically by
default:

| Bound | Knob / default |
|---|---|
| Per-call deadline | `GENIE_LLM_TIMEOUT`=30s |
| Circuit threshold | `GENIE_LLM_CIRCUIT`=5 consecutive failures; cooldown fixed 30s |
| Daily token budget (per agent name) | `GENIE_LLM_BUDGET`=1e6/day |
| Cache TTL | `GENIE_LLM_CACHE_TTL`=600s |

**Honest limitation, stated in-source (`llmguard.go:24-30`):** this bounds the
*streaming* RunFunc at the message layer. It is **not** the byte-identical
`pkg/llm.Provider` chain — there is no `CompletionRequest`/`Usage` at this layer,
so token counting is approximate (`len(text)/4`, `llmguard.go:300-311`) and
cost/observability is out of scope here.

**Deterministic stages get no guard.** All nine `/v1/ask` stages are
`NewGovernedDeterministic` (no LLM to bound); only agents that actually call a
model carry `GuardMiddleware`.

### 11.2 Non-pipeline LLM in `cmd/api` — the legacy `pkg/llm` chain

`cmd/api` still calls `buildLLMStack` (`cmd/api/llmstack.go`) for the LLM uses
that are **not** in the question pipeline: the RAG embedder, the educator's
citations, and the auditor's constitutional judge (`cmd/api/main.go:265-276`).
On the **Ollama** path the real wrapper nesting (outermost → innermost, i.e.
call order) is `cmd/api/llmstack.go:84-89`:

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

**The default path is different.** `GENIE_LLM` defaults to `mock`, and the mock
stack is a **bare `*llm.Mock` with no wrappers at all** (`llmstack.go:58`). The
autonomy-bounding onion exists **only on the Ollama path**. Any claim that
"every LLM call is circuit-broken" is true only when `GENIE_LLM=ollama`.

Two second-order consequences of the real nesting: **Cache sits inside Budget**,
so a cache *hit* still passes through the budget wrapper and is counted; and
**Cost/OTel sits inside Cache**, so cache hits emit **no** cost/latency span.

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
  `maxSteps` defaulting to **5**, each iteration one `Complete` call. Returns an
  error on "max steps reached." The runaway cutoff is layered: the loop caps
  iterations; `Deadline` kills a hung call; `Circuit` trips after N errors;
  `Budget` returns `ErrBudgetExceeded` when the day's tokens are spent.
- **Reflexion** (`reflexion.go:25-79`): **not a loop** — exactly three fixed
  calls (initial → critique → refine). Bounded by construction.

Embeddings use a separate `rag.Embedder` interface and do **not** flow through
either wrapper chain — they are unbudgeted, uncached, and not circuit-broken.

### 11.5 LLM-as-judge (`pkg/constitution`)

`config/constitution.yaml` encodes the FREE-AI "7 Sutras." `Critique`
(`constitution.go:83-98`) sends the sutras + a candidate answer at
`Temperature:0` and parses a `SCORE: <0..10>` verdict (unparseable ⇒ 0). The
auditor wires this via `WithJudge` (`cmd/api/main.go:274-276`). (`pkg/eval` is a
record store, not a judge; `pkg/safety` is heuristic+LLM jailbreak/toxicity
detectors.)

---

## 12. Failure model and business continuity

### 12.1 Failure taxonomy — what happens in each case

| Failure | Detected at | System response |
|---|---|---|
| **Policy deny** | `GovMiddleware`, `factory.go:74-76` | yields `*DeniedError`; `Answer` aborts the run; edge returns **403, no fallback** (`ask.go:112-116`) |
| **Policy op-error** | `GovMiddleware`, `factory.go:70-73` | yields wrapped `"governance evaluation error"`; edge returns 502 |
| **Stage returns error** | `qaCollect`, `qa_service.go:248-254` | `Answer` returns first error; edge returns 502 pipeline error (`ask.go:121`) |
| **LLM timeout / circuit-open / budget** | `GuardMiddleware` (LLM agents only) | surfaces as a stage error ⇒ 502 path |
| **Stage panic** | `mid.Recovery`, `router.go:39` | caught in the request goroutine ⇒ HTTP 500 (§9.4) |
| **Request timeout** | request `context` deadline | `context.DeadlineExceeded` ⇒ HTTP 504 (`ask.go:117-120`) |

**Deferred with the bus removal** (tracked follow-ups, noted in-source at
`cmd/api/main.go:286-290`, *not* silently dropped): the orchestrator's
incident-on-policy-deny / incident-on-agent-error hooks, the auditor's
eval-on-every-message subscription, and MCP BusTool tools. The `/v1/incidents`
form and store remain wired for **manual** reporting (`router.go:86-87`); the
automatic incident emission the bus performed on deny/error is not yet
reattached to the pipeline.

### 12.2 Fallback — where it still lives

The QA pipeline **does not rescue a failed stage** — a deny is a 403 and a stage
error is a 502. The deterministic fallback mechanism survives at the **framework
catalog edge** (`cmd/af-serve` via `afg.NewHandler` → `Registry.RunWithFallback`,
`pkg/afg/httpedge.go`), and the fallback *agents* themselves remain registered
in the `cmd/api` inventory (`fallback.NewFor("portfolio_advisor")`,
`fallback.NewFor("recommender")`, `cmd/api/main.go:204-205`) and are surfaced in
`/v1/ai-inventory` with the `fallbacks` map (`main.go:298-301,345`). A fallback
agent responds to any message with a deterministic, no-LLM "a human reviewer will
follow up" notice.

> **Correction of record vs. the pre-cutover doc:** the old lifecycle showed the
> orchestrator catching an agent error and publishing a `fallback_request` on the
> bus. That path is gone from `/v1/ask`. Fallback rescue is now a property of the
> `RunWithFallback` catalog edge, not the question pipeline.

### 12.3 Business continuity is a CI target

`make bcp-drill` exercises the fallback machinery (FREE-AI Rec 21). Continuity is
a test, not a promise.

---

## 13. Security architecture

### 13.1 Authentication — hand-rolled HS256 JWT (`pkg/auth`)

No third-party JWT library. `Issue` builds `Claims{Subject, Email, Roles, iat,
exp, iss, aud}` and signs with `hmac.New(sha256.New, secret)` (`jwt.go:129-133`).
`Verify` (`jwt.go:59-102`) **rejects any `alg != "HS256"`**, does a constant-time
`hmac.Equal`, and checks `exp`/`iss`/`aud`. Secret from `GENIE_JWT_SECRET`; TTL
60 min; issuer/audience `"genie-api"` (`cmd/api/main.go:154`). Roles: `user`,
`advisor`, `admin`.

### 13.2 Authorization — two layers

- **Route-level RBAC** (`mid.RequireRole`, `pkg/web/mid/auth.go:63-80`):
  OR-semantics (any one listed role passes); must run *after* `mid.Auth`. Admin
  routes: `/v1/incidents` (GET), `/v1/ai-inventory`, `/v1/aibom`,
  `/debug/pprof/*` (`router.go:87-94`).
- **Message-level RBAC** (`RBACPolicy`, §10.4): gates by message `Type` using
  `metadata["user_roles"]`, which the ask handler copies from the JWT claims
  (`ask.go:84-88,98`).

The roles→metadata→policy path: `mid.Auth` verifies the token and stores claims →
the ask handler stringifies `claims.Roles` into `metadata["user_roles"]` and
attaches the governance `Message` to the context → `GovMiddleware` reads it and
`RBACPolicy.Evaluate` checks it at every stage. That chain is the *actual* access
control on sensitive message types.

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
base64-decode to **exactly 32 bytes** (default KEK id `local-env-v1`,
`cmd/api/main.go:150`). A `KMSKeyResolver` + `KMSClient` interface is the
production seam — Wrap/Unwrap delegate to a KMS; unimplemented by default.

**What is actually encrypted:** only `documents.payload` and `mcp_tokens.payload`
(both JSONB envelopes). `users`, `accounts`, `incidents`, `audit_log`, and
`rag_embeddings` are plaintext at rest. Encryption happens in the HTTP handlers,
not the Postgres repos — `handlers/documents.go` encrypts before insert,
`handlers/ask.go` decrypts after select (`ask.go:76`); the repos only
marshal/unmarshal the envelope. Plaintext exists only in memory during those
handler calls.

### 13.4 Consent and audit (`pkg/compliance`)

- **Consent ledger:** `Grant/Revoke/HasActive` over four categories
  (`transactions`, `portfolio`, `recommendations`, `third_party_share`).
  `ConsentPolicy` denies a mapped message type when no active consent exists.
  The ledger is wired into `BuildComposite` at `cmd/api/main.go:162,175`.
- **Audit log:** a **SHA-256 hash chain** — each `AuditEntry.RowHash =
  sha256(prev || canonical-json(row))`, and `Verify` walks from `"genesis"` and
  fails on any mismatch (`audit.go:77-105`). Tamper-evidence without encryption.
  (Note: the audit log is constructed at `main.go:163` but currently reserved —
  `_ = auditLog` — for incident-correlated entries; see §19.)

### 13.5 Privacy primitives (`pkg/privacy`)

An HMAC-SHA256 `Tokeniser` (`tok_<hex>`) for deterministic pseudonymisation and
Laplace/Gaussian differential-privacy helpers. **There is no PII redaction of
message content** — PII control is deny-at-the-gate (§10.4) plus optional
tokenisation a caller must invoke.

---

## 14. Observability

**One trace per request.** The chi `Trace` middleware opens a server span at the
edge (`router.go:41`). Because the pipeline is now an **inline call chain** (not
fire-and-forget bus deliveries), stage work happens inside the request goroutine
and the framework agents' own spans nest naturally under the HTTP span — there is
no cross-goroutine traceparent re-parenting to do on this path.

**Span tree per question (afg pipeline):**

```
http POST /v1/ask                     (SpanKindServer, pkg/web)
└─ QA.Answer  (inline)
   ├─ stage qa_ingestor … qa_reporter (governed, sequential)
   │    └─ GovMiddleware.Evaluate      (the gate, per stage)
   └─ (LLM agents only) GuardMiddleware → provider
```

**SSE per-hop progress is collapsed.** The legacy bus edge streamed one
`agent.handle` event per hop (analyzer, forecaster, …). The afg `QAService` runs
the pipeline as one governed call, so `AskStream` emits a **single**
`agent.handle` / `pipeline_running` progress event between `trace` and `report`
(`pkg/web/handlers/ask_stream.go:18-27,108-109`). The SSE event vocabulary
(`ai_disclosure` / `trace` / `agent.handle` / `report`) is preserved for the
console; restoring per-hop progress is a tracked follow-up.

**Metrics** (`pkg/observability/metrics.go`) and **exporters** are unchanged:
OTLP gRPC when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, else stdout
(`cmd/api/main.go:119-127`).

**Live inventory, never stale:** `GET /v1/ai-inventory` reads the registry at
request time (`handlers/inventory.go`), reporting each agent's risk class via
`RiskOf`. The AIBOM endpoint (`/v1/aibom`) and disclosures (`/v1/disclosures`)
are the other read surfaces for the same registry data. Remember (§1, §3): this
registry is now a *reporting catalog*, not a dispatcher.

**Two honesty flags:** (1) `pkg/observability/bq` (a BigQuery/warehouse `Sink`
with a `JSONLSink` reference impl) is **implemented but not wired** — no call
site, so no live warehouse export path. (2) OTel tracer/meter scope names are
hard-coded `github.com/c2siorg/genie/…` (e.g. `router.go:41`), stale relative to
the actual module path `github.com/PratikDhanave/…`.

---

## 15. The legacy bus architecture (CLI + preserved branch)

The message-bus design is **not deleted and not dead** — it is the engine of the
CLI demo and is preserved whole for reference. This section documents it as
**history and CLI**, not as the live API.

**Where the bus still runs today:**

- **`cmd/genie`** — the CLI demo — constructs the bus and orchestrator directly:
  `bus := comm.NewInMemoryBus()` (`cmd/genie/main.go:96`),
  `orch := orchestration.NewOrchestrator(reg, bus, policy, env)`
  (`cmd/genie/main.go:157`), then `bus.Publish(ctx, question)` and waits on a
  `bus.Subscribe("user", …)` handler. This is the original message-driven loop,
  intact.
- **`legacy/bus-architecture`** (tag `ec27f6b`) — the full pre-cutover tree,
  including the `cmd/api` that ran on the bus.

**Where the bus no longer runs:** `cmd/api`. Confirm it: `comm.NewInMemoryBus`,
`orchestration.NewOrchestrator`, `busio.Correlator`, and `EventTap` **do not
appear anywhere under `cmd/api/`** (Appendix A greps this). The registry there is
static inventory only (`cmd/api/main.go:156-159`, comment).

### 15.1 The bus core loop (as it runs in the CLI)

> **Agents never call each other. A request becomes a `protocol.Message`
> published to a bus; agents subscribe and emit follow-up messages.** Every
> agent's contract is `HandleMessage(ctx, msg, env) ([]Message, error)`
> (`pkg/agent/types.go`).

```
publish(finance_question) ─▶ orchestrator handler (per agent):
   1. ExtractTraceContext(metadata)
   2. span "agent.handle"
   3. governance.Composite.Evaluate     ◀── THE GATE (one bus chokepoint)
   4. agent.HandleMessage(...)
   5. success → Publish each output      error → record incident + route to fallback
```

- **Bus** (`pkg/comm/bus.go`): in-memory pub/sub, *goroutine-per-matching-handler-per-publish*,
  fire-and-forget. Explicit non-guarantees stated in source
  (`bus.go:52-54`): no backpressure, no ordering, no panic recovery.
- **Correlator** (`pkg/busio/correlator.go`): sync request/reply, `Await(traceID)`
  registers a buffered(1) channel *before* publish; a subscriber on the terminal
  recipient (`"user"`, `"mcp"`) matches the reply by `trace_id`. Timeout owned by
  the caller.

### 15.2 Fan-out / fan-in on the bus (the design the QA pipeline mirrors)

- **Fan-out:** the analyzer emits **four** identical `analysis_result` messages
  (`agents/analyzer/analyzer.go:104-107`) — to forecaster, anomaly, recommender,
  and the supervisor (which caches the snapshot). On the bus the three analysis
  branches run **concurrently**, so latency is `max(branch)`.
- **Fan-in:** the supervisor holds a per-`trace_id` `session` with four named
  slots (`agents/supervisor/supervisor.go:42-58`); `isReady()` is true only when
  all four are non-nil (`supervisor.go:50`), and `maybeFinalize` fires **once**
  then deletes the session (`supervisor.go:123`). Readiness is **structural**
  (presence of named fields), so a duplicate or out-of-order delivery cannot
  miscount — there is no numeric counter to get wrong.

The afg `QAService` (§8) reuses this exact supervisor session logic — it replays
the same `HandleMessage` sequence in-process — but runs the branches
**sequentially** for a data-race-free single-shot driver. The assembled bundle is
content-identical because the branches are pure functions of the analyzer output.

### 15.3 What the CLI path bounds (and doesn't)

The bus has **no cycle detection and no recursion-depth guard** — mutually
publishing agents would loop unboundedly, each hop spawning a fresh goroutine;
only the LLM wrappers bound LLM-backed work. Neither the bus nor the orchestrator
recovers panics; a panicking handler goroutine is unrecovered (unlike the new
inline edge, §9.4). These remain honest limitations of the CLI/legacy path.

---

## 16. Scaling and evolution: the seams

The production pipeline is single-process today. Scaling axes and where they'd
change:

| Axis | Today (reference impl) | Production seam | What breaks / must change |
|---|---|---|---|
| **Pipeline execution** | `afg.QAService` inline, sequential, single goroutine | Framework agents can be hosted out-of-process behind a broker or RPC | Sequential stages become network hops; you regain the bus's `max(branch)` parallelism the QA driver traded away |
| **Registry** | `registry.NewInMemory()` — static inventory/AIBOM/disclosures | `Registry` interface → shared store / service discovery | Inventory becomes dynamic; the QA pipeline's stage set is fixed in code and unaffected |
| **LLM** | Mock / Ollama, wrapped (`GuardMiddleware` on framework agents, `pkg/llm` chain on RAG/judge) | `Provider` interface → cloud providers coded (`anthropic.go`, `gemini.go`, `openai.go`) but unwired | Wire them in; add a cloud sibling to `NewGovernedOllama`; mind residency (`Region()`) |
| **Budget** | `InMemoryBudget` (per-process). Framework guard keys on **agent name**; legacy chain keys on `req.Residency.Region` | Shared ledger (Redis) keyed on a real principal | Fix the principal in whichever path you keep |
| **Key mgmt** | `EnvKeyResolver` (32-byte env KEK) | `KMSKeyResolver` + `KMSClient` (interface present) | Implement the KMS client; rotate KEK ids |
| **Warehouse** | none wired | `observability/bq.Sink` (interface + JSONL ref impl present) | Provide a BigQuery/Snowflake `Sink` and a call site |
| **Deferred bus features** | incident hooks / auditor-per-message / MCP bus-tools were the orchestrator's job | Reattach as framework middleware or a post-run hook | Tracked in `cmd/api/main.go:286-290` |

**Back-of-envelope for the single process:** the `/v1/ask` pipeline is a
bounded, sequential call chain — its cost is roughly `sum(stage costs)` per
request and it spawns no fan-out goroutines of its own (the framework `Guard`
goroutine per LLM call aside). Treat the current process as a
*correctness/reference* target and move stages behind a broker before
load-testing for scale.

---

## 17. System invariants → enforcing code

The table a reviewer should attack — each row is a property and the exact code
that enforces it. If the code doesn't back the claim, the claim is a bug.

| # | Invariant | Enforced by |
|---|---|---|
| I1 | No framework agent runs its model/tool without passing the composite policy | `pkg/afg/factory.go:59-78` (GovMiddleware before `next`) |
| I2 | Governance cannot be bypassed — every agent is built through one door | `pkg/afg/singledoor_test.go:18` (`TestSingleConstructionDoor`) |
| I3 | The first policy deny short-circuits with a single reason | `pkg/governance/policy.go:69-88` |
| I4 | A governance deny aborts the run and is **not** rescued by a fallback | `pkg/afg/qa_service.go:248-254`, `pkg/web/handlers/ask.go:112-116` |
| I5 | The `/v1/ask` report is byte-identical to the legacy bus pipeline | `pkg/afg/qa_service_test.go:3` (oracle parity) |
| I6 | A message class may not exceed its recipient ceiling | `pkg/governance/classification.go:35-42` |
| I7 | PII/secret classified data cannot leave the home region | `pkg/governance/sovereignty.go:40-46` |
| I8 | High-classification data is encrypted at rest with per-payload DEKs | `pkg/crypto/envelope.go:68-100` (documents, mcp_tokens) |
| I9 | JWT verification rejects non-HS256 and forged signatures | `pkg/auth/jwt.go:59-102` |
| I10 | The audit log is tamper-evident (hash chain) | `pkg/compliance/audit.go:77-105` |
| I11 | An LLM call cannot exceed the per-call deadline / repeated failures trip a breaker | `pkg/afg/llmguard.go:135-207` (framework); `pkg/llm/deadline.go`, `circuit.go` (RAG/judge) |
| I12 | A reasoning loop cannot iterate unbounded | `pkg/reasoning/reasoning.go:79-135` (maxSteps) |
| I13 | The published capability inventory is read live from the registry | `pkg/web/handlers/inventory.go` |
| I14 | The governance policy is adversarially tested in CI | `cmd/red-team/main.go` (`make red-team`) |
| I15 | Legacy fan-in fires exactly once per trace and cannot miscount (CLI path) | `agents/supervisor/supervisor.go:123` (session delete on finalise) |

**Invariants deliberately *not* claimed** (so no one can claim we implied them):
per-hop SSE progress on `/v1/ask`, automatic incident emission on deny/error in
the pipeline, delivery guarantees / ordering on the CLI bus, cycle prevention,
risk-class-based dispatch gating, and encryption of
`users`/`incidents`/`audit` tables.

---

## 18. Alternatives considered and rejected

| Alternative | Why rejected for this system |
|---|---|
| **Keep the message bus in the flagship API** | The framework gives a native agent model, tool loop, and provider ecosystem; keeping a bespoke bus alongside it duplicated the wiring. The cutover kept the bus's one non-negotiable — the governance gate — via the single construction door, and kept the bus itself as the CLI/legacy story. |
| **Reimplement each agent's logic natively in the framework** | Would risk silent drift from the legacy output embedded in the final report. Driving the real `HandleMessage` makes byte-parity structural (I5). |
| **Attach governance per-call at each call site** | Fragile — one forgotten call site is a silent bypass. `GovMiddleware` + the single-door test makes bypass a **compile/CI failure**, not a code-review hope. |
| **Run the fan-out branches concurrently in `QAService`** | Would reintroduce shared-state hazards around the legacy supervisor session for marginal latency gain on a deterministic path. The branches are pure functions of one input, so sequential execution is content-identical and race-free (§8.2). |
| **Rescue a policy deny with a fallback answer** | A denied message must not be silently answered. `DeniedError` is kept distinct from an execution error precisely so fallback logic can refuse it (§9.2, §12.2). |
| **A third-party agent framework with a hidden control loop** | Regulated-finance auditability needs the gate and the trace seam to be *ours* and readable; `pkg/afg` is thin glue over the framework, not a black box. |
| **Encrypt every table** | Rejected in favour of encrypting only sensitive payloads (documents, MCP tokens) and making the audit log tamper-evident via hashing. |

---

## 19. Known limitations

Named up front so they inform, not ambush:

1. **Deferred bus features not yet reattached.** Automatic incident emission on
   policy-deny / agent-error, the auditor's eval-on-every-message subscription,
   and MCP BusTool tools were the orchestrator's job and are **tracked follow-ups**
   (`cmd/api/main.go:286-290`), not shipped on the pipeline. `/v1/incidents`
   supports manual reporting only.
2. **Per-hop SSE progress is collapsed** to one event on `/v1/ask/stream` and
   `/chat/ws` because the pipeline is one inline call (§14).
3. **`/v1/ask` latency is `sum(stages)`**, not the bus's `max(branch)` — the QA
   driver runs the analysis trio sequentially (§8.2).
4. **The audit log is constructed but reserved** (`_ = auditLog`,
   `cmd/api/main.go:164`) — the hash-chain primitive is real and tested, but no
   pipeline call site writes to it yet.
5. **Config surface exceeds enforcement in two spots.** `SchemaPolicy` is never
   wired; `ClassificationPolicy.AllowedTo` and
   `sovereignty.allow_cross_border_for_public` are ignored by `BuildComposite`
   (§10.3).
6. **The autonomy onion is Ollama-only** on the legacy `pkg/llm` chain; the
   default `mock` provider is unwrapped (§11.2). Framework LLM agents always
   carry `GuardMiddleware`, but the QA pipeline stages are deterministic and use
   no LLM at all.
7. **Budget principals are stand-ins.** The framework guard keys the daily cap on
   the agent *name*; the legacy chain keys on `req.Residency.Region` — neither is
   a user id (§11.1–11.3).
8. **Embeddings bypass the wrapper chains** — unbudgeted, uncached, not
   circuit-broken (§11.4).
9. **Risk class is reporting-only**, not a dispatch gate.
10. **Rate-limit keying quirk.** For authenticated `/v1` routes the limiter runs
    before `mid.Auth` populates claims (`router.go:60-68`), so it keys on
    `RemoteAddr`, not the JWT subject.
11. **Legacy/CLI bus limitations persist** for `cmd/genie`: no delivery
    guarantees, no cycle guard, panics unrecovered (§15.3).
12. **Unwired seams:** `observability/bq` (no warehouse export), cloud LLM
    providers, `KMSKeyResolver`, and three specialist agent packages
    (`h_supervisor`, `moa_recommender`, `receipt_ocr`).
13. **Stale instrumentation scope names** hard-coded to `c2siorg/genie` vs the
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

# §3 / §15 the bus is GONE from cmd/api (expect: no results), and the QAService is wired
grep -rn 'NewInMemoryBus\|NewOrchestrator\|busio\|Correlator\|EventTap' cmd/api/   # expect: none
grep -n 'afg.NewQAService' cmd/api/main.go

# §3 the single construction door + its CI enforcement
sed -n '96,129p' pkg/afg/factory.go          # NewGovernedDeterministic / NewGovernedOllama
sed -n '18,71p'  pkg/afg/singledoor_test.go  # TestSingleConstructionDoor

# §3 / §10 the gate is per-stage middleware; deny short-circuits WITHOUT calling next
sed -n '59,91p' pkg/afg/factory.go           # GovMiddleware + DeniedError

# §8 the nine stages + oracle parity
sed -n '211,240p' pkg/afg/qa_service.go      # QAService struct + NewQAService (9 stages)
sed -n '272,324p' pkg/afg/qa_service.go      # Answer: sequential stage chain
head -5 pkg/afg/qa_service_test.go           # "oracle parity test"

# §7 the edge: gov message, WithGovMessage, QA.Answer, DeniedError→403
sed -n '82,123p' pkg/web/handlers/ask.go

# §11 framework guard order + legacy pkg/llm order (Circuit→Deadline→Budget→Cache→Cost→Ollama)
sed -n '130,207p' pkg/afg/llmguard.go
sed -n '84,89p'  cmd/api/llmstack.go
sed -n '58,58p'  cmd/api/llmstack.go         # mock stack is bare

# §14 SSE per-hop progress collapsed to one event
sed -n '18,27p'   pkg/web/handlers/ask_stream.go
sed -n '108,109p' pkg/web/handlers/ask_stream.go

# §15 the bus still powers the CLI, and the legacy branch/tag exist
grep -n 'NewInMemoryBus\|NewOrchestrator' cmd/genie/main.go
git log --oneline -1 ec27f6b
git branch -a | grep legacy/bus-architecture

# §5 / af-serve: the Postgres-free full-catalog governed edge
sed -n '39,53p' cmd/af-serve/main.go
grep -n 'func NewHandler' pkg/afg/httpedge.go

# §13 crypto AES-256-GCM, 32-byte KEK; HS256 JWT
grep -n 'AES-256-GCM' pkg/crypto/envelope.go
grep -n 'HS256' pkg/auth/jwt.go
```

If any of these disagree with the prose, the prose is wrong — fix it.

---

## Appendix B — package map

**Governed pipeline (the flagship runtime):** `pkg/afg` — `factory.go`
(GovMiddleware + the two construction doors), `llmguard.go` (GuardMiddleware),
`qa_service.go` (the 9-stage QAService), `httpedge.go` (`NewHandler`),
`catalog/` (the 58-agent governed catalog), `registry.go` (framework registry +
`RunWithFallback`).

**Protocol / governance:** `pkg/protocol` (Message, Classification), `pkg/agent`
(Agent/Environment interfaces, RiskClass), `pkg/governance` (10 policies),
`pkg/policy` (YAML → composite), `pkg/incidents` (FREE-AI incident form),
`pkg/sovereignty` (region).

**Legacy messaging core (CLI + legacy branch):** `pkg/comm` (bus), `pkg/busio`
(correlator), `pkg/orchestration` (dispatch), `pkg/registry` (agent map — used
as static inventory by `cmd/api`).

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
58 specialists registered in `cmd/api/main.go` as the inventory catalog; nine of
them are driven by the `afg.QAService` pipeline.

**Binaries:** `cmd/api` (flagship governed HTTP edge, Postgres), `cmd/af-serve`
(Postgres-free full-catalog governed edge), `cmd/genie` (CLI demo, still on the
bus), `cmd/demo`, `cmd/scaffold`, `cmd/red-team`.

---

## Where to go next

- [agents/README.md](agents/README.md) — the agent contract and how to add one
- [api.md](api.md) — every HTTP endpoint with sample curl
- [protocols.md](protocols.md) — message types and payload shapes
- [operations.md](operations.md) — running the stack and required env
- [free-ai-mapping.md](free-ai-mapping.md) — every FREE-AI recommendation → file path
