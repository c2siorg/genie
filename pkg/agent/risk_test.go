package agent

import (
	"context"
	"testing"
)

type noopAgent struct{}

func (noopAgent) ID() string             { return "noop" }
func (noopAgent) Name() string           { return "noop" }
func (noopAgent) Capabilities() []string { return nil }
func (noopAgent) HandleMessage(_ context.Context, _ Message, _ Environment) ([]Message, error) {
	return nil, nil
}

type highRisk struct{ noopAgent }

func (highRisk) RiskLevel() RiskClass { return RiskHigh }

func TestRiskOf_DefaultLow(t *testing.T) {
	if got := RiskOf(noopAgent{}); got != RiskLow {
		t.Fatalf("default risk should be low, got %s", got)
	}
}

func TestRiskOf_RespectsRiskAware(t *testing.T) {
	if got := RiskOf(highRisk{}); got != RiskHigh {
		t.Fatalf("expected high, got %s", got)
	}
}
