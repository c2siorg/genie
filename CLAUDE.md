# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Genie is an AI financial assistant in Go, built on Microsoft's Multi-Agent Reference
Architecture (MARA) and aligned with the RBI FREE-AI report (Aug 2025). It ships 60+
specialist finance agents behind a message bus, an HTTP/WebSocket API, and a full GenAI
layer (RAG, reasoning, memory, eval, safety). Default LLM is on-prem Ollama; a `mock`
provider makes everything run with no external dependencies.

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
  → Orchestrator.dispatch(msg)
      → governance.Composite.Evaluate(msg)   ← THE GATE, deny-on-first-failure
      → registry.Get(msg.To); enforce risk ceiling
      → agent.HandleMessage(ctx, msg, env) → returns 0..N follow-up messages
      → bus.Publish each output → loops back to dispatch
  → reporter emits final_report
  → busio.Correlator wakes the waiting HTTP handler (matched by correlation_id)
  → HTTP response
```

Fan-out/fan-in: an agent (e.g. `analyzer`) emits parallel messages; the `supervisor`
counts fan-ins by `trace_id`/`correlation_id` and fires the `reporter` when all arrive.
Latency = max(stages), not sum.

### The seven load-bearing packages
- `pkg/protocol` — `Message{ID, From, To, Role, Type, Content, CreatedAt, Metadata}`. `Type` carries routing semantics; `Metadata` carries `trace_id`/`traceparent`, `user_id`, `user_roles`, `classification` (public|internal|pii|secret), `region`, `correlation_id`, and domain payloads. `pkg/agent` re-exports these as type aliases to avoid import cycles.
- `pkg/registry` — `NewInMemory()`; drives the live `GET /v1/ai-inventory`.
- `pkg/comm` — `NewInMemoryBus()`; fire-and-forget pub/sub with OTel spans. Sync request/response is layered on top via `pkg/busio.Correlator`.
- `pkg/orchestration` — the ~300-line dispatch loop: extract trace → evaluate policy → look up handler → enforce risk ceiling → invoke → publish outputs → record incidents → fire fallback on high-risk failure.
- `pkg/governance` — composite policy chain (length, required-metadata, RBAC, classification ceiling, data-residency, consent, explainability, PII regex, prompt-injection, JSON-schema). Loaded from `config/ai-policy.example.yaml` — risk team edits YAML, system obeys.
- `pkg/agent/risk.go` — `RiskClass` (low|medium|high). `RiskHigh` agents (AML, VaR, KYC, payments) can't run on a message lacking `advisor`/`admin` in `metadata.user_roles`.
- `agents/fallback` — deterministic (no LLM/network) degraded answers when a primary times out, circuit-breaks, or panics.

### LLM provider wrapper chain
`cmd/api/llmstack.go` wraps the base provider (Mock or Ollama) in layers that bound
autonomous reasoning: `Cached` → `Budgeted` (daily per-principal token cap) →
`Deadline` (per-call timeout) → `Circuit` (breaker after N errors). A ReAct/Reflexion
loop (`pkg/reasoning`) that runs away gets cut off by these wrappers, not by the agent.

## Where things live

- `cmd/` — `api` (the HTTP service edge), `genie` (CLI demo), `demo`, `scaffold`, `red-team`.
- `agents/<id>/<id>.go` — one package per agent; `New()` constructor, exported `ID`/`Capability`/`Type*` constants, `HandleMessage`, optional `RiskLevel()`. All agents are wired into the registry in `cmd/api/main.go` (`run()`), which is the source of truth for what's live.
- `pkg/` — platform packages (see above) plus `llm`, `rag`, `graphrag`, `reasoning`, `memory`, `eval`, `safety`, `privacy`, `crypto`, `auth`, `identity`, `mcp`, `a2a`, `compliance`, `storage/postgres`, `web` (chi router + handlers + middleware in `web/mid`), etc.
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
