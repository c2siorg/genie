package catalog

import (
	"testing"
)

// --- cyber_guardian: medium label, 3-failed tier, churn, near/instant travel ---

func TestCyber_MediumTierAndSkips(t *testing.T) {
	h := CyberGuardianSpec().Handle

	// 3 failed (+0.15) + 3 distinct enrolled fingerprints (+0.15) = 0.30 -> medium.
	// The successful events share one location so the travel check hits distKM<50.
	m := runHandle(t, h, `{"user_id":"u","known_device_fps":["fp1","fp2","fp3"],"events":[
	  {"successful_auth":false},{"successful_auth":false},{"successful_auth":false},
	  {"successful_auth":true,"lat":28.6,"lng":77.2,"unix_millis":1000,"device_fingerprint":"fp1"},
	  {"successful_auth":true,"lat":28.6,"lng":77.2,"unix_millis":2000,"device_fingerprint":"fp2"},
	  {"successful_auth":true,"lat":28.6,"lng":77.2,"unix_millis":3000,"device_fingerprint":"fp3"}
	]}`)
	if m["label"] != "medium" {
		t.Errorf("label = %v, want medium (score %v)", m["label"], m["risk_score_0_1"])
	}

	// Two far-apart successful logins at the SAME instant -> hours<=0 -> no
	// impossible-travel flag (the divide-by-zero guard).
	inst := runHandle(t, h, `{"user_id":"u","known_device_fps":["fp"],"events":[
	  {"successful_auth":true,"lat":28.6,"lng":77.2,"unix_millis":1000,"device_fingerprint":"fp"},
	  {"successful_auth":true,"lat":19.0,"lng":72.8,"unix_millis":1000,"device_fingerprint":"fp"}
	]}`)
	flags, _ := inst["flags"].([]any)
	for _, f := range flags {
		if s, _ := f.(string); contains(s, "Impossible travel") {
			t.Error("same-instant logins must not flag impossible travel")
		}
	}
	assertBadInputNoPanic(t, "cyber", h)
}

// --- subscription_detector: parse/skip branches, std filter, inconsistency, sort ---

func TestSubscription_SkipsAndFilters(t *testing.T) {
	h := SubscriptionDetectorSpec().Handle

	// Credit (amount>=0) and merchant-less debit are both skipped upstream.
	skips := runHandle(t, h, `{"transactions":[
	  {"merchant":"","amount_cents":-100,"date":"2024-01-01"},
	  {"merchant":"X","amount_cents":100,"date":"2024-01-01"}
	]}`)
	if subs, _ := skips["subscriptions"].([]any); len(subs) != 0 {
		t.Errorf("expected no subscriptions from skipped txns, got %d", len(subs))
	}

	// Day-of-month std too high (1st/15th/31st) -> merchant filtered out.
	highStd := runHandle(t, h, `{"transactions":[
	  {"merchant":"Erratic","amount_cents":-50000,"date":"2024-01-01"},
	  {"merchant":"Erratic","amount_cents":-50000,"date":"2024-02-15"},
	  {"merchant":"Erratic","amount_cents":-50000,"date":"2024-03-31"}
	]}`)
	if subs, _ := highStd["subscriptions"].([]any); len(subs) != 0 {
		t.Errorf("erratic day cadence should be filtered, got %d", len(subs))
	}

	// All three dates malformed (short, non-digit day, out-of-range day) -> no
	// parseable amounts -> merchant skipped.
	badDates := runHandle(t, h, `{"transactions":[
	  {"merchant":"BadDates","amount_cents":-50000,"date":"2024-1-05"},
	  {"merchant":"BadDates","amount_cents":-50000,"date":"2024-01-0x"},
	  {"merchant":"BadDates","amount_cents":-50000,"date":"2024-01-99"}
	]}`)
	if subs, _ := badDates["subscriptions"].([]any); len(subs) != 0 {
		t.Errorf("all-malformed dates should yield no subscription, got %d", len(subs))
	}

	// Exactly one parseable date among three charges -> earlier[] empty ->
	// median falls back to all amounts; still a valid single-amount subscription.
	oneGood := runHandle(t, h, `{"transactions":[
	  {"merchant":"OneGood","amount_cents":-30000,"date":"2024-01-05"},
	  {"merchant":"OneGood","amount_cents":-30000,"date":"bad"},
	  {"merchant":"OneGood","amount_cents":-30000,"date":"2024-99-99"}
	]}`)
	if subs, _ := oneGood["subscriptions"].([]any); len(subs) != 1 {
		t.Errorf("expected 1 subscription from the single parseable charge, got %d", len(subs))
	}

	// Inconsistent amounts across >1 earlier charges -> merchant filtered out.
	inconsistent := runHandle(t, h, `{"transactions":[
	  {"merchant":"Wobbly","amount_cents":-10000,"date":"2024-01-05"},
	  {"merchant":"Wobbly","amount_cents":-20000,"date":"2024-02-05"},
	  {"merchant":"Wobbly","amount_cents":-30000,"date":"2024-03-05"},
	  {"merchant":"Wobbly","amount_cents":-15000,"date":"2024-04-05"}
	]}`)
	if subs, _ := inconsistent["subscriptions"].([]any); len(subs) != 0 {
		t.Errorf("wobbly amounts should be filtered, got %d", len(subs))
	}
}

func TestSubscription_MultipleSorted(t *testing.T) {
	h := SubscriptionDetectorSpec().Handle
	// Two clean monthly subscriptions of different size -> both detected, sorted
	// by annualised spend (bigger first).
	m := runHandle(t, h, `{"transactions":[
	  {"merchant":"Cheap","amount_cents":-10000,"date":"2024-01-05"},
	  {"merchant":"Cheap","amount_cents":-10000,"date":"2024-02-05"},
	  {"merchant":"Cheap","amount_cents":-10000,"date":"2024-03-05"},
	  {"merchant":"Pricey","amount_cents":-99900,"date":"2024-01-06"},
	  {"merchant":"Pricey","amount_cents":-99900,"date":"2024-02-06"},
	  {"merchant":"Pricey","amount_cents":-99900,"date":"2024-03-06"}
	]}`)
	subs, _ := m["subscriptions"].([]any)
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subs))
	}
	if subs[0].(map[string]any)["merchant"] != "Pricey" {
		t.Errorf("expected Pricey first (higher annualised), got %v", subs[0])
	}
}
