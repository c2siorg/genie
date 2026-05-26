package agent

import (
	"context"
	"testing"
)

type productionAgent struct{}

func (productionAgent) ID() string             { return "prod" }
func (productionAgent) Name() string           { return "Prod" }
func (productionAgent) Capabilities() []string { return []string{"x"} }
func (productionAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}
func (productionAgent) Tier() Tier { return TierProduction }

type sketchAgent struct{}

func (sketchAgent) ID() string             { return "sketch" }
func (sketchAgent) Name() string           { return "Sketch" }
func (sketchAgent) Capabilities() []string { return []string{"x"} }
func (sketchAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}
func (sketchAgent) Tier() Tier { return TierSketch }

type undeclaredAgent struct{}

func (undeclaredAgent) ID() string             { return "undeclared" }
func (undeclaredAgent) Name() string           { return "Undeclared" }
func (undeclaredAgent) Capabilities() []string { return []string{"x"} }
func (undeclaredAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}

func TestTierOfReturnsDeclaredTier(t *testing.T) {
	if TierOf(productionAgent{}) != TierProduction {
		t.Errorf("production agent must report TierProduction")
	}
	if TierOf(sketchAgent{}) != TierSketch {
		t.Errorf("sketch agent must report TierSketch")
	}
}

func TestTierOfDefaultsToPrototype(t *testing.T) {
	if TierOf(undeclaredAgent{}) != TierPrototype {
		t.Errorf("undeclared agent must default to TierPrototype (safe default)")
	}
}

func TestProductionPredicate(t *testing.T) {
	if !Production(TierProduction) {
		t.Errorf("Production(TierProduction) must be true")
	}
	for _, x := range []Tier{TierSketch, TierPrototype, TierBeta} {
		if Production(x) {
			t.Errorf("Production(%s) must be false", x)
		}
	}
}

func TestTierOrdinalOrdering(t *testing.T) {
	expected := []Tier{TierSketch, TierPrototype, TierBeta, TierProduction}
	for i, t1 := range expected {
		for j, t2 := range expected {
			if i < j && TierOrdinal(t1) >= TierOrdinal(t2) {
				t.Errorf("%s should rank below %s", t1, t2)
			}
		}
	}
}

func TestAtLeastFloor(t *testing.T) {
	if AtLeast(TierSketch, TierBeta) {
		t.Errorf("sketch is not at least beta")
	}
	if !AtLeast(TierBeta, TierPrototype) {
		t.Errorf("beta is at least prototype")
	}
	if !AtLeast(TierProduction, TierProduction) {
		t.Errorf("production satisfies production floor")
	}
}

func TestUnknownTierOrdinalIsNegative(t *testing.T) {
	if TierOrdinal(Tier("invalid")) >= 0 {
		t.Errorf("unknown tier must return negative ordinal so it fails AtLeast checks")
	}
}
