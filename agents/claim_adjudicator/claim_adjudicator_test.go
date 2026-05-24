package claim_adjudicator

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

func basePolicy() Policy {
	return Policy{
		ProductCode:       "HOSP-001",
		WaitingPeriodDays: 30,
		SumInsured:        500_000,
		DeductibleRupees:  5_000,
		CoPayPct:          0.10,
		Exclusions:        []string{"cosmetic", "self-inflicted"},
		SubLimits:         map[string]float64{"hospitalization": 300_000},
		NetworkOnlyPerils: []string{"cashless"},
	}
}

func TestWaitingPeriodDenial(t *testing.T) {
	d := New().Adjudicate(Claim{ClaimID: "c1", IncurredRupees: 50_000, DaysSinceIssue: 10, Peril: "hospitalization"}, basePolicy())
	if d.Action != "deny" {
		t.Fatalf("expected deny within waiting period, got %s", d.Action)
	}
}

func TestExclusionDenial(t *testing.T) {
	d := New().Adjudicate(Claim{ClaimID: "c2", IncurredRupees: 50_000, DaysSinceIssue: 90,
		Peril: "hospitalization", Diagnosis: "Cosmetic surgery rhinoplasty"}, basePolicy())
	if d.Action != "deny" {
		t.Fatalf("expected deny on exclusion, got %s", d.Action)
	}
}

func TestPartialApprovalWithDeductibleAndCopay(t *testing.T) {
	d := New().Adjudicate(Claim{ClaimID: "c3", IncurredRupees: 50_000, DaysSinceIssue: 90,
		Peril: "hospitalization", HospitalInNetwork: true, Diagnosis: "fever"}, basePolicy())
	if d.Action != "approve_partial" {
		t.Fatalf("expected approve_partial after deductible+copay, got %s", d.Action)
	}
	// 50000 - 5000 = 45000, less 10% copay = 40500
	if d.PayoutRupees != 40_500 {
		t.Errorf("expected payout 40500, got %.2f", d.PayoutRupees)
	}
}

func TestSubLimitCap(t *testing.T) {
	d := New().Adjudicate(Claim{ClaimID: "c4", IncurredRupees: 400_000, DaysSinceIssue: 90,
		Peril: "hospitalization", HospitalInNetwork: true, Diagnosis: "fever"}, basePolicy())
	// HITL trigger at >= 200000; verify the cap reason still surfaces.
	hit := false
	for _, r := range d.Reasons {
		if strings.Contains(r, "sub-limit") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected sub-limit cap reason; got %+v", d.Reasons)
	}
	if d.PayoutRupees > 300_000 {
		t.Errorf("payout should be capped at sub-limit; got %.2f", d.PayoutRupees)
	}
}

func TestHITLOnLargeClaim(t *testing.T) {
	d := New().Adjudicate(Claim{ClaimID: "c5", IncurredRupees: 250_000, DaysSinceIssue: 90,
		Peril: "hospitalization", HospitalInNetwork: true, Diagnosis: "fever"}, basePolicy())
	if d.Action != "hitl" {
		t.Errorf("expected hitl for claim ≥ ₹2L, got %s", d.Action)
	}
}

func TestNetworkOnlyDenial(t *testing.T) {
	d := New().Adjudicate(Claim{ClaimID: "c6", IncurredRupees: 30_000, DaysSinceIssue: 90,
		Peril: "cashless", HospitalInNetwork: false, Diagnosis: "fever"}, basePolicy())
	if d.Action != "deny" {
		t.Fatalf("expected deny for non-network cashless peril, got %s", d.Action)
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(Request{Claim: Claim{ClaimID: "c", IncurredRupees: 10_000, DaysSinceIssue: 90,
		Peril: "hospitalization", HospitalInNetwork: true, Diagnosis: "fever"}, Policy: basePolicy()})
	msg := agent.NewMessage("up", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	d := New().Adjudicate(Claim{}, basePolicy())
	if d.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}
