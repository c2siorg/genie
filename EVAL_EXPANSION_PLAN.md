# Golden Dataset & Evaluation Expansion Plan
**Status**: Phase 0 ✅ · Phase 1 ✅ · Phase 2 🟡 in progress
**Date**: 2026-06-25 · **Revision**: v2 (verified against source + adversarially reviewed)
**Author**: Claude (Genie agent-platform track)

> ## ✅ IMPLEMENTED (2026-06-25, all tested, `go test ./...` = 0 FAIL)
> - **`pkg/eval/golden/`** — canonical `Case`/`Dataset`, `io/fs` loader (`LoadGoldenDir` + `//go:embed` `LoadEmbedded`) that **rejects pre-baked output**, typed `EvaluateCase` dispatch to the deterministic judges, `Evaluate`/`Report` aggregation.
> - **Honest gate** — `golden.TestGoldenGate` loads + scores the real dataset; `TestEvaluateCase_GateCanFail` proves it is not tautological. Runs on every PR via `go test ./...`.
> - **5 deterministic judges wired** — settlement/compliance/orchestration/lineage (existing `Mock*Judge`) + **new `MockMerchantJudge`** (`pkg/eval/judges`).
> - **56 real, correctly-keyed cases** across `data/*.json` covering **38 distinct failure modes** (FM-SE-001..010, FM-CO-001..009, FM-OR-001..008, FM-LA-001..007, FM-MC-001..004). Every asserted verdict matches the judge's computed verdict.
> - **`cmd/eval-golden` CLI** + `make eval-golden` + **repointed `ci-eval-golden`** (the old target silently passed on a deleted test — fixed) + CI step + report artifact.
> - **Tautology decommissioned** — `pkg/eval/regression_test.go` gutted (1468→stub); `golden_datasets/` marked `DEPRECATED.md`.
>
> - **Advisor execution eval** — `pkg/eval/advisoreval/` RUNS the 3 real advisor agents (profile-analyzer/financial-analyst/recommendation-generator) against **11 cases** and asserts on output. Offline/CI-safe because those agents are deterministic. `TestAdvisorEval` (11/11) + a non-vacuous regression test. `cmd/eval-golden` reports both tracks: **56 Track-A judge cases + 11 Track-B advisor cases = 67 real executed cases.**
>
> **Remaining (Track B / LLM):** `agent_behavior` domain needs an LLM judge (deferred deliberately — a deterministic AB judge would be circular). Wire real LLM judges into the multi-turn `LLMJudge` stub (needs Ollama). Raising per-failure-mode coverage to ≥5 needs richer judges.

> ⚠️ **Read the honest assessment first.** Naively "adding more cases" would inflate a system that is currently **evaluation theater**. The numbers (300 → 93 → 55) hide a deeper problem: the part that *is* executed doesn't actually evaluate anything. This plan fixes the machinery **before** scaling the data, so that growth means real signal, not bigger green checkmarks.

> 🔬 **v2 changes (from a 12-agent design + 3-agent adversarial review, all claims re-verified against source):**
> - **No real `SettlementJudge` exists** — only `MockSettlementJudge`. But it implements **genuine deterministic rules** (amount match, CBDC-committed, reconciliation), so settlement is scorable offline without an LLM. Rename mocks → "deterministic judges."
> - **The `Judge` interface (`types.go:202`) is dead code.** It declares `Evaluate(input interface{})`, but every real judge implements `Evaluate(ctx, TypedInput)` and nothing asserts `var _ Judge`. Dispatch must be **typed per domain**, not through the interface (the design agents wrongly assumed interface dispatch — it wouldn't compile).
> - **All 3 real judges (Compliance/Orchestration/Lineage) call the LLM.** "Lineage is deterministic" was false. Offline gate must use the **deterministic (mock-named) judges**; LLM judges run nightly.
> - **The 3 advisor agents already exist** (built in Week 1) — advisor eval is **not** blocked.
> - **Verified counts:** 55 Go-literal cases (`buildGoldenDataset`), 93 JSON cases, **51 distinct failure modes** referenced (`failure_modes.json` formally defines ~38 → drift to reconcile). The "300/380" figures are comment-only fiction.

---

## PART 1 — GROUND TRUTH (verified by source inspection)

There are **three disjoint "eval" systems** in `pkg/eval/`, and they don't talk to each other.

### System A — Go-literal "golden dataset" (`regression_test.go`)
- `GoldenDataset = buildGoldenDataset()` synthesizes cases as **Go struct literals** (`settlementGoldenCases()`, `complianceGoldenCases()`, …).
- **Actual count: 55 cases** (not the "300+" the comments claim).
- Executed by `TestGoldenDataset_AllCases` with a **"90% pass-rate gate"**.
- 🔴 **The gate is theater.** `evaluateTestCase()` (regression_test.go:1414) never calls an agent or validator. It checks that metadata/input/expected are non-empty and then `return TestResult{Pass: true}`. Its own comment says *"In production, this would call actual validation logic."* Result: **55/55 = 100.0% pass, always.** The gate cannot fail on a real regression.
- `TestFailureModeCoverage` reports **51 distinct failure modes, 50 of which have <5 cases** (it only warns, doesn't fail).

### System B — JSON golden datasets (`golden_datasets/*/*.json`)
- **Orphaned.** Loaded by **zero** Go code (`grep golden_datasets *.go` = nothing).
- **Actual count: 93 cases**, while `INDEX.json` claims **300** and per-file headers sum to **220**. Triple inflation.
- Missing files referenced by INDEX: `compliance/kyc_rejected_cases.json`, `compliance/aml_cases.json`, `compliance/edge_cases.json`, `orchestration/concurrent_cases.json`; `settlement/edge_and_regression_cases.json` exists but is empty of cases.
- Each case already has `actual_output` and `evaluation_results` **pre-filled** — so even if loaded, comparing pre-baked actual==expected is also tautological.

### System C — Runnable LLM harnesses (`singleturn/`, `multiturn/`) — **the only real evals**
- **Single-turn** (`singleturn/executor.go`): feeds a prompt + tool list to a real LLM (Ollama default), checks which tools the model selects vs `expected_tools`. **13 cases** (8 file-tools + 5 shell-tools). No CLI entry point.
- **Multi-turn** (`multiturn/executor.go` + `runner.go`, `cmd/eval-multiturn`): real agent loop with mock tools, deterministic scoring (`ToolOrderCorrect`, `ToolsAvoided`) + optional `LLMJudge`. **5 conversations.** Pass threshold 0.6.
- These genuinely exercise a model. They are tiny (**18 cases total**) and don't touch any of the 34 registered agents or the 3 new advisor agents.

### Supporting registries (real, underused)
- `rubrics.json`: **18 rubrics** (INDEX claims 39).
- `failure_modes.json`: **38 failure modes** defined (dataset references 51 distinct strings — drift).
- `judges/`: real LLM-as-judge implementations (settlement/compliance/orchestration/lineage) with calibration (TPR/TNR). **Not wired** into System A's gate or System C's harnesses.

### The one-paragraph summary
> We have **18 real eval cases** (System C), **55 fake-passing cases** (System A, tautological evaluator), and **93 orphaned fixtures** (System B). The headline "300" exists nowhere in executable form. Real LLM-judge code exists but is disconnected. **Coverage of actual Genie agents (settlement, KYC, AML, advisor pipeline, …) by a real executed eval ≈ 0.**

---

## PART 2 — DESIGN PRINCIPLES (so growth = signal)

1. **An eval case is worthless unless something executes it against real logic.** Every case we add must be loaded and run by `go test` or a harness that calls an agent/judge.
2. **One source of truth.** Collapse Systems A and B into a single dataset format that is both human-authorable (JSON) and machine-executed (Go loader). Delete the duplicate.
3. **No pre-baked `actual_output`.** The system under test must *produce* the actual output at run time; the case carries only `input` + `expected` + scoring rubric.
4. **Honest gates.** A pass-rate gate must be capable of failing. Wire `evaluateTestCase` to real validators/judges before trusting any percentage.
5. **Coverage measured against real targets** — the 34 registered agents + 3 advisor agents + 38 failure modes — not against an invented denominator.
6. **Deterministic-first, LLM-judge-second.** Prefer cheap deterministic assertions; use the LLM judge only where correctness is subjective. Keep `make eval` runnable offline.

---

## PART 2b — TWO TRACKS (the framing that resolves "fake execution")

The adversarial review's sharpest critique was *"where does `actual_output` come from?"* The answer is that there are **two legitimately different kinds of eval**, and conflating them is what produced theater:

- **Track A — Rule/judge validation (offline, deterministic, gates every PR).** A case's `input` is fed to a **deterministic judge** that re-derives a verdict from encoded business rules; compare to `expected`. This tests *the rule encoding* (e.g., "amount mismatch ⇒ fail", "velocity over threshold ⇒ block"). It does **not** run a live agent, and it must never carry a pre-baked `actual_output`. The deterministic judges already exist (the `Mock*Judge` types — misnamed; they contain real rules). This is the honest replacement for the tautological gate.
- **Track B — Agent execution (live/nightly, LLM-backed).** A case's `input` is fed to a **real agent** (the single-turn / multi-turn harness, and the 3 advisor agents); the agent **produces** `actual_output` at runtime; an LLM judge (Compliance/Orchestration/Lineage) or deterministic check scores it. This tests *the agents*.

Phase 0 makes **Track A** honest. Phases 1–2 grow **Track B** (real agents) and add cases to both.

---

## PART 3 — PHASED PLAN (v2, verified)

### Phase 0 — Stop the theater (foundation; ~3–4 days)
**Goal: a gate that CAN fail, one dataset, registries that match reality. No new cases yet.**

- **0.1 Promote the deterministic judges.** The `Mock{Settlement,Compliance,Orchestration,Lineage}Judge` types in `judges/mock.go` contain real rule logic and run offline. Rename to `Deterministic*Judge` (keep aliases) and treat them as Track-A scorers. **Do not** invent a new `SettlementJudge` for Phase 0 — `MockSettlementJudge`'s rules suffice.
- **0.2 Typed dispatch (NOT the dead `Judge` interface).** Add `pkg/eval/golden/evaluate.go` with `EvaluateCase(c Case) Result` that switches on `c.Metadata["domain"]`, builds the domain's typed judge input from `c.Input`/`c.Expected` (with key-presence validation — no blind `.(string)` assertions), calls the deterministic judge, and returns pass/score/reason. Handle `domain==""`/`failure_mode=="none"` explicitly (happy-path ⇒ assert `expected` directly). **Decide separately** whether to also fix the dead `Judge` interface in `types.go:202` to `Evaluate(ctx, interface{})` or delete it — it's currently satisfied by nothing.
- **0.3 Canonical schema + loader.** `pkg/eval/golden/types.go` (`Case`, `Dataset`, `Registry`) + `LoadGoldenDir(dir)`. `Case` = `{ID, FailureMode, RubricRefs, Scenario, Input, Expected, Metadata}` — **no `actual_output`, no `evaluation_results`**. The loader must **reject** any file still carrying pre-baked `actual_output` (enforces the principle, addresses review FIX #3/#5).
- **0.4 Migrate + decommission.** Convert the 55 Go literals AND the 93 JSON cases into the unified format (strip pre-baked fields). **Delete** `buildGoldenDataset()` + the per-domain literal funcs + the tautological `evaluateTestCase`, and rewrite `TestGoldenDataset_AllCases` to load from disk and call `EvaluateCase` (review FIX #4 — explicit decommission, not parallel addition).
- **0.5 Reconcile registries + integrity test.** Fix `INDEX.json` (300→real), reconcile `failure_modes.json` (define all 51 referenced modes or fix the references), make `total_cases` computed. Add `TestIndexIntegrity` failing if any declared count ≠ actual on disk.
- **0.6 Honest baseline.** Run the now-real Track-A gate; record the *true* pass rate (expected to drop below 100%). Set the CI floor just under the measured baseline.

**Exit criteria:** one disk dataset; a Track-A gate scored by deterministic judges that *can* fail; registries matching reality; tautological code deleted.

### Phase 1 — Wire CI + Track-B harness (~2–3 days)
- **1.1** Add `cmd/eval-singleturn/main.go` (executor exists, no CLI).
- **1.2** Replace the multi-turn `LLMJudge` stub with the **real LLM judges** (Compliance/Orchestration/Lineage) via a typed adapter; reuse the same input builders from 0.2.
- **1.3** CI stages in `.github/workflows/ci.yml`: **every PR** runs `make ci-eval-golden` (Track-A, deterministic, offline, no Ollama → fast, gating); **nightly** runs `make eval-live` + `make ci-judge-validation` (Track-B, Ollama). Judge calibration (TPR/TNR) gates the **nightly** job (not PRs), but a failing calibration must surface as a red nightly build, not be silently skipped (addresses review's "suppressed signal").
- **1.4** Emit a per-domain pass-rate + FM-coverage **markdown artifact** (define the exact table; CI fails if generation errors, so it can't silently vanish).

### Phase 2 — Expand coverage where it's zero (~1–2 weeks)
Target **≥5 executed cases per failure mode** (`TestFailureModeCoverage` already wants this — currently only warns) and **≥1 real case per agent**. The design workflow produced a concrete **171-case backlog** (see Part 4). Priority by regulatory/financial risk:

1. **Compliance / KYC / AML (highest risk):** author missing `kyc_rejected`, `aml_cases`, `compliance_edge` (~31 cases). Track-A via `compliance` deterministic judge + Track-B via `aml_monitor`/`kyc` agents.
2. **Settlement edge/regression:** fill the empty `edge_and_regression_cases.json` (~20). Double-spend, partial-failure, netting-imbalance, finality-violation. Scored by the settlement deterministic judge.
3. **Lineage:** hash-chain break / tamper / timestamp-monotonicity (~34 proposed). Track-A deterministic where possible (hash checks ARE deterministic even though the current LLM judge isn't).
4. **Advisor pipeline (the agents we built Week 1, 0 coverage):** single-turn + multi-turn cases for profile-analyzer / financial-analyst / recommendation-generator — risk-tolerance mapping, compliance-constraint propagation, impact-confidence by risk profile (~15+). **Not blocked** — agents exist.
5. **Orchestration concurrent** (~deferred set): race conditions, cross-replica session loss (ties to the supervisor decoupling finding).
6. **Merchant/Customer + Agent-behavior:** these have **no judge yet** — author deterministic checks or an LLM-judge prompt first (Phase 2 prerequisite, flagged by review).

---

## PART 3b — VERIFIED MUST-FIXES (survived source re-verification)

The adversarial review raised 20+ issues; these are the ones I **confirmed against source** and that must be honored:

| # | Must-fix | Verified evidence |
|---|----------|-------------------|
| MF-1 | **Delete** the tautological `evaluateTestCase` + `TestGoldenDataset_AllCases`, don't add a parallel gate. | `regression_test.go:1414` returns `Pass:true` unconditionally; run shows 55/55=100%. |
| MF-2 | **No real `SettlementJudge`** — use `MockSettlementJudge`'s deterministic rules (rename, don't rebuild). | `judges/`: only Compliance/Orchestration/Lineage real; `mock.go` has all 4 mocks w/ real rules. |
| MF-3 | **Don't dispatch via the `Judge` interface** — it's dead (`Evaluate(input interface{})` satisfied by nothing). Use typed per-domain dispatch. | `types.go:204` vs `compliance_judge.go:36` (`Evaluate(ctx, ComplianceJudgeInput)`); no `var _ Judge`. |
| MF-4 | **Strip pre-baked `actual_output`/`evaluation_results`** when migrating the 93 JSON cases; loader rejects them. | JSON cases carry both fields today. |
| MF-5 | **Handle `failure_mode=="none"` / `domain==""`** in dispatch (happy-path ⇒ assert `expected`, no judge). | Design agents' domain-from-FM parsing panics on "none". |
| MF-6 | **Validate input keys** before type assertions in judge-input builders (no blind `.(string)`). | Review flagged panic risk; confirmed pattern in proposed code. |
| MF-7 | **Reconcile counts honestly**: 55 + 93 actual, 51 FMs; kill the 300/380 comments. | Verified by run + `jq`. |
| MF-8 | **Merchant/Customer + Agent-Behavior have no judge** — building one is a Phase-2 prerequisite, not assumed. | `judges/` has no Merchant/AgentBehavior types. |
| MF-9 | Advisor agents **exist** (Week 1) — keep their eval in Phase 2, not blocked. | `agents/agents/{profile-analyzer,financial-analyst,recommendation-generator}/agent.go` present + tested. |

**Optional decision (not blocking):** also repair the dead `Judge` interface (`types.go:202`) to `Evaluate(ctx context.Context, input interface{})` so judges can be asserted against it, or delete the interface. Currently it misleads readers into thinking there's polymorphic dispatch when there isn't.

### Phase 3 — Generation & scale (optional, ~1 week)
- **3.1 Parameterized generators** for mechanical families (e.g., netting permutations, velocity-threshold boundaries) — code that emits JSON cases, checked in, so they're reviewable and deterministic.
- **3.2 Judge calibration loop:** use `TestJudgeAccuracy_OnTestSet` honestly — hand-label a held-out set, enforce TPR/TNR ≥ a real measured bound.
- **3.3 Drift/regression tracking** via the existing `pkg/eval/drift` package on each nightly run.

---

## PART 4 — COVERAGE TARGET MATRIX (with verified backlog)

The design workflow produced a **concrete 171-case backlog** (specific id/scenario/input/expected/judge per case, stored in the workflow result). Counts below are verified.

| Domain | FMs | Real executed today | Backlog proposed | Phase-2 target | Scorer (Track A / Track B) |
|--------|-----|---------------------|------------------|----------------|----------------------------|
| Settlement | 10 (FM-SE) | 0 (55 literals fake-pass) | 20 | ≥50 | `MockSettlementJudge` rules / — |
| Compliance (KYC/AML) | 9 (FM-CO) | 0 | 31 | ≥51 | compliance det. + LLM judge |
| Orchestration | 9 (FM-OR) | 0 | 28 | ≥50 | orchestration det. + LLM judge |
| Lineage/Audit | 7 (FM-LA) | 0 | 34 | ≥35 | hash-chain det. (LLM today) |
| Merchant/Customer | 9 (FM-MC) | 0 | 25 | ≥45 | **judge to build first** |
| Advisor + Agent-behavior | 7 (FM-AB) | 0 | 33 | ≥35 | advisor agents (Track B) + LLM |
| Tool-selection (single-turn) | — | 13 | — | ≥40 | exact-match (real, exists) |
| Multi-turn agent | — | 5 | — | ≥20 | det. + LLM judge |
| **Total** | **51** | **~18 real** | **171** | **≥240 real** | |

"Real executed today = 0" for the golden domains because the only thing executing them (`evaluateTestCase`) is tautological. Phase 0 converts the existing 55+93 authored cases into *real* Track-A signal before any of the 171 new cases are written.
| **Total executed** | **51 modes** | **~18 real** | **≥240 real** | |

"Real executed cases today = 0" for the golden domains because System A's evaluator is tautological and System B isn't loaded. Phase 0 is what converts the existing 55+93 authored cases into *real* executed cases.

---

## PART 5 — EFFORT, RISK, SEQUENCING

| Phase | Effort | Risk if skipped |
|-------|--------|-----------------|
| 0 — de-theater + unify | 2–3 d | Every later case inflates a meaningless number |
| 1 — CI + judge wiring | 2 d | Regressions land silently; evals never run on PRs |
| 2 — coverage expansion | 1–2 wk | Real agents (compliance!) remain unverified |
| 3 — generators/calibration | 1 wk | Manual authoring won't scale past ~250 cases |

**Hard dependency:** Phase 0 **must** precede Phase 2. Authoring 200 new cases against a tautological evaluator produces 200 guaranteed-green, zero-signal tests — the exact trap this project's "honest coverage" history warns against.

**Interaction with the decoupling track:** the advisor-pipeline cases (Phase 2.3) should be authored against the agents we built in Week 1, and the multi-turn harness will need the same `traceparent`/OTel wiring scheduled for decoupling Week 6–7 — so Phase 1.2 and decoupling Week 6 can share work.

---

## PART 6 — OPEN DECISIONS (for the user)

1. **Scope now:** do all of Phase 0–2, or start with Phase 0 only (de-theater) and decide on expansion after seeing the honest baseline?
2. **Single source of truth:** confirm we collapse System A (Go literals) **into** System B (JSON) — i.e., delete `buildGoldenDataset()`. (Recommended.)
3. **CI cost:** gate every PR with the deterministic golden run (fast, offline) and reserve LLM-judge runs for nightly? (Recommended — keeps PRs fast, no Ollama dependency in PR CI.)
4. **Honest gate floor:** set the CI pass-rate floor at the *measured* post-Phase-0 baseline, accepting it may start below 90%? (Recommended over keeping a fake 100%.)
