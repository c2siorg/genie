package kyc_orchestrator

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

func TestCleanProfileApproves(t *testing.T) {
	v := New().Decide(Application{
		CustomerID:         "c-1",
		PANNumber:          "ABCPS1234F", // 4th=P, 5th=S matches "Singh"
		NameOnPAN:          "Asha Singh",
		AadhaarOfflineKYC:  true,
		NameOnAadhaar:      "Asha Singh",
		AddressMatchScore:  0.95,
		LivenessScore:      0.90,
	})
	if v.Decision != "approve" {
		t.Fatalf("clean profile should approve, got %s (score=%.2f reasons=%v)", v.Decision, v.RiskScore, v.Reasons)
	}
	if v.Tier != "sdd" {
		t.Errorf("clean profile should fall in sdd tier, got %s", v.Tier)
	}
}

func TestSanctionsAutoReject(t *testing.T) {
	v := New().Decide(Application{
		CustomerID:        "c-2",
		PANNumber:         "ABCPS1234F",
		NameOnPAN:         "Asha Singh",
		AadhaarOfflineKYC: true,
		NameOnAadhaar:     "Asha Singh",
		SanctionsHit:      true,
	})
	if v.Decision != "reject" {
		t.Fatalf("sanctions hit must auto-reject, got %s", v.Decision)
	}
	if v.IncidentPayload == "" {
		t.Errorf("reject must include Annexure VI payload")
	}
	var incident map[string]any
	if err := json.Unmarshal([]byte(v.IncidentPayload), &incident); err != nil {
		t.Fatalf("incident payload must be valid JSON: %v", err)
	}
	if incident["annexure"] != "VI" {
		t.Errorf("incident must reference Annexure VI")
	}
}

func TestPEPRoutesToEDD(t *testing.T) {
	v := New().Decide(Application{
		CustomerID:        "c-3",
		PANNumber:         "ABCPS1234F",
		NameOnPAN:         "Asha Singh",
		AadhaarOfflineKYC: true,
		NameOnAadhaar:     "Asha Singh",
		AddressMatchScore: 0.70, // weak address adds risk
		LivenessScore:     0.85,
		PEPHit:            true,
		HighRiskCountry:   true,
		OccupationHighRisk: true,
	})
	if v.Decision != "edd" {
		t.Fatalf("PEP + high-risk geo + occupation must route to EDD, got %s (score=%.2f)", v.Decision, v.RiskScore)
	}
}

func TestPANStructuralCheck(t *testing.T) {
	v := New().Decide(Application{
		CustomerID:        "c-4",
		PANNumber:         "ABCPX1234F", // 5th char X vs surname Singh → S expected
		NameOnPAN:         "Asha Singh",
		AadhaarOfflineKYC: true,
		NameOnAadhaar:     "Asha Singh",
	})
	hit := false
	for _, r := range v.Reasons {
		if strings.Contains(r, "PAN failed structural") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected PAN structural failure reason; got %+v", v.Reasons)
	}
}

func TestMissingAadhaarOfflineKYC(t *testing.T) {
	v := New().Decide(Application{
		CustomerID:        "c-5",
		PANNumber:         "ABCPS1234F",
		NameOnPAN:         "Asha Singh",
		AadhaarOfflineKYC: false,
		NameOnAadhaar:     "Asha Singh",
	})
	found := false
	for _, n := range v.NextSteps {
		if strings.Contains(n, "UIDAI offline KYC") {
			found = true
		}
	}
	if !found {
		t.Errorf("missing offline KYC should appear in next steps; got %+v", v.NextSteps)
	}
}

func TestNameMismatchCounted(t *testing.T) {
	v := New().Decide(Application{
		CustomerID:        "c-6",
		PANNumber:         "ABCPB1234F",
		NameOnPAN:         "Alpha Bravo",
		NameOnAadhaar:     "Charlie Delta",
		AadhaarOfflineKYC: true,
	})
	hit := false
	for _, r := range v.Reasons {
		if strings.Contains(r, "Name on PAN") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected name-mismatch reason; got %+v", v.Reasons)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	if New().Decide(Application{}).Disclaimer == "" {
		t.Errorf("disclaimer must always be present")
	}
}

func TestHandleMessage_DispatchesDownstream(t *testing.T) {
	app := Application{
		CustomerID: "c-9", PANNumber: "ABCPS1234F", NameOnPAN: "Asha Singh",
		NameOnAadhaar: "Asha Singh", AadhaarOfflineKYC: true,
		AddressMatchScore: 0.9, LivenessScore: 0.9,
	}
	body, _ := json.Marshal(app)
	msg := agent.NewMessage("up", ID, agent.RoleAgent, TypeIn, string(body), nil)
	out, err := New().HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeOut || out[0].To != NextAgent {
		t.Fatalf("expected dispatch to %s/%s; got %+v", NextAgent, TypeOut, out)
	}
}

func TestRiskClassIsHigh(t *testing.T) {
	if New().RiskLevel() != agent.RiskHigh {
		t.Errorf("KYC must be RiskHigh")
	}
}
