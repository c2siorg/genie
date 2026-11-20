package catalog

import (
	"fmt"
	"testing"
)

// --- synthetic_identity: stacked signals (cap + high) and a medium score ---

func TestSyntheticIdentity_HighAndMedium(t *testing.T) {
	h := SyntheticIdentitySpec().Handle

	// Every rule fires -> raw score 1.4 -> capped to 1.0 -> high.
	high := runHandle(t, h, `{
	  "stated_age":30,"bureau_tenure_months":2,"address_velocity":3,
	  "name_on_pan":"Rahul Sharma","name_on_aadhaar":"John Doe",
	  "pan_number":"ABCDE1234F","email_domain":"mailinator.com","phone_created_days":10
	}`)
	if high["label"] != "high" {
		t.Errorf("label = %v, want high (score %v)", high["label"], high["score_0_1"])
	}
	if s, _ := high["score_0_1"].(float64); s != 1.0 {
		t.Errorf("score = %v, want 1.0 (capped)", s)
	}
	if r, _ := high["reasons"].([]any); len(r) < 5 {
		t.Errorf("expected >=5 reasons, got %v", r)
	}

	// Thin file (+0.30) + throwaway domain (+0.15) = 0.45 -> medium.
	med := runHandle(t, h, `{"stated_age":30,"bureau_tenure_months":2,"email_domain":"tempmail.com"}`)
	if med["label"] != "medium" {
		t.Errorf("label = %v, want medium (score %v)", med["label"], med["score_0_1"])
	}
	assertBadInputNoPanic(t, "synthetic_identity", h)
}

// --- var_calculator: every z-score arm, empty returns, and default conf/horizon ---

func TestVar_ConfidenceArmsAndDefaults(t *testing.T) {
	h := VarCalculatorSpec().Handle
	rets := `"daily_returns":[-0.08,-0.05,-0.03,-0.02,-0.01,0,0.01,0.02,0.03,0.05],"portfolio_value_rupees":1000000`

	// Each confidence exercises a distinct zScore branch (incl. conf=100 -> idx<0
	// clamp, conf=50 -> default z=0).
	for _, conf := range []float64{100, 99.9, 99.5, 99, 97.5, 95, 90, 50} {
		in := fmt.Sprintf(`{%s,"confidence_pct":%v,"horizon_days":5}`, rets, conf)
		m := runHandle(t, h, in)
		if _, ok := m["historical"]; !ok {
			t.Fatalf("conf %v: missing historical block", conf)
		}
	}

	// Absent confidence_pct (-> 95) and horizon_days (-> 1) hit the default arms.
	def := runHandle(t, h, fmt.Sprintf(`{%s}`, rets))
	if _, ok := def["parametric"]; !ok {
		t.Fatal("expected parametric block on defaults")
	}

	// Empty returns -> the "cannot compute" note path.
	empty := runHandle(t, h, `{"daily_returns":[],"confidence_pct":99}`)
	if note, _ := empty["note"].(string); note == "" {
		t.Errorf("expected a note for empty returns, got %v", empty)
	}
	assertBadInputNoPanic(t, "var", h)
}

// --- mf_screener: normalisation extremes + each hard-filter rejection ---

func TestMfScreener_NormExtremesAndFilters(t *testing.T) {
	h := MfScreenerSpec().Handle

	// No filter; one fund maxes every normalised term, the other floors them.
	m := runHandle(t, h, `{"funds":[
	  {"scheme":"Hi","five_year_cagr":0.30,"std_dev":0.01,"expense_ratio":0,"neg_quarters_last_5yr":0},
	  {"scheme":"Lo","five_year_cagr":-0.05,"std_dev":0,"expense_ratio":0.05,"neg_quarters_last_5yr":10}
	]}`)
	ranked, _ := m["ranked"].([]any)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked, got %d", len(ranked))
	}
	if ranked[0].(map[string]any)["scheme"] != "Hi" {
		t.Errorf("expected Hi first, got %v", ranked[0])
	}

	// Each fund fails a different hard filter (AUM, then expense, then 3yr CAGR).
	f := runHandle(t, h, `{"filter":{"min_aum_crore":100,"max_expense_ratio":0.01,"min_three_yr_cagr":0.10},"funds":[
	  {"scheme":"AumFail","aum_crore":50,"expense_ratio":0.005,"three_year_cagr":0.20},
	  {"scheme":"ExpFail","aum_crore":200,"expense_ratio":0.05,"three_year_cagr":0.20},
	  {"scheme":"CagrFail","aum_crore":200,"expense_ratio":0.005,"three_year_cagr":0.05}
	]}`)
	if fo, _ := f["filtered_out"].(float64); fo != 3 {
		t.Errorf("filtered_out = %v, want 3", fo)
	}
}

// --- sip_vs_lumpsum: real simulation loop favouring lumpsum ---

func TestSipVsLumpsum_LumpsumFavoured(t *testing.T) {
	h := SipVsLumpsumSpec().Handle
	// High return + low vol over a long horizon -> lumpsum wins most paths.
	m := runHandle(t, h, `{"amount_rupees":120000,"horizon_months":36,"expected_annual_return":0.15,"annual_volatility":0.05,"paths":400}`)
	if p, _ := m["lumpsum_win_probability"].(float64); p < 0.70 {
		t.Errorf("expected lumpsum win prob >=0.70, got %v", p)
	}
	if rec, _ := m["recommendation"].(string); rec == "" {
		t.Error("expected a recommendation")
	}
	if l, _ := m["lumpsum_p50"].(float64); l <= 0 {
		t.Errorf("expected positive lumpsum p50, got %v", l)
	}
	assertBadInputNoPanic(t, "sip_vs_lumpsum", h)
}

// --- asset_allocator: risk arms, horizon bump, and a real rebalance ---

func TestAssetAllocator_RebalanceAndRiskArms(t *testing.T) {
	h := AssetAllocatorSpec().Handle

	// Aggressive + long horizon + an all-equity current book -> sell equity, buy others.
	m := runHandle(t, h, `{"age":30,"horizon_years":15,"risk_tolerance":"aggressive","current":{"equity_rupees":100000,"debt_rupees":0,"gold_rupees":0,"cash_rupees":0}}`)
	reb, _ := m["rebalance"].([]any)
	actions := map[string]string{}
	for _, r := range reb {
		rm := r.(map[string]any)
		actions[rm["asset"].(string)] = rm["action"].(string)
	}
	if actions["equity"] != "sell" {
		t.Errorf("equity action = %v, want sell", actions["equity"])
	}
	if actions["debt"] != "buy" {
		t.Errorf("debt action = %v, want buy", actions["debt"])
	}
	cur, _ := m["current_allocation"].(map[string]any)
	if eq, _ := cur["equity"].(float64); eq != 1 {
		t.Errorf("current equity allocation = %v, want 1", eq)
	}

	// Conservative arm lowers equity.
	c := runHandle(t, h, `{"age":40,"risk_tolerance":"conservative"}`)
	tgt, _ := c["target_allocation"].(map[string]any)
	if eq, _ := tgt["equity"].(float64); eq >= 0.6 {
		t.Errorf("conservative equity = %v, expected reduced", eq)
	}
	assertBadInputNoPanic(t, "asset_allocator", h)
}

// --- invoice_processor: each GSTIN structural branch + match failures + hold ---

func TestInvoiceProcessor_GstinBranches(t *testing.T) {
	h := InvoiceProcessorSpec().Handle
	// Each 15-char string is valid except at exactly one position.
	malformed := []string{
		"A7AAAAA0000A1Z5", // g[0] not a digit
		"27A1AAA0000A1Z5", // g[3] not a letter
		"27AAAAAX000A1Z5", // g[7] not a digit
		"27AAAAA000011Z5", // g[11] not a letter
		"27AAAAA0000A-Z5", // g[12] not alphanumeric
		"27AAAAA0000A1X5", // g[13] not 'Z'
	}
	for _, g := range malformed {
		in := fmt.Sprintf(`{"invoice":{"irn":"I","supplier_gstin":%q,"buyer_gstin":"29BBBBB1111B2Z6","total_rupees":100,"po_reference":"P","items":[]},"reference":{"po_total_rupees":100,"vendor_active":true}}`, g)
		m := runHandle(t, h, in)
		if iss, _ := m["issues"].([]any); len(iss) == 0 {
			t.Errorf("gstin %q: expected a GSTIN issue", g)
		}
	}
}

func TestInvoiceProcessor_MatchFailuresAndHold(t *testing.T) {
	h := InvoiceProcessorSpec().Handle
	valid := `"supplier_gstin":"27AAAAA0000A1Z5","buyer_gstin":"29BBBBB1111B2Z6"`

	// PO total deviates beyond tolerance.
	po := runHandle(t, h, `{"invoice":{"irn":"I",`+valid+`,"total_rupees":20000,"po_reference":"P","items":[]},"reference":{"po_total_rupees":10000,"vendor_active":true}}`)
	issuesContain(t, po, "PO")

	// HSN absent on the GRN.
	grnMiss := runHandle(t, h, `{"invoice":{"irn":"I",`+valid+`,"total_rupees":100,"po_reference":"P","items":[{"hsn":"9999","quantity":1,"unit_price_rupees":100,"gst_pct":0,"line_total_rupees":100}]},"reference":{"po_total_rupees":100,"vendor_active":true,"grn_qty_by_hsn":{"1001":10}}}`)
	issuesContain(t, grnMiss, "GRN")

	// GRN qty differs beyond tolerance (grn 100 vs invoice 200); qty 0 rows skipped.
	grnDev := runHandle(t, h, `{"invoice":{"irn":"I",`+valid+`,"total_rupees":100,"po_reference":"P","items":[{"hsn":"1001","quantity":200,"unit_price_rupees":1,"gst_pct":0,"line_total_rupees":200},{"hsn":"2002","quantity":5,"unit_price_rupees":1,"gst_pct":0,"line_total_rupees":5}]},"reference":{"po_total_rupees":100,"vendor_active":true,"grn_qty_by_hsn":{"1001":100,"2002":0}}}`)
	issuesContain(t, grnDev, "quantity")

	// Line total mismatch (expected 11800, stated 5000).
	line := runHandle(t, h, `{"invoice":{"irn":"I",`+valid+`,"total_rupees":100,"po_reference":"P","items":[{"hsn":"1001","quantity":10,"unit_price_rupees":1000,"gst_pct":18,"line_total_rupees":5000}]},"reference":{"po_total_rupees":100,"vendor_active":true,"grn_qty_by_hsn":{"1001":10}}}`)
	issuesContain(t, line, "Line total")

	// Only the vendor is inactive (-0.25) -> confidence 0.75 -> hold.
	hold := runHandle(t, h, `{"invoice":{"irn":"I",`+valid+`,"total_rupees":100,"po_reference":"P","items":[]},"reference":{"po_total_rupees":100,"vendor_active":false}}`)
	if hold["action"] != "hold" {
		t.Errorf("vendor-inactive only: action = %v, want hold (conf %v)", hold["action"], hold["confidence_0_1"])
	}
	assertBadInputNoPanic(t, "invoice_processor", h)
}

func issuesContain(t *testing.T, m map[string]any, substr string) {
	t.Helper()
	iss, _ := m["issues"].([]any)
	for _, i := range iss {
		if s, ok := i.(string); ok && contains(s, substr) {
			return
		}
	}
	t.Errorf("expected an issue containing %q, got %v", substr, iss)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- coordination_of_benefits: OOP cap, unknown-primary, many claims, rupee fmt ---

func TestCob_OOPCapAndUnknownPrimary(t *testing.T) {
	// A low OOP-max caps the patient share on the primary plan.
	capped := cobRun(t, `{"mode":"individual","plans":{
	  "A":{"deductible_paise":5000000,"coinsurance_rate":0.2,"oop_max_paise":100000},
	  "B":{"deductible_paise":3000000,"coinsurance_rate":0.1}
	},"claims":[{"claim_id":"c1","patient":"P","billed_paise":10000000,"own_plan_id":"A","other_plan_id":"B"}]}`)
	if capped.Claims[0].Payments[0].PaysPaise <= 0 {
		t.Errorf("expected primary to pay something under an OOP cap, got %d", capped.Claims[0].Payments[0].PaysPaise)
	}

	// Standard ordering's primary (own plan) is itself unknown -> error at the
	// primary lookup.
	if _, err := cobHandle(`{"plans":{"B":{"deductible_paise":1,"coinsurance_rate":0.1}},"claims":[{"claim_id":"c1","patient":"P","billed_paise":100,"own_plan_id":"MISSING","other_plan_id":"B"}]}`); err == nil {
		t.Error("expected error when the primary plan is unknown")
	}

	// Bad JSON.
	if _, err := cobHandle(`{not json`); err == nil {
		t.Error("expected bad-input error")
	}
}

func TestCob_TooManyClaimsSkipsOptimizer(t *testing.T) {
	// 17 claims (> 16) -> optimizer skipped, standard ordering reported.
	claims := make([]string, 0, 17)
	for i := 0; i < 17; i++ {
		claims = append(claims, fmt.Sprintf(`{"claim_id":"c%d","patient":"P","billed_paise":100000,"own_plan_id":"A","other_plan_id":"B"}`, i))
	}
	var sb []byte
	sb = append(sb, `{"mode":"individual","plans":{"A":{"deductible_paise":100,"coinsurance_rate":0.1},"B":{"deductible_paise":100,"coinsurance_rate":0.1}},"claims":[`...)
	for i, c := range claims {
		if i > 0 {
			sb = append(sb, ',')
		}
		sb = append(sb, c...)
	}
	sb = append(sb, `]}`...)
	r := cobRun(t, string(sb))
	if !contains(r.Recommendation, "Too many claims") {
		t.Errorf("expected 'Too many claims' recommendation, got %q", r.Recommendation)
	}
}

func TestCobRupees_NegativeSign(t *testing.T) {
	if got := cobRupees(-123456); got != "-₹1234.56" {
		t.Errorf("cobRupees(-123456) = %q, want -₹1234.56", got)
	}
	if got := cobRupees(0); got != "₹0.00" {
		t.Errorf("cobRupees(0) = %q, want ₹0.00", got)
	}
}
