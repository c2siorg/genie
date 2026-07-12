# Genie — Claude Code Session Log

**Date:** 2026-07-11
**Repo:** `github.com/PratikDhanave/genie` (Go module `github.com/PratikDhanave/multi-agent-reference-architecture-go`)
**Branch:** `main`
**Model:** Opus 4.8 (1M context)
**Modes:** explanatory + learning output style; later `/effort` → ultracode (xhigh + dynamic workflow orchestration)

This file is a faithful record of the full session: the `/init` audit, the discussion
about rewriting Genie onto the Microsoft Agent Framework for Go, the research findings,
the decisions taken, and the design artifact produced.

---

## Part 1 — `/init` (audit the existing CLAUDE.md)

**User:** `init`

**What I did:** rather than regenerate the already-thorough `genie/CLAUDE.md`, I verified
its load-bearing claims against the live code and looked for drift/gaps.

### Verification (all claims still exact)

| Claim in CLAUDE.md | Ground truth checked | Result |
|---|---|---|
| 61 specialist packages under `agents/` | 62 dirs − `fallback` = 61 | ✓ |
| 58 specialists + 2 fallbacks wired | 60 `register()` calls − 2 `fallback.NewFor` = 58 + 2 | ✓ |
| Module ≠ dir (`…/multi-agent-reference-architecture-go`) | matches `go.mod` | ✓ |
| Registration lives in `cmd/api/main.go run()` | `register := func(a agent.Agent)` at `main.go:182` | ✓ |
| Only `portfolio_advisor` + `recommender` have fallbacks | `main.go:208-209` | ✓ |
| Go 1.25 | `go 1.25.0` | ✓ |

Consistent with the `honesty-in-repo-claims` memory: every doc number matched code by grep.

### Verdict
The existing `CLAUDE.md` is in excellent shape and does **not** need a rewrite. A
regeneration would likely *lose* hard-won specifics (e.g. "no `correlation_id`", "does not
recover panics", "mock provider is unwrapped").

### The only real gaps found (small, verified)
1. `make openapi-validate` is a real Makefile target but undocumented in the command list.
2. The browser UI's location (`pkg/web/handlers/ui/index.html`) isn't stated.
3. `deploy/local/` and `data/sample.csv` (the `make smoke` upload fixture) aren't mapped.

**Recommendation:** don't regenerate; apply those three surgical additions only. (User did
not act on this before pivoting to the migration topic.)

---

## Part 2 — "Rewrite this project using microsoft/agent-framework-go"

**User:** *"claude code advisor we have to rewrite this project using
https://github.com/microsoft/agent-framework-go"*

I treated this as an advisory/architecture task and researched the framework before advising
(no answering from memory on an external framework).

### Framework facts (verified via GitHub + Microsoft Learn)
- **Module:** `github.com/microsoft/agent-framework-go`; **MIT** license.
- **Status:** *public preview*, **no GitHub releases published**. RAG, declarative agents,
  CodeAct, functional workflows **explicitly not yet available**.
- **Go providers shipped:** `foundryprovider`, `openaiprovider`, `anthropicprovider`,
  `geminiprovider`, `a2aprovider`, `aguiprovider`, `copilotprovider`, `otelprovider`.
  **No Ollama or mock provider.**
- **Core Go API:**
  - `agent.Agent` — methods `Run`, `RunText`, `RunMessage`; results via `.Collect()` or
    streaming `for update, err := range ...`.
  - `agent.Config{ Name, Instructions, Tools []tool.Tool, Middlewares []agent.Middleware, ContextProviders }`.
  - `agent.ProviderConfig` — *"run function, session management"* (the custom-provider seam).
  - `agent.Middleware` — per-agent cross-cutting seam.
  - `workflow.NewBuilder` — graph of executors joined by edges (sequential, concurrent,
    conditional routing, subworkflows).
  - Sessions: `a.CreateSession(ctx)`, `agent.WithSession(session)`.

### The core architectural conflict I flagged
- **Genie today:** message bus with a **single governance chokepoint**
  (`governance.Composite.Evaluate(msg)` on every hop) — one seam for governance, tracing,
  fallback, and the live inventory.
- **Framework:** **workflow graph with per-agent middleware** — no single gate.
- Therefore the hard part of the rewrite is **re-proving the unbypassable-gate property in a
  topology designed without one gate.**

### Two de-risking findings
1. **Provider gap is solvable:** mock = deterministic `ProviderConfig` run func; Ollama rides
   in via `openaiprovider` pointed at Ollama's OpenAI-compatible `/v1` (Microsoft's docs
   sanction "Ollama (OpenAI-compatible)"); cloud via `foundryprovider`.
2. **Governance seam** becomes a construction-discipline problem: one `NewGovernedAgent()`
   factory + a registry invariant test asserting every agent carries the gate.

### Initial advice (as advisor)
Recommended **not** a big-bang rewrite — prefer strangler-fig/adapter + a one-agent pilot —
because the framework is pre-release, release-less, and missing RAG. Then asked the user to
choose intent and on-prem posture.

---

## Part 3 — Decisions taken (via AskUserQuestion)

| Question | User's answer |
|---|---|
| Migration intent | **Full rewrite onto framework** |
| On-prem Ollama + offline mock a hard requirement? | **"both support"** — keep on-prem (Ollama + mock) **and** add cloud (Foundry/Azure OpenAI) |

I respected the decision (full rewrite, both provider paths) while holding two conditions:
(1) even a full rewrite starts with **one vertical slice** before batch-porting 58 agents;
(2) a **design doc with Mermaid diagrams is approved before any agent code** — matching the
repo's stated workflow preference.

---

## Part 4 — Deliverable produced

**File:** `docs/migration-agent-framework.md` (draft for approval; no code written).

Contents:
1. Honest risk statement (pre-release framework; compliance layer is *not* inherited).
2. Verified framework API surface.
3. Core architectural shift with two Mermaid diagrams (current bus topology vs. target graph
   topology with governance re-imposed as mandatory middleware).
4. Component disposition table — keep / re-host / rebuild / replace / drop for every Genie asset.
5. Agent porting template (LLM agent / deterministic run-func / fan-in coordinator).
6. **The critical section:** keeping the governance gate unbypassable via three enforced
   layers — single `NewGovernedAgent()` construction door, a front-door executor, and a
   registry invariant test — with a Mermaid sequence diagram. Governance stays *data, not code*.
7. Provider strategy (Mermaid): mock run-func / Ollama via `openaiprovider` OpenAI-compat /
   cloud via `foundryprovider`; embeddings stay on our own `rag.Embedder`.
8. Phased plan with approval gates:
   - **Phase 0** — repo scaffold + throwaway spike (Ollama tool-calling; mock run-func); ½–1 wk.
   - **Phase 1** — vertical slice: `currency` agent end-to-end, governed, both providers; 1–2 wk.
   - **Phase 2** — orchestration backbone (analyzer→currency/macro/rates→supervisor→reporter); 1–2 wk.
   - **Phase 3** — platform re-host (registry/inventory/AIBOM, fallback edges, RAG, wrapper chain, auth, UI); 2–3 wk.
   - **Phase 4** — batch port remaining ~55 agents; 3–5 wk.
   - **Phase 5** — parity + cutover (smoke/e2e/red-team/bcp-drill vs v1.0.0 oracle); 1–2 wk.
9. Repo-location recommendation: **new sibling repo `../geniemaf/`** (not in-place), so the
   shipped v1.0.0 remains the parity oracle for cutover — mirrors how `geniepython/` sits
   beside `genie/`.
10. Open questions/risks: API churn (pin commit SHA in `go.mod`), Ollama tool-calling parity,
    session vs `trace_id` mapping, AIBOM/inventory fully on us, middleware ordering, ~10–15
    engineer-weeks for full parity (Phases 0–2 ≈ 4 wk = the go/no-go point).

---

## Part 5 — Current state / open decisions

- **(a) DECIDED: in-place rewrite in `genie/`** (user, 2026-07-11). No sibling repo.
  Oracle preserved via git: `v1.0.0` tag (`ec27f6b`) + **`legacy/bus-architecture` branch
  (`3116b5e`, created this session)** — do not delete until Phase 5 parity is signed off.
  `main` becomes the framework rewrite. Design doc §9 updated to match.
- **(b) PHASE 0 DONE — GO (no blockers).** Ran a repo-safe recon+feasibility **workflow**
  (8 agents) reading real framework source, then a **compiled scratchpad spike**. Results:
  - Module go-gettable; pin `agent-framework-go v0.0.0-20260710122455-75d0a647f739` +
    `openai-go/v3 v3.41.1`; framework needs go 1.25.0 (genie already is). No toolchain bump.
  - Spike compiled + ran first try. Proof: mock provider (no network) ✅; governance middleware
    **ALLOW** ✅ and **DENY** (short-circuits before provider; canned text suppressed, deny error
    propagates) ✅ — the unbypassable-gate property survives the bus→middleware move; Ollama
    via `openaiprovider`+`option.WithBaseURL` constructs ✅.
  - Design doc corrected (§1.5 added; fictional `agent.NewCustom`/`agent.Request` and
    `Instructions`-on-`Config` fixed to compiled signatures; `foundryprovider` marked UNVERIFIED).
  - Spike lives in scratchpad (`afg_spike/`); genie repo still unmutated (only untracked docs).

### Phase 1 (in-place) — DONE, CI GREEN
First heavyweight in-place mutation, executed and verified:
- **Deps pinned** into `go.mod`: `agent-framework-go v0.0.0-20260710122455-75d0a647f739` +
  `openai-go/v3 v3.41.1`. MVS bumped OTel 1.43→1.44, grpc 1.80→1.82, x/sync 0.20→0.22 —
  **whole repo still builds**, legacy `agents/currency` still passes (oracle intact).
- **New package `pkg/afg/`** — the integration layer + single construction door:
  - `factory.go` — `GovMiddleware` (bridges framework `[]*message.Message` → `protocol.Message`,
    runs `governance.Policy.Evaluate` pre-provider, denies without calling `next`);
    `NewGovernedDeterministic` + `NewGovernedOllama` (the ONLY two constructors; both inject the
    gate outermost + `DisableFuncAutoCall=true`); `WithGovMessage`, `ResponseText`.
  - `currency.go` — framework-native `currency` port (deterministic `RunFunc`; rate table
    reproduced, legacy agent left untouched as oracle).
  - `afg_test.go` — allow ✅; **deny short-circuits before provider** (sentinel proves RunFunc
    never runs) ✅; **real board policy** (`config/ai-policy.example.yaml`) allows benign msg ✅;
    **Ollama path denies before network** (both provider paths governed) ✅.
  - `singledoor_test.go` — CI guard: no framework constructor in any framework-importing file
    outside `pkg/afg` (word-boundary regex so legacy `*_agent.New()` don't false-positive).
- **`cmd/af-hello/`** — runnable slice; against the real board policy prints: allow
  (5000 USD→415000 INR), deny (`classification exceeds recipient ceiling`), Ollama constructed.
- **Gate:** `make vet` clean; `go test -race ./...` → **exit 0, no failures**.

### Phases 2–5 (in-place) — one-hour push, CI GREEN throughout
- **Phase 2 (DONE):** `pkg/afg/orchestrate.go` `FanOut` — concurrent fan-out/fan-in via
  `agentworkflow.NewConcurrentWorkflowBuilder` + `inproc.Default.Run`; each `OutputEvent.Output`
  is `*agent.ResponseUpdate`. Governed specialists run concurrently (latency = max(stages)).
  Test: `TestFanOut_GovernedSpecialists`.
- **Phase 3 (backbone DONE):** `pkg/afg/registry.go` — `Registry`/`Inventory()` (port of
  `/v1/ai-inventory`+AIBOM) + `RunWithFallback` (BCP seam). Typed `DeniedError` so governance
  denials are NOT rescued by fallbacks (only execution errors are). Tests: `TestRegistry_*`.
  RAG/wrapper-chain/auth/UI carried over from legacy, not re-touched.
- **Phase 4 (template proven):** `pkg/afg/specialists.go` — `det()` one-liner template;
  ported `currency`,`fx_rates` (faithful), `tax_estimator`,`macro` (representative) +
  `DefaultRegistry`. 4 of ~58; rest mechanical. Test: `TestDefaultRegistry_BatchPortedSpecialists`.
- **Phase 5 (partial):** `go test -race ./...` green across the WHOLE repo (legacy + port
  together); legacy `currency` intact as oracle. Pending: full smoke/e2e/red-team parity +
  HTTP-edge cutover.

Verified workflow API via a scratchpad spike (`afg_spike/`, throwaway): concurrent builder,
`inproc.Default.Run`, `OutputEvent.Output = *agent.ResponseUpdate`. Commits: 07823d7 (Phase 1),
then Phases 2–4 (this push).

Standing engineering notes carried into the rewrite:
- Pin the framework by **commit SHA** in `go.mod` (no releases exist; `@latest` will break silently).
- The governance-seam design (§6 of the migration doc) is the single highest-risk item —
  validate it in Phase 1, not week 10.
- Keep governance data-driven (`pkg/policy` + `config/ai-policy.example.yaml` feed the
  middleware); only the injection point moves from bus to middleware.

---

## Sources referenced this session
- microsoft/agent-framework-go — <https://github.com/microsoft/agent-framework-go>
- Agent Framework, Agent types (Go zone) — <https://learn.microsoft.com/en-us/agent-framework/agents/>
- Agent Framework, Workflows — <https://learn.microsoft.com/en-us/agent-framework/workflows/>
- Agent Framework, Overview — <https://learn.microsoft.com/en-us/agent-framework/overview/>

## Artifacts created this session
- `docs/migration-agent-framework.md` — the migration design (draft, awaiting approval).
- `session.md` — this log.
