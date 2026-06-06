# Genie — UI/UX Representation Plan

**Author**: Claude (advisory)
**Date**: 2026-06-06
**Status**: Draft for discussion — opinionated, decision-oriented
**Scope**: How to represent the Genie platform through a coherent UI/UX, grounded in the system as it actually exists today.

> This is not a feature wishlist. It is a critical analysis of *what Genie is*, *who uses it*, *where the current UI falls short*, and *what to build, in what order, and why*. Where I make a recommendation, I say so and give the trade-off. Where there's a genuine fork, I flag it for a human decision instead of pretending there's one right answer.

---

## 1. Grounding: what Genie actually is (verified from the codebase)

Before designing anything, the honest current state:

**Backend** exposes ~80 routes spanning five distinct domains:
- **Conversational AI** — `/ask`, `/ask/stream` (SSE), `/chat/ws` (WebSocket), `/documents`
- **e-Rupee / CBDC commerce** — `/commerce/order`, `/payment`, `/settlement`, `/merchant`, `/onboard`, `/account(s)`, `/transaction`, `/block`, `/transfer`
- **Compliance & risk** — `/compliance`, `/aml`, `/check`, `/score`, `/account/{id}/velocity`, `/account/{id}/fraud-history`, `/limits/{id}`, `/admin/reset-velocity`
- **AI governance & safety** — `/killswitch`, `/elevation/requests`, `/hitl/approvals`, `/incidents`, `/opa`, `/consent`, `/grant`, `/revoke`, `/audit`, `/trust/{agentID}`
- **Evaluation** — `/evaluate`, `/traces`, `/clusters`, `/feedback`, `/score`, `/search`, `/lineage`

**Frontend** is ~3,062 lines of dependency-free vanilla JS/HTML/CSS:
- `index.html` + `app.js` — auth, a single "Ask" chat view, documents, a thin governance tab, settings. Session via HttpOnly cookie + CSRF token (well-implemented).
- `eval_review.html` + `eval_review.js` — a separate trace-annotation workbench (pass/fail, failure mode, confidence, keyboard shortcuts).
- A real accessibility effort already exists (`a11y-verify.py`, `A11Y_VERIFICATION.md`, ARIA roles in markup).

**The gap**: the UI surfaces maybe 15–20% of the platform's capability. The rich parts of Genie — multi-agent orchestration, settlement state machines, compliance case-work, governance controls, RBI FREE-AI audit trails — have **no representation** in the UI at all.

**The most important observation**: the team *already split* evaluation into its own page (`eval_review.html`). That instinct is correct and is the seed of this entire plan.

---

## 2. The central insight: Genie is five products wearing one URL

The single biggest UX decision is recognizing that Genie is **not one app**. It is five products with disjoint users, mental models, and information densities:

| # | Product surface | Primary user | Core mental model | Density |
|---|----------------|--------------|-------------------|---------|
| 1 | **Customer Assistant** | Retail customer | "Ask a question, get a trustworthy answer" | Low, conversational |
| 2 | **Merchant Commerce** | Merchant / ops | "Where is my money?" (orders → payment → settlement → payout) | Medium, transactional |
| 3 | **Compliance Console** | Compliance officer | "Triage cases, decide, defend the decision" | High, case-centric |
| 4 | **Governance & Safety Ops** | Risk operator | "Are the agents behaving? Can I stop them?" | High, monitoring + control |
| 5 | **Evaluation Workbench** | ML / eval engineer | "Where do the agents fail, and is my judge trustworthy?" | Very high, analytical |

Trying to serve all five through one navigation tree produces a UI that is wrong for everyone. **The plan is therefore role-based workspaces sharing one shell**, not a single mega-app.

This is the load-bearing decision. Everything below follows from it.

---

## 3. Critical analysis: what this domain *demands* of the UI

Fintech + autonomous AI agents + RBI FREE-AI regulation impose constraints most apps don't have. A generic dashboard template will fail here. Specifically:

**3.1 Provenance is not optional — it's the product.**
RBI FREE-AI mandates human oversight and complete audit trails. Every AI-produced number or decision must be able to answer, in one click: *which agent produced this, from what inputs, with what confidence, and where is the audit/lineage record?* The `/lineage` and `/audit` endpoints exist precisely for this. **Provenance must be a first-class, ever-present UI pattern**, not a buried "details" link.

**3.2 Money must be unambiguous.**
The backend stores amounts in **paise** (`total_paise`, `amount_paise`). The #1 source of catastrophic fintech UI bugs is paise/rupee confusion. The design system must own money formatting centrally (one component, never ad-hoc), always show currency, and make magnitude unmistakable (₹1,00,000.00 Indian grouping).

**3.3 State is the story, for both money and workflows.**
Settlement runs a state machine: OrderCreated → PaymentInitiated → PaymentConfirmed → SettlementInitiated → SettlementCompleted → Fulfilled. Users don't want a status *string*; they want a **visual state machine** showing where a transaction is, what's next, what's blocked, and what can't be undone. Same for compliance decisions (APPROVED/BLOCKED/PENDING) and HITL approvals.

**3.4 Irreversibility demands friction by design.**
Kill-switch, fund transfer, settlement confirmation, compliance BLOCK, consent revoke — these are irreversible or high-consequence. The UI must apply *deliberate* friction (typed confirmation, two-step, reason capture) **only** to these, while keeping everything else frictionless. Friction everywhere = friction ignored.

**3.5 Multi-agent observability is novel UX territory.**
60+ specialist agents orchestrate handoffs. There is no off-the-shelf pattern for "watch a swarm of financial agents." Users need: which agents are active, who handed off to whom, agent trust score (`/trust/{agentID}`), and where a request currently sits. This is closer to a *distributed-tracing UI* (think a trace waterfall) than a CRUD dashboard.

**3.6 Compliance is adversarial and high-stakes.**
A false BLOCK frustrates a legitimate merchant; a false APPROVE is a legal/financial breach. The compliance console is *case triage software*: fast queue, evidence side-by-side, one-glance risk signals (AML score, velocity, sanctions match, PEP), and a decision trail that defends the officer later.

**3.7 Trust is earned through calibrated confidence, not hidden.**
Showing an AI answer as if it were fact erodes trust the moment it's wrong. Surface confidence, show sources/retrieved documents, and make "the system is unsure → routed to a human" a *visible, reassuring* state, not a silent failure.

---

## 4. Information architecture

```
┌──────────────────────────────────────────────────────────────┐
│  GENIE SHELL                                                   │
│  ┌──────────┐                                  ┌────────────┐  │
│  │ Workspace│  Top bar: env badge, user, role  │ Provenance │  │
│  │ switcher │  search, notifications, kill-     │ drawer     │  │
│  │ (role-   │  switch (guarded, ops only)       │ (global,   │  │
│  │  scoped) │                                   │  on-demand)│  │
│  └──────────┘                                   └────────────┘  │
│                                                                │
│   WORKSPACE (one of five, gated by role/permission)            │
│   1 Assistant   2 Commerce   3 Compliance   4 Ops   5 Eval     │
│                                                                │
│   + Audit/Regulator read-only lens (cross-cutting)             │
└──────────────────────────────────────────────────────────────┘
```

- **One shell, role-switched workspaces.** A user only sees workspaces their role grants. A retail customer sees *only* the Assistant. A compliance officer sees Compliance (+ Audit lens). An admin may see all.
- **Global provenance drawer.** Slides in from the right on any "why?" affordance, anywhere. Fed by `/lineage`, `/audit`, agent trace. This is the connective tissue that satisfies FREE-AI and builds trust.
- **Global command palette / search** (`/search`) — power users jump to an order, account, trace, or case by ID.
- **The kill-switch lives in the shell**, not a workspace — it's a platform-level emergency control, permission-gated, visually distinct, behind confirmation.

---

## 5. Workspace-by-workspace design

Each workspace below lists: **user · jobs-to-be-done · key screens · signature components · backing endpoints · "done" criteria.** Components reuse the shared design system (§7).

### 5.1 Customer Assistant
- **User**: retail customer. **JTBD**: get a trustworthy financial answer; act on it.
- **Key screens**: conversation (streaming), answer with sources, document upload/context, action hand-off (e.g., "start a payment").
- **Signature components**:
  - *Streaming answer* with token-by-token render (`/ask/stream` SSE, `/chat/ws`).
  - *Source/confidence chips* under each answer — click → provenance drawer (which agent, retrieved docs, confidence).
  - *"Routed to a human" state* when the system is unsure (ties to HITL) — framed as care, not failure.
- **Endpoints**: `/ask`, `/ask/stream`, `/chat/ws`, `/documents`, `/auth/*`.
- **Done when**: a customer can ask, see the answer stream in, understand *why* it's trustworthy, and never sees a raw error or silent stall.

### 5.2 Merchant Commerce
- **User**: merchant / merchant-ops. **JTBD**: "Where is my money and why?"
- **Key screens**: order list + detail, the **settlement state-machine timeline**, payouts/reconciliation, merchant onboarding (`/onboard`), account & balance.
- **Signature components**:
  - *Settlement timeline* — horizontal state machine (Created→…→Fulfilled) with timestamps, current node highlighted, blocked/failed nodes in alert color, each node expandable to its lineage entry.
  - *Money ledger row* — central money component; paise→₹ formatting, never ambiguous.
  - *Reconciliation badge* — VERIFIED / MISMATCH with one-click drill into the discrepancy.
  - *Netting view* — when batch settlement nets across merchants, show the consolidation math transparently.
- **Endpoints**: `/commerce/order`, `/payment`, `/settlement`, `/merchant`, `/onboard`, `/account(s)`, `/transaction`, `/transaction/{id}`, `/history/{txn_id}`.
- **Done when**: a merchant can locate any order's exact state, see the money math, and trust the payout — without contacting support.

### 5.3 Compliance Console
- **User**: compliance officer. **JTBD**: triage cases, decide, defend the decision.
- **Key screens**: case queue (sortable by risk), case detail with evidence, decision capture, audit history.
- **Signature components**:
  - *Risk signal cluster* — AML score (0–100 with band), velocity gauge vs limit, sanctions-match indicator, PEP flag, KYC status — all visible **without scrolling**.
  - *Decision panel* — APPROVE / BLOCK / ESCALATE with **mandatory reason capture** (BLOCK is high-consequence → typed confirmation).
  - *Evidence side-by-side* — customer/merchant profile next to the triggering transaction.
  - *Decision trail* — every prior decision on this entity, who/when/why (defends the officer in audit).
- **Endpoints**: `/compliance`, `/aml`, `/check`, `/check/{id}`, `/score`, `/account/{id}/velocity`, `/account/{id}/fraud-history`, `/limits/{id}`, `/admin/reset-velocity` (guarded).
- **Done when**: an officer can clear a case in <60s for clear-cut ones, with full evidence and an audit-defensible record, and false-positive friction is minimized.

### 5.4 Governance & Safety Ops
- **User**: risk/governance operator. **JTBD**: confirm agents are behaving; intervene when not.
- **Key screens**: agent fleet status, HITL approval inbox, incidents, policy (OPA) view, consent registry, **kill-switch** (shell-level).
- **Signature components**:
  - *Agent fleet board* — 60+ agents as a health grid; trust score (`/trust/{agentID}`), active/idle/degraded, last action. Drill into an agent's recent traces.
  - *HITL approval inbox* — pending elevations/approvals (`/hitl/approvals`, `/elevation/requests`) with full context to approve/deny; SLA timers.
  - *Incident stream* (`/incidents`) — severity-ranked, with linked lineage.
  - *Kill-switch* — guarded, two-step, reason-required; shows blast radius before firing.
- **Endpoints**: `/killswitch`, `/hitl/approvals`, `/elevation/requests`, `/incidents`, `/opa`, `/consent`, `/grant`, `/revoke`, `/trust/{agentID}`, `/audit`.
- **Done when**: an operator can see fleet health at a glance, action HITL requests with full context, and reach emergency controls fast but never fire them by accident.

### 5.5 Evaluation Workbench
- **User**: ML / eval engineer. **JTBD**: find where agents fail; trust the judges.
- **Key screens**: trace queue + annotation (the existing `eval_review.html`, evolved), failure-mode dashboard, judge calibration, cluster explorer.
- **Signature components**:
  - *Trace annotator* (already exists) — keep keyboard-first flow; add inline lineage.
  - *Failure-mode dashboard* — distribution across the 40-mode taxonomy, trends, drill to exemplar traces.
  - *Judge calibration view* — TPR/TNR with confidence intervals, confusion matrix, calibration curve; makes "is this judge trustworthy?" answerable.
  - *Cluster explorer* (`/clusters`, `/search`) — semantic grouping of failures.
- **Endpoints**: `/evaluate`, `/traces`, `/clusters`, `/feedback`, `/score`, `/search`, `/lineage`.
- **Done when**: an engineer can annotate efficiently, see failure distribution, and judge whether a judge meets the TPR/TNR bar — all in one workspace.

### 5.6 Audit / Regulator lens (cross-cutting, read-only)
- **User**: internal auditor or RBI examiner. **JTBD**: verify the system is governable and traceable.
- **Design**: not a sixth silo — a **read-only lens** that renders any entity (order, decision, agent action) with its full lineage and hash-chain integrity status. Exports (`/export/{entity_id}`).
- **Done when**: an examiner can pick any decision and reconstruct the complete, tamper-evident trail without engineering help.

---

## 6. The signature pattern: provenance-first design

This is the one idea that, if done well, differentiates Genie. Every AI/agent output carries a small, consistent **provenance affordance**:

```
┌─ Answer / amount / decision ────────────────┐
│  "Settlement: ₹1,00,000.00  ✓ Reconciled"   │
│  ⓘ via settlement-coordinator · conf 0.98 · │
│     2 agents · lineage ↗                     │
└──────────────────────────────────────────────┘
        click ⓘ →  Provenance drawer:
        ├─ Agent chain (waterfall): who did what, when
        ├─ Inputs consumed
        ├─ Confidence + judge verdict (if evaluated)
        ├─ Lineage hash-chain (integrity ✓/✗)
        └─ Audit entry link + export
```

One pattern, used everywhere, fed by `/lineage` + `/audit` + agent trace. It simultaneously: builds user trust, satisfies FREE-AI oversight, and gives support/audit a single mental model.

---

## 7. Cross-cutting design system

- **Design tokens**: color (semantic: success/warn/danger/info + risk bands), spacing, type scale, radii — defined once as CSS custom properties (the codebase already uses plain CSS, so tokens fit naturally).
- **Money component**: the single source of truth for paise→₹ rendering, Indian digit grouping, sign, currency. *Nothing* formats money inline.
- **State machine / timeline component**: reused by settlement, compliance decision, HITL, workflow.
- **Risk-signal components**: score band, velocity gauge, match indicator — shared by Compliance and Ops.
- **Provenance drawer**: global, §6.
- **Confirmation patterns**: tiered — light (toast-undo) for reversible, heavy (typed confirm + reason) for irreversible.
- **Real-time primitives**: SSE consumer for streaming answers; WebSocket consumer for live agent/fleet status; polling fallback.
- **Accessibility**: build on the existing a11y work — WCAG 2.1 AA as a gate, keyboard-first (the eval workbench already is), live regions for streaming/alerts, focus management in drawers/modals. Don't regress what's already verified.
- **Empty/error/loading states**: every async surface has all three designed — no raw spinners-forever, no unstyled errors (the customer assistant especially).

---

## 8. The architecture fork you must decide (honest trade-off)

The current UI is **vanilla JS, zero build step, zero dependencies** — a real virtue for a security-sensitive fintech (tiny supply-chain surface, trivial to audit). But five rich, stateful, real-time consoles in hand-rolled vanilla JS will become very expensive to build and maintain.

**This is a genuine fork. I won't pretend it's obvious.**

| Option | Pros | Cons | Best when |
|--------|------|------|-----------|
| **A. Stay vanilla** (web components + tokens) | No build, minimal supply chain, easy to audit, no churn | Slow to build complex stateful views; you'll reinvent routing/state | The team is small, security/audit is paramount, scope stays modest |
| **B. Lightweight framework** (Svelte / SolidJS) | Fast to build, small bundle, reactive state for real-time, still auditable | Adds a build step + some deps | You're committing to all 5 workspaces and real-time fleet views |
| **C. React + ecosystem** | Largest talent pool, richest component libs | Heaviest supply chain (worst for a fintech audit), bundle size | You need to hire fast and lean on existing component libraries |

**My recommendation**: **B (Svelte) for the high-density workspaces (Commerce, Compliance, Ops, Eval), keep the Customer Assistant near-vanilla** if you want it embeddable/lightweight. Rationale: reactivity genuinely pays off for settlement timelines and live fleet status; Svelte's compile-away model keeps the audited bundle small; and you avoid React's dependency weight, which matters more than usual in a regulated financial product. **But this is a team decision** about staffing and audit posture, not a purely technical one — flag it for whoever owns front-end strategy.

---

## 9. Critical risks & anti-patterns to avoid

- **❌ One dashboard to rule them all.** The fastest way to make this fail. Honor the five-workspace split.
- **❌ Provenance as an afterthought.** If lineage is a buried link, FREE-AI compliance and user trust both suffer. It's a first-class pattern or it's nothing.
- **❌ Ad-hoc money formatting.** Guarantees a paise/rupee incident eventually. Centralize on day one.
- **❌ Friction everywhere.** If every click confirms, users reflexively confirm the dangerous ones too. Reserve friction for the irreversible.
- **❌ Hiding AI uncertainty.** Presenting low-confidence answers as fact destroys trust permanently. Show confidence; make human-routing visible.
- **❌ Designing the fleet view as a table.** 60+ agents with handoffs is a *trace/graph* problem, not a CRUD grid.
- **❌ Over-building before validating.** See §12 — several assumptions here need user contact before heavy investment.

---

## 10. Phased roadmap (grounded, not fantasy)

Sequenced by *user value × foundation-laying*, each phase tied to endpoints that already exist.

- **Phase 0 — Foundation (shell + design system).** Workspace shell, role-gated nav, design tokens, the **Money** and **Provenance drawer** components, auth/session hardening. *Everything else depends on these.*
- **Phase 1 — Merchant Commerce.** Highest concrete value ("where's my money"), exercises money + state-machine + provenance + reconciliation. Proves the design system.
- **Phase 2 — Compliance Console.** High stakes, clear ROI (officer efficiency + audit defense). Reuses risk components.
- **Phase 3 — Governance & Safety Ops.** Fleet board, HITL inbox, kill-switch. The novel multi-agent UX; do it after the patterns are proven.
- **Phase 4 — Evaluation Workbench v2.** Evolve the existing eval page into failure-mode + judge-calibration dashboards.
- **Phase 5 — Customer Assistant polish + Audit lens.** Harden streaming/sources/confidence; ship the read-only regulator lens.

Each phase ships an independently useful workspace. No phase is blocked on a later one.

---

## 11. Success metrics (UX outcomes, not vanity)

- **Merchant**: % of "where's my money" answered self-serve (target: support tickets ↓); time-to-locate an order's state.
- **Compliance**: median time-to-decision for clear-cut cases; false-positive friction (decisions reversed on appeal); % decisions with complete audit trail (target 100%).
- **Ops**: time-to-acknowledge an incident; HITL approval SLA adherence; kill-switch reachable in <3 clicks, fired-by-accident = 0.
- **Eval**: traces annotated/hour; % failure modes with exemplars; judges meeting TPR/TNR bar.
- **Trust (cross-cutting)**: % AI outputs where a user opened provenance (engagement with transparency); accessibility = WCAG 2.1 AA maintained (no regressions vs current verified state).

---

## 12. What I'd validate *before* building (honest assumptions to test)

This plan rests on assumptions that deserve a reality check with real users — cheaper to test now than to rebuild later:

1. **Are these really five distinct user populations**, or does one person wear several hats (e.g., a small-merchant who is also their own compliance/ops)? If roles collapse, the workspace split changes.
2. **Is the Customer Assistant even a product surface you ship to end-customers**, or is Genie a B2B/internal platform where "customers" are actually staff? This flips priorities (Assistant may be lowest priority, not a given).
3. **Who is the buyer/primary user** — banks (internal ops/compliance/governance) or merchants? That determines which workspace is Phase 1.
4. **Regulatory UI requirements** — does RBI FREE-AI prescribe specific audit/oversight UI affordances we must conform to, beyond "have an audit trail"?
5. **Real-time expectations** — does fleet status need to be live (WebSocket) or is periodic refresh acceptable? Affects architecture (§8).

I'd run lightweight interviews / look at who actually logs in today before committing Phase 1 scope.

---

## 13. Summary

- Genie is **five products**, not one. The whole plan flows from designing **role-based workspaces on a shared shell**.
- The domain (fintech + autonomous agents + RBI FREE-AI) makes **provenance, unambiguous money, visible state, calibrated confidence, and guarded irreversibility** non-negotiable — these are the product, not decoration.
- The current vanilla-JS UI is a fine, auditable foundation but won't scale to five rich consoles unaided → a **framework decision (§8)** is the key technical fork; my lean is Svelte for dense workspaces, flagged for human sign-off.
- Build in value-ordered phases (Commerce → Compliance → Ops → Eval → Assistant/Audit), each independently shippable.
- **Validate the persona/buyer assumptions (§12) before heavy investment** — the riskiest part of this plan is not the design, it's whether the five-persona model matches reality.

---

*End of plan. This document is intentionally opinionated; every recommendation is open to challenge with better information about users and business goals.*
