# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Genie is an AI financial assistant in Go, built on Microsoft's Multi-Agent Reference
Architecture (MARA) and aligned with the RBI FREE-AI report (Aug 2025). It implements
60+ specialist finance agents (61 specialist packages under `agents/`, plus a fallback
package), of which 58 — plus 2 deterministic fallbacks — are currently wired into the
running API; all sit behind a
message bus, an HTTP/WebSocket API, and a full GenAI layer (RAG, reasoning, memory,
eval, safety). Default LLM is on-prem Ollama; a `mock` provider makes everything run
with no external dependencies.

**Module path ≠ directory name.** The repo directory is `genie` but the Go module is
`github.com/PratikDhanave/multi-agent-reference-architecture-go`. All imports use the
module path, never "genie".

**This file governs the Go project only.** Two untracked sibling subprojects live beside
it and are NOT part of this Go module — each has its own CLAUDE.md, Makefile, and toolchain.
Don't run their commands here (or these here): `geniepython/` is the Python port of the same
product (uv / ruff / pytest, FastAPI, Azure AI Foundry-only — no mock provider; `make check`,
`make run`), and `microsoftagentframeworklearning/` is a standalone 74-lesson learning
workspace unrelated to the product. When a request is about the Python port or the lessons,
`cd` into that directory and follow its CLAUDE.md; otherwise stay at the repo root.

## Commands

```bash
make build          # compile every binary under cmd/ into bin/
make test           # go test -race ./...   (the CI gate; also runs go vet)
make vet            # go vet ./...
make tidy           # go mod tidy

# run a single test / package
go test -race ./agents/supervisor/...
go test -race -run TestName ./pkg/governance/

make run-cli        # CLI demo — no HTTP, no Postgres, no LLM needed
make run-api-mock   # HTTP API with the mock LLM (no Ollama needed); still needs Postgres + JWT/KEK env
make run-api        # HTTP API with Ollama (default GENIE_LLM=ollama)

make up             # docker compose: build, boot, wait for /readyz, print URLs (UI :8080, Grafana :3000)
make down           # tear the local stack down (-v)
make smoke          # end-to-end curl script against a running stack (signup → upload → /ask → disclosures)
make e2e            # go test -tags=e2e ./tests/sim/... against a running stack

make scaffold name=<id> cap=<capability> in=<intype> out=<outtype> next=<agent>  # generate a new agent + test
make red-team       # run adversarial probe corpus against config/ai-policy.example.yaml (FREE-AI Rec 20)
make bcp-drill      # force portfolio_advisor failure to verify fallback fires (FREE-AI Rec 21)
make openapi-validate  # validate docs/openapi.yaml against the OpenAPI schema (needs npx swagger-cli)
make ui             # rebuild the Next.js console (web-next/) and refresh the embedded export (needs Node)
```

CI (`.circleci/config.yml`) runs `go vet ./...` then `go test -race ./...` on Go 1.25,
then a docker build on `main`. Match that locally with `make vet && make test`.

## Architecture — the core loop

A request becomes a **message**, not a function call. This is the load-bearing decision:
agents never call each other directly. Everything flows through the bus, which gives one
seam each for governance, tracing, fallbacks, and the live capability inventory.

```
HTTP POST /v1/ask + JWT
  → chi router + middleware (RequestID, Recovery, Log, OTel, JWT auth, RBAC, rate-limit)
  → Orchestrator (per-agent bus subscriber, set up in Start(ctx))
      → governance.Composite.Evaluate(msg)   ← THE GATE, deny-on-first-failure
      → agent.HandleMessage(ctx, msg, env) → returns 0..N follow-up messages
      → bus.Publish each output → delivered to the next agent's subscriber
      → on agent error: record incident + route to a registered fallback
  → reporter emits final_report → to "user"
  → busio.Correlator wakes the waiting HTTP handler (matched by trace_id)
  → HTTP response
```

Fan-out/fan-in: an agent (e.g. `analyzer`) emits parallel messages; the `supervisor`
collects them into a per-`trace_id` session and fires the `reporter` once all the
expected named fields are present (structural readiness, not a numeric count).
Latency = max(stages), not sum. See `docs/architecture.md` for the verified deep dive.

### The seven load-bearing packages
- `pkg/protocol` — `Message{ID, From, To, Role, Type, Content, CreatedAt, Metadata}`. `Type` carries routing semantics (a plain string; no `MessageType` constants); `Metadata` carries `trace_id`/`traceparent`, `user_id`, `user_roles`, `classification` (public|internal|pii|secret), `region`, and domain payloads. (There is no `correlation_id` — sync correlation is keyed on `trace_id`.) `pkg/agent` re-exports `Message`/`MessageRole` as type aliases to avoid import cycles.
- `pkg/registry` — `NewInMemory()`; drives the live `GET /v1/ai-inventory`.
- `pkg/comm` — `NewInMemoryBus()`; fire-and-forget pub/sub with OTel spans. Sync request/response is layered on top via `pkg/busio.Correlator`.
- `pkg/orchestration` — the orchestrator subscribes one handler per registered agent to the bus at `Start(ctx)`; each handler does: extract trace → evaluate policy → `HandleMessage` → publish outputs (on success) or record incident + route to a registered fallback (on returned error). Note: it does not call `registry.Get` per message, does not enforce a risk ceiling, and does not recover panics.
- `pkg/governance` — composite policy chain, deny-on-first-failure (length, required-metadata, RBAC-by-message-Type, classification ceiling, data-residency, consent, explainability, PII regex, prompt-injection). A `SchemaPolicy` exists but is not wired by the YAML loader. Assembled by `pkg/policy` from `config/ai-policy.example.yaml` — risk team edits YAML, system obeys.
- `pkg/agent/risk.go` — `RiskClass` (low|medium|high); `RiskOf` defaults to `low`. Risk class is read by reporting surfaces (`/v1/ai-inventory`, AIBOM, disclosures), **not** by the dispatcher. Role gating on sensitive actions is done by `RBACPolicy` keyed on message `Type` (the shipped YAML gates `finance_question`/`portfolio_request` to `user`/`advisor`/`admin`).
- `agents/fallback` — deterministic (no LLM/network) "a human will follow up" notice, published when a primary agent returns an error (not on panic). Only `portfolio_advisor` and `recommender` have fallbacks wired.

### LLM provider wrapper chain
`cmd/api/llmstack.go` wraps the base provider in layers that bound autonomous
reasoning. On the **Ollama** path the real nesting (outermost→innermost, i.e. call
order) is `Circuit → Deadline → Budget → Cache → Cost → Ollama`. The default `mock`
provider is **unwrapped** (bare). A ReAct/Reflexion loop (`pkg/reasoning`, ReAct
capped at `maxSteps`) that runs away is cut off by the Ollama-path wrappers, not by
the agent. Embeddings use a separate `rag.Embedder` and bypass this chain.

## The Microsoft Agent Framework port (`pkg/afg`)

Genie now runs on `pkg/afg/`, built on `github.com/microsoft/agent-framework-go`
(pinned to a pseudo-version in `go.mod` — the framework is public-preview with no
releases; do not `go get -u` it). **The message bus is gone from the production edge:**
`cmd/api` no longer constructs `comm.Bus` / `orchestration` / `busio` (Correlator /
EventTap); `/v1/ask`, `/ask/stream`, and `/chat/ws` run the governed agent-framework
pipeline (`afg.QAService`) inline. The `registry.NewInMemory()` there survives only as
the AI-inventory / AIBOM / disclosures source (a static list, not a runtime). Full
design + phase log: `docs/migration-agent-framework.md`, `docs/phase5-cutover-goal.md`,
`session.md`. The pre-cutover bus architecture is preserved as the behavioural oracle on
branch `legacy/bus-architecture` (`ec27f6b`) — not in `main`.

**The flagship pipeline reuses, not reimplements, the legacy domain logic.** `afg.QAService`
(`pkg/afg/qa_service.go`) reproduces the supervisor question-flow (ingestor→normalizer→
enricher→analyzer→forecaster/anomaly/recommender→reporter) by driving the **real
`agents/<id>.HandleMessage`** inside governed framework stages. So the report is
**byte-identical** to the legacy pipeline (oracle parity test drives the actual legacy
agents — `pkg/afg/qa_service_test.go`), and the `agents/` packages remain live
dependencies (as libraries), not dead code. What was deleted is the bus/orchestrator
*runtime*, not the domain logic.

**Why it exists / the core shift.** Genie's governance is a *single bus chokepoint*;
the framework only offers *per-agent middleware* (`agent.Middleware` attached via
`Config.Middlewares`). A per-agent seam is bypassable by construction, so the port's
whole difficulty is re-proving the single-gate property. It does so three ways, and
these are the load-bearing invariants — preserve them in any afg change:
1. **One construction door.** Every agent is built by a `NewGoverned*` factory
   (`pkg/afg/factory.go`) that injects the governance middleware. No agent is built
   from a raw provider constructor.
2. **A front-door / node-level gate.** The governance `Composite.Evaluate` (from the
   same `pkg/policy` YAML loader — governance stays *data*, not code) runs before the
   provider. A governance `DeniedError` is a policy rejection and is **not** rescued by
   a fallback; only execution errors route to `RunWithFallback`.
3. **A registry-invariant test** (`pkg/afg/singledoor_test.go`) asserts every
   registered agent's middleware chain contains the gate — CI fails if any agent was
   constructed by another path. This is the afg analogue of `tests/agents_registry/`.

**Phase status** (all ✅ rows compile + `go test -race ./pkg/afg/...` passes):

| Phase | Status | Where |
|---|---|---|
| 0 — framework spike, go/no-go | ✅ GO | pinned deps in `go.mod` |
| 1 — vertical slice + single-door gate | ✅ | `factory.go`, `currency.go`, `singledoor_test.go`, `cmd/af-hello` |
| 2 — concurrent orchestration (fan-out) | ✅ | `orchestrate.go` (`NewConcurrentWorkflowBuilder`) |
| 3 — registry / inventory / fallback | ✅ | `registry.go` (`Inventory()`, `RunWithFallback`) |
| 4 — all **58** agents ported | ✅ | `pkg/afg/catalog/` (39 deterministic + 6 advisory) + 4 hand-ported + 9 pipeline (`pipeline.go`) |
| HTTP edge (`/v1/ask`, `/v1/ai-inventory`) | ✅ | `httpedge.go` + `cmd/af-serve` — **no Postgres, no bus** |
| 5 — parity + **cutover** | ✅ | bus removed from `cmd/api`; `/v1/ask` on `QAService`; full repo `go test -race ./...` green |
| auth | ✅ | `httpedge.go` + `cmd/af-serve` behind real `mid.Auth` JWT/RBAC; identity from claims |
| LLM wrapper chain | ✅ | `llmguard.go` `GuardMiddleware` (Circuit/Deadline/Budget/Cache), wired into `NewGovernedOllama` inside the gate |
| RAG grounding | 🟡 component | `rag_context.go` `NewGovernedAdvisory` + `RetrievalMiddleware` (grounds *inside* the gate); built & tested, per-agent corpus wiring pending |

**Deferred with the bus removal** (documented in `cmd/api/main.go`, tracked — not silently dropped):
- **Incident-on-deny / on-agent-error hooks** and the **auditor's eval-on-every-message**
  subscription (were the orchestrator's / bus's job).
- **MCP bus-tools** (`explain_finance`/`macro_context`/`rate_outlook`).
- **Per-hop SSE streaming**: `/ask/stream` + `/chat/ws` now emit one `pipeline_running`
  progress event then the final `report`, not per-agent events (cosmetic for a
  deterministic pipeline; event vocabulary preserved for the console).
- **BCP fallback for the QA pipeline**: `afg.Registry.RunWithFallback` + fallbacks exist,
  but the `QAService` stages don't yet route to fallbacks. (`make bcp-drill` still tests
  `cmd/genie`, which is untouched; note it fails pre-existing on an OTel schema conflict
  unrelated to this work.)
- **Not yet run against the new stack:** `make smoke`/`e2e` (need a live Postgres + stack).

## Where things live

- `cmd/` — `api` (the production HTTP service edge — now afg-backed: bus removed, `/v1/ask` runs `afg.QAService`), `genie` (CLI demo — still the legacy bus wiring), `demo`, `scaffold`, `red-team`; plus `af-hello`/`af-serve` (Postgres-free agent-framework demos; `af-serve` shares the same governed `afg.NewHandler` edge as `cmd/api`).
- `agents/<id>/<id>.go` — one package per agent; `New()` constructor, exported `ID`/`Capability`/`Type*` constants, `HandleMessage`, optional `RiskLevel()`. Live agents are wired into the registry in `cmd/api/main.go` (`run()`), which is the source of truth for what's actually served — note that not every agent package under `agents/` is wired in (currently 58 specialists + 2 fallbacks of 61 specialist packages).
- `pkg/` — platform packages (see above) plus `llm`, `rag`, `graphrag`, `reasoning`, `memory`, `eval`, `safety`, `privacy`, `crypto`, `auth`, `identity`, `mcp`, `a2a`, `compliance`, `storage/postgres`, `web` (chi router + handlers + middleware in `web/mid`), etc. `pkg/afg` is the separate agent-framework port (see its section above), not part of the bus architecture.
- **`web-next/`** — the browser console: a **Next.js** app (App Router, TypeScript) that is **statically exported** and committed into `pkg/web/handlers/ui/`, which `ui.go` embeds via `//go:embed all:ui` (the `all:` is required — the export's `_next/` dir would otherwise be skipped). `go build` needs **no Node** (it embeds the committed export); regenerate the export with `make ui` after changing the UI, then commit `pkg/web/handlers/ui/`. The console is asserted against the Go handlers by the bundle-contract tests in `pkg/web/handlers/ui_contract_test.go` (API paths / auth fields / classification / SSE events / storage keys must survive in the compiled bundle).
- `config/` — `ai-policy.example.yaml` (the board-approved governance policy) and `constitution.yaml` (LLM-as-judge rules).
- `docs/` — deep reference: `architecture.md`, `operations.md`, `api.md`, `protocols.md`, `free-ai-mapping.md` (every FREE-AI recommendation → file path), `agents/<id>.md`, `packages/<name>.md`, plus `openapi.yaml` / `asyncapi.yaml`.
- `tests/` — `agents_registry/` (enforces agent ID uniqueness), `sim/` (e2e user simulation, `-tags=e2e`), `integration_test.go`.

## Conventions

- **Adding an agent:** `make scaffold ...` generates `agents/<id>/<id>.go` + a passing test. Then: implement `HandleMessage`, declare a `RiskLevel()`, give every output a disclaimer (high-risk rejects need an incident payload), register it in `cmd/api/main.go`, and add a paired `docs/agents/<id>.md` following the 13-section template — the doc is treated as part of the contract. `tests/agents_registry/` will fail on a duplicate ID.
- **Governance is data, not code.** New cross-cutting rules go into the composite policy (config YAML / `pkg/governance`), applied to every message — not scattered into agents.
- **Message-driven, always.** Don't wire agent-to-agent Go method calls; emit a message with the right `Type` and let the bus route it. That preserves the single audit/trace/fallback/inventory seams.
- Finance domain uses Indian English + regulator terms (CRR, IDV, NPA, GSTIN, PAN, IFSC); ₹ amounts are rupees (₹1L = 100,000, ₹1cr = 10,000,000). Money is generally handled in integer cents/paise (`*_cents` fields).
- Docs code blocks are grep-able against real files; keep a doc in sync when you change the behaviour it describes.

## Running `cmd/api` locally (required env)

`GENIE_JWT_SECRET`, `GENIE_KEK_BASE64` (base64 32-byte key), and `GENIE_DB_DSN` are
required. Others default sanely: `GENIE_HTTP_ADDR=:8080`, `GENIE_LLM=mock`,
`GENIE_AI_POLICY=config/ai-policy.example.yaml`, plus `GENIE_LLM_{BUDGET,CACHE_TTL,TIMEOUT,CIRCUIT}`
and `GENIE_OLLAMA_{URL,CHAT,EMBED}`. OTLP export turns on when `OTEL_EXPORTER_OTLP_ENDPOINT`
is set (else stdout). `docker compose up` / `make up` provisions Postgres and the observability
stack for you.
