package payment_orchestrator

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

func midday() time.Time { return time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC) }

func newAt(now time.Time) *Agent { return &Agent{Clock: func() time.Time { return now }} }

func TestUPISmallTrustedSubmits(t *testing.T) {
	ins := newAt(midday()).Plan(Request{
		IdempotencyKey: "k1", PayerID: "u", AmountRupees: 5_000,
		BeneficiaryVPA: "x@upi", IsTrustedBeneficiary: true,
	})
	if ins.Action != "submit" || ins.Rail != "upi" {
		t.Errorf("expected submit via upi; got %s/%s", ins.Action, ins.Rail)
	}
}

func TestLargeUntrustedHolds(t *testing.T) {
	ins := newAt(midday()).Plan(Request{
		IdempotencyKey: "k2", AmountRupees: 60_000,
		BeneficiaryVPA: "x@upi", IsTrustedBeneficiary: false,
	})
	if ins.Action != "hold_hitl" {
		t.Errorf("untrusted ≥₹50k must be HITL; got %s", ins.Action)
	}
}

func TestRTGSWindowAndAmount(t *testing.T) {
	ins := newAt(midday()).Plan(Request{
		IdempotencyKey: "k3", AmountRupees: 250_000,
		BeneficiaryIFSC: "HDFC0000001", BeneficiaryAcct: "00112233",
		IsTrustedBeneficiary: true,
	})
	if ins.Rail != "rtgs" {
		t.Errorf("₹2.5L midday should pick RTGS; got %s", ins.Rail)
	}
	if ins.Action != "hold_hitl" {
		t.Errorf("≥₹50k should still HITL; got %s", ins.Action)
	}
}

func TestRTGSClosedFallsBackToIMPS(t *testing.T) {
	night := time.Date(2026, 5, 14, 22, 0, 0, 0, time.UTC)
	ins := newAt(night).Plan(Request{
		IdempotencyKey: "k4", AmountRupees: 300_000,
		BeneficiaryIFSC: "HDFC0000001", BeneficiaryAcct: "00112233",
		IsTrustedBeneficiary: true,
	})
	// RTGS closed at 22:00, ₹3L is within IMPS ₹5L cap, urgency not "any" → IMPS.
	if ins.Rail != "imps" {
		t.Errorf("RTGS closed should pick IMPS as next-best instant rail; got %s", ins.Rail)
	}
}

func TestRTGSClosedNonUrgentPicksNEFT(t *testing.T) {
	night := time.Date(2026, 5, 14, 22, 0, 0, 0, time.UTC)
	ins := newAt(night).Plan(Request{
		IdempotencyKey: "k4b", AmountRupees: 300_000,
		BeneficiaryIFSC: "HDFC0000001", BeneficiaryAcct: "00112233",
		IsTrustedBeneficiary: true,
		Urgency:              "any",
	})
	if ins.Rail != "neft" {
		t.Errorf("non-urgent night transfer should fall through to NEFT; got %s", ins.Rail)
	}
}

func TestNonINRRejected(t *testing.T) {
	ins := newAt(midday()).Plan(Request{IdempotencyKey: "k5", Currency: "USD", AmountRupees: 100})
	if ins.Action != "reject" {
		t.Errorf("non-INR should reject")
	}
}

func TestZeroAmountRejected(t *testing.T) {
	ins := newAt(midday()).Plan(Request{IdempotencyKey: "k6", AmountRupees: 0})
	if ins.Action != "reject" {
		t.Errorf("zero amount should reject")
	}
}

func TestMissingIdempotencyKeyRejected(t *testing.T) {
	ins := newAt(midday()).Plan(Request{AmountRupees: 100, BeneficiaryVPA: "x@upi"})
	if ins.Action != "reject" {
		t.Errorf("missing idempotency key must reject")
	}
}

func TestNoRailOptions(t *testing.T) {
	ins := newAt(midday()).Plan(Request{
		IdempotencyKey:       "k8",
		AmountRupees:         200_000, // > UPI cap
		IsTrustedBeneficiary: true,
		// no VPA, no IFSC/Acct
	})
	if ins.Action != "reject" {
		t.Errorf("no rail should reject; got %s", ins.Action)
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(Request{IdempotencyKey: "k", AmountRupees: 100, BeneficiaryVPA: "x@upi", IsTrustedBeneficiary: true})
	msg := agent.NewMessage("u", ID, agent.RoleUser, TypeIn, string(body), nil)
	out, err := newAt(midday()).HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimer(t *testing.T) {
	ins := newAt(midday()).Plan(Request{IdempotencyKey: "k", AmountRupees: 100, BeneficiaryVPA: "x@upi", IsTrustedBeneficiary: true})
	if ins.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}

func TestRiskHigh(t *testing.T) {
	if New().RiskLevel() != agent.RiskHigh {
		t.Errorf("payment orchestrator must be RiskHigh")
	}
}
