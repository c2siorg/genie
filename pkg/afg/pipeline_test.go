package afg

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
)

// The governed pipeline threads raw input through the preprocess chain and fans out
// to specialists, producing a report — the framework port of the bus fan-out/fan-in.
func TestPipeline_EndToEnd(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1 << 20})
	p := NewPipeline(gate)

	disc := NewGovernedDeterministic(gate, "disclaimer", cannedRun("Informational only."))
	risk := NewGovernedDeterministic(gate, "risk_note", cannedRun("Markets fluctuate."))

	raw := `{"transactions":[{"amount_minor":10000,"category":"food"},{"amount_minor":30000,"category":"rent"}]}`
	rep, err := p.Run(context.Background(), raw, disc, risk)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	// analysis reflects the ingested totals (40000 paise across 2 txns)
	if !strings.Contains(rep.Analysis, `"total_minor":40000`) || !strings.Contains(rep.Analysis, `"count":2`) {
		t.Fatalf("analysis missing computed totals: %q", rep.Analysis)
	}
	if len(rep.Specialists) != 2 {
		t.Fatalf("want 2 specialist outputs, got %d", len(rep.Specialists))
	}
	if rep.Disclosure == "" {
		t.Fatal("report must carry a disclosure")
	}
}

// A denied stage aborts the whole pipeline (governance holds across the chain).
func TestPipeline_DeniedStageAborts(t *testing.T) {
	deny := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 1})
	p := NewPipeline(deny)
	_, err := p.Run(context.Background(), `{"transactions":[{"amount_minor":1,"category":"x"}]}`)
	if err == nil || !strings.Contains(err.Error(), "governance denied") {
		t.Fatalf("expected governance denial to abort pipeline, got: %v", err)
	}
}
