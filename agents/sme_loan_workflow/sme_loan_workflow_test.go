package sme_loan_workflow

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func cleanApp() Application {
	return Application{
		BorrowerID:         "sme-1",
		UDYAMRegistered:    true,
		Sector:             "manufacturing",
		AnnualTurnover:     20_000_000, // ₹2 cr
		GSTFilingRegular:   true,
		RequestedAmount:    3_000_000, // ₹30L (within 30% of turnover)
		RequestedTenorMths: 36,
		CashflowScore0to1:  0.75,
	}
}

func TestHappyPathApproval(t *testing.T) {
	o, err := New().Process(context.Background(), cleanApp(), true)
	if err != nil {
		t.Fatal(err)
	}
	if o.Decision != "approved" {
		t.Fatalf("expected approved, got %s (rationale=%v)", o.Decision, o.Rationale)
	}
	if !o.CGTMSEEligible {
		t.Errorf("UDYAM + covered sector + within ticket should be CGTMSE eligible")
	}
	if o.OfferedAmount != 3_000_000 {
		t.Errorf("expected requested amount honoured at ₹30L, got %.0f", o.OfferedAmount)
	}
	if o.MonthlyEMIRupees <= 0 {
		t.Errorf("EMI should be positive")
	}
}

func TestCashflowFloorRejection(t *testing.T) {
	app := cleanApp()
	app.CashflowScore0to1 = 0.20
	o, _ := New().Process(context.Background(), app, true)
	if o.Decision != "rejected" {
		t.Errorf("low cashflow should reject, got %s", o.Decision)
	}
}

func TestTurnoverCap(t *testing.T) {
	app := cleanApp()
	app.AnnualTurnover = 5_000_000 // ₹50L
	app.RequestedAmount = 4_000_000 // ₹40L exceeds 30% of turnover (₹15L)
	o, _ := New().Process(context.Background(), app, true)
	if o.OfferedAmount > 1_500_000+1 {
		t.Errorf("offer must cap at 30%% of turnover; got %.0f", o.OfferedAmount)
	}
}

func TestCGTMSEIneligibleNonCoveredSector(t *testing.T) {
	app := cleanApp()
	app.Sector = "agriculture"
	o, _ := New().Process(context.Background(), app, true)
	if o.CGTMSEEligible {
		t.Errorf("agriculture sector should not be CGTMSE eligible in this stub")
	}
}

func TestInPrincipleWhenNoHumanApproval(t *testing.T) {
	o, err := New().Process(context.Background(), cleanApp(), false)
	// without approval the workflow may return an error (awaiting), but offer
	// should still reflect in_principle.
	_ = err
	if o.Decision != "in_principle" && o.Decision != "rejected" {
		t.Errorf("without approval expected in_principle or rejected, got %s", o.Decision)
	}
}

func TestRateRiskPremium(t *testing.T) {
	low := cleanApp()
	low.CashflowScore0to1 = 0.40
	high := cleanApp()
	high.CashflowScore0to1 = 0.95

	loRate, _ := New().Process(context.Background(), low, true)
	hiRate, _ := New().Process(context.Background(), high, true)
	if loRate.IndicativeRatePct <= hiRate.IndicativeRatePct {
		t.Errorf("weaker cashflow should attract higher rate; low=%.2f high=%.2f",
			loRate.IndicativeRatePct, hiRate.IndicativeRatePct)
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(cleanApp())
	msg := agent.NewMessage("up", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch of %s, got %+v", TypeOut, out)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	o, _ := New().Process(context.Background(), cleanApp(), true)
	if o.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}

func TestRationaleMentionsCGTMSE(t *testing.T) {
	o, _ := New().Process(context.Background(), cleanApp(), true)
	hit := false
	for _, r := range o.Rationale {
		if strings.Contains(r, "CGTMSE") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("CGTMSE eligibility should appear in rationale")
	}
}
