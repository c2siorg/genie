package auto_insurance

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

func mumbaiGarages() map[string][]string {
	return map[string][]string{
		"mumbai": {"Garage A", "Garage B"},
		"delhi":  {"Garage C"},
	}
}

func TestFNOLPartialLoss(t *testing.T) {
	r := New(mumbaiGarages()).Service(Request{
		Kind:                "fnol",
		PolicyNumber:        "P-1",
		IDVRupees:           500_000,
		EstRepairCostRupees: 100_000,
		LocationLat:         19.05, LocationLng: 72.85, // Mumbai
	})
	if r.TotalLoss {
		t.Errorf("100k of 500k IDV should not be total loss")
	}
	if r.Action != "register_claim" {
		t.Errorf("expected register_claim, got %s", r.Action)
	}
	if len(r.NetworkGarages) != 2 {
		t.Errorf("expected Mumbai garages, got %+v", r.NetworkGarages)
	}
}

func TestFNOLTotalLoss(t *testing.T) {
	r := New(mumbaiGarages()).Service(Request{
		Kind:                "fnol",
		IDVRupees:           500_000,
		EstRepairCostRupees: 400_000, // 80% of IDV
		LocationLat:         28.6, LocationLng: 77.2,
	})
	if !r.TotalLoss {
		t.Errorf("400k repair on 500k IDV should be total loss")
	}
	if r.SettlementHint != 500_000 {
		t.Errorf("total loss settlement should equal IDV; got %.2f", r.SettlementHint)
	}
}

func TestRoadsideDispatch(t *testing.T) {
	r := New(mumbaiGarages()).Service(Request{
		Kind:        "roadside",
		LocationLat: 19.05, LocationLng: 72.85,
	})
	if r.Action != "dispatch_partner" {
		t.Errorf("expected dispatch_partner, got %s", r.Action)
	}
}

func TestRenewalQuoteCleanYear(t *testing.T) {
	r := New(nil).Service(Request{
		Kind:        "renewal_quote",
		IDVRupees:   500_000,
		NCBPct:      20,
		ClaimedThisYear: false,
	})
	if r.NewNCBPct != 25 {
		t.Errorf("clean year on 20%% NCB should bump to 25%%; got %.2f", r.NewNCBPct)
	}
}

func TestRenewalQuoteClaimResetsNCB(t *testing.T) {
	r := New(nil).Service(Request{
		Kind:            "renewal_quote",
		IDVRupees:       500_000,
		NCBPct:          35,
		ClaimedThisYear: true,
	})
	if r.NewNCBPct != 0 {
		t.Errorf("any claim should reset NCB to 0; got %.2f", r.NewNCBPct)
	}
}

func TestRenewalUrgencyHint(t *testing.T) {
	r := New(nil).Service(Request{
		Kind:          "renewal_quote",
		IDVRupees:     500_000,
		HoursToExpiry: 48,
	})
	found := false
	for _, s := range r.NextSteps {
		if contains(s, "uninsured") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected urgency hint when close to expiry; got %+v", r.NextSteps)
	}
}

func TestUnknownKindFails(t *testing.T) {
	r := New(nil).Service(Request{Kind: "what_is_this"})
	if r.Action != "unknown" {
		t.Errorf("expected unknown action; got %s", r.Action)
	}
}

func TestHandleMessage(t *testing.T) {
	body, _ := json.Marshal(Request{Kind: "fnol", IDVRupees: 100000, EstRepairCostRupees: 1000})
	msg := agent.NewMessage("u", ID, agent.RoleUser, TypeIn, string(body), nil)
	out, err := New(nil).HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected dispatch, got %+v", out)
	}
}

func TestDisclaimer(t *testing.T) {
	r := New(nil).Service(Request{Kind: "fnol"})
	if r.Disclaimer == "" {
		t.Errorf("disclaimer required")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
