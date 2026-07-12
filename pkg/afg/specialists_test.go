package afg

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
)

// The batch-port template scales: DefaultRegistry stands up several governed
// specialists; each runs through the single door and appears in the inventory.
func TestDefaultRegistry_BatchPortedSpecialists(t *testing.T) {
	gate := governance.NewComposite(governance.MaxContentLengthPolicy{Max: 4096})
	reg := DefaultRegistry(gate)
	ctx := context.Background()

	inv := reg.Inventory()
	if len(inv) != 4 {
		t.Fatalf("want 4 ported specialists, got %d: %+v", len(inv), inv)
	}
	for _, r := range inv {
		if !r.Governed {
			t.Fatalf("specialist %s not governed", r.ID)
		}
	}

	// rates — faithful to the legacy fx table (USD->INR = 83).
	if txt, _, err := reg.RunWithFallback(ctx, RatesName, `{"from":"usd","to":"inr"}`); err != nil || !strings.Contains(txt, `"rate":83`) {
		t.Fatalf("rates wrong: err=%v txt=%q", err, txt)
	}

	// tax_estimator — 12L income (paise=120000000) new regime => 90,000 rupees = 9,000,000 paise.
	if txt, _, err := reg.RunWithFallback(ctx, TaxEstimatorName, `{"income_minor":120000000}`); err != nil || !strings.Contains(txt, `"tax_minor":9000000`) {
		t.Fatalf("tax wrong: err=%v txt=%q", err, txt)
	}

	// macro — canned snapshot.
	if txt, _, err := reg.RunWithFallback(ctx, MacroName, "{}"); err != nil || !strings.Contains(txt, "repo_rate_pct") {
		t.Fatalf("macro wrong: err=%v txt=%q", err, txt)
	}
}
