package health_preauth

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

func basePlan() Plan {
	return Plan{
		ProductCode:            "HC-001",
		SumInsuredRupees:       500_000,
		RoomRentSubLimitRupees: 5_000,
		ICURentSubLimitRupees:  10_000,
		CoPayPct:               0,
		PEDWaitingMonths:       36,
		SpecificWaitingMonths:  24,
		ExcludedProcedures:     []string{"cosmetic surgery"},
		ProcedurePackageRupees: map[string]float64{"cataract": 30_000},
	}
}

func TestDeniesNonNetwork(t *testing.T) {
	d := New().Decide(Request{PreauthID: "p1", NetworkPPN: false}, basePlan())
	if d.Action != "deny" {
		t.Errorf("non-PPN should deny, got %s", d.Action)
	}
}

func TestDeniesExcludedProcedure(t *testing.T) {
	d := New().Decide(Request{PreauthID: "p2", NetworkPPN: true, Procedure: "Cosmetic Surgery", PolicyMonthsAtAdmit: 60}, basePlan())
	if d.Action != "deny" {
		t.Errorf("exclusion should deny, got %s", d.Action)
	}
}

func TestDeniesPEDWaiting(t *testing.T) {
	d := New().Decide(Request{
		PreauthID: "p3", NetworkPPN: true, Procedure: "diabetes_complication",
		IsPED: true, PolicyMonthsAtAdmit: 12,
	}, basePlan())
	if d.Action != "deny" {
		t.Errorf("PED within waiting period should deny, got %s", d.Action)
	}
}

func TestRoomRentProportionateDeduction(t *testing.T) {
	d := New().Decide(Request{
		PreauthID: "p4", NetworkPPN: true, Procedure: "appendectomy",
		PolicyMonthsAtAdmit: 60,
		EstimatedBillRupees: 100_000,
		RoomRentPerDayRupees: 10_000, // 2× the sub-limit
		LengthOfStayDays:     3,
	}, basePlan())
	if d.Action == "approve_full" {
		t.Errorf("expected partial approval with deduction, got %s", d.Action)
	}
	if d.DeductionsRupees == 0 {
		t.Errorf("expected non-zero deductions for over-sublimit room rent")
	}
	if !containsAny(d.DeductionReasons, "proportionate") {
		t.Errorf("expected proportionate-deduction reason; got %+v", d.DeductionReasons)
	}
}

func TestProcedurePackageCap(t *testing.T) {
	d := New().Decide(Request{
		PreauthID: "p5", NetworkPPN: true, Procedure: "cataract",
		PolicyMonthsAtAdmit: 60,
		EstimatedBillRupees: 60_000,
		RoomRentPerDayRupees: 4_000, // within sub-limit
		LengthOfStayDays:     1,
	}, basePlan())
	if d.Action == "approve_full" {
		t.Errorf("cataract package cap should produce partial approval")
	}
	if !containsAny(d.DeductionReasons, "package cap") {
		t.Errorf("expected package-cap reason; got %+v", d.DeductionReasons)
	}
}

func TestHITLOnLargeBill(t *testing.T) {
	d := New().Decide(Request{
		PreauthID: "p6", NetworkPPN: true, Procedure: "cabg",
		PolicyMonthsAtAdmit: 60,
		EstimatedBillRupees: 600_000,
		RoomRentPerDayRupees: 4_000,
		LengthOfStayDays:     5,
	}, basePlan())
	if d.Action != "hitl" {
		t.Errorf("bill ≥₹5L should route to HITL; got %s", d.Action)
	}
}

func TestApproveFullCleanCase(t *testing.T) {
	d := New().Decide(Request{
		PreauthID: "p7", NetworkPPN: true, Procedure: "fever",
		PolicyMonthsAtAdmit: 60,
		EstimatedBillRupees: 25_000,
		RoomRentPerDayRupees: 4_000,
		LengthOfStayDays:     2,
	}, basePlan())
	if d.Action != "approve_full" {
		t.Errorf("clean small claim should approve_full; got %s (reasons=%v)", d.Action, d.DeductionReasons)
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(struct {
		Request Request `json:"request"`
		Plan    Plan    `json:"plan"`
	}{Request{PreauthID: "p", NetworkPPN: true, Procedure: "fever", PolicyMonthsAtAdmit: 60,
		EstimatedBillRupees: 10_000, RoomRentPerDayRupees: 4000, LengthOfStayDays: 1}, basePlan()})
	msg := agent.NewMessage("h", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimer(t *testing.T) {
	d := New().Decide(Request{NetworkPPN: true, Procedure: "x", PolicyMonthsAtAdmit: 60}, basePlan())
	if d.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}

func containsAny(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
