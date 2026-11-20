package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

// runHandle invokes a spec's deterministic Handle and returns the raw output.
func runHandle(t *testing.T, handle func(string) (string, error), input string) map[string]any {
	t.Helper()
	out, err := handle(input)
	if err != nil {
		t.Fatalf("handle error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output not JSON object: %v (%s)", err, out)
	}
	return m
}

// assertBadInputNoPanic feeds malformed and empty inputs and asserts the handler
// returns cleanly (error or empty result) rather than panicking.
func assertBadInputNoPanic(t *testing.T, name string, handle func(string) (string, error)) {
	t.Helper()
	for _, in := range []string{"", "{", "not json", "[]", `{"unexpected":true}`, "null"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked on %q: %v", name, in, r)
				}
			}()
			_, _ = handle(in)
		}()
	}
}

func mustDisclaimer(t *testing.T, m map[string]any) {
	t.Helper()
	d, _ := m["disclaimer"].(string)
	if strings.TrimSpace(d) == "" {
		t.Errorf("output missing non-empty disclaimer: %v", m)
	}
}

// --- fraud_detector ---

func TestFraud_VelocityBurst(t *testing.T) {
	h := FraudSpec().Handle
	input := `{"transactions":[
	  {"transaction_id":"a","date":"2024-01-01","amount_cents":-100,"description":"TIME:10:00"},
	  {"transaction_id":"b","date":"2024-01-01","amount_cents":-100,"description":"TIME:10:01"},
	  {"transaction_id":"c","date":"2024-01-01","amount_cents":-100,"description":"TIME:10:02"},
	  {"transaction_id":"d","date":"2024-01-01","amount_cents":-100,"description":"TIME:10:03"},
	  {"transaction_id":"e","date":"2024-01-01","amount_cents":-100,"description":"TIME:10:04"}
	]}`
	m := runHandle(t, h, input)
	sigs, _ := m["signals"].([]any)
	if len(sigs) != 5 {
		t.Fatalf("expected 5 velocity_burst signals, got %d", len(sigs))
	}
	first := sigs[0].(map[string]any)
	if first["pattern"] != "velocity_burst" || first["severity"] != "high" {
		t.Errorf("unexpected signal: %v", first)
	}
	mustDisclaimer(t, m)
}

func TestFraud_HighRiskCategory(t *testing.T) {
	h := FraudSpec().Handle
	input := `{"transactions":[{"transaction_id":"x","date":"2024-01-01","amount_cents":-5000,"description":"buy","category":"crypto"}]}`
	m := runHandle(t, h, input)
	sigs, _ := m["signals"].([]any)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(sigs))
	}
	if sigs[0].(map[string]any)["pattern"] != "high_risk_category" {
		t.Errorf("expected high_risk_category, got %v", sigs[0])
	}
}

func TestFraud_EmptyBatch(t *testing.T) {
	h := FraudSpec().Handle
	m := runHandle(t, h, `{"transactions":[]}`)
	sigs, _ := m["signals"].([]any)
	if len(sigs) != 0 {
		t.Errorf("expected no signals for empty batch, got %d", len(sigs))
	}
	mustDisclaimer(t, m)
}

func TestFraud_BadInput(t *testing.T) { assertBadInputNoPanic(t, "fraud", FraudSpec().Handle) }

// --- aml_monitor ---

func TestAml_CTRThreshold(t *testing.T) {
	h := AmlMonitorSpec().Handle
	// ₹10L cash deposit -> CTR.
	input := `{"transactions":[{"transaction_id":"t1","account_id":"acc","date":"2024-01-01","amount_cents":100000000,"description":"cash deposit"}]}`
	m := runHandle(t, h, input)
	strs, _ := m["strs"].([]any)
	if len(strs) != 1 {
		t.Fatalf("expected 1 STR, got %d", len(strs))
	}
	if strs[0].(map[string]any)["rule_hit"] != "ctr_threshold" {
		t.Errorf("expected ctr_threshold, got %v", strs[0])
	}
	mustDisclaimer(t, m)
}

func TestAml_AdverseMedia(t *testing.T) {
	h := AmlMonitorSpec().Handle
	input := `{"transactions":[{"transaction_id":"t1","account_id":"acc","date":"2024-01-01","amount_cents":-5000,"description":"transfer to ofac-listed party"}]}`
	m := runHandle(t, h, input)
	strs, _ := m["strs"].([]any)
	if len(strs) != 1 || strs[0].(map[string]any)["rule_hit"] != "adverse_media" {
		t.Fatalf("expected adverse_media STR, got %v", strs)
	}
}

func TestAml_Structuring(t *testing.T) {
	h := AmlMonitorSpec().Handle
	// 3 cash txns each 80-99% of ₹10L within 7 days, same account.
	input := `{"transactions":[
	  {"transaction_id":"s1","account_id":"acc","date":"2024-01-01","amount_cents":90000000,"description":"cash deposit"},
	  {"transaction_id":"s2","account_id":"acc","date":"2024-01-02","amount_cents":90000000,"description":"cash deposit"},
	  {"transaction_id":"s3","account_id":"acc","date":"2024-01-03","amount_cents":90000000,"description":"cash deposit"}
	]}`
	m := runHandle(t, h, input)
	strs, _ := m["strs"].([]any)
	found := false
	for _, s := range strs {
		if s.(map[string]any)["rule_hit"] == "structuring" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a structuring STR, got %v", strs)
	}
}

func TestAml_NoFlags(t *testing.T) {
	h := AmlMonitorSpec().Handle
	m := runHandle(t, h, `{"transactions":[{"transaction_id":"t1","account_id":"a","date":"2024-01-01","amount_cents":-100,"description":"coffee"}]}`)
	strs, _ := m["strs"].([]any)
	if len(strs) != 0 {
		t.Errorf("expected no STRs, got %d", len(strs))
	}
	mustDisclaimer(t, m)
}

func TestAml_BadInput(t *testing.T) { assertBadInputNoPanic(t, "aml", AmlMonitorSpec().Handle) }

// --- synthetic_identity_detector ---

func TestSynthID_HighRisk(t *testing.T) {
	h := SyntheticIdentitySpec().Handle
	// thin file (+0.30) + address velocity (+0.30) + name mismatch (+0.25) = 0.85.
	input := `{"stated_age":30,"bureau_tenure_months":0,"address_velocity":3,"name_on_pan":"John Doe","name_on_aadhaar":"Alice Smith"}`
	m := runHandle(t, h, input)
	if m["label"] != "high" {
		t.Errorf("expected high label, got %v (score %v)", m["label"], m["score_0_1"])
	}
	if s, _ := m["score_0_1"].(float64); s < 0.7 {
		t.Errorf("expected score >= 0.7, got %v", s)
	}
	rec, _ := m["recommendation"].(string)
	if !strings.Contains(strings.ToLower(rec), "due diligence") {
		t.Errorf("high-risk recommendation should mention EDD: %q", rec)
	}
	mustDisclaimer(t, m)
}

func TestSynthID_LowRisk(t *testing.T) {
	h := SyntheticIdentitySpec().Handle
	input := `{"stated_age":40,"bureau_tenure_months":120,"address_velocity":0,"name_on_pan":"Ravi Kumar","name_on_aadhaar":"Ravi Kumar"}`
	m := runHandle(t, h, input)
	if m["label"] != "low" {
		t.Errorf("expected low label, got %v", m["label"])
	}
}

func TestSynthID_BadInput(t *testing.T) {
	assertBadInputNoPanic(t, "synthid", SyntheticIdentitySpec().Handle)
}

// --- tax_harvester ---

func TestTaxHarvester_Golden(t *testing.T) {
	h := TaxHarvesterSpec().Handle
	// One STCL loser: cost 100000, value 80000 -> loss 20000. STCG budget 50000.
	// Saved = 20000 * 0.20 = 4000. Unused STCL = 30000.
	input := `{
	  "as_of_date":"2025-01-01",
	  "realised_gains":{"short_term_rupees":50000,"long_term_rupees":0},
	  "holdings":[{"symbol":"X","quantity":100,"cost_basis_rupees":100000,"current_price_rupees":800,"purchase_date":"2024-06-01"}]
	}`
	m := runHandle(t, h, input)
	if got, _ := m["total_tax_saved_rupees"].(float64); got != 4000 {
		t.Errorf("total_tax_saved_rupees = %v, want 4000", got)
	}
	if got, _ := m["unused_stcl_offset_rupees"].(float64); got != 30000 {
		t.Errorf("unused_stcl_offset_rupees = %v, want 30000", got)
	}
	opps, _ := m["opportunities"].([]any)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}
	opp := opps[0].(map[string]any)
	if opp["gain_category"] != "STCL" {
		t.Errorf("gain_category = %v, want STCL", opp["gain_category"])
	}
	if w, _ := opp["wash_sale_warning"].(string); !strings.Contains(w, "30 days") {
		t.Errorf("expected wash-sale warning, got %q", w)
	}
	mustDisclaimer(t, m)
}

func TestTaxHarvester_NoLosers(t *testing.T) {
	h := TaxHarvesterSpec().Handle
	// A gainer (value > cost) is not harvestable.
	input := `{"realised_gains":{"short_term_rupees":50000},"holdings":[{"symbol":"Y","quantity":10,"cost_basis_rupees":1000,"current_price_rupees":200,"purchase_date":"2024-06-01"}]}`
	m := runHandle(t, h, input)
	if got, _ := m["total_tax_saved_rupees"].(float64); got != 0 {
		t.Errorf("expected 0 tax saved, got %v", got)
	}
	mustDisclaimer(t, m)
}

func TestTaxHarvester_BadInput(t *testing.T) {
	assertBadInputNoPanic(t, "tax_harvester", TaxHarvesterSpec().Handle)
}

// --- var_calculator ---

func TestVar_HistoricalGolden(t *testing.T) {
	h := VarCalculatorSpec().Handle
	// 5 returns, 95% conf, horizon 1, ₹1,000,000 portfolio.
	// Sorted worst = -0.05; alpha=0.05; idx=0; histVaR = 0.05 -> 5% -> ₹50,000.
	input := `{"daily_returns":[-0.05,-0.03,-0.01,0.02,0.04],"portfolio_value_rupees":1000000,"confidence_pct":95,"horizon_days":1}`
	m := runHandle(t, h, input)
	hist := m["historical"].(map[string]any)
	if got, _ := hist["var_pct"].(float64); got != 5 {
		t.Errorf("historical var_pct = %v, want 5", got)
	}
	if got, _ := hist["var_rupees"].(float64); got != 50000 {
		t.Errorf("historical var_rupees = %v, want 50000", got)
	}
	// Parametric section must be present and non-negative.
	para := m["parametric"].(map[string]any)
	if got, _ := para["var_pct"].(float64); got < 0 {
		t.Errorf("parametric var_pct negative: %v", got)
	}
}

func TestVar_EmptyReturns(t *testing.T) {
	h := VarCalculatorSpec().Handle
	m := runHandle(t, h, `{"daily_returns":[],"portfolio_value_rupees":1000000}`)
	note, _ := m["note"].(string)
	if !strings.Contains(note, "No returns") {
		t.Errorf("expected 'No returns' note, got %q", note)
	}
}

func TestVar_HorizonScaling(t *testing.T) {
	h := VarCalculatorSpec().Handle
	// horizon 4 -> sqrt(4)=2x the 1-day VaR.
	input := `{"daily_returns":[-0.05,-0.03,-0.01,0.02,0.04],"portfolio_value_rupees":1000000,"confidence_pct":95,"horizon_days":4}`
	m := runHandle(t, h, input)
	hist := m["historical"].(map[string]any)
	if got, _ := hist["var_pct"].(float64); got != 10 {
		t.Errorf("horizon-scaled var_pct = %v, want 10", got)
	}
}

func TestVar_BadInput(t *testing.T) { assertBadInputNoPanic(t, "var", VarCalculatorSpec().Handle) }

// --- sme_loan_workflow ---

func TestSme_Approved(t *testing.T) {
	h := SmeLoanWorkflowSpec().Handle
	input := `{"borrower_id":"b1","udyam_registered":true,"sector":"manufacturing","annual_turnover_rupees":10000000,"gst_filing_regular":true,"requested_amount_rupees":1000000,"requested_tenor_months":12,"cashflow_score_0_1":0.8,"collateral_rupees":0}`
	m := runHandle(t, h, input)
	if m["decision"] != "approved" {
		t.Errorf("decision = %v, want approved", m["decision"])
	}
	if m["cgtmse_eligible"] != true {
		t.Errorf("cgtmse_eligible = %v, want true", m["cgtmse_eligible"])
	}
	// rate = 10.5 + (1-0.8)*5 - 0.5 = 11.0
	if got, _ := m["indicative_rate_pct"].(float64); got != 11.0 {
		t.Errorf("indicative_rate_pct = %v, want 11.0", got)
	}
	if got, _ := m["offered_amount_rupees"].(float64); got != 1000000 {
		t.Errorf("offered_amount_rupees = %v, want 1000000", got)
	}
	if got, _ := m["monthly_emi_rupees"].(float64); got <= 0 {
		t.Errorf("expected positive EMI, got %v", got)
	}
	mustDisclaimer(t, m)
}

func TestSme_RejectedLowCashflow(t *testing.T) {
	h := SmeLoanWorkflowSpec().Handle
	input := `{"borrower_id":"b2","udyam_registered":true,"sector":"services","annual_turnover_rupees":5000000,"gst_filing_regular":true,"requested_amount_rupees":500000,"requested_tenor_months":12,"cashflow_score_0_1":0.1}`
	m := runHandle(t, h, input)
	if m["decision"] != "rejected" {
		t.Errorf("decision = %v, want rejected", m["decision"])
	}
}

func TestSme_TurnoverCap(t *testing.T) {
	h := SmeLoanWorkflowSpec().Handle
	// Requested 5,000,000 but 30% of 6,000,000 turnover caps at 1,800,000.
	input := `{"borrower_id":"b3","udyam_registered":false,"sector":"trading","annual_turnover_rupees":6000000,"requested_amount_rupees":5000000,"requested_tenor_months":24,"cashflow_score_0_1":0.9}`
	m := runHandle(t, h, input)
	if got, _ := m["offered_amount_rupees"].(float64); got != 1800000 {
		t.Errorf("offered_amount_rupees = %v, want 1800000 (turnover-capped)", got)
	}
}

func TestSme_BadInput(t *testing.T) { assertBadInputNoPanic(t, "sme", SmeLoanWorkflowSpec().Handle) }

// --- kyc_orchestrator ---

func TestKyc_SanctionsAutoReject(t *testing.T) {
	h := KycOrchestratorSpec().Handle
	input := `{"customer_id":"c1","sanctions_hit":true}`
	m := runHandle(t, h, input)
	if m["decision"] != "reject" {
		t.Errorf("decision = %v, want reject", m["decision"])
	}
	payload, _ := m["incident_payload"].(string)
	if !strings.Contains(payload, "annexure") || !strings.Contains(payload, "VI") {
		t.Errorf("expected Annexure VI incident payload, got %q", payload)
	}
	mustDisclaimer(t, m)
}

func TestKyc_CleanApprove(t *testing.T) {
	h := KycOrchestratorSpec().Handle
	// Valid PAN (4th=P, 5th=K matches surname Kumar), offline KYC, name match,
	// good address/liveness, no flags -> score 0 -> approve, sdd tier.
	input := `{"customer_id":"c2","pan_number":"ABCPK1234Z","name_on_pan":"Ravi Kumar","aadhaar_offline_kyc":true,"name_on_aadhaar":"Ravi Kumar","address_match_score_0_1":0.9,"liveness_score_0_1":0.8}`
	m := runHandle(t, h, input)
	if m["decision"] != "approve" {
		t.Errorf("decision = %v, want approve (reasons %v)", m["decision"], m["reasons"])
	}
	if m["tier"] != "sdd" {
		t.Errorf("tier = %v, want sdd", m["tier"])
	}
}

func TestKyc_EDDOnPEP(t *testing.T) {
	h := KycOrchestratorSpec().Handle
	// PEP (+0.35) + high-risk country (+0.20) + missing offline KYC (+0.20) = 0.75 >= 0.70 -> EDD.
	input := `{"customer_id":"c3","pan_number":"ABCPK1234Z","name_on_pan":"Ravi Kumar","name_on_aadhaar":"Ravi Kumar","address_match_score_0_1":0.9,"pep_hit":true,"high_risk_country":true}`
	m := runHandle(t, h, input)
	if m["decision"] != "edd" {
		t.Errorf("decision = %v, want edd (score %v)", m["decision"], m["risk_score_0_1"])
	}
}

func TestKyc_BadInput(t *testing.T) { assertBadInputNoPanic(t, "kyc", KycOrchestratorSpec().Handle) }
