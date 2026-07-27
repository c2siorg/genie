# Goal: Phase 5 — cut Genie over to the Microsoft Agent Framework (single-day)

**Owner:** Claude Code · **Branch to work on:** new `feat/afg-cutover` off current HEAD
**Oracle (do NOT touch/delete):** `legacy/bus-architecture` @ `ec27f6b` (v1.0.0)
**Prime directive:** the governance gate stays unbypassable (the `singledoor`
invariant) and every doc number matches code (honesty rule). No claim of "done"
without a verifying command in this doc's Definition of Done.

---

## The one-sentence goal

Make `pkg/afg` the real, production-shaped Genie edge — behind real JWT/RBAC auth, with
the LLM wrapper chain, RAG, and the UI re-hosted onto it, served by a single binary — and
keep `go build ./...` + `go test -race ./...` green the whole way, with the offline
governance/BCP gates passing against the new stack.

---

## P0 — must finish and self-verify today (offline, mock provider)

### P0.1 — Real auth/RBAC in front of the afg edge
- **Now:** `pkg/afg/httpedge.go` reads identity from the **request body**
  (`UserID/Roles/Classification/Region`) — not production-safe.
- **Do:** wrap the afg chi router with the same middleware `cmd/api` uses
  (`pkg/web/mid`: RequestID, Recovery, Log, OTel, **JWT auth**, **RBAC**, RateLimit).
  Change `NewHandler` to read identity from **request context** (JWT claims injected by
  the middleware) via a small `identityFromContext(ctx)` helper; keep the body fields
  only as an explicit test/offline override guarded off by default.
- **Files:** `pkg/afg/httpedge.go`, new `pkg/afg/edge_auth.go`, `cmd/af-serve/main.go`.
- **Done when:** an unauthenticated `POST /v1/ask` returns 401; a valid-JWT low-role
  request to a gated agent returns 403 from RBAC; a valid privileged request succeeds —
  all shown by a new `httpedge_auth_test.go` (mock provider, no network).

### P0.2 — Re-host the LLM wrapper chain onto the framework path
- **Now:** `pkg/afg/factory.go` `NewGovernedOllama` builds a **bare** openai-go client —
  it bypasses `Circuit→Deadline→Budget→Cache→Cost` (which today only wraps the legacy
  `llm.Provider` in `cmd/api/llmstack.go`).
- **Do:** interpose the bounded chain on the framework path. Preferred: a framework
  `agent.Middleware` (`GovMiddleware` sits outermost; the bound-chain middleware sits
  just inside it, before the provider) that enforces deadline/budget/circuit/cache/cost
  semantics, reusing `pkg/llm` primitives. The gate must remain strictly outermost
  (`DisableFuncAutoCall` already handled).
- **Files:** `pkg/afg/factory.go`, new `pkg/afg/llmguard.go` (+ test).
- **Done when:** a test proves a runaway/slow provider is cut off by the deadline and a
  second identical call is served from cache — with the gate still evaluated first.

### P0.3 — Wire RAG/GraphRAG into the advisory agents
- **Now:** the 6 advisory catalog agents have no grounding; framework ships no RAG.
- **Do:** expose `pkg/rag` (+ `graphrag`) to advisory agents via framework
  `ContextProviders` or a retrieval `tool.Tool`, constructed through the single door so
  the gate still wraps them. Embeddings stay on `rag.Embedder` (unchanged).
- **Files:** `pkg/afg/factory.go` (a `NewGovernedAdvisory` variant), the 6 catalog files
  under `pkg/afg/catalog/` that are advisory, new `pkg/afg/rag_context.go` (+ test).
- **Done when:** a test shows an advisory agent receives retrieved context in its prompt
  path, and the retrieval call is itself governed (no ungated data egress).

### P0.4 — Re-point the UI/web `/v1/ask` handler off the bus onto the registry
- **Now:** `pkg/web/handlers/ask.go` publishes to the bus and awaits `busio.Correlator`.
- **Do:** add a registry-backed code path so the handler calls
  `afg.Registry.RunWithFallback(ctx, agent, input)` directly (no bus, no Correlator),
  selected by `GENIE_ENGINE=afg`. UI bundle unchanged; the bundle-contract tests in
  `pkg/web/handlers/ui_contract_test.go` must still pass.
- **Files:** `pkg/web/handlers/ask.go` (+ `ask_stream.go`, `chat_ws.go` for parity),
  `pkg/web/deps` wiring, `cmd/api/main.go`.
- **Done when:** with `GENIE_ENGINE=afg` the existing ask handler test passes against the
  registry path, and `ui_contract_test.go` is green.

### P0.5 — Replace the bus entirely: afg is the ONLY engine in `cmd/api`
- **Decision (locked):** rip the bus out of the production binary. `main` becomes the
  framework rewrite (migration doc §9). No `GENIE_ENGINE` flag — there is one engine.
- **Do:** delete the bus/orchestration wiring from `cmd/api/main.go` (the
  `comm.NewInMemoryBus`, `orchestration.*`, `busio.Correlator` setup and the per-agent
  bus subscribers) and boot the governed afg `Registry` + the P0.1 HTTP edge instead.
  Remove the now-dead bus imports. The web handlers keep only the registry path from
  P0.4 (drop the bus/Correlator branch, not gate it behind a flag).
- **PRECONDITION:** P0.1–P0.4 must be green first — once the bus is gone the afg path is
  production, so a gap becomes a shipped regression. Do NOT start P0.5 until they pass.
- **Preserved oracle:** the bus architecture survives only on `legacy/bus-architecture`
  (`ec27f6b`); parity diffing is now done by running THAT branch as a separate process,
  not a second engine in one binary.
- **Files:** `cmd/api/main.go`, `cmd/api/llmstack.go`, `pkg/web/handlers/ask*.go`,
  `pkg/web/handlers/chat_ws.go`, `pkg/web/deps`; delete/retire now-unused bus glue.
- **Done when:** `make run-api-mock` boots with **no bus code compiled in** (grep proves
  `comm.NewInMemoryBus`/`busio.Correlator` no longer referenced by `cmd/api` or the live
  web handlers), `/readyz` ready, `/v1/ai-inventory` returns **58**, `/v1/ask` allow+deny
  both behave, and `go build ./...` is clean with the dead bus glue removed or isolated.

### P0.6 — Whole-repo green + offline governance/BCP gates
- **Done when, run verbatim and pasted into the PR:**
  - `make vet` clean
  - `go build ./...` clean
  - `go test -race ./...` all pass (legacy + afg together)
  - `make red-team` passes against `config/ai-policy.example.yaml` (mock; FREE-AI Rec 20)
  - `make bcp-drill` shows the fallback firing (FREE-AI Rec 21)
  - `GENIE_ENGINE=afg make smoke` passes end-to-end against the mock stack

---

## P1 — stretch if P0 lands with time to spare

- **P1.1** Live Ollama tool-calling parity through `openaiprovider` (`OLLAMA_LIVE=1`).
- **P1.2** Framework-native fan-out for the analyzer→supervisor→reporter path via
  `orchestrate.go`, replacing the bus fan-in for the multi-agent question flow.
- **P1.3** AIBOM/`/v1/ai-inventory` field-by-field parity check afg vs legacy.

---

## Residual — explicitly NOT closeable by me today (hand to human / next session)

These need a live model, a running stack over time, or regulatory judgement — I will
**not** mark them done:
- **Byte-faithful fidelity** of the 6 advisory + 9 pipeline agents vs the legacy oracle
  (currently *representative*). Requires diffing live outputs.
- **Full behavioural red-team + e2e diff** vs `legacy/bus-architecture` under a real
  Ollama, across denials/fallbacks/edge cases.
- **Production cutover sign-off** (flip default `GENIE_ENGINE`, retire the bus, delete
  the legacy branch) — a human/regulatory decision, not a code change.

---

## Execution order (dependency-correct — P0.5 is now LAST, and destructive)

P0.2 → P0.1 → P0.3 → P0.4 → **P0.5 (rip out the bus)** → P0.6. The bus deletion is moved
to the end because it is irreversible-in-`main` and requires every re-host (auth,
wrapper, RAG, UI) already green — otherwise `main` ships a regression.

## Guardrails
- Work on `feat/afg-cutover`; never commit to `main` or `legacy/bus-architecture`.
- **No safety default anymore:** the bus is being removed, not flagged off. That makes
  P0.5 a point of no return within this branch — confirm P0.1–P0.4 DoD all pass first.
- Before deleting bus glue, confirm `legacy/bus-architecture` @ `ec27f6b` still resolves
  (`git rev-parse legacy/bus-architecture`) so the oracle is safe outside this branch.
- After each P0 item: `go build ./... && go test -race ./...` before moving on.
- Update `CLAUDE.md`'s afg section + `docs/migration-agent-framework.md` Phase 5 row to
  reflect real state — numbers grep-verified, no overstatement.
- Open one PR at the end with the DoD command outputs pasted in the body.
```
