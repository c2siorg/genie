# pkg/agent.Tier — promotion pipeline for AI-generated and human-authored agents

> **Where:** `pkg/agent/tier.go`
> **Lines of code:** ~90 · **Tests:** 6 unit + integration coverage in `tests/security_envelope_test.go`
> **FREE-AI alignment:** Rec 17 (Product Approval), Rec 23 (AI Inventory)

---

## Overview

A four-stage promotion model that production AI shops have been
converging on for AI-generated code. The original problem: an
engineer pastes a prompt into Cursor, gets a working agent, and
deploys it to production within an hour. The agent looks fine; it
passes the demo; nothing in the codebase tells the on-call team
that the new agent has never been red-teamed, has no fallback, and
has no audit hooks. By the time anyone notices, real customer
traffic has touched it.

`Tier` is the small, mandatory column that fixes this. Every agent
advertises one of four tiers — Sketch, Prototype, Beta, Production
— and the orchestrator refuses to dispatch customer-facing traffic
to anything below Production unless the host is in sandbox mode.
The risk team reads the tier off the `/v1/ai-inventory` endpoint
(FREE-AI Rec 23) and gates promotion at policy review.

Default-to-Prototype is intentional: an agent that doesn't advertise
a tier is treated as "engineer owns it, but not blessed for
production" — never as Production. The dispatch gate fails closed,
which is the safe choice.

---

## Surface

```go
type Tier string

const (
    TierSketch     Tier = "sketch"     // AI-generated prototype, sandbox only
    TierPrototype  Tier = "prototype"  // engineer owns it; basic tests; dev RBAC
    TierBeta       Tier = "beta"       // passes governance composite; staging; metrics
    TierProduction Tier = "production" // red-teamed; fallback; in inventory; audit-traced
)

// TierAware is the optional interface an agent advertises.
type TierAware interface {
    Tier() Tier
}

// TierOf returns the agent's declared tier, or TierPrototype if unspecified.
// The default is intentional: undeclared = not production.
func TierOf(a Agent) Tier

// Production reports whether the tier is TierProduction.
func Production(t Tier) bool

// TierOrdinal — Sketch=0, Prototype=1, Beta=2, Production=3, unknown=-1.
func TierOrdinal(t Tier) int

// AtLeast reports whether got is at least required.
func AtLeast(got, required Tier) bool
```

---

## The four tiers

| Tier | Who writes it | Tests | RBAC | Audit | Eligible to dispatch on |
|---|---|---|---|---|---|
| **Sketch** | AI scaffolding tool, untouched | None or generated | None | None | Sandbox env only |
| **Prototype** | Human engineer owns it, AI may have started it | Unit tests only | Dev token | Per-call log | Internal dev/test traffic |
| **Beta** | Human-authored; ready for staging | Unit + integration | Staging RBAC | Per-call audit + metrics | Staging customers (consent) |
| **Production** | Human-authored, reviewed | Unit + integration + adversarial corpus | Prod RBAC | Hash-chained audit | Customer-facing prod traffic |

The promotion criteria mirror the FREE-AI Rec 17 (Product Approval)
gate. Each promotion is a decision artefact — board minutes or risk
review — not just a code change. The model below the human author is
the policy: "you don't get to grant your own production tier."

---

## Default-to-Prototype rationale

Why "undeclared = Prototype" rather than "undeclared = compile error":

- A compile error would force every existing agent to be touched on
  the same change that adds the tier model. That's a flag day.
- A compile error would also prevent third-party agents (e.g.
  experiments in `/agents/research/`) from existing without being
  promoted.
- A default of *Production* would let an agent slip past the gate
  by simply not implementing `TierAware`. The dispatch gate would
  then have to inspect every agent and reject the missing
  declaration — same effect, more code.

Prototype is the right default: it lets agents exist and run in
test environments, but the production dispatch gate refuses them
until the human explicitly says "Tier() returns TierProduction."

---

## Wire-up at the orchestrator

The host integrates the tier check at message dispatch:

```go
target := registry.Get(ctx, msg.To)
if !agent.Production(agent.TierOf(target)) && env != "sandbox" {
    return ErrTierNotPermitted
}
```

In Genie's reference orchestrator, this lives in the policy stack
(see `tests/security_envelope_test.go::tierPolicy`) so it composes
with the other governance checks — RBAC, tenant, content length —
and shares the same denial-emission and metrics path.

```go
policy := governance.NewComposite(
    tierPolicy{reg: reg},                    // ← here
    governance.TenantPolicy{...},
    governance.RBACPolicy{...},
    governance.MaxContentLengthPolicy{Max: 16 * 1024},
)
```

When a Sketch-tier agent receives a customer-facing message, the
policy returns a denial whose reason starts with `"tier below
production"`. The orchestrator emits an `OnPolicyDeny` hook so the
on-call team can wire a metric and an alert.

---

## Inventory integration

The `/v1/ai-inventory` handler (FREE-AI Rec 23) emits the tier as a
JSON field on every agent:

```json
{
  "id": "kyc_orchestrator",
  "name": "KYC Orchestrator",
  "capabilities": ["kyc.decide"],
  "risk_class": "high",
  "tier": "production",       ← new column
  "has_fallback": true,
  "fallback_id": "kyc_fallback"
}
```

The risk team reads this column to spot non-production agents
serving customer traffic. The corresponding UI test
(`pkg/web/handlers/ui_security_test.go::TestInventory_ListIncludesTier`)
pins the field name so a refactor cannot accidentally drop it.

---

## Risk class vs tier

`pkg/agent.RiskClass` (low/medium/high) and `pkg/agent.Tier`
(sketch/prototype/beta/production) describe different things:

| | RiskClass | Tier |
|---|---|---|
| What it measures | Impact when the agent fails or is misused | Maturity of the agent's engineering process |
| Set by | The capability owner (board policy) | The promotion gate (engineering + risk review) |
| Changes how often | Rarely — risk class is a function of what the agent does | Every promotion step |
| Read at | Governance policy decisions, capability disclosure | Dispatch gate, inventory, BCP drill scope |

Both are inputs to the orchestrator's dispatch decision. A
`RiskHigh` Sketch-tier agent must not run in production *because*
its blast radius is large *and* it hasn't been vetted. A `RiskLow`
Sketch-tier agent might still be allowed in a sandbox.

---

## What this package does *not* do

- **It doesn't run the red-team corpus.** That lives in
  `pkg/safety` plugins and the per-agent test suites. The tier
  attribute *records* the corpus has been run; it doesn't run it.
- **It doesn't enforce the dispatch gate by itself.** Enforcement
  is the orchestrator's job (or any policy stack the host wires).
  This package supplies the predicate and the data model.
- **It doesn't track tier history.** When an agent's tier changes,
  the change is in git and in the deployment record — there's no
  "previous tier" field on the runtime value.
- **It doesn't manage feature-flag rollouts.** Per-tenant beta
  rollouts are an orthogonal concern; combine `TierBeta` with a
  tenant allowlist in the host.

---

## Tests

`pkg/agent/tier_test.go` covers:

| Test | Asserts |
|---|---|
| `TestTierOfReturnsDeclaredTier` | Production / Sketch agents report their declared tier |
| `TestTierOfDefaultsToPrototype` | Undeclared agents default to Prototype, not Production |
| `TestProductionPredicate` | Only TierProduction passes `Production()` |
| `TestTierOrdinalOrdering` | Ordinals are strictly increasing across declared tiers |
| `TestAtLeastFloor` | Floor check accepts equal and higher tiers |
| `TestUnknownTierOrdinalIsNegative` | Unknown tier returns -1 so it fails `AtLeast` against any required floor |

Integration:
- `tests/security_envelope_test.go::TestSecurityEnvelope_SketchTierIsBlocked` — end-to-end denial of a Sketch-tier agent through the policy stack.

---

## FREE-AI mapping

- **Rec 17 — Product Approval.** The tier model *is* the approval
  pipeline. Each promotion is a documented decision; the
  Production tier is the only one eligible for customer-facing
  dispatch.
- **Rec 23 — AI Inventory.** The inventory surface reads the tier
  field off every registered agent; the risk team uses it to spot
  agents that have skipped the promotion gate.

---

## Pointers

- Implementation: `pkg/agent/tier.go`
- Risk-class counterpart: `pkg/agent/risk.go`
- Inventory consumer: `pkg/web/handlers/inventory.go`
- Tests: `pkg/agent/tier_test.go`, `tests/security_envelope_test.go`,
  `pkg/web/handlers/ui_security_test.go`
- Promotion checklist: `docs/operations.md` — tier promotion section
