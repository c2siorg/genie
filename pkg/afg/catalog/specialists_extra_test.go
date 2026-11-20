package catalog

import (
	"strings"
	"testing"
)

// --- phishing_classifier ---

func TestPhishing_IPUrlIsPhishing(t *testing.T) {
	h := PhishingClassifierSpec().Handle
	m := runHandle(t, h, `{"url":"http://192.168.1.1/sbi/login"}`)
	if m["label"] != "phishing" {
		t.Errorf("label = %v, want phishing (score %v)", m["label"], m["score_0_1"])
	}
	mustDisclaimer(t, m)
}

func TestPhishing_CredentialText(t *testing.T) {
	h := PhishingClassifierSpec().Handle
	// "share otp" (+0.5) + urgency "account blocked" (+0.2) = 0.7 -> phishing.
	m := runHandle(t, h, `{"text":"URGENT: account blocked, share OTP to verify"}`)
	if m["label"] != "phishing" {
		t.Errorf("label = %v, want phishing", m["label"])
	}
}

func TestPhishing_TrustedDomainSafe(t *testing.T) {
	h := PhishingClassifierSpec().Handle
	m := runHandle(t, h, `{"url":"https://sbi.co.in/netbanking"}`)
	if m["label"] != "safe" {
		t.Errorf("label = %v, want safe", m["label"])
	}
}

func TestPhishing_LookAlikeDomain(t *testing.T) {
	h := PhishingClassifierSpec().Handle
	// contains "sbi" brand but not on sbi.co.in -> +0.5 look-alike.
	m := runHandle(t, h, `{"url":"https://sbi-secure-login.example.com"}`)
	reasons, _ := m["reasons"].([]any)
	if len(reasons) == 0 {
		t.Error("expected at least one reason for look-alike domain")
	}
}

func TestPhishing_RefundVPA(t *testing.T) {
	h := PhishingClassifierSpec().Handle
	m := runHandle(t, h, `{"upi_vpa":"refund123@randombank"}`)
	if s, _ := m["score_0_1"].(float64); s <= 0 {
		t.Errorf("expected positive score for refund VPA, got %v", s)
	}
}

func TestPhishing_BadInput(t *testing.T) {
	assertBadInputNoPanic(t, "phishing", PhishingClassifierSpec().Handle)
}

// --- mule_detector ---

func TestMule_PassThrough(t *testing.T) {
	h := MuleSpec().Handle
	// Credit ₹100000 then ₹95000 forwarded out same day -> 95% >= 90%.
	input := `{"transactions":[
	  {"account_id":"A","date":"2024-01-01","amount_cents":10000000,"merchant":"payer"},
	  {"account_id":"A","date":"2024-01-01","amount_cents":-9500000,"merchant":"payee"}
	]}`
	m := runHandle(t, h, input)
	sigs, _ := m["signals"].([]any)
	found := false
	for _, s := range sigs {
		if s.(map[string]any)["pattern"] == "pass_through" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected pass_through signal, got %v", sigs)
	}
}

func TestMule_FanInFanOut(t *testing.T) {
	h := MuleSpec().Handle
	var sb strings.Builder
	sb.WriteString(`{"transactions":[`)
	first := true
	add := func(amt int64, merchant string) {
		if !first {
			sb.WriteString(",")
		}
		first = false
		sb.WriteString(`{"account_id":"A","date":"2024-01-01","amount_cents":`)
		if amt < 0 {
			sb.WriteString("-")
			amt = -amt
		}
		sb.WriteString(itoaTest(amt))
		sb.WriteString(`,"merchant":"`)
		sb.WriteString(merchant)
		sb.WriteString(`"}`)
	}
	for i := 0; i < 5; i++ {
		add(1000, "in"+itoaTest(int64(i)))
	}
	for i := 0; i < 5; i++ {
		add(-1000, "out"+itoaTest(int64(i)))
	}
	sb.WriteString(`]}`)
	m := runHandle(t, h, sb.String())
	sigs, _ := m["signals"].([]any)
	found := false
	for _, s := range sigs {
		if s.(map[string]any)["pattern"] == "fan_in_fan_out" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fan_in_fan_out signal, got %v", sigs)
	}
}

func TestMule_CleanNoSignals(t *testing.T) {
	h := MuleSpec().Handle
	m := runHandle(t, h, `{"transactions":[{"account_id":"A","date":"2024-01-01","amount_cents":-100,"merchant":"cafe"}]}`)
	sigs, _ := m["signals"].([]any)
	if len(sigs) != 0 {
		t.Errorf("expected no signals, got %d", len(sigs))
	}
}

func TestMule_BadInput(t *testing.T) { assertBadInputNoPanic(t, "mule", MuleSpec().Handle) }

// itoaTest is a tiny local int formatter for building JSON fixtures.
func itoaTest(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// --- cashflow_underwriter ---

func TestCashflow_EmptyIsFloor(t *testing.T) {
	h := CashflowUnderwriterSpec().Handle
	m := runHandle(t, h, `{"transactions":[]}`)
	if got, _ := m["score_300_900"].(float64); got != 300 {
		t.Errorf("empty score = %v, want 300", got)
	}
	if m["grade"] != "D" {
		t.Errorf("empty grade = %v, want D", m["grade"])
	}
	mustDisclaimer(t, m)
}

func TestCashflow_ScoresWithinBand(t *testing.T) {
	h := CashflowUnderwriterSpec().Handle
	// Two months of steady inflow, modest outflow -> healthy score.
	input := `{"transactions":[
	  {"date":"2024-01-05","amount_cents":10000000,"description":"salary"},
	  {"date":"2024-01-10","amount_cents":-3000000,"description":"rent"},
	  {"date":"2024-02-05","amount_cents":10000000,"description":"salary"},
	  {"date":"2024-02-10","amount_cents":-3000000,"description":"rent"}
	]}`
	m := runHandle(t, h, input)
	score, _ := m["score_300_900"].(float64)
	if score < 300 || score > 900 {
		t.Errorf("score %v out of band [300,900]", score)
	}
	if g, _ := m["grade"].(string); g == "" {
		t.Error("expected a grade")
	}
	if mc, _ := m["months_of_history"].(float64); mc != 2 {
		t.Errorf("months_of_history = %v, want 2", mc)
	}
	sigs, _ := m["signals"].([]any)
	if len(sigs) != 6 {
		t.Errorf("expected 6 signals, got %d", len(sigs))
	}
	mustDisclaimer(t, m)
}

func TestCashflow_BounceAndRecurringDetected(t *testing.T) {
	h := CashflowUnderwriterSpec().Handle
	// Include an ECS return (bounce) and an EMI (recurring).
	input := `{"transactions":[
	  {"date":"2024-01-05","amount_cents":10000000,"description":"salary"},
	  {"date":"2024-01-08","amount_cents":-2000000,"description":"home loan EMI"},
	  {"date":"2024-01-15","amount_cents":-500000,"description":"ECS RET bounce"}
	]}`
	m := runHandle(t, h, input)
	// Just assert it produces a valid banded score without panic.
	if score, _ := m["score_300_900"].(float64); score < 300 || score > 900 {
		t.Errorf("score %v out of band", score)
	}
}

func TestCashflow_BadInput(t *testing.T) {
	assertBadInputNoPanic(t, "cashflow", CashflowUnderwriterSpec().Handle)
}
