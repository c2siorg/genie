// tier_test.go — contract tests for the four-tier promotion model.
//
// ─── What's pinned here ────────────────────────────────────────────────────
//
// Six tests covering the invariants the dispatch gate depends on:
//
//   1. A declared tier is reported back as-is.
//   2. An undeclared tier defaults to TierPrototype (the safe default
//      that keeps undeclared agents out of production traffic).
//   3. The Production predicate is true ONLY for TierProduction.
//   4. TierOrdinal is strictly increasing across the four declared tiers.
//   5. AtLeast respects the floor (equal counts, lower fails).
//   6. An unknown tier returns ordinal -1 so AtLeast always fails it
//      against any required floor — fail-closed for typo'd tiers.
//
// ─── Test fixtures ─────────────────────────────────────────────────────────
//
// Three fixture agents at file scope: productionAgent, sketchAgent,
// undeclaredAgent. They implement the bare Agent interface plus
// (selectively) the optional TierAware interface. Keeping them at file
// scope means individual tests don't need their own ceremony.
package agent

import (
	"context"
	"testing"
)

// productionAgent is a fixture that declares TierProduction. Used to
// verify the "declared tier is reported back" path.
type productionAgent struct{}

func (productionAgent) ID() string             { return "prod" }
func (productionAgent) Name() string           { return "Prod" }
func (productionAgent) Capabilities() []string { return []string{"x"} }
func (productionAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}

// Tier satisfies the optional TierAware interface. Returning the
// constant directly is the simplest form of declaration — in real
// agents this is typically a one-line method.
func (productionAgent) Tier() Tier { return TierProduction }

// sketchAgent is a fixture that declares TierSketch. Used to verify
// the dispatch gate rejects it (in tests/security_envelope_test.go),
// and to verify TierOf reports the declared value back here.
type sketchAgent struct{}

func (sketchAgent) ID() string             { return "sketch" }
func (sketchAgent) Name() string           { return "Sketch" }
func (sketchAgent) Capabilities() []string { return []string{"x"} }
func (sketchAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}
func (sketchAgent) Tier() Tier { return TierSketch }

// undeclaredAgent is a fixture that does NOT implement TierAware.
// Used to verify the default-to-Prototype behaviour. This is the
// shape every existing agent had before the tier model landed — the
// default ensures those agents continued to compile and work, with
// the safe constraint that they couldn't take production traffic
// without an explicit promotion.
type undeclaredAgent struct{}

func (undeclaredAgent) ID() string             { return "undeclared" }
func (undeclaredAgent) Name() string           { return "Undeclared" }
func (undeclaredAgent) Capabilities() []string { return []string{"x"} }
func (undeclaredAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}

// (Intentionally no Tier method — that's the whole point of this fixture.)

// TestTierOfReturnsDeclaredTier asserts that TierOf surfaces whatever
// the agent's Tier() method returns. Both production and sketch are
// checked — covering the "good" and "explicitly dangerous" ends of
// the spectrum.
func TestTierOfReturnsDeclaredTier(t *testing.T) {
	if TierOf(productionAgent{}) != TierProduction {
		t.Errorf("production agent must report TierProduction")
	}
	if TierOf(sketchAgent{}) != TierSketch {
		t.Errorf("sketch agent must report TierSketch")
	}
}

// TestTierOfDefaultsToPrototype is the security-critical test of this
// file. If an agent doesn't declare a tier, it MUST default to
// Prototype — never to Production. A regression of this invariant
// would let new agents slip into production by simply not
// implementing TierAware, which is the opposite of fail-closed.
func TestTierOfDefaultsToPrototype(t *testing.T) {
	if TierOf(undeclaredAgent{}) != TierPrototype {
		t.Errorf("undeclared agent must default to TierPrototype (safe default)")
	}
}

// TestProductionPredicate verifies the Production() helper is true
// ONLY for TierProduction — none of the other three tiers (Sketch,
// Prototype, Beta) should evaluate as production.
//
// The dispatch gate uses this predicate; if Production(TierBeta)
// returned true, beta agents would silently graduate to production
// without going through the promotion gate.
func TestProductionPredicate(t *testing.T) {
	if !Production(TierProduction) {
		t.Errorf("Production(TierProduction) must be true")
	}
	// Exhaustively check every other declared tier returns false.
	for _, x := range []Tier{TierSketch, TierPrototype, TierBeta} {
		if Production(x) {
			t.Errorf("Production(%s) must be false", x)
		}
	}
}

// TestTierOrdinalOrdering asserts the ordinals are strictly increasing.
// This is what makes AtLeast work — a tier floor check uses
// TierOrdinal under the hood. If two tiers shared an ordinal,
// AtLeast(beta, production) would return true (≥) and beta would
// satisfy a production floor, which is a security bug.
//
// The double loop checks every pair: for any i<j in the declared
// order, ordinal(i) < ordinal(j).
func TestTierOrdinalOrdering(t *testing.T) {
	// Expected order, lowest-privilege to highest.
	expected := []Tier{TierSketch, TierPrototype, TierBeta, TierProduction}
	for i, t1 := range expected {
		for j, t2 := range expected {
			if i < j && TierOrdinal(t1) >= TierOrdinal(t2) {
				t.Errorf("%s should rank below %s", t1, t2)
			}
		}
	}
}

// TestAtLeastFloor covers the three boundary cases for the AtLeast
// floor check:
//   - Lower than required → false
//   - Equal to required → true (inclusive floor)
//   - Higher than required (and equal-to-floor of a higher floor) → true
//
// AtLeast is used by host code that needs "is this agent at least
// Beta?" or "is this at least Production?" — getting the equality
// boundary wrong would either reject Beta from Beta floors (too
// strict) or accept Prototype from Beta floors (too loose). The
// inclusive interpretation is correct because that's how every other
// "at least" floor in software works (HTTP minor versions, semver
// matchers, etc.).
func TestAtLeastFloor(t *testing.T) {
	// Sketch is not at least Beta — too low.
	if AtLeast(TierSketch, TierBeta) {
		t.Errorf("sketch is not at least beta")
	}
	// Beta is at least Prototype — higher than floor.
	if !AtLeast(TierBeta, TierPrototype) {
		t.Errorf("beta is at least prototype")
	}
	// Production satisfies the Production floor — inclusive boundary.
	if !AtLeast(TierProduction, TierProduction) {
		t.Errorf("production satisfies production floor")
	}
}

// TestUnknownTierOrdinalIsNegative pins the unknown-tier escape hatch.
//
// If an agent returns a typo'd tier (e.g. "produciton"), TierOrdinal
// returns -1. That -1 fails every AtLeast check against any required
// floor (-1 < ordinal_of_anything_declared), so the typoed tier is
// treated as "no privilege at all" — fail-closed.
//
// If we ever changed unknown-tier to default to 0 (TierSketch's
// ordinal) "for safety," that would actually be LESS safe — a typo'd
// tier would satisfy any AtLeast(TierSketch) check. -1 is the right
// answer.
func TestUnknownTierOrdinalIsNegative(t *testing.T) {
	if TierOrdinal(Tier("invalid")) >= 0 {
		t.Errorf("unknown tier must return negative ordinal so it fails AtLeast checks")
	}
}
