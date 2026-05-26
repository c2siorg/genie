// tier.go — promotion tiers for the AI-Assisted Development Standard.
//
// ─── What this file is ──────────────────────────────────────────────────────
//
// Modelled on the four-tier pipeline that production AI shops are
// converging on: Sketch (AI-generated prototype) → Prototype (someone
// owns it) → Beta (passes governance) → Production (passes adversarial
// verification, has fallback, audit-traced).
//
// Each tier has a different governance posture. The orchestrator
// enforces tier-appropriate guardrails — a Sketch agent can run in the
// sandbox but cannot dispatch on production traffic; a Production agent
// must declare RiskLevel(), have an inventory entry, and pass the red-
// team probe corpus.
//
// ─── The problem this solves ────────────────────────────────────────────────
//
// Without an enforced tier model, an engineer pastes a prompt into a
// scaffolding tool, gets a working agent, and deploys it within an
// hour. The agent looks fine; it passes the demo; nothing in the
// codebase tells the on-call team that the agent has never been red-
// teamed, has no fallback, and has no audit hooks. By the time anyone
// notices, real customer traffic has touched it.
//
// Tier is the small, mandatory column that fixes this. Every agent
// advertises one of four tiers — Sketch, Prototype, Beta, Production —
// and the orchestrator refuses to dispatch customer-facing traffic to
// anything below Production unless the host is in sandbox mode. The
// risk team reads the tier off the /v1/ai-inventory endpoint (FREE-AI
// Rec 23) and gates promotion at policy review.
//
// ─── Default-to-Prototype rationale ─────────────────────────────────────────
//
// An agent that doesn't advertise a tier is treated as "engineer owns
// it, but not blessed for production" — never as Production. The
// dispatch gate fails closed, which is the safe choice.
//
// Why this default instead of "compile error on undeclared":
//   - Compile error would force every existing agent to be touched on
//     the same change that adds the tier model. Flag day.
//   - Compile error would prevent third-party agents (experiments under
//     agents/research/) from existing at all without being formally
//     promoted, killing experimentation.
//   - A default of Production would let an agent slip past the gate by
//     simply not implementing TierAware. The dispatch gate would then
//     have to inspect every agent and reject the missing declaration —
//     same effect, more code, more chances to forget.
//
// Default-to-Prototype is the right balance: agents exist and run in
// test environments, but production refuses them until the human
// explicitly says "Tier() returns TierProduction."
//
// ─── FREE-AI alignment ─────────────────────────────────────────────────────
//
// Rec 17 (Product Approval) — the tier model is the approval pipeline
// for new AI capabilities. Each promotion is a documented decision
// (board minutes or risk review), not just a code change. "You don't
// get to grant your own production tier."
//
// Rec 23 (AI Inventory) — reads tier off every registered agent. The
// risk team scans the tier column on /v1/ai-inventory to spot non-
// production agents serving customer traffic.
package agent

// Tier names the promotion stage of an agent.
//
// String-typed (not an enum) because:
//   - Stable wire format — tier names appear in JSON over the inventory
//     endpoint and in logs. A string is portable.
//   - Forward compat — a future TierAlpha or TierCanary can be added
//     without breaking existing code that already parses the string.
//   - Easier debugging — a tier in a stack trace reads as "production"
//     not as "3".
//
// Tiers, lowest privilege to highest:
//
//   - TierSketch: AI-generated prototype, may not be production-tested.
//     Allowed only in sandbox environments.
//   - TierPrototype: an engineer owns it; basic tests; dev RBAC only.
//   - TierBeta: passes governance composite; staging RBAC; metrics wired.
//   - TierProduction: passes red-team corpus, has fallback, in inventory,
//     audit-traced. Eligible for customer-facing dispatch.
type Tier string

// Tier constant values. Keep these stable — the strings appear in JSON
// (inventory endpoint), in policy YAML (promotion thresholds), and in
// audit log details. Renaming any of them is a wire-breaking change.
const (
	// TierSketch — AI scaffold, often untouched by humans. May be
	// committed to a branch as a starting point but not eligible for
	// any RBAC, audit, or dispatch beyond the sandbox.
	TierSketch Tier = "sketch"

	// TierPrototype — a human engineer owns the agent. Has unit tests.
	// Allowed on dev tokens and internal test traffic. Default for
	// agents that don't declare a tier (see TierOf).
	TierPrototype Tier = "prototype"

	// TierBeta — passes the governance composite, has metrics wired,
	// has been in staging for some period the team agrees on. Allowed
	// on staged customers who have consented to beta features.
	TierBeta Tier = "beta"

	// TierProduction — passes the adversarial corpus (cmd/red-team),
	// has a fallback wired (orchestrator.SetFallback), is audit-traced,
	// is in the inventory. Eligible for customer-facing dispatch in
	// production environments.
	TierProduction Tier = "production"
)

// TierAware is the optional interface an agent advertises when it
// declares a tier. The orchestrator inspects this via type assertion;
// agents that don't implement it default to TierPrototype — explicitly
// non-production, so accidental omission denies dispatch rather than
// granting it.
//
// Optional-by-design: we don't extend the base Agent interface with a
// Tier() method because:
//   - It would force every existing agent (and every test fixture) to
//     implement the method to compile.
//   - The optional pattern via type assertion is idiomatic Go for cross-
//     cutting concerns (sql.Scanner, http.Hijacker, etc.).
//   - Adding a required method later is non-breaking (existing default
//     implementations adapt); removing one is. Keep optionality.
type TierAware interface {
	Tier() Tier
}

// TierOf returns the agent's declared tier, or TierPrototype if
// unspecified. The default is intentional: undeclared = not production.
//
// This is the only place outside of registry code that should reach for
// the tier — callers should use TierOf, not type-assert TierAware
// themselves. Centralising the default ensures every call site agrees
// on what "undeclared" means.
func TierOf(a Agent) Tier {
	// Type-assert through the optional interface. If the agent
	// implements TierAware, use its declared tier; otherwise fall
	// through to the safe default.
	if t, ok := a.(TierAware); ok {
		return t.Tier()
	}
	// Default: Prototype. Not Production. See file header for the
	// rationale.
	return TierPrototype
}

// Production reports whether the tier is TierProduction. The
// orchestrator uses this as the gate for customer-facing dispatch:
//
//	if !agent.Production(agent.TierOf(handler)) && env != "sandbox" {
//	    return ErrTierNotPermitted
//	}
//
// Why a helper instead of t == TierProduction at every call site:
//   - Self-documenting: Production(tier) reads as English.
//   - Single point of change — if a future tier (e.g. TierLockdown)
//     should also count as "production," we update one function.
//   - Catches typos — a string literal "production" compared at a
//     call site might be mis-spelled; the named constant doesn't.
func Production(t Tier) bool { return t == TierProduction }

// TierOrdinal returns a numeric rank for a tier. Higher ordinal =
// more vetted, more restrictive promotion gate.
//
// Used by AtLeast to express tier-floor checks. Returns -1 for unknown
// tiers so AtLeast(unknown, …) fails closed — an agent that returns a
// typoed tier (e.g. "produciton") gets -1, which is below every floor,
// and is rejected.
func TierOrdinal(t Tier) int {
	switch t {
	case TierSketch:
		return 0
	case TierPrototype:
		return 1
	case TierBeta:
		return 2
	case TierProduction:
		return 3
	}
	// Unknown tier — treat as below sketch. Fail-closed; any AtLeast
	// check returns false; any equality check returns false; any
	// Production check returns false. Typo-safe.
	return -1
}

// AtLeast reports whether the agent's tier is at least the required tier.
// Use this when a code path is conditional on a tier floor.
//
//	if !agent.AtLeast(agent.TierOf(handler), agent.TierBeta) {
//	    // refuse to route production traffic
//	}
//
// Why a helper instead of TierOrdinal(got) >= TierOrdinal(required) at
// each call site:
//   - Reads as English: AtLeast(got, beta) is clearer than the ordinal
//     arithmetic.
//   - Hides the ordinal encoding — a future migration to a different
//     encoding (e.g. ordered by enum order) doesn't touch the call
//     sites.
//   - Pairs naturally with the unknown-tier-is-negative behaviour, so
//     unknown tier always fails any floor check.
func AtLeast(got, required Tier) bool {
	return TierOrdinal(got) >= TierOrdinal(required)
}
