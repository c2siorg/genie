# Genie — Master Implementation Plan (UI → Backend → Testing → Evaluation)

**Author**: Claude (advisory)
**Date**: 2026-06-06
**Status**: Draft for approval — opinionated, grounded, honest
**Companion docs**: `UIUXplan.md` (UI/UX representation), this file supersedes the prior testing-only `plan.md`.

> This plan is written in deep-reasoning ("ultrathink") mode. It is grounded in the **verified current state of the repository**, not aspirational claims. Earlier in this session a workflow reported "2,060 tests, 100% coverage" — that was not true (much of it was non-compiling test theater). This plan starts by telling the truth about where we are, then sequences the work so that **every phase is provably complete end-to-end before the next begins.**

---

## 0. Honest baseline — where we *actually* are (verified 2026-06-06)

| Dimension | Verified reality | Implication |
|-----------|------------------|-------------|
| **Build** | `go build ./...` **fails** — `agents/kyc` does not compile (undefined `DocumentProcessor`, `IdentityVerifier`, `SanctionsChecker`, `OnboardingApprover`, `kyc.VerificationResult`). `pkg/...` builds clean. | **Nothing is trustworthy until the build is green.** This is the critical-path root. |
| **Tests** | 3 prior-workflow test files were broken: `erupeepayment/payment_flow_test.go` (deleted — pure map-theater, real tests already existed), `eval/judge_calibration_test.go` and `web/handlers/phase4_handlers_test.go` (now compile). Most `pkg/` tests pass. | Test *count* is meaningless; test *quality* varies wildly. Some existing tests assert on data they just created. |
| **Coverage** | Real overall ≈ 61–72% (not 95%). Strong: `auth`, `governance`, `mid` (CSRF/cookies ~100%), `commerce` workflow. Weak/0%: parts of `db`, `settlement`, `merchant`, several `[no test files]` packages. | "100% line coverage" is the wrong target (see §6.1). Criticality-tiered targets are. |
| **UI** | ~1,874 lines of vanilla JS/HTML (+ CSS). Two surfaces only: chat (`index.html`/`app.js`) and eval annotation (`eval_review.html`). Good a11y groundwork, solid CSRF/HttpOnly session handling. | UI represents ~15–20% of backend capability. Four of five product surfaces (§ UIUXplan) don't exist yet. |
| **Backend** | ~80 routes across 5 domains (conversational AI, e-Rupee commerce, compliance, governance/safety, evaluation). Rich and largely functional. | The backend is the mature part. The work is *exposing*, *hardening contracts*, and *proving* it — not building it from scratch. |

**Starting line, stated plainly**: a capable backend, a thin UI, a broken build, and an inflated test history. The plan's first job is to make the foundation *true*.

---

## 1. The one strategic decision that shapes everything: vertical slices, not horizontal layers

The earlier failure in this session came from **horizontal layering** — "write all the frontend, then all the backend tests, then all the eval." That approach produced tests written against *imagined* APIs (because the contract wasn't pinned) and UI against endpoints that didn't return what was needed. It's why the tests didn't compile.

**This plan slices vertically by domain.** Each phase takes ONE product surface and drives it through the *entire* stack to "done":

```
   A VERTICAL SLICE  (e.g. "Commerce")
   ┌─────────────────────────────────────────────┐
   │ 1. Pin the API contract  (request/response,  │
   │    error shapes, money in paise)             │
   │ 2. Harden the backend handler/service to     │
   │    that contract (+ provenance/lineage)      │
   │ 3. Build the UI workspace against the real    │
   │    contract                                  │
   │ 4. Test the slice: unit (logic) + handler    │
   │    (httptest) + E2E (Playwright) — all REAL  │
   │ 5. Evaluate the slice: judge + failure modes │
   │    for that domain (where AI decisions exist)│
   │ 6. Exit criteria met → slice is shippable    │
   └─────────────────────────────────────────────┘
```

Why this is the right call:
- **Every phase is independently shippable and provably working** — no "big bang" integration at the end.
- **Tests are written against real, pinned contracts** — eliminating the theater problem at its root.
- **The contract is the artifact that aligns UI + backend + tests + eval** — write it once, four disciplines consume it.
- **Value lands early** — the highest-value workspace ships first, not last.

The trade-off: it feels slower than parallel layer-work because you can't claim "all the UI is done." But layer-work's "done" was fiction. Slice "done" is real.

---

## 2. Target architecture (where we're going)

```
┌───────────────────────────────────────────────────────────────┐
│ GENIE SHELL  — role-gated, provenance drawer, command palette, │
│               guarded kill-switch                              │
├───────────────────────────────────────────────────────────────┤
│  Assistant │ Commerce │ Compliance │ Gov/Safety │ Evaluation   │  ← 5 workspaces
│            │          │            │            │              │
│  each backed by a PINNED API CONTRACT over the existing ~80    │
│  endpoints, each with provenance via /lineage + /audit         │
├───────────────────────────────────────────────────────────────┤
│  BACKEND (Go, mature)  — handlers · services · agents · CBDC   │
│  ledger · compliance engine · eval/judges · lineage hash-chain │
├───────────────────────────────────────────────────────────────┤
│  CROSS-CUTTING:  design system (tokens, Money, state-machine,  │
│  provenance drawer) · auth (HttpOnly+CSRF) · CI/CD gates       │
├───────────────────────────────────────────────────────────────┤
│  TESTING:  unit → handler(httptest) → E2E(Playwright) →        │
│            evaluation(judges, failure modes, golden datasets)  │
└───────────────────────────────────────────────────────────────┘
```

The full stack, unified by **contracts** (vertical alignment) and the **design system + provenance pattern** (horizontal consistency).

---

## 3. Dependency graph / critical path

```
Phase 0 (FIX BASELINE) ──────────────────────────────► everything
   │  build green (agents/kyc) · honest coverage map · CI gate
   ▼
Phase 1 (FOUNDATION) ────────────────────────────────► all UI work
   │  shell · design tokens · Money · provenance drawer · contract format
   ▼
   ├─► Phase 2  Commerce slice   (highest value; proves the patterns)
   │
   ├─► Phase 3  Compliance slice (reuses risk components)
   │
   ├─► Phase 4  Governance/Safety slice (novel fleet UX; after patterns proven)
   │
   ├─► Phase 5  Evaluation slice (evolve existing eval UI + judge calibration)
   │
   └─► Phase 6  Assistant polish + Audit/Regulator lens
   ▼
Phase 7 (HARDENING) ─ perf, security review, a11y gate, release
```

**Nothing starts before Phase 0.** You cannot honestly test or measure a codebase that doesn't compile. Phases 2–6 are independent once Phase 1 lands, and *could* parallelize across people — but each is a full vertical slice owned end-to-end.

---

## 4. Phased execution plan

Each phase: **Goal · Backend work · UI work · Testing · Evaluation · Exit criteria · Honest effort.**
Effort is in ideal engineer-weeks for a small team; treat as ranges, not promises.

### Phase 0 — Fix the baseline (make the foundation true) ✅ COMPLETE (2026-06-06)
> **Done.** `go build ./...` = 0; `go test ./...` = 0 (134 pkgs pass); honest coverage map committed (`COVERAGE.md`, total **60.0%**); CI gate live (`make ci-gate` + `.github/workflows/ci.yml`, ratchet floor 60%). Theater purged (5 non-compiling/no-real-assertion test files removed), one real latent bug fixed (`isValidGST`/`isValidPAN` accepted lowercase), stale UI tests updated to the secure cookie+CSRF model. See COVERAGE.md for the full change list.
- **Goal**: green build, honest coverage map, a CI gate that prevents regression to "lies."
- **Backend**: fix `agents/kyc` compile errors (implement or stub `DocumentProcessor`, `IdentityVerifier`, `SanctionsChecker`, `OnboardingApprover`, `kyc.VerificationResult` to match their usage). Audit prior-workflow test files; delete remaining theater, keep/repair real ones.
- **UI**: none.
- **Testing**: `go build ./...` exit 0; `go test ./...` green; generate a **real** per-package coverage report (the honest map that replaces the fictional 95%).
- **Evaluation**: confirm `pkg/eval` + `pkg/eval/judges` compile and their existing tests pass.
- **Exit criteria**: ✅ `go build ./...` = 0 · ✅ `go test ./...` green (or known-skipped documented) · ✅ coverage report committed · ✅ CI runs build+test+coverage on every push and **fails on build break or coverage drop**.
- **Effort**: 1–2 weeks. *This is the highest-ROI work in the whole plan.*

### Phase 1 — Foundation (shell + design system + contract discipline) ✅ COMPLETE (2026-06-06)
> **Done & verified.** Framework decision: **React + TS + Vite** (user call). Built in `frontend/`, compiled into the Go embed tree (`pkg/web/handlers/ui/app`), served at `/ui/app/` with SPA fallback — legacy UI untouched at `/ui/`. Delivered: design tokens; **Money** component (BigInt-precise paise→₹, Indian grouping); **StateMachine/timeline**; **provenance drawer** (real POST `/lineage/query`+`/verify`, hash-chain integrity); **role-gated shell** (gated against the real `user`/`advisor`/`admin` roles); cookie+CSRF **API client** + **session** (no localStorage tokens). **44 frontend tests + 4 Go SPA-serving tests pass; `go build`/`go test ./...` green; `make ui-build`/`ui-test` + CI wired.** Per-workspace contract docs remain to be written per-slice (Phases 2-6).
- **Goal**: the scaffolding every workspace depends on.
- **Backend**: stabilize auth/session contract; ensure `/lineage` + `/audit` return enough to power the provenance drawer; document the contract format (one OpenAPI-ish doc per domain).
- **UI**: workspace shell (role-gated nav), design tokens (CSS custom properties), the **Money** component (paise→₹, single source of truth), the **provenance drawer**, **state-machine/timeline** component, tiered confirmation patterns, real-time primitives (SSE + WS consumers). **Decide the framework fork (§7).**
- **Testing**: component tests for Money (paise rounding, grouping, sign), provenance drawer, state-machine; a11y harness wired into CI (build on existing `a11y-verify`).
- **Evaluation**: n/a (infrastructure).
- **Exit criteria**: ✅ shell renders role-gated · ✅ Money component has exhaustive unit tests incl. edge cases (0, max int64, negative, decimal paise) · ✅ provenance drawer pulls real `/lineage` data · ✅ a11y gate in CI.
- **Effort**: 2–3 weeks (longer if framework adoption chosen — see §7).

### Phase 2 — Commerce slice (where's my money) ⭐ highest value
- **Goal**: a merchant can locate any order's exact state, see the money math, trust the payout.
- **Backend**: pin contracts for `/commerce/order`, `/payment`, `/settlement`, `/merchant`, `/onboard`, `/transaction`, `/history/{txn_id}`; ensure each settlement response carries state + lineage ref + reconciliation status.
- **UI**: Commerce workspace — order list/detail, **settlement state-machine timeline**, reconciliation badge, netting view, onboarding.
- **Testing**: unit (settlement amount/netting/reconciliation logic — *real* assertions on real functions, replacing the deleted theater); handler tests (`httptest` against pinned contracts); E2E (Playwright: order→payment→settlement→fulfilled, plus error/retry/concurrency corner cases).
- **Evaluation**: settlement judge (amount-hallucination, netting, CBDC alignment, reconciliation) calibrated to TPR/TNR ≥ 0.90; ≥5 golden cases per settlement failure mode.
- **Exit criteria**: ✅ contract documented · ✅ UI works against real backend · ✅ slice tests green (unit+handler+E2E) · ✅ settlement judge calibrated · ✅ money never ambiguous (verified by tests).
- **Effort**: 3–4 weeks.

### Phase 3 — Compliance slice
- **Goal**: an officer triages, decides, and the decision is audit-defensible.
- **Backend**: pin `/compliance`, `/aml`, `/check(/{id})`, `/score`, `/account/{id}/velocity`, `/fraud-history`, `/limits/{id}`; ensure decisions write lineage with reason + actor.
- **UI**: Compliance Console — risk-signal cluster (AML band, velocity gauge, sanctions/PEP), decision panel (BLOCK = typed confirm + reason), evidence side-by-side, decision trail.
- **Testing**: unit (velocity/AML scoring logic); handler tests; E2E (KYC→AML→velocity→decision; block-on-failure; concurrent checks).
- **Evaluation**: compliance judge (KYC justification, AML threshold, velocity enforcement, false-positive rate) calibrated; golden cases per compliance failure mode.
- **Exit criteria**: ✅ clear-cut case decided <60s in UX testing · ✅ every decision has complete audit trail (test-verified) · ✅ compliance judge calibrated.
- **Effort**: 3–4 weeks.

### Phase 4 — Governance & Safety slice
- **Goal**: operators see fleet health and can intervene safely.
- **Backend**: pin `/hitl/approvals`, `/elevation/requests`, `/incidents`, `/trust/{agentID}`, `/killswitch`, `/consent`, `/grant`, `/revoke`; live status feed (WS).
- **UI**: agent fleet board (health grid + trust score), HITL approval inbox (SLA timers), incident stream, **guarded kill-switch** (two-step, reason, blast-radius preview), consent registry.
- **Testing**: unit (approval/elevation state); handler tests; E2E (approve/deny HITL with context; kill-switch confirmation flow; incident triage).
- **Evaluation**: orchestration judge (state-machine validity, step sequence, multi-agent consistency) calibrated.
- **Exit criteria**: ✅ kill-switch reachable <3 clicks, impossible to fire accidentally (test-verified) · ✅ HITL inbox actions write audit · ✅ orchestration judge calibrated.
- **Effort**: 4–5 weeks (novel fleet UX; budget for iteration).

### Phase 5 — Evaluation slice
- **Goal**: engineers find failures fast and can trust the judges.
- **Backend**: pin `/evaluate`, `/traces`, `/clusters`, `/feedback`, `/search`; expose judge calibration metrics.
- **UI**: evolve `eval_review` — keep keyboard-first annotator; add failure-mode dashboard (40-mode taxonomy distribution + trends), judge calibration view (TPR/TNR + CI + confusion matrix), cluster explorer.
- **Testing**: unit (annotation/clustering logic); component (annotator); E2E (annotate→submit; filter by failure mode).
- **Evaluation**: lineage judge (hash-chain integrity, decision traceability) calibrated; **failure-mode coverage gate**: every mode has ≥5 golden cases + ≥1 judge test + ≥1 E2E.
- **Exit criteria**: ✅ all 4 judges calibrated TPR/TNR ≥ 0.90 · ✅ 100% failure-mode coverage (the *meaningful* 100% — see §6) · ✅ annotator UX unchanged-or-better.
- **Effort**: 3 weeks.

### Phase 6 — Assistant polish + Audit/Regulator lens
- **Goal**: trustworthy conversational surface; read-only regulator view.
- **Backend**: harden `/ask/stream` (SSE), `/chat/ws`; `/export/{entity_id}` for audit.
- **UI**: streaming answers with source/confidence chips → provenance drawer; "routed to human" state; Audit lens (render any entity + full lineage + hash-chain status, read-only).
- **Testing**: E2E (streaming render, source display, error/stall handling, human-routing); audit-lens read-only verification.
- **Evaluation**: hallucination/faithfulness checks on assistant answers.
- **Exit criteria**: ✅ no raw errors/stalls in assistant · ✅ regulator can reconstruct any decision trail without engineering.
- **Effort**: 2–3 weeks.

### Phase 7 — Hardening & release
- **Goal**: production-ready.
- **Work**: perf (settlement <2s, batch SLAs), security review (the financial-grade pass), full a11y audit (WCAG 2.1 AA, no regressions), load/concurrency, docs, runbooks.
- **Exit criteria**: ✅ perf SLAs met · ✅ security review passed · ✅ a11y AA verified · ✅ CI gates all green.
- **Effort**: 2–3 weeks.

**Total honest range: ~20–27 engineer-weeks** for a small team, sequenced. Not "14 weeks to 2,060 tests" — real, verifiable increments.

---

## 5. Cross-cutting concerns (apply in every phase)

- **Contracts first.** No UI or test is written until the slice's API contract is pinned and documented. The contract is the alignment artifact.
- **Provenance everywhere.** Every AI/agent output carries the provenance affordance (§UIUXplan §6). Non-negotiable for RBI FREE-AI.
- **Money discipline.** One Money component; nothing formats paise inline; tested exhaustively.
- **Security.** Preserve HttpOnly+CSRF patterns; irreversible actions get tiered friction; never weaken what's already at ~100% (CSRF/cookies).
- **CI gates** (added in Phase 0, enforced thereafter): build must pass · tests must pass · coverage must not drop · a11y AA must hold · (from Phase 5) failure-mode coverage must hold.

---

## 6. Testing & evaluation strategy — and the truth about "100%"

### 6.1 Why "100% line coverage" is an anti-goal (critical thinking, as requested)
Chasing 100% line coverage is exactly what produced the test theater earlier this session: tests written to *touch* lines, asserting on data they just created, testing nothing. 100% line coverage with worthless assertions is **worse than 80% with real ones** — it gives false confidence and rots maintenance.

**What we target instead — criticality-tiered, meaningful coverage:**

| Tier | Code | Line-coverage target | Assertion standard |
|------|------|---------------------|--------------------|
| **Critical** | money math, settlement, compliance decisions, CBDC ledger, auth/CSRF | **90–95%** | every branch + error path + concurrency + boundary (0, max, negative) |
| **Core** | handlers, agents, lineage, payment flow | **80–90%** | happy + error + key edge cases, via httptest against real contracts |
| **Glue** | wiring, config, simple DTOs | **best-effort** | smoke tests; don't write theater to inflate |

**The "100%" we *do* commit to** are the ones that are meaningful and verifiable:
- **100% of E2E workflows** — every user journey in every workspace has at least one real Playwright path (happy) + its critical error paths.
- **100% failure-mode coverage** — all 40 modes have ≥5 golden cases, ≥1 judge validation, ≥1 E2E.
- **100% of critical money/compliance paths** exercised with real assertions.

This is *more* rigorous than "100% lines," not less — it just refuses to count theater.

### 6.2 The four test layers (per slice, real only)
1. **Unit** — pure logic, real functions, real assertions (e.g. netting math with known inputs/outputs).
2. **Handler** — `httptest` against the *pinned contract*: status codes, error shapes, auth, CSRF, money correctness.
3. **E2E** — Playwright against the running stack: full journeys + corner cases (concurrency, timeout, retry, session expiry).
4. **Evaluation** — judges + golden datasets + failure-mode coverage for AI decisions.

### 6.3 Evaluation lifecycle (Analyze → Measure → Improve)
- **Judges**: settlement, compliance, orchestration, lineage — each calibrated to **TPR/TNR ≥ 0.90** on a held-out test set with 95% CIs (bootstrapped). A judge below bar is fixed before it gates anything.
- **Failure modes**: the 40-mode taxonomy; each with golden exemplars; coverage enforced in CI.
- **Online (later)**: sample production traces, run judges, track drift — out of scope until post-release but designed for.

---

## 7. The architecture fork to decide now (§8 of UIUXplan, restated for commitment)

Current UI is vanilla JS, zero-build, minimal supply chain — a real virtue for an auditable fintech. But five rich, real-time consoles in hand-rolled JS will be slow and brittle.

- **A. Stay vanilla** (web components + tokens) — keep audit simplicity; pay in build velocity.
- **B. Svelte for dense workspaces, keep Assistant near-vanilla** — *recommended*: reactivity pays off for timelines/fleet, compile-away keeps bundle small/auditable.
- **C. React** — biggest talent pool, heaviest supply chain (worst for financial audit).

**Decision owner**: front-end + security leads. **This blocks Phase 1**, so decide first. My lean remains **B**, but it's a staffing/audit-posture call, not purely technical.

---

## 8. Risks, forks, and what to validate

**Top risks**
1. **Persona model may not match reality** (UIUXplan §12) — *the riskiest assumption.* If buyers are banks (internal ops) not merchants, Phase 2 may not be Commerce. **Validate who logs in / who buys before committing Phase 2 scope.**
2. **`agents/kyc` breakage may be the tip of an iceberg** — Phase 0 must scope the full extent of compile/test debt, not just the surface error.
3. **Contract drift** — if backend responses change under the UI, tests rot. Mitigation: contracts are committed artifacts; CI checks them.
4. **Judge calibration may not hit 0.90** — if a judge can't be calibrated, it can't gate; document and fall back to human review for that mode.
5. **Framework adoption cost** (if B/C) — budget Phase 1 accordingly.

**Validate before heavy build** (cheap, high-leverage): the §12 persona/buyer questions; regulatory UI requirements beyond "have an audit trail"; whether fleet status must be truly live (WS) or periodic.

---

## 9. Definition of done & success metrics

**A slice is done when**: contract pinned & documented · backend conforms · UI works against real backend · unit+handler+E2E green with real assertions · domain judge calibrated (where AI decisions exist) · exit criteria met · CI gates hold.

**The product is done when**: all five workspaces shipped · build+test+coverage+a11y+failure-mode CI gates green · perf/security/a11y hardening passed · a regulator can reconstruct any decision via the Audit lens.

**Success metrics** (outcomes, not vanity):
- Merchant: self-serve "where's my money"; support tickets ↓.
- Compliance: clear-cut decision <60s; 100% audit-trail completeness; false-positive reversals ↓.
- Ops: incident ack time; HITL SLA adherence; kill-switch accidents = 0.
- Eval: all judges ≥0.90 TPR/TNR; 100% failure-mode coverage; annotation throughput.
- Trust/quality: provenance-drawer engagement; WCAG 2.1 AA maintained; **zero non-compiling or theater tests in the repo** (a direct lesson from this session).

---

## 10. Immediate next actions (first two weeks)

1. **Approve this plan** (or redirect — esp. the §8 persona question and §7 framework fork).
2. **Phase 0 kickoff**: fix `agents/kyc`; audit & purge test theater; commit the honest coverage map; wire the CI gate (build+test+coverage).
3. **In parallel**: answer the persona/buyer validation questions (UIUXplan §12) to lock Phase 2 scope.
4. **Decide the framework fork** so Phase 1 can start clean.

---

## Appendix A — Mapping to existing endpoints (so the plan is concrete, not abstract)

| Workspace | Primary endpoints (already exist) |
|-----------|-----------------------------------|
| Assistant | `/ask`, `/ask/stream`, `/chat/ws`, `/documents`, `/auth/*` |
| Commerce | `/commerce/order`, `/payment`, `/settlement`, `/merchant`, `/onboard`, `/account(s)`, `/transaction`, `/history/{txn_id}` |
| Compliance | `/compliance`, `/aml`, `/check(/{id})`, `/score`, `/account/{id}/velocity`, `/fraud-history`, `/limits/{id}`, `/admin/reset-velocity` |
| Gov/Safety | `/killswitch`, `/hitl/approvals`, `/elevation/requests`, `/incidents`, `/opa`, `/consent`, `/grant`, `/revoke`, `/trust/{agentID}`, `/audit` |
| Evaluation | `/evaluate`, `/traces`, `/clusters`, `/feedback`, `/score`, `/search`, `/lineage` |
| Audit lens | `/lineage`, `/audit/{consent_id}`, `/export/{entity_id}`, `/history/{txn_id}` |

The backend already exposes what the five workspaces need. The work is **contract-hardening, UI, real tests, and calibrated evaluation** — sliced vertically, proven phase by phase.

---

*End of master plan. Honest about the start, rigorous about "done," and structured so that no phase can claim completion it hasn't earned.*
