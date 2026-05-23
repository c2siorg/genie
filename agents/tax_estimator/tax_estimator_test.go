package tax_estimator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestTax_NewRegime_Rebate(t *testing.T) {
	body, _ := json.Marshal(Request{GrossIncomeRupees: 700_000, Regime: "new"})
	msg := agent.NewMessage("u", ID, agent.RoleUser, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var r Response
	_ = json.Unmarshal([]byte(out[0].Content), &r)
	if r.TaxRupees != 0 {
		t.Fatalf("expected zero tax at ₹7L, got %f", r.TaxRupees)
	}
}

func TestTax_NewRegime_TwentyL(t *testing.T) {
	body, _ := json.Marshal(Request{GrossIncomeRupees: 2_000_000, Regime: "new"})
	msg := agent.NewMessage("u", ID, agent.RoleUser, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var r Response
	_ = json.Unmarshal([]byte(out[0].Content), &r)
	// Slabs: 0-3 free, 3-6@5%=15k, 6-9@10%=30k, 9-12@15%=45k, 12-15@20%=60k, 15-20@30%=150k → 300k * 1.04 = 312k
	if r.TaxRupees < 310_000 || r.TaxRupees > 315_000 {
		t.Fatalf("unexpected tax for ₹20L: %f", r.TaxRupees)
	}
}
