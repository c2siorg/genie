package catalog

import (
	"testing"
)

// --- phishing_classifier: url sub-signals, text lures, VPA, and score capped ---

func TestPhishing_URLSignals(t *testing.T) {
	h := PhishingClassifierSpec().Handle

	// Punycode host.
	puny := runHandle(t, h, `{"url":"http://xn--80ak6aa92e.com/login"}`)
	if _, ok := puny["reasons"]; !ok {
		t.Error("expected reasons for punycode URL")
	}
	// Deep subdomain chain (>3 dots).
	deep := runHandle(t, h, `{"url":"http://a.b.c.d.example.com/pay"}`)
	if s, _ := deep["score_0_1"].(float64); s <= 0 {
		t.Errorf("expected positive score for deep subdomain, got %v", s)
	}
	// '@' credential-confusion in the raw URL.
	at := runHandle(t, h, `{"url":"http://legit.com@evil.example/login"}`)
	if s, _ := at["score_0_1"].(float64); s <= 0 {
		t.Errorf("expected positive score for @ URL, got %v", s)
	}
	// Hostless string -> scoreURL returns nothing.
	none := runHandle(t, h, `{"url":"notaurl"}`)
	if s, _ := none["score_0_1"].(float64); s != 0 {
		t.Errorf("hostless URL score = %v, want 0", s)
	}
}

func TestPhishing_TextVPAAndCap(t *testing.T) {
	h := PhishingClassifierSpec().Handle

	// Lottery + fake-KYC lures.
	lure := runHandle(t, h, `{"text":"You have won a lottery! kyc update required now."}`)
	if s, _ := lure["score_0_1"].(float64); s < 0.4 {
		t.Errorf("expected suspicious score for lottery+kyc, got %v", s)
	}
	// VPA without an '@' -> scoreVPA early return.
	novpa := runHandle(t, h, `{"upi_vpa":"noatsign"}`)
	if s, _ := novpa["score_0_1"].(float64); s != 0 {
		t.Errorf("VPA without @ score = %v, want 0", s)
	}
	// Refund-themed VPA on an unknown PSP handle.
	refund := runHandle(t, h, `{"upi_vpa":"refund@randombank"}`)
	if s, _ := refund["score_0_1"].(float64); s <= 0 {
		t.Errorf("expected positive score for refund VPA, got %v", s)
	}
	// Combined IP + credential + lottery -> score exceeds 1 and is capped.
	capped := runHandle(t, h, `{"url":"http://192.168.1.1/login","text":"share otp now, you have won a lottery"}`)
	if s, _ := capped["score_0_1"].(float64); s != 1 {
		t.Errorf("expected capped score 1, got %v", s)
	}
	if capped["label"] != "phishing" {
		t.Errorf("label = %v, want phishing", capped["label"])
	}
	assertBadInputNoPanic(t, "phishing", h)
}

// --- advance_tax_planner: Jan-Mar FY rollover, past-due shortfall, overpaid ---

func TestAdvanceTax_ShortfallAndFY(t *testing.T) {
	h := AdvanceTaxPlannerSpec().Handle

	// As-of February: month < April -> FY rollover to previous year. With nothing
	// paid, the three already-passed instalments carry a shortfall note.
	feb := runHandle(t, h, `{"projected_annual_tax_rupees":100000,"paid_so_far_rupees":0,"as_of_date":"2025-02-10"}`)
	sched, _ := feb["schedule"].([]any)
	if len(sched) != 4 {
		t.Fatalf("expected 4 instalments, got %d", len(sched))
	}
	pastNote := false
	for _, s := range sched {
		if n, _ := s.(map[string]any)["shortfall_note"].(string); n != "" {
			pastNote = true
		}
	}
	if !pastNote {
		t.Error("expected at least one shortfall note for missed instalments")
	}

	// Overpaid: still_due floors at 0.
	over := runHandle(t, h, `{"projected_annual_tax_rupees":100000,"paid_so_far_rupees":500000,"as_of_date":"2024-06-01"}`)
	os, _ := over["schedule"].([]any)
	for _, s := range os {
		if d, _ := s.(map[string]any)["still_due_rupees"].(float64); d != 0 {
			t.Errorf("overpaid still_due = %v, want 0", d)
		}
	}
	assertBadInputNoPanic(t, "advance_tax", h)
}

// --- tax_harvester: LTCL offset, budget exhaustion, bad date, exemption floor ---

func TestTaxHarvester_LTCLAndBudget(t *testing.T) {
	h := TaxHarvesterSpec().Handle

	// Realised LTCG well above the ₹1.25L exemption -> a positive LTCL budget.
	// Two long-held losers: the bigger one soaks the whole budget, the smaller
	// one then gets zero offset and is skipped. A third holding has a bad date.
	in := `{"realised_gains":{"short_term_rupees":0,"long_term_rupees":300000},"as_of_date":"2025-01-01","holdings":[
	  {"symbol":"BIG","quantity":100,"cost_basis_rupees":400000,"current_price_rupees":2000,"purchase_date":"2023-01-01"},
	  {"symbol":"SMALL","quantity":10,"cost_basis_rupees":100000,"current_price_rupees":2000,"purchase_date":"2023-01-01"},
	  {"symbol":"BADDATE","quantity":10,"cost_basis_rupees":50000,"current_price_rupees":1000,"purchase_date":"not-a-date"}
	]}`
	m := runHandle(t, h, in)
	opps, _ := m["opportunities"].([]any)
	if len(opps) == 0 {
		t.Fatal("expected at least one LTCL opportunity")
	}
	// First (biggest) opportunity is LTCL and saved tax > 0.
	first := opps[0].(map[string]any)
	if first["gain_category"] != "LTCL" {
		t.Errorf("expected LTCL category, got %v", first["gain_category"])
	}
	if ts, _ := m["total_tax_saved_rupees"].(float64); ts <= 0 {
		t.Errorf("expected positive tax saved, got %v", ts)
	}

	// LTCG below exemption -> taxableLTCG floors at 0 (max0 negative branch).
	low := runHandle(t, h, `{"realised_gains":{"long_term_rupees":50000},"holdings":[]}`)
	if b, _ := low["unused_ltcl_offset_rupees"].(float64); b != 0 {
		t.Errorf("expected 0 LTCL budget below exemption, got %v", b)
	}
	assertBadInputNoPanic(t, "tax_harvester", h)
}

// --- health_preauth: procedure-package capped, sum-insured capped, negative approved ---

func TestHealthPreauth_CapsAndZero(t *testing.T) {
	h := HealthPreauthSpec().Handle

	// Procedure package capped bites (gross 100k > capped 50k).
	pkg := runHandle(t, h, `{"request":{"preauth_id":"a","network_ppn":true,"procedure":"knee","estimated_bill_rupees":100000},"plan":{"sum_insured_rupees":500000,"procedure_package_rupees":{"knee":50000}}}`)
	if d, _ := pkg["deductions_rupees"].(float64); d < 50000 {
		t.Errorf("expected >=50000 package-capped deduction, got %v", d)
	}

	// Sum-insured capped bites (approved 100k > SI 40k).
	si := runHandle(t, h, `{"request":{"preauth_id":"b","network_ppn":true,"procedure":"minor","estimated_bill_rupees":100000},"plan":{"sum_insured_rupees":40000}}`)
	if a, _ := si["approved_rupees"].(float64); a != 40000 {
		t.Errorf("approved = %v, want capped at 40000", a)
	}

	// Deductions exceed the bill -> approved floors at 0 -> deny.
	zero := runHandle(t, h, `{"request":{"preauth_id":"c","network_ppn":true,"estimated_bill_rupees":10000,"room_rent_per_day_rupees":100000,"length_of_stay_days":10},"plan":{"room_rent_sublimit_rupees":1000,"sum_insured_rupees":500000}}`)
	if zero["action"] != "deny" {
		t.Errorf("action = %v, want deny (approved %v)", zero["action"], zero["approved_rupees"])
	}
	assertBadInputNoPanic(t, "health_preauth", h)
}
