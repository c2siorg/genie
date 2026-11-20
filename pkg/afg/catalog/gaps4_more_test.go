package catalog

import (
	"strings"
	"testing"
)

// --- kyc_orchestrator: clean pass, empty-name PAN escape, stacked EDD cap ---

func TestKyc_CleanApproveAndOverlap(t *testing.T) {
	h := KycOrchestratorSpec().Handle
	// Valid PAN (4th 'P', 5th matches surname), overlapping names, strong scores,
	// no risk flags -> score 0 -> SDD tier, approve. Exercises the name-overlap
	// "true" path and the passing PAN structural check.
	m := runHandle(t, h, `{"customer_id":"k1","pan_number":"ABPPS1234C","name_on_pan":"Rahul Sharma",
	  "aadhaar_offline_kyc":true,"name_on_aadhaar":"Rahul Sharma","address_match_score_0_1":0.95,
	  "liveness_score_0_1":0.9}`)
	if m["decision"] != "approve" {
		t.Errorf("decision = %v, want approve", m["decision"])
	}
	if m["tier"] != "sdd" {
		t.Errorf("tier = %v, want sdd", m["tier"])
	}
}

func TestKyc_EmptyNameEscape(t *testing.T) {
	h := KycOrchestratorSpec().Handle
	// PAN structurally fine (4th 'P') but no name -> surname check is skipped
	// (returns valid, no penalty). Aadhaar offline present, clean otherwise.
	m := runHandle(t, h, `{"customer_id":"k2","pan_number":"ABPPS1234C","name_on_pan":"",
	  "aadhaar_offline_kyc":true,"address_match_score_0_1":0.95}`)
	// No PAN penalty -> low score.
	if s, _ := m["risk_score_0_1"].(float64); s > 0.3 {
		t.Errorf("empty-name PAN should not be penalised; score = %v", s)
	}
}

func TestKyc_StackedEDDAndSanctions(t *testing.T) {
	h := KycOrchestratorSpec().Handle

	// Every negative signal fires -> score caps at 1.0 -> EDD. PAN 4th char is
	// not 'P' (structural fail), names mismatch, weak address+liveness, PEP,
	// high-risk country, high-risk occupation.
	edd := runHandle(t, h, `{"customer_id":"k3","pan_number":"ABCDS1234C","name_on_pan":"Rahul Sharma",
	  "aadhaar_offline_kyc":false,"name_on_aadhaar":"John Doe","address_match_score_0_1":0.4,
	  "liveness_score_0_1":0.5,"pep_hit":true,"high_risk_country":true,"occupation_high_risk":true}`)
	if edd["decision"] != "edd" {
		t.Errorf("decision = %v, want edd", edd["decision"])
	}
	if s, _ := edd["risk_score_0_1"].(float64); s != 1 {
		t.Errorf("stacked score = %v, want capped 1.0", s)
	}

	// A sanctions hit is an automatic reject with an Annexure VI incident payload.
	rej := runHandle(t, h, `{"customer_id":"k4","pan_number":"ABPPS1234C","name_on_pan":"Rahul Sharma","sanctions_hit":true}`)
	if rej["decision"] != "reject" {
		t.Errorf("sanctions: decision = %v, want reject", rej["decision"])
	}
	if p, _ := rej["incident_payload"].(string); !strings.Contains(p, "VI") {
		t.Errorf("expected Annexure VI incident payload, got %q", p)
	}
	assertBadInputNoPanic(t, "kyc", h)
}

// --- complaint_triage: each low-severity suggestAction branch + long summary ---

func TestComplaint_SuggestActionBranches(t *testing.T) {
	h := ComplaintTriageSpec().Handle

	cases := []struct {
		text     string
		wantCat  string
		wantHint string
	}{
		{"my upi transfer is delayed", "digital_banking_upi_imps", "NPCI"},
		{"my debit card failed at the atm counter", "cards", "chargeback"},
		{"there is a service charge i do not understand", "charges_and_fees", "tariff"},
		{"the branch staff was unhelpful and dismissive", "staff_conduct", "branch head"},
		{"this insurance policy was mis sold to me", "mis_selling", "suitability"},
	}
	for _, c := range cases {
		m := runHandle(t, h, `{"user_id":"u","complaint_text":`+jsonStr(c.text)+`}`)
		if m["category"] != c.wantCat {
			t.Errorf("%q: category = %v, want %v", c.text, m["category"], c.wantCat)
		}
		inc, _ := m["incident_draft"].(map[string]any)
		if act, _ := inc["suggested_action"].(string); !strings.Contains(act, c.wantHint) {
			t.Errorf("%q: suggested_action %q lacks %q", c.text, act, c.wantHint)
		}
	}

	// A >240-char complaint exercises the summary-truncation branch.
	long := strings.Repeat("my loan emi problem ", 20) // 400 chars
	m := runHandle(t, h, `{"user_id":"u","complaint_text":`+jsonStr(long)+`}`)
	inc, _ := m["incident_draft"].(map[string]any)
	if sum, _ := inc["summary"].(string); !strings.HasSuffix(sum, "…") {
		t.Errorf("expected truncated summary ending in ellipsis, got %q", sum)
	}

	// A high-severity escalator drives severity to high (rankSeverity high arm).
	hi := runHandle(t, h, `{"user_id":"u","complaint_text":"money was stolen from my savings account"}`)
	if hi["severity"] != "high" {
		t.Errorf("severity = %v, want high", hi["severity"])
	}
	assertBadInputNoPanic(t, "complaint_triage", h)
}

// jsonStr quotes a string for safe embedding in a JSON literal.
func jsonStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
