package catalog

import (
	"strings"
	"testing"
)

// --- fraud_detector (impossible travel + after-hours branches) ---

func TestFraud_ImpossibleTravel(t *testing.T) {
	h := FraudSpec().Handle
	// Delhi at 10:00 then London at 10:30 -> ~6700km in 0.5h -> impossible travel.
	input := `{"transactions":[
	  {"transaction_id":"a","date":"2024-01-01","amount_cents":-100,"description":"TIME:10:00 GEO:28.6,77.2"},
	  {"transaction_id":"b","date":"2024-01-01","amount_cents":-100,"description":"TIME:10:30 GEO:51.5,-0.12"}
	]}`
	m := runHandle(t, h, input)
	sigs, _ := m["signals"].([]any)
	found := false
	for _, s := range sigs {
		if s.(map[string]any)["pattern"] == "impossible_travel" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected impossible_travel signal, got %v", sigs)
	}
}

func TestFraud_AfterHoursLargeDebit(t *testing.T) {
	h := FraudSpec().Handle
	// ₹60k debit at 02:00 local -> after_hours_large_debit.
	input := `{"transactions":[{"transaction_id":"n","date":"2024-01-01","amount_cents":-6000000,"description":"TIME:02:00 withdrawal"}]}`
	m := runHandle(t, h, input)
	sigs, _ := m["signals"].([]any)
	found := false
	for _, s := range sigs {
		if s.(map[string]any)["pattern"] == "after_hours_large_debit" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected after_hours_large_debit signal, got %v", sigs)
	}
}

// --- loan_advisor ---

func TestLoan_EligibleAndNot(t *testing.T) {
	h := LoanSpec().Handle
	// High monthly net -> DSCR above threshold -> eligible.
	m := runHandle(t, h, `{"principal_cents":12000000,"apr_pct":12,"term_months":24,"monthly_net_cents":10000000}`)
	if m["eligible"] != true {
		t.Errorf("expected eligible, got %v (dscr %v)", m["eligible"], m["dscr"])
	}
	if got, _ := m["emi_cents"].(float64); got <= 0 {
		t.Errorf("expected positive EMI, got %v", got)
	}
	// Low monthly net -> ineligible.
	m2 := runHandle(t, h, `{"principal_cents":12000000,"apr_pct":12,"term_months":24,"monthly_net_cents":100000}`)
	if m2["eligible"] != false {
		t.Errorf("expected ineligible, got %v", m2["eligible"])
	}
	// Zero APR path (division-free amortisation branch).
	m3 := runHandle(t, h, `{"principal_cents":1200000,"apr_pct":0,"term_months":12,"monthly_net_cents":5000000}`)
	if got, _ := m3["emi_cents"].(float64); got != 100000 {
		t.Errorf("zero-apr emi = %v, want 100000", got)
	}
}

func TestLoan_InvalidParams(t *testing.T) {
	h := LoanSpec().Handle
	if _, err := h(`{"principal_cents":0,"apr_pct":10,"term_months":12}`); err == nil {
		t.Error("expected error for zero principal")
	}
	if _, err := h(`{"principal_cents":100,"apr_pct":10,"term_months":0}`); err == nil {
		t.Error("expected error for zero term")
	}
	assertBadInputNoPanic(t, "loan", h)
}

// --- subscription_detector ---

func TestSubscription_DetectsRecurringAndZombie(t *testing.T) {
	h := SubscriptionDetectorSpec().Handle
	// Netflix debited on the 5th for 4 months; last charge jumps 25% -> zombie.
	input := `{"transactions":[
	  {"merchant":"Netflix","date":"2024-01-05","amount_cents":-49900},
	  {"merchant":"Netflix","date":"2024-02-05","amount_cents":-49900},
	  {"merchant":"Netflix","date":"2024-03-05","amount_cents":-49900},
	  {"merchant":"Netflix","date":"2024-04-05","amount_cents":-64900}
	]}`
	m := runHandle(t, h, input)
	subs, _ := m["subscriptions"].([]any)
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d (%v)", len(subs), m)
	}
	s := subs[0].(map[string]any)
	if w, _ := s["zombie_warning"].(string); w == "" {
		t.Errorf("expected zombie warning on 25%% price jump, got %v", s)
	}
	if got, _ := m["total_annualised_rupees"].(float64); got <= 0 {
		t.Errorf("expected positive annualised total, got %v", got)
	}
	if r, _ := m["recommendation"].(string); r == "" {
		t.Error("expected a recommendation")
	}
}

func TestSubscription_TooFewOccurrences(t *testing.T) {
	h := SubscriptionDetectorSpec().Handle
	m := runHandle(t, h, `{"transactions":[
	  {"merchant":"Gym","date":"2024-01-05","amount_cents":-100000},
	  {"merchant":"Gym","date":"2024-02-05","amount_cents":-100000}
	]}`)
	if subs, _ := m["subscriptions"].([]any); len(subs) != 0 {
		t.Errorf("2 charges should not qualify, got %d", len(subs))
	}
	assertBadInputNoPanic(t, "subscription", h)
}

// --- health_preauth ---

func TestHealthPreauth_Branches(t *testing.T) {
	h := HealthPreauthSpec().Handle

	// Not in PPN network -> deny.
	deny := runHandle(t, h, `{"request":{"preauth_id":"p1","network_ppn":false,"estimated_bill_rupees":10000},"plan":{}}`)
	if deny["action"] != "deny" {
		t.Errorf("non-network should deny, got %v", deny["action"])
	}
	mustDisclaimer(t, deny)

	// Excluded procedure -> deny.
	excl := runHandle(t, h, `{"request":{"preauth_id":"p2","network_ppn":true,"procedure":"cosmetic","estimated_bill_rupees":10000},"plan":{"excluded_procedures":["cosmetic"]}}`)
	if excl["action"] != "deny" {
		t.Errorf("excluded proc should deny, got %v", excl["action"])
	}

	// PED within waiting -> deny.
	ped := runHandle(t, h, `{"request":{"preauth_id":"p3","network_ppn":true,"is_pre_existing":true,"policy_months_at_admit":5,"estimated_bill_rupees":10000},"plan":{"ped_waiting_months":24}}`)
	if ped["action"] != "deny" {
		t.Errorf("PED waiting should deny, got %v", ped["action"])
	}

	// Specific waiting -> deny.
	spec := runHandle(t, h, `{"request":{"preauth_id":"p4","network_ppn":true,"is_specific_waiting":true,"policy_months_at_admit":1,"estimated_bill_rupees":10000},"plan":{"specific_waiting_months":12}}`)
	if spec["action"] != "deny" {
		t.Errorf("specific waiting should deny, got %v", spec["action"])
	}

	// Room-rent + ICU over sub-limits + copay + package cap + sum insured -> deductions.
	ded := runHandle(t, h, `{"request":{"preauth_id":"p5","network_ppn":true,"procedure":"knee","estimated_bill_rupees":200000,"room_rent_per_day_rupees":8000,"icu_rent_per_day_rupees":15000,"length_of_stay_days":5,"icu_days":2},"plan":{"sum_insured_rupees":150000,"room_rent_sublimit_rupees":5000,"icu_rent_sublimit_rupees":10000,"copay_pct":0.1,"procedure_package_rupees":{"knee":180000}}}`)
	if d, _ := ded["deductions_rupees"].(float64); d <= 0 {
		t.Errorf("expected deductions, got %v", d)
	}
	reasons, _ := ded["deduction_reasons"].([]any)
	if len(reasons) == 0 {
		t.Error("expected deduction reasons")
	}

	// Large bill (>=5L) -> HITL.
	hitl := runHandle(t, h, `{"request":{"preauth_id":"p6","network_ppn":true,"procedure":"cardiac","estimated_bill_rupees":600000},"plan":{"sum_insured_rupees":1000000}}`)
	if hitl["action"] != "hitl" {
		t.Errorf("bill>=5L should be hitl, got %v", hitl["action"])
	}

	// Clean approve_full.
	full := runHandle(t, h, `{"request":{"preauth_id":"p7","network_ppn":true,"procedure":"minor","estimated_bill_rupees":20000},"plan":{"sum_insured_rupees":500000}}`)
	if full["action"] != "approve_full" {
		t.Errorf("clean claim should approve_full, got %v", full["action"])
	}
	assertBadInputNoPanic(t, "health_preauth", h)
}

// --- auto_insurance ---

func TestAutoInsurance_AllKinds(t *testing.T) {
	h := AutoInsuranceSpec().Handle

	// FNOL total loss (repair >= 75% IDV).
	tl := runHandle(t, h, `{"kind":"fnol","idv_rupees":500000,"est_repair_cost_rupees":400000,"lat":28.6,"lng":77.2}`)
	if tl["total_loss_flag"] != true {
		t.Errorf("expected total loss, got %v", tl["total_loss_flag"])
	}
	if tl["action"] != "register_claim_total_loss" {
		t.Errorf("action = %v", tl["action"])
	}
	// FNOL non-total.
	nt := runHandle(t, h, `{"kind":"fnol","idv_rupees":500000,"est_repair_cost_rupees":100000}`)
	if nt["action"] != "register_claim" {
		t.Errorf("action = %v, want register_claim", nt["action"])
	}
	// Roadside.
	rs := runHandle(t, h, `{"kind":"roadside","lat":19.0,"lng":72.9}`)
	if rs["action"] != "dispatch_partner" {
		t.Errorf("action = %v, want dispatch_partner", rs["action"])
	}
	// Renewal quote, clean year, zero-dep, near expiry.
	rq := runHandle(t, h, `{"kind":"renewal_quote","idv_rupees":500000,"ncb_pct":20,"claimed_this_year":false,"zero_dep_addon":true,"hours_to_expiry":24}`)
	if rq["action"] != "quote_ready" {
		t.Errorf("action = %v, want quote_ready", rq["action"])
	}
	if got, _ := rq["new_ncb_pct"].(float64); got != 25 {
		t.Errorf("new NCB = %v, want 25 (bumped from 20)", got)
	}
	// Renewal quote after a claim -> NCB reset to 0.
	rc := runHandle(t, h, `{"kind":"renewal_quote","idv_rupees":500000,"ncb_pct":35,"claimed_this_year":true}`)
	if got, _ := rc["new_ncb_pct"].(float64); got != 0 {
		t.Errorf("claimed year NCB = %v, want 0", got)
	}
	// Unknown kind.
	uk := runHandle(t, h, `{"kind":"weird"}`)
	if uk["action"] != "unknown" {
		t.Errorf("action = %v, want unknown", uk["action"])
	}
	assertBadInputNoPanic(t, "auto_insurance", h)
}

// --- mf_screener ---

func TestMfScreener_RanksAndFilters(t *testing.T) {
	h := MfScreenerSpec().Handle
	input := `{"filter":{"category":"equity","min_aum_crore":100,"max_expense_ratio":0.02,"min_three_yr_cagr":0.05},"funds":[
	  {"scheme":"Good","category":"equity","aum_crore":5000,"expense_ratio":0.01,"three_year_cagr":0.15,"five_year_cagr":0.18,"std_dev":0.12,"neg_quarters_last_5yr":2},
	  {"scheme":"Cheap","category":"equity","aum_crore":2000,"expense_ratio":0.005,"three_year_cagr":0.10,"five_year_cagr":0.12,"std_dev":0.10,"neg_quarters_last_5yr":4},
	  {"scheme":"WrongCat","category":"debt","aum_crore":9000,"expense_ratio":0.004,"three_year_cagr":0.09,"five_year_cagr":0.08,"std_dev":0.03,"neg_quarters_last_5yr":1}
	]}`
	m := runHandle(t, h, input)
	ranked, _ := m["ranked"].([]any)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked (1 filtered), got %d", len(ranked))
	}
	if fo, _ := m["filtered_out"].(float64); fo != 1 {
		t.Errorf("filtered_out = %v, want 1", fo)
	}
	// Best-first: "Good" should outrank "Cheap".
	if ranked[0].(map[string]any)["scheme"] != "Good" {
		t.Errorf("expected Good first, got %v", ranked[0])
	}
	assertBadInputNoPanic(t, "mf_screener", h)
}

// --- prepayment_advisor ---

func TestPrepayment_WaterfallAndFlags(t *testing.T) {
	h := PrepaymentAdvisorSpec().Handle
	input := `{"prepayment_amount_rupees":200000,"borrower_slab_pct":30,"loans":[
	  {"name":"CreditCard","outstanding_rupees":150000,"apr":0.36,"months_remaining":24,"is_fixed_rate":true},
	  {"name":"HomeLoan","outstanding_rupees":3000000,"apr":0.09,"months_remaining":180,"tax_deductible":true}
	]}`
	m := runHandle(t, h, input)
	sug, _ := m["suggestions"].([]any)
	if len(sug) == 0 {
		t.Fatal("expected suggestions")
	}
	// Highest effective rate first: credit card (0.36) over tax-adjusted home loan.
	first := sug[0].(map[string]any)
	if first["loan_name"] != "CreditCard" {
		t.Errorf("expected CreditCard first, got %v", first["loan_name"])
	}
	if flags, _ := first["flags"].([]any); len(flags) == 0 {
		t.Error("expected fixed-rate foreclosure flag on credit card")
	}
	if got, _ := m["total_saving_rupees"].(float64); got <= 0 {
		t.Errorf("expected positive total saving, got %v", got)
	}
	mustDisclaimer(t, m)
	assertBadInputNoPanic(t, "prepayment", h)
}

// --- options_explainer ---

func TestOptions_CallPutAndInvalid(t *testing.T) {
	h := OptionsExplainerSpec().Handle
	call := runHandle(t, h, `{"side":"call","underlying_price":100,"strike":100,"days_to_expiry":30,"implied_volatility_pct":25,"risk_free_rate_pct":6,"lot_size":50}`)
	g := call["greeks_per_share"].(map[string]any)
	if d, _ := g["delta"].(float64); d <= 0 || d > 1 {
		t.Errorf("call delta = %v, want (0,1]", d)
	}
	if pc, _ := call["payoff_curve"].([]any); len(pc) != 21 {
		t.Errorf("payoff curve len = %d, want 21", len(pc))
	}
	put := runHandle(t, h, `{"side":"put","underlying_price":100,"strike":110,"days_to_expiry":30,"implied_volatility_pct":25,"risk_free_rate_pct":6}`)
	gp := put["greeks_per_share"].(map[string]any)
	if d, _ := gp["delta"].(float64); d >= 0 {
		t.Errorf("put delta = %v, want negative", d)
	}
	// Invalid (zero days) -> cannot price disclaimer.
	inv := runHandle(t, h, `{"side":"call","underlying_price":100,"strike":100,"days_to_expiry":0,"implied_volatility_pct":25}`)
	if d, _ := inv["disclaimer"].(string); !strings.Contains(d, "cannot price") {
		t.Errorf("expected cannot-price disclaimer, got %q", d)
	}
	assertBadInputNoPanic(t, "options", h)
}

// --- google_trends ---

func TestGoogleTrends_SurgeFadeSteady(t *testing.T) {
	h := GoogleTrendsSpec().Handle
	input := `{"geo":"IN","window_points":2,"series":[
	  {"keyword":"gold loan","points":[10,10,10,10,50,60]},
	  {"keyword":"crypto","points":[80,80,80,80,10,10]},
	  {"keyword":"savings","points":[40,40,40,40,41,42]}
	]}`
	m := runHandle(t, h, input)
	signals, _ := m["signals"].([]any)
	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d", len(signals))
	}
	dirs := map[string]string{}
	for _, s := range signals {
		sm := s.(map[string]any)
		dirs[sm["keyword"].(string)] = sm["direction"].(string)
	}
	if dirs["gold loan"] != "surging" {
		t.Errorf("gold loan dir = %v, want surging", dirs["gold loan"])
	}
	if dirs["crypto"] != "fading" {
		t.Errorf("crypto dir = %v, want fading", dirs["crypto"])
	}
	if hints, _ := m["downstream_hints"].([]any); len(hints) == 0 {
		t.Error("expected downstream hints for surge/fade")
	}
	assertBadInputNoPanic(t, "google_trends", h)
}

// --- cyber_guardian ---

func TestCyber_HighRiskStacked(t *testing.T) {
	h := CyberGuardianSpec().Handle
	// 5 failed auths (+0.30) + impossible travel Delhi->London in 1h (+0.35)
	// + unknown device (+0.20) -> high.
	input := `{"user_id":"u1","known_device_fps":["fp-known"],"events":[
	  {"successful_auth":false},{"successful_auth":false},{"successful_auth":false},
	  {"successful_auth":false},{"successful_auth":false},
	  {"successful_auth":true,"lat":28.6,"lng":77.2,"unix_millis":1000000000000,"device_fingerprint":"fp-known"},
	  {"successful_auth":true,"lat":51.5,"lng":-0.12,"unix_millis":1000003600000,"device_fingerprint":"fp-new"}
	]}`
	m := runHandle(t, h, input)
	if m["label"] != "high" {
		t.Errorf("label = %v, want high (score %v)", m["label"], m["risk_score_0_1"])
	}
	if flags, _ := m["flags"].([]any); len(flags) < 3 {
		t.Errorf("expected >=3 flags, got %v", flags)
	}
	// Clean/low case.
	low := runHandle(t, h, `{"user_id":"u2","known_device_fps":["fp"],"events":[{"successful_auth":true,"device_fingerprint":"fp","lat":28.6,"lng":77.2,"unix_millis":1}]}`)
	if low["label"] != "low" {
		t.Errorf("clean label = %v, want low", low["label"])
	}
	assertBadInputNoPanic(t, "cyber", h)
}

// --- debt_optimizer ---

func TestDebt_AvalancheAndSnowball(t *testing.T) {
	h := DebtOptimizerSpec().Handle
	base := `"debts":[
	  {"name":"Card","balance_rupees":50000,"apr":0.36,"min_payment_rupees":2000},
	  {"name":"Personal","balance_rupees":200000,"apr":0.14,"min_payment_rupees":5000}
	],"extra_per_month_rupees":10000`
	av := runHandle(t, h, `{`+base+`,"strategy":"avalanche"}`)
	if av["strategy"] != "avalanche" {
		t.Errorf("strategy = %v", av["strategy"])
	}
	order, _ := av["order"].([]any)
	if len(order) != 2 {
		t.Fatalf("expected 2 in payoff order, got %d", len(order))
	}
	if av["order"].([]any)[0] != "Card" {
		t.Errorf("avalanche should clear high-APR Card first, got %v", order)
	}
	if mo, _ := av["months_to_freedom"].(float64); mo <= 0 {
		t.Errorf("expected positive months, got %v", mo)
	}
	// Snowball clears smallest balance first (Card is also smallest here).
	sb := runHandle(t, h, `{`+base+`,"strategy":"snowball"}`)
	if sb["strategy"] != "snowball" {
		t.Errorf("strategy = %v", sb["strategy"])
	}
	// Default (empty strategy) -> avalanche.
	def := runHandle(t, h, `{`+base+`}`)
	if def["strategy"] != "avalanche" {
		t.Errorf("default strategy = %v, want avalanche", def["strategy"])
	}
	assertBadInputNoPanic(t, "debt", h)
}

// --- bulk_statement_analyzer ---

func TestBulk_DedupAndCategories(t *testing.T) {
	h := BulkStatementAnalyzerSpec().Handle
	// An inter-account transfer pair (credit+debit same amount/desc/day) is dropped;
	// remaining debits are categorised.
	input := `{"applicant_id":"a1","transactions":[
	  {"account_id":"A","date":"2024-01-01","description":"transfer","amount_rupees":10000,"type":"debit"},
	  {"account_id":"B","date":"2024-01-01","description":"transfer","amount_rupees":10000,"type":"credit"},
	  {"account_id":"A","date":"2024-01-05","description":"Swiggy order","amount_rupees":800,"type":"debit"},
	  {"account_id":"A","date":"2024-01-10","description":"Rent","amount_rupees":25000,"type":"debit"},
	  {"account_id":"A","date":"2024-02-01","description":"Salary","amount_rupees":100000,"type":"credit"}
	]}`
	m := runHandle(t, h, input)
	if raw, _ := m["txn_count_raw"].(float64); raw != 5 {
		t.Errorf("raw count = %v, want 5", raw)
	}
	if dd, _ := m["txn_count_dedup"].(float64); dd != 3 {
		t.Errorf("dedup count = %v, want 3 (transfer pair removed)", dd)
	}
	if ac, _ := m["account_count"].(float64); ac != 2 {
		t.Errorf("account_count = %v, want 2", ac)
	}
	cats, _ := m["top_debit_categories"].(map[string]any)
	if _, ok := cats["housing:rent"]; !ok {
		t.Errorf("expected housing:rent category, got %v", cats)
	}
	mustDisclaimer(t, m)
	assertBadInputNoPanic(t, "bulk", h)
}

// --- invoice_discounter ---

func TestInvoiceDiscounter_GreedySelect(t *testing.T) {
	h := InvoiceDiscounterSpec().Handle
	input := `{"as_of_date":"2024-01-01","target_cash_rupees":150000,"annualised_rate_bp":1200,"invoices":[
	  {"id":"inv1","counterparty_rating":"AAA","face_value_rupees":100000,"due_on":"2024-02-01"},
	  {"id":"inv2","counterparty_rating":"BBB","face_value_rupees":100000,"due_on":"2024-03-01"},
	  {"id":"inv3","counterparty_rating":"AAA","face_value_rupees":100000,"due_on":"2023-12-01"}
	]}`
	m := runHandle(t, h, input)
	sel, _ := m["selected"].([]any)
	if len(sel) == 0 {
		t.Fatal("expected some selected invoices")
	}
	// Past-due inv3 must be excluded.
	for _, c := range sel {
		if c.(map[string]any)["invoice_id"] == "inv3" {
			t.Error("past-due invoice should be excluded")
		}
	}
	if tot, _ := m["total_net_cash_rupees"].(float64); tot <= 0 {
		t.Errorf("expected positive net cash, got %v", tot)
	}
	assertBadInputNoPanic(t, "invoice_discounter", h)
}

// --- payment_orchestrator ---

func TestPayment_RoutingBranches(t *testing.T) {
	h := PaymentOrchestratorSpec().Handle

	// Small trusted UPI -> submit.
	sub := runHandle(t, h, `{"idempotency_key":"k1","beneficiary_vpa":"a@bank","amount_rupees":1000,"is_trusted_beneficiary":true,"currency":"INR"}`)
	if sub["action"] != "submit" || sub["rail"] != "upi" {
		t.Errorf("small UPI: action=%v rail=%v", sub["action"], sub["rail"])
	}
	// Untrusted + >=50k -> hold_hitl.
	hold := runHandle(t, h, `{"idempotency_key":"k2","beneficiary_vpa":"a@bank","amount_rupees":60000,"is_trusted_beneficiary":false}`)
	if hold["action"] != "hold_hitl" {
		t.Errorf("untrusted large: action=%v, want hold_hitl", hold["action"])
	}
	// Non-INR -> reject with incident payload.
	rej := runHandle(t, h, `{"idempotency_key":"k3","amount_rupees":100,"currency":"USD","beneficiary_vpa":"a@bank"}`)
	if rej["action"] != "reject" {
		t.Errorf("non-INR: action=%v, want reject", rej["action"])
	}
	if p, _ := rej["incident_payload"].(string); !strings.Contains(p, "VI") {
		t.Errorf("expected Annexure VI incident payload, got %q", p)
	}
	// Zero amount -> reject.
	if z := runHandle(t, h, `{"idempotency_key":"k4","amount_rupees":0,"beneficiary_vpa":"a@bank"}`); z["action"] != "reject" {
		t.Errorf("zero amount: action=%v, want reject", z["action"])
	}
	// Missing idempotency key -> reject.
	if mk := runHandle(t, h, `{"amount_rupees":100,"beneficiary_vpa":"a@bank"}`); mk["action"] != "reject" {
		t.Errorf("missing key: action=%v, want reject", mk["action"])
	}
	// No usable rail -> reject.
	if nr := runHandle(t, h, `{"idempotency_key":"k5","amount_rupees":100}`); nr["action"] != "reject" {
		t.Errorf("no rail: action=%v, want reject", nr["action"])
	}
	// Trusted large via IFSC/account NEFT -> hold_hitl (>=50k), rail neft.
	neft := runHandle(t, h, `{"idempotency_key":"k6","beneficiary_ifsc":"HDFC0001","beneficiary_account":"123","amount_rupees":80000,"is_trusted_beneficiary":true,"urgency":"any"}`)
	if neft["action"] != "hold_hitl" {
		t.Errorf("trusted 80k: action=%v, want hold_hitl", neft["action"])
	}
	assertBadInputNoPanic(t, "payment", h)
}

// --- supply_chain_finance ---

func TestSCF_ConcentrationAndTReDS(t *testing.T) {
	h := SupplyChainFinanceSpec().Handle
	// One buyer dominates (>40%) and has aged e-invoiced receivables.
	input := `{"supplier_id":"s1","invoices":[
	  {"invoice_id":"i1","buyer_id":"BigCo","buyer_rating":"A","amount_rupees":800000,"days_outstanding":60,"gst_e_invoiced":true},
	  {"invoice_id":"i2","buyer_id":"SmallCo","buyer_rating":"BBB","amount_rupees":100000,"days_outstanding":20,"gst_e_invoiced":true}
	]}`
	m := runHandle(t, h, input)
	if m["concentration_ok"] != false {
		t.Errorf("expected concentration risk flagged, got %v", m["concentration_ok"])
	}
	if cand, _ := m["treds_candidates"].([]any); len(cand) != 1 {
		t.Errorf("expected 1 TReDS candidate (aged+einvoiced), got %d", len(cand))
	}
	if steps, _ := m["next_steps"].([]any); len(steps) == 0 {
		t.Error("expected next steps")
	}
	// Healthy chain (balanced, no aged candidates).
	healthy := runHandle(t, h, `{"supplier_id":"s2","invoices":[
	  {"invoice_id":"i3","buyer_id":"X","amount_rupees":100000,"days_outstanding":10},
	  {"invoice_id":"i4","buyer_id":"Y","amount_rupees":100000,"days_outstanding":10},
	  {"invoice_id":"i5","buyer_id":"Z","amount_rupees":100000,"days_outstanding":10}
	]}`)
	if healthy["concentration_ok"] != true {
		t.Errorf("balanced chain should be concentration_ok, got %v", healthy["concentration_ok"])
	}
	assertBadInputNoPanic(t, "scf", h)
}

// --- carbon_estimator ---

func TestCarbon_FootprintAndMoM(t *testing.T) {
	h := CarbonEstimatorSpec().Handle
	input := `{"transactions":[
	  {"amount_cents":-500000,"category":"fuel","date":"2024-01-05"},
	  {"amount_cents":-300000,"category":"utilities","date":"2024-01-10"},
	  {"amount_cents":-200000,"category":"food","date":"2024-01-15"},
	  {"amount_cents":-600000,"category":"fuel","date":"2024-02-05"},
	  {"amount_cents":100000,"category":"income","date":"2024-02-01"}
	]}`
	m := runHandle(t, h, input)
	if tot, _ := m["total_kg_co2e"].(float64); tot <= 0 {
		t.Errorf("expected positive footprint, got %v", tot)
	}
	byCat, _ := m["by_category"].([]any)
	if len(byCat) == 0 {
		t.Fatal("expected category breakdown")
	}
	// fuel should top the list.
	if byCat[0].(map[string]any)["category"] != "fuel" {
		t.Errorf("expected fuel top emitter, got %v", byCat[0])
	}
	if sug, _ := m["suggestions"].([]any); len(sug) == 0 {
		t.Error("expected reduction suggestions")
	}
	// Two months present -> MoM computed (non-nil field).
	if _, ok := m["month_over_month_change_pct"]; !ok {
		t.Error("expected MoM change field")
	}
	assertBadInputNoPanic(t, "carbon", h)
}

// --- invoice_processor ---

func TestInvoiceProcessor_MatchAndReject(t *testing.T) {
	h := InvoiceProcessorSpec().Handle
	// Clean 3-way match -> post.
	good := runHandle(t, h, `{"invoice":{"irn":"IRN1","supplier_gstin":"27AAAAA0000A1Z5","buyer_gstin":"29BBBBB1111B2Z6","total_rupees":11800,"po_reference":"PO1","items":[{"hsn":"1001","quantity":10,"unit_price_rupees":1000,"gst_pct":18,"line_total_rupees":11800}]},"reference":{"po_total_rupees":11800,"vendor_active":true,"grn_qty_by_hsn":{"1001":10}}}`)
	if good["action"] != "post" {
		t.Errorf("clean invoice: action=%v, want post (issues %v)", good["action"], good["issues"])
	}
	// Bad GSTIN + inactive vendor + no PO -> low confidence -> reject.
	bad := runHandle(t, h, `{"invoice":{"irn":"IRN2","supplier_gstin":"BAD","buyer_gstin":"ALSO_BAD","total_rupees":5000,"items":[]},"reference":{"vendor_active":false}}`)
	if bad["action"] != "reject" {
		t.Errorf("bad invoice: action=%v, want reject (conf %v)", bad["action"], bad["confidence_0_1"])
	}
	if iss, _ := bad["issues"].([]any); len(iss) == 0 {
		t.Error("expected issues on bad invoice")
	}
	assertBadInputNoPanic(t, "invoice_processor", h)
}

// --- emergency_fund ---

func TestEmergencyFund_CoverageTiers(t *testing.T) {
	h := EmergencyFundSpec().Handle
	txns := `"transactions":[
	  {"date":"2024-01-05","amount_cents":-3000000},
	  {"date":"2024-02-05","amount_cents":-3000000},
	  {"date":"2024-03-05","amount_cents":-3000000}
	]`
	// Stable, no dependents -> 3 months.
	stable := runHandle(t, h, `{`+txns+`,"liquid_reserves_rupees":10000,"income_profile":"stable"}`)
	if cov, _ := stable["coverage_months"].(float64); cov != 3 {
		t.Errorf("stable coverage = %v, want 3", cov)
	}
	// Variable income -> 6 months.
	variable := runHandle(t, h, `{`+txns+`,"liquid_reserves_rupees":10000,"income_profile":"variable"}`)
	if cov, _ := variable["coverage_months"].(float64); cov != 6 {
		t.Errorf("variable coverage = %v, want 6", cov)
	}
	// Dependents -> 9 months.
	dep := runHandle(t, h, `{`+txns+`,"liquid_reserves_rupees":10000,"has_dependents":true}`)
	if cov, _ := dep["coverage_months"].(float64); cov != 9 {
		t.Errorf("dependents coverage = %v, want 9", cov)
	}
	// Above target -> gap 0.
	above := runHandle(t, h, `{`+txns+`,"liquid_reserves_rupees":100000000,"income_profile":"stable"}`)
	if gap, _ := above["gap_rupees"].(float64); gap != 0 {
		t.Errorf("above-target gap = %v, want 0", gap)
	}
	assertBadInputNoPanic(t, "emergency_fund", h)
}

// --- claim_adjudicator ---

func TestClaimAdjudicator_Branches(t *testing.T) {
	h := ClaimAdjudicatorSpec().Handle

	// Waiting period -> deny.
	wait := runHandle(t, h, `{"claim":{"claim_id":"c1","days_since_issue":10,"incurred_rupees":50000},"policy":{"waiting_period_days":30}}`)
	if wait["action"] != "deny" {
		t.Errorf("waiting: action=%v, want deny", wait["action"])
	}
	// Exclusion -> deny.
	excl := runHandle(t, h, `{"claim":{"claim_id":"c2","days_since_issue":100,"diagnosis":"cosmetic surgery","incurred_rupees":50000},"policy":{"exclusions":["cosmetic"]}}`)
	if excl["action"] != "deny" {
		t.Errorf("exclusion: action=%v, want deny", excl["action"])
	}
	// Network-only peril off network -> deny.
	net := runHandle(t, h, `{"claim":{"claim_id":"c3","days_since_issue":100,"peril":"dental","hospital_in_network":false,"incurred_rupees":50000},"policy":{"network_only_perils":["dental"]}}`)
	if net["action"] != "deny" {
		t.Errorf("network peril: action=%v, want deny", net["action"])
	}
	// Partial with deductible + copay + sub-limit.
	part := runHandle(t, h, `{"claim":{"claim_id":"c4","days_since_issue":100,"peril":"surgery","incurred_rupees":100000},"policy":{"deductible_rupees":10000,"copay_pct":0.1,"sub_limits":{"surgery":50000},"sum_insured":500000}}`)
	if part["action"] != "approve_partial" {
		t.Errorf("partial: action=%v, want approve_partial", part["action"])
	}
	if p, _ := part["payout_rupees"].(float64); p <= 0 {
		t.Errorf("expected positive payout, got %v", p)
	}
	// Large claim -> hitl.
	hitl := runHandle(t, h, `{"claim":{"claim_id":"c5","days_since_issue":100,"incurred_rupees":300000},"policy":{"sum_insured":1000000}}`)
	if hitl["action"] != "hitl" {
		t.Errorf("large: action=%v, want hitl", hitl["action"])
	}
	assertBadInputNoPanic(t, "claim_adjudicator", h)
}

// --- working_capital ---

func TestWorkingCapital_ForecastAndRecs(t *testing.T) {
	h := WorkingCapitalSpec().Handle
	// Long CCC (>90) should surface the TReDS recommendation.
	m := runHandle(t, h, `{"monthly_revenue_rupees":1000000,"gross_margin_pct":30,"operating_costs_monthly_rupees":200000,"dso_days":90,"dio_days":60,"dpo_days":30,"opening_cash_rupees":500000,"horizon_months":6}`)
	if ccc, _ := m["cash_conversion_cycle_days"].(float64); ccc != 120 {
		t.Errorf("ccc = %v, want 120", ccc)
	}
	if rec, _ := m["recommendation"].(string); !strings.Contains(rec, "TReDS") {
		t.Errorf("expected TReDS rec for CCC>90, got %q", rec)
	}
	fc, _ := m["forecast"].([]any)
	if len(fc) != 6 {
		t.Errorf("forecast length = %d, want 6", len(fc))
	}
	// Cash-strapped scenario -> short runway rec.
	burn := runHandle(t, h, `{"monthly_revenue_rupees":100000,"gross_margin_pct":10,"operating_costs_monthly_rupees":500000,"dso_days":30,"dio_days":10,"dpo_days":10,"opening_cash_rupees":100000,"horizon_months":6}`)
	if rec, _ := burn["recommendation"].(string); rec == "" {
		t.Error("expected a recommendation")
	}
	assertBadInputNoPanic(t, "working_capital", h)
}

// --- goal_planner ---

func TestGoalPlanner_MonteCarlo(t *testing.T) {
	h := GoalPlannerSpec().Handle
	// Modest target with steady contributions -> high success; deterministic seed.
	input := `{"current_corpus_rupees":500000,"monthly_contribution_rupees":20000,"target_corpus_rupees":2000000,"horizon_months":60,"expected_annual_return":0.10,"annual_volatility":0.12,"paths":200}`
	a := runHandle(t, h, input)
	b := runHandle(t, h, input)
	if a["success_probability_0_1"] != b["success_probability_0_1"] {
		t.Error("goal planner not reproducible for a fixed seed")
	}
	if p, _ := a["success_probability_0_1"].(float64); p < 0 || p > 1 {
		t.Errorf("success prob out of [0,1]: %v", p)
	}
	if p10, _ := a["p10_corpus_rupees"].(float64); p10 <= 0 {
		t.Errorf("p10 corpus = %v, want positive", p10)
	}
	mustDisclaimer(t, a)
	assertBadInputNoPanic(t, "goal_planner", h)
}

// --- complaint_triage ---

func TestComplaintTriage_Categories(t *testing.T) {
	h := ComplaintTriageSpec().Handle
	// UPI + fraud -> digital banking, high severity, ombudsman eligible.
	upi := runHandle(t, h, `{"user_id":"u1","complaint_text":"My UPI payment was a fraud, money missing money","channel_hint":"app"}`)
	if upi["category"] != "digital_banking_upi_imps" {
		t.Errorf("category = %v, want digital_banking_upi_imps", upi["category"])
	}
	if upi["severity"] != "high" {
		t.Errorf("severity = %v, want high", upi["severity"])
	}
	inc, _ := upi["incident_draft"].(map[string]any)
	if inc["ombudsman_eligible"] != true {
		t.Errorf("expected ombudsman eligible, got %v", inc["ombudsman_eligible"])
	}
	// Vague -> other, needs review.
	vague := runHandle(t, h, `{"user_id":"u2","complaint_text":"I am unhappy"}`)
	if vague["category"] != "other" {
		t.Errorf("category = %v, want other", vague["category"])
	}
	if vague["needs_human_review"] != true {
		t.Errorf("vague should need review, got %v", vague["needs_human_review"])
	}
	// Loan keyword.
	loan := runHandle(t, h, `{"user_id":"u3","complaint_text":"penal interest on my home loan EMI"}`)
	if loan["category"] != "loans_and_advances" {
		t.Errorf("category = %v, want loans_and_advances", loan["category"])
	}
	assertBadInputNoPanic(t, "complaint_triage", h)
}

// --- mpc_research ---

func TestMpcResearch_HikeCutSteady(t *testing.T) {
	h := MpcResearchSpec().Handle
	// Repo hike 25bps, hawkish surprise vs consensus 0.
	hike := runHandle(t, h, `{"current":{"meeting_date":"2024-06-07","repo_rate_pct":6.75,"stance":"withdrawal_of_accommodation","cpi_projection_pct":5.0,"gdp_projection_pct":7.0,"consensus_repo_change_bps":0},"previous":{"repo_rate_pct":6.50,"stance":"neutral","cpi_projection_pct":4.5,"gdp_projection_pct":7.0}}`)
	if got, _ := hike["repo_change_bps"].(float64); got != 25 {
		t.Errorf("repo_change_bps = %v, want 25", got)
	}
	if hike["stance_shift"] != "tightening" {
		t.Errorf("stance_shift = %v, want tightening", hike["stance_shift"])
	}
	if hike["hawkishness_delta"] != "more_hawkish" {
		t.Errorf("hawkishness = %v, want more_hawkish", hike["hawkishness_delta"])
	}
	if hike["surprise"] != "hawkish_surprise" {
		t.Errorf("surprise = %v, want hawkish_surprise", hike["surprise"])
	}
	// Repo cut, easing.
	cut := runHandle(t, h, `{"current":{"meeting_date":"2024-08-08","repo_rate_pct":6.25,"stance":"accommodative","consensus_repo_change_bps":-25},"previous":{"repo_rate_pct":6.50,"stance":"neutral"}}`)
	if got, _ := cut["repo_change_bps"].(float64); got != -25 {
		t.Errorf("repo_change_bps = %v, want -25", got)
	}
	if cut["stance_shift"] != "easing" {
		t.Errorf("stance_shift = %v, want easing", cut["stance_shift"])
	}
	// Steady.
	steady := runHandle(t, h, `{"current":{"meeting_date":"2024-10-09","repo_rate_pct":6.50,"stance":"neutral","consensus_repo_change_bps":0},"previous":{"repo_rate_pct":6.50,"stance":"neutral"}}`)
	if got, _ := steady["repo_change_bps"].(float64); got != 0 {
		t.Errorf("steady repo_change_bps = %v, want 0", got)
	}
	assertBadInputNoPanic(t, "mpc_research", h)
}

// --- dividend_planner ---

func TestDividendPlanner_Reinvest(t *testing.T) {
	h := DividendPlannerSpec().Handle
	reinv := runHandle(t, h, `{"shares":1000,"current_price_rupees":100,"dividend_per_share_rupees":5,"dividend_growth_annual":0.08,"price_appreciation_annual":0.10,"horizon_years":5,"tds_and_slab_pct":10,"reinvest_dividends":true}`)
	sched, _ := reinv["schedule"].([]any)
	if len(sched) != 5 {
		t.Fatalf("schedule length = %d, want 5", len(sched))
	}
	if tv, _ := reinv["terminal_holding_value_rupees"].(float64); tv <= 0 {
		t.Errorf("terminal value = %v, want positive", tv)
	}
	if td, _ := reinv["total_dividend_received_net_rupees"].(float64); td <= 0 {
		t.Errorf("total dividend = %v, want positive", td)
	}
	// No reinvest path.
	noreinv := runHandle(t, h, `{"shares":500,"current_price_rupees":50,"dividend_per_share_rupees":2,"horizon_years":3,"tds_and_slab_pct":10,"reinvest_dividends":false}`)
	if sc, _ := noreinv["schedule"].([]any); len(sc) != 3 {
		t.Errorf("no-reinvest schedule = %d, want 3", len(sc))
	}
	mustDisclaimer(t, reinv)
	assertBadInputNoPanic(t, "dividend_planner", h)
}

// --- lcr_projector ---

func TestLcrProjector_ComplianceAndCaps(t *testing.T) {
	h := LcrProjectorSpec().Handle
	// Ample Level-1 HQLA -> compliant (>100%).
	comp := runHandle(t, h, `{"hqla":{"level1_cash_govsec_rupees":10000000},"outflows":{"less_stable_retail_deposits":5000000,"unsecured_wholesale_funding":1000000},"inflows":{"contractual_retail_inflows":1000000}}`)
	if comp["compliant_with_100pct"] != true {
		t.Errorf("expected compliant, got %v (lcr %v)", comp["compliant_with_100pct"], comp["lcr_pct"])
	}
	// Thin HQLA relative to big outflows -> non-compliant.
	non := runHandle(t, h, `{"hqla":{"level1_cash_govsec_rupees":100000},"outflows":{"unsecured_wholesale_funding":10000000,"non_operational_corporate_deposits":5000000}}`)
	if non["compliant_with_100pct"] != false {
		t.Errorf("expected non-compliant, got %v", non["compliant_with_100pct"])
	}
	// Level-2 heavy mix exercises the 2B and combined-2 caps.
	caps := runHandle(t, h, `{"hqla":{"level1_cash_govsec_rupees":100000,"level2a_corp_bonds_aa_plus_rupees":500000,"level2b_lower_rupees":800000},"outflows":{"less_stable_retail_deposits":1000000},"inflows":{"other_inflows":500000}}`)
	if hq, _ := caps["total_hqla_after_haircut_rupees"].(float64); hq <= 0 {
		t.Errorf("expected positive HQLA after caps, got %v", hq)
	}
	assertBadInputNoPanic(t, "lcr", h)
}
