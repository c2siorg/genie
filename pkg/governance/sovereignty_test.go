package governance

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/sovereignty"
)

func TestDataResidency_DenyPIICrossBorder(t *testing.T) {
	p := NewResidencyPolicy(sovereignty.RegionIN)
	msg := protocol.Message{Metadata: map[string]any{
		sovereignty.MetaKeyRegion:    string(sovereignty.RegionUS),
		protocol.MetaKeyClassification: string(protocol.ClassPII),
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionDeny {
		t.Fatalf("expected deny PII cross-border, got %s", res.Reason)
	}
}

func TestDataResidency_AllowOnPrem(t *testing.T) {
	p := NewResidencyPolicy(sovereignty.RegionIN)
	msg := protocol.Message{Metadata: map[string]any{
		sovereignty.MetaKeyRegion:    string(sovereignty.RegionOnPrem),
		protocol.MetaKeyClassification: string(protocol.ClassSecret),
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow on-prem, got %s", res.Reason)
	}
}

func TestDataResidency_PublicMayCross(t *testing.T) {
	p := NewResidencyPolicy(sovereignty.RegionIN)
	msg := protocol.Message{Metadata: map[string]any{
		sovereignty.MetaKeyRegion:    string(sovereignty.RegionUS),
		protocol.MetaKeyClassification: string(protocol.ClassPublic),
	}}
	res, _ := p.Evaluate(context.Background(), msg)
	if res.Decision != DecisionAllow {
		t.Fatalf("expected allow public, got %s", res.Reason)
	}
}

func TestProviderRegistry_Allowed(t *testing.T) {
	r := sovereignty.NewRegistry()
	r.Register(sovereignty.Provider{
		Name:                   "anthropic",
		Region:                 sovereignty.RegionUS,
		AllowedClassifications: []protocol.Classification{protocol.ClassPublic, protocol.ClassInternal},
	})
	if r.Allowed("anthropic", protocol.ClassPII) {
		t.Fatal("should not allow PII")
	}
	if !r.Allowed("anthropic", protocol.ClassPublic) {
		t.Fatal("should allow public")
	}
	if r.Allowed("unknown", protocol.ClassPublic) {
		t.Fatal("unknown providers default-deny")
	}
}
