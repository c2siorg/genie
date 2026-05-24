package complaint_triage

import (
	"strings"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 5, 23, 9, 30, 0, 0, time.UTC) }

func TestClassify_UPIFraud_HighSeverity(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Classify(Request{
		UserID:        "u1",
		ComplaintText: "My UPI account had an unauthorised debit of ₹12,500 to an unknown VPA. This is fraud.",
	})
	if res.Category != CatDigitalBanking {
		t.Errorf("category=%s want %s", res.Category, CatDigitalBanking)
	}
	if res.Severity != "high" {
		t.Errorf("severity=%s want high", res.Severity)
	}
	if !res.Incident.OmbudsmanEligible {
		t.Errorf("high-severity UPI fraud should be ombudsman eligible")
	}
	if !strings.Contains(res.Incident.SuggestedAction, "Escalate") {
		t.Errorf("high-severity suggested action should escalate; got %q", res.Incident.SuggestedAction)
	}
}

func TestClassify_LoanEMI_Medium(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Classify(Request{
		UserID:        "u2",
		ComplaintText: "My loan EMI bounced because of penal interest applied incorrectly.",
	})
	if res.Category != CatLoansAdvances {
		t.Errorf("category=%s want %s", res.Category, CatLoansAdvances)
	}
	if res.Severity != "medium" {
		t.Errorf("severity=%s want medium", res.Severity)
	}
}

func TestClassify_VagueComplaintNeedsReview(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Classify(Request{
		UserID:        "u3",
		ComplaintText: "I'm unhappy with the service overall.",
	})
	if res.Category != CatOther {
		t.Errorf("vague complaint should land in CatOther, got %s", res.Category)
	}
	if !res.NeedsHumanReview {
		t.Errorf("vague complaint should need human review")
	}
}

func TestClassify_StaffConduct(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Classify(Request{
		UserID:        "u4",
		ComplaintText: "The branch staff was rude when I asked for my passbook.",
	})
	if res.Category != CatStaffConduct {
		t.Errorf("category=%s want %s", res.Category, CatStaffConduct)
	}
}

func TestClassify_IncidentDraftPopulated(t *testing.T) {
	a := &Agent{Now: fixedNow}
	res := a.Classify(Request{
		UserID:        "u5",
		ComplaintText: "Hidden charges of ₹250 on my savings account.",
		ChannelHint:   "app",
	})
	d := res.Incident
	if d.UserID != "u5" {
		t.Errorf("UserID not carried through: %+v", d)
	}
	if d.Channel != "app" {
		t.Errorf("Channel not carried through")
	}
	if d.DraftedAt.IsZero() {
		t.Errorf("DraftedAt unset")
	}
	if d.OccurredOn != "2026-05-23" {
		t.Errorf("OccurredOn defaulted wrong: %q", d.OccurredOn)
	}
}

func TestClassify_ConfidenceScalesWithMatches(t *testing.T) {
	a := &Agent{Now: fixedNow}
	low := a.Classify(Request{ComplaintText: "EMI issue."}).Confidence
	high := a.Classify(Request{ComplaintText: "Loan EMI on mortgage, foreclosure pending."}).Confidence
	if low >= high {
		t.Errorf("confidence should grow with matches; low=%.2f high=%.2f", low, high)
	}
}

func TestClassify_DisclaimerPresent(t *testing.T) {
	a := New()
	res := a.Classify(Request{ComplaintText: "loan emi"})
	if !strings.Contains(res.Disclaimer, "Ombudsman") {
		t.Errorf("disclaimer should reference Ombudsman scheme; got %q", res.Disclaimer)
	}
}
