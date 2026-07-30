package catalog

import (
	"encoding/json"
	"testing"
)

func cobRun(t *testing.T, input string) cobResult {
	t.Helper()
	out, err := cobHandle(input)
	if err != nil {
		t.Fatalf("cobHandle: %v", err)
	}
	var r cobResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return r
}

// Exact adjudication + secondary non-duplication on a single claim.
// Plan A (own/primary): deductible ₹50,000, coinsurance 20%.
// Plan B (secondary):    deductible ₹30,000, coinsurance 10%.
// Billed ₹1,00,000.
//
//	Primary A: ded 50,000 + 20%*50,000 = 60,000 patient share; A pays 40,000.
//	B as primary: ded 30,000 + 10%*70,000 = 37,000 share; B-as-primary pays 63,000.
//	Secondary pays min(max(0, 63,000-40,000), 60,000) = 23,000.
//	Patient OOP = 1,00,000 - 40,000 - 23,000 = 37,000.
func TestCob_SingleClaimAdjudication(t *testing.T) {
	input := `{
	  "mode":"individual",
	  "plans":{
	    "A":{"deductible_paise":5000000,"coinsurance_rate":0.2},
	    "B":{"deductible_paise":3000000,"coinsurance_rate":0.1}
	  },
	  "claims":[{"claim_id":"c1","patient":"P","billed_paise":10000000,"own_plan_id":"A","other_plan_id":"B"}]
	}`
	r := cobRun(t, input)
	if len(r.Claims) != 1 {
		t.Fatalf("want 1 claim result, got %d", len(r.Claims))
	}
	c := r.Claims[0]
	if c.PatientOOPPaise != 3700000 {
		t.Errorf("patient OOP = %d, want 3700000", c.PatientOOPPaise)
	}
	if c.Payments[0].PaysPaise != 4000000 {
		t.Errorf("primary pays = %d, want 4000000", c.Payments[0].PaysPaise)
	}
	if c.Payments[1].PaysPaise != 2300000 {
		t.Errorf("secondary pays = %d, want 2300000", c.Payments[1].PaysPaise)
	}
	if r.TotalPatientOOPPaise != 3700000 {
		t.Errorf("total OOP = %d, want 3700000", r.TotalPatientOOPPaise)
	}
}

// COB invariants must hold on every claim: 0 <= patient_oop <= billed, and the
// two plans together never pay more than the billed amount (non-duplication).
func TestCob_Invariants(t *testing.T) {
	input := `{
	  "mode":"family",
	  "plans":{
	    "A":{"deductible_paise":4000000,"coinsurance_rate":0.25,"oop_max_paise":15000000},
	    "B":{"deductible_paise":2000000,"coinsurance_rate":0.15,"oop_max_paise":12000000}
	  },
	  "claims":[
	    {"claim_id":"c1","patient":"X","billed_paise":8000000,"own_plan_id":"A","other_plan_id":"B"},
	    {"claim_id":"c2","patient":"Y","billed_paise":6000000,"own_plan_id":"B","other_plan_id":"A"},
	    {"claim_id":"c3","patient":"X","billed_paise":9000000,"own_plan_id":"A","other_plan_id":"B"}
	  ]
	}`
	r := cobRun(t, input)
	var sum int64
	for _, c := range r.Claims {
		if c.PatientOOPPaise < 0 || c.PatientOOPPaise > c.BilledPaise {
			t.Errorf("claim %s: patient OOP %d out of [0,%d]", c.ClaimID, c.PatientOOPPaise, c.BilledPaise)
		}
		payout := c.Payments[0].PaysPaise + c.Payments[1].PaysPaise
		if payout > c.BilledPaise {
			t.Errorf("claim %s: total plan payout %d exceeds billed %d", c.ClaimID, payout, c.BilledPaise)
		}
		if c.Payments[0].PaysPaise+c.Payments[1].PaysPaise+c.PatientOOPPaise != c.BilledPaise {
			t.Errorf("claim %s: payouts + OOP != billed", c.ClaimID)
		}
		sum += c.PatientOOPPaise
	}
	if sum != r.TotalPatientOOPPaise {
		t.Errorf("sum of claim OOP %d != total %d", sum, r.TotalPatientOOPPaise)
	}
	// The optimizer must never do worse than the standard ordering.
	if r.TotalPatientOOPPaise > r.StandardOOPPaise {
		t.Errorf("optimized OOP %d worse than standard %d", r.TotalPatientOOPPaise, r.StandardOOPPaise)
	}
	if r.SavingsVsStandardPaise < 0 {
		t.Errorf("negative savings %d", r.SavingsVsStandardPaise)
	}
	if r.Disclaimer == "" {
		t.Error("output must carry a disclaimer")
	}
}

func TestCob_Determinism(t *testing.T) {
	input := `{"mode":"individual","plans":{"A":{"deductible_paise":5000000,"coinsurance_rate":0.2},"B":{"deductible_paise":3000000,"coinsurance_rate":0.1}},"claims":[{"claim_id":"c1","patient":"P","billed_paise":10000000,"own_plan_id":"A","other_plan_id":"B"}]}`
	a, _ := cobHandle(input)
	b, _ := cobHandle(input)
	if a != b {
		t.Error("cobHandle is not deterministic")
	}
}

func TestCob_UnknownPlan(t *testing.T) {
	input := `{"plans":{"A":{"deductible_paise":1,"coinsurance_rate":0.1}},"claims":[{"claim_id":"c1","patient":"P","billed_paise":100,"own_plan_id":"A","other_plan_id":"MISSING"}]}`
	if _, err := cobHandle(input); err == nil {
		t.Error("expected error for unknown plan reference")
	}
}
