# LangGraph concepts → Genie equivalents

A glossary of **every LangGraph concept**, each mapped to how the same idea shows up
**here in Genie** (which is *not* built on LangGraph — it's MARA + a message bus in Go,
so several LangGraph primitives have no direct equivalent, and this says so honestly).

For Genie's own vocabulary, see **[glossary.md](glossary.md)**. LangGraph definitions
are distilled from the official docs (`docs.langchain.com/oss/python/langgraph`).

> Why both? Genie's flow is *implicit* — there is no graph object; the "graph" is the
> set of message `Type`s routed across the bus by the orchestrator. So most LangGraph
> mappings are conceptual, not one-to-one.

---

## 1 · Graph model

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **StateGraph / CompiledStateGraph** | The graph class you define state, nodes, edges on, then `.compile()`. | No graph object. The graph is implicit: message `Type`s routed by the orchestrator (`pkg/orchestration`) over the bus (`pkg/comm`). |
| **State schema** | Typed dict/Pydantic model that is the graph's snapshot. | `protocol.Message{Content, Metadata}` plus the supervisor's per-`trace_id` session that accumulates named fields. |
| **Channels** | State keys nodes read/write, each with its own reducer. | The named fields the `supervisor` collects into a session; there is no per-key channel machinery. |
| **Reducers** | `(old, new) -> merged` functions that combine node updates. | The `supervisor` fan-in: it merges each agent's named output into the session until structurally ready. |
| **`add_messages` / `MessagesState`** | Prebuilt reducer/state that appends messages by id. | — no direct type; appending agent outputs into the session is the analog. |
| **Nodes** | Functions `state -> state-update` encoding agent logic. | **Agents** — `agents/<id>`, `HandleMessage(ctx, msg, env)`. |
| **START / END** | Virtual entry and terminal nodes. | `START` ≈ `POST /v1/ask`; `END` ≈ the `reporter` emitting `final_report` to `"user"`. |
| **Normal edges** (`add_edge`) | Fixed transition from one node to the next. | An agent emitting a message with the next `Type` — routing is *data*, not a declared edge. |
| **Entry / finish points** | First/last nodes, via START/END edges. | The web edge (entry) and `reporter` (finish). |
| **Graph compilation** (`.compile()`) | Validate structure + attach runtime (checkpointer, breakpoints). | `orchestrator.Start(ctx)` — subscribes one handler per registered agent to the bus. |

---

## 2 · Dynamic control flow

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **Conditional edges** | A function that picks the next node from state. | Agent logic choosing which `Type` to emit; the **governance composite** is a hard conditional gate — a deny short-circuits before the agent runs. |
| **`Send`** | Route custom per-instance state to a node — the map-reduce primitive (fan-out). | `analyzer` emits parallel messages; `supervisor` fans them in. Latency = max(stages), not sum. |
| **`Command`** | Combine a state `update` with routing (`goto`) in one return. | An agent returning `0..N` follow-up messages — payload **and** routing in one, the closest analog. |
| **`goto`** | The `Command` field naming the next node(s). | The outgoing message's `To` / `Type`. |
| **Runtime / context** (`context_schema`) | Data passed to nodes that isn't graph state. | The `env` envelope passed to `HandleMessage`, plus `GENIE_*` config. |

---

## 3 · Execution model

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **Super-step** | One iteration over active nodes (Pregel BSP). | One bus hop: deliver → `HandleMessage` → publish. Genie is async pub/sub, **not** literally Pregel/BSP. |
| **Pregel / BSP** | The bulk-synchronous engine under `CompiledStateGraph`. | — no synchronous barrier engine; the bus + `busio.Correlator` provide async delivery + sync request/response. |
| **`recursion_limit`** | Max super-steps before `GraphRecursionError` (default 1000). | ReAct `maxSteps` cap (`pkg/reasoning`) + the Ollama-path wrappers (**Circuit → Deadline → Budget**) that cut off runaway loops. |
| **`CachePolicy`** | Node-level result caching (key fn + TTL). | The LLM **Cache** wrapper (`cmd/api/llmstack.go`) — response-level, not per-node (`GENIE_LLM_CACHE_TTL`). |
| **`RetryPolicy`** | Per-node retry on failure. | — no per-node retries; the **Circuit** breaker + **Deadline** wrappers bound failures instead. |
| **Deferred nodes** (`defer=True`) | Wait until all pending paths finish before running (fan-in). | The `supervisor`'s structural-readiness gate — fires the `reporter` only once every expected field is present. |

---

## 4 · Persistence & memory

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **Checkpointer** | Saves a state snapshot at every super-step for a thread. | `pkg/storage/postgres` persists domain state; genie does **not** snapshot per-hop graph state. (The `harness-engineering-pratice-golang` sibling's `durable.Store` is the closer checkpointer analog.) |
| **Checkpoint** | A single saved snapshot. | — no per-hop snapshot; persisted rows are domain records, not graph state. |
| **Thread / `thread_id`** | A scoped conversation/session. | `trace_id` is the correlation/session key (`pkg/busio`). |
| **`checkpoint_id` / `checkpoint_ns`** | Ids addressing a specific snapshot/namespace. | — no equivalent. |
| **`StateSnapshot` / `get_state` / `get_state_history`** | Read current/historical state of a thread. | — no equivalent (genie has no time-indexed graph state). |
| **`update_state`** | Manually write into a thread's state. | — no equivalent. |
| **Replay / time travel / fork** | Re-run from, or branch off, a past checkpoint. | — no equivalent. |
| **Pending writes** | Partial writes staged within a super-step. | — no equivalent. |
| **Backends** (`InMemorySaver`, `SqliteSaver`, `PostgresSaver`) | Where checkpoints live. | Postgres (`pkg/storage/postgres`); in-memory/mock for tests. |
| **`Store` / `BaseStore`** (long-term, cross-thread) | Key-value memory shared across threads. | `pkg/memory` (thread + knowledge stores) + RAG retrieval (`pkg/rag`, `rag/pgvector`, `pkg/graphrag`). |
| **Short-term vs long-term memory** | Thread-scoped vs cross-thread persistence. | Short-term = the `trace_id` session; long-term = `pkg/memory` + the vector/graph stores. |
| **Namespaces** | Hierarchical keys in the store. | Memory namespacing in `pkg/memory` (roughly). |
| **Durability modes** (`exit` / `async` / `sync`) | When checkpoints are flushed. | — no equivalent. |

---

## 5 · Human-in-the-loop

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **`interrupt()`** | Pause mid-node and surface a value for a human. | — genie has **no in-flow HITL**. The sibling harness repo's `internal/hitl` (`RequestApproval`/`Resume`) is the analog. |
| **`Command(resume=…)`** | Resume an interrupted run with the human's input. | — no equivalent (see `internal/hitl.Resume` in the sibling). |
| **Static breakpoints** (`interrupt_before/after`) | Pause before/after named nodes. | — no equivalent. |
| **`fallback` (Genie's HITL-adjacent idea)** | — | Deterministic "a human will follow up" notice published when a primary agent errors (`agents/fallback`). Out-of-band, not an interrupt. |

---

## 6 · Streaming

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **`stream_mode`** | How `.stream()` yields output. | The SSE edge `POST /v1/ask/stream`. |
| **`values`** | Full state after each step. | — genie streams events, not full state. |
| **`updates`** | State deltas per node. | `event: agent.handle` frames (each agent's contribution). |
| **`messages`** | Token/message stream. | `event: report` (the assembled answer) + `event: ai_disclosure`. |
| **`custom` / `debug`** | Arbitrary user data / debug events. | — bus events are already emitted as OTel spans (`pkg/observability`) rather than a custom stream mode. |

---

## 7 · Subgraphs & multi-agent

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **Subgraph** | A graph used as a node in another graph. | The pipeline chain (`afg.PipelineSpecs`: ingestor → normalizer → enricher → analyzer → … → reporter) — a composed sub-flow. |
| **Supervisor** (multi-agent) | A router agent that delegates to workers. | `agents/supervisor`, and `agents/h_supervisor` (hierarchical). |
| **Swarm / handoffs** | Peer agents hand control to each other. | `Type`-based routing over the bus; `moa_recommender` (mixture-of-agents) blends specialist outputs. |

---

## 8 · Prebuilt & Functional API

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **`create_react_agent`** | Prebuilt ReAct (reason+act) agent. | `pkg/reasoning` — a ReAct loop (capped at `maxSteps`) + Reflexion. |
| **`ToolNode` / `tools_condition`** | Node that runs tools / edge that routes to them. | Tool-calling via `pkg/mcp` (Model Context Protocol) and `pkg/toolkit` (agent-skill registry). |
| **Functional API** (`@entrypoint`, `@task`) | Checkpointed workflows without an explicit graph. | — no equivalent; genie is message/handler-based end to end. |

---

## 9 · Platform & deployment

| LangGraph | What it is | In Genie (here) |
|---|---|---|
| **LangGraph Platform / Server** | Hosted runtime + task queue for graphs. | `cmd/api` (the HTTP/WS service) and the framework-native `cmd/af-serve` edge; `docker compose` / `make up` for the stack. |
| **LangGraph Studio** | Visual graph debugger. | — no equivalent; Grafana + OTel traces (`pkg/observability`) are the runtime-inspection surface. |
| **Assistants / Cron / Webhooks / double-texting** | Managed config versions, schedules, callbacks. | — no equivalent. |
| **LangGraph CLI** | Scaffold/run/deploy graphs. | `make scaffold` generates a new agent + test; `make run-api` / `run-cli` run locally. |

---

*Sources: LangGraph docs — [Graph API](https://docs.langchain.com/oss/python/langgraph/graph-api),
[Persistence](https://docs.langchain.com/oss/python/langgraph/persistence),
[Types reference](https://reference.langchain.com/python/langgraph/types/Send).
Genie mappings are grounded in this repo; verify identifiers with grep before relying on them.*
