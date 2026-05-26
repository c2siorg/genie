// tier.go — promotion tiers for the AI-Assisted Development Standard.
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
// FREE-AI alignment: Rec 17 (Product Approval) — the tier model is the
// approval pipeline for new AI capabilities. Rec 23 (AI Inventory)
// reads tier off every registered agent.
package agent

// Tier names the promotion stage of an agent.
//
//   - TierSketch: AI-generated prototype, may not be production-tested.
//     Allowed only in sandbox environments.
//   - TierPrototype: an engineer owns it; basic tests; dev RBAC only.
//   - TierBeta: passes governance composite; staging RBAC; metrics wired.
//   - TierProduction: passes red-team corpus, has fallback, in inventory,
//     audit-traced. Eligible for customer-facing dispatch.
type Tier string

const (
	TierSketch     Tier = "sketch"
	TierPrototype  Tier = "prototype"
	TierBeta       Tier = "beta"
	TierProduction Tier = "production"
)

// TierAware is the optional interface an agent advertises when it
// declares a tier. The orchestrator inspects this via type assertion;
// agents that don't implement it default to TierPrototype — explicitly
// non-production, so accidental omission denies dispatch rather than
// granting it.
type TierAware interface {
	Tier() Tier
}

// TierOf returns the agent's declared tier, or TierPrototype if
// unspecified. The default is intentional: undeclared = not production.
func TierOf(a Agent) Tier {
	if t, ok := a.(TierAware); ok {
		return t.Tier()
	}
	return TierPrototype
}

// Production reports whether the tier is TierProduction. The
// orchestrator uses this as the gate for customer-facing dispatch:
//
//	if !agent.Production(agent.TierOf(handler)) && env != "sandbox" {
//	    return ErrTierNotPermitted
//	}
func Production(t Tier) bool { return t == TierProduction }

// Ordering — higher tier is more restrictive / more vetted.
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
	return -1
}

// AtLeast reports whether the agent's tier is at least the required tier.
// Use this when a code path is conditional on a tier floor.
//
//	if !agent.AtLeast(agent.TierOf(handler), agent.TierBeta) {
//	    // refuse to route production traffic
//	}
func AtLeast(got, required Tier) bool {
	return TierOrdinal(got) >= TierOrdinal(required)
}
