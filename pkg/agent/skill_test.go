package agent

import (
	"context"
	"strings"
	"testing"
)

type skillyAgent struct{}

func (skillyAgent) ID() string             { return "tax-bot" }
func (skillyAgent) Name() string           { return "Tax Bot" }
func (skillyAgent) Capabilities() []string { return []string{"tax"} }
func (skillyAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}
func (skillyAgent) Skills() []Skill {
	return []Skill{
		{
			ID: "tax_planning_in", Title: "India tax planning",
			Summary: "Old vs new regime, 80C ceilings, advance tax.",
			Tools: func(_ context.Context) []SkillTool {
				return []SkillTool{
					{Name: "compare_regimes", Description: "Compute net tax under old vs new"},
				}
			},
		},
		{
			ID: "tax_harvest_in", Title: "STCG/LTCL harvesting",
			Summary: "Identify equity lots to harvest before FY end.",
		},
	}
}

type plainAgent struct{}

func (plainAgent) ID() string             { return "plain" }
func (plainAgent) Name() string           { return "Plain" }
func (plainAgent) Capabilities() []string { return []string{"x"} }
func (plainAgent) HandleMessage(ctx context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}

func TestSkillsOfReturnsSkillsForProviders(t *testing.T) {
	if SkillsOf(skillyAgent{}) == nil {
		t.Errorf("expected non-nil for provider")
	}
	if SkillsOf(plainAgent{}) != nil {
		t.Errorf("expected nil for non-provider")
	}
}

func TestSkillRegistryRegisterAndManifest(t *testing.T) {
	r := NewSkillRegistry()
	n, err := r.RegisterAgent(skillyAgent{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 skills registered; got %d", n)
	}
	manifest := r.Manifest()
	if len(manifest) != 2 {
		t.Errorf("expected 2 manifest lines; got %d", len(manifest))
	}
	if !strings.Contains(manifest[0], "tax_") {
		t.Errorf("manifest should start with a tax_ id; got %q", manifest[0])
	}
}

func TestSkillRegistryInvoke(t *testing.T) {
	r := NewSkillRegistry()
	_, _ = r.RegisterAgent(skillyAgent{})
	tools, err := r.Invoke(context.Background(), "tax_planning_in")
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "compare_regimes" {
		t.Errorf("expected compare_regimes tool; got %+v", tools)
	}
}

func TestInvokeUnknownSkill(t *testing.T) {
	r := NewSkillRegistry()
	if _, err := r.Invoke(context.Background(), "missing"); err == nil {
		t.Errorf("expected error on unknown skill")
	}
}

func TestOwnerLookup(t *testing.T) {
	r := NewSkillRegistry()
	_, _ = r.RegisterAgent(skillyAgent{})
	owner, ok := r.OwnerOf("tax_harvest_in")
	if !ok || owner != "tax-bot" {
		t.Errorf("expected owner=tax-bot; got %s,%v", owner, ok)
	}
}

func TestNonProviderRegistrationIsNoOp(t *testing.T) {
	r := NewSkillRegistry()
	n, err := r.RegisterAgent(plainAgent{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("non-provider should register 0 skills; got %d", n)
	}
}
