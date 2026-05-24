package phishing_classifier

import "testing"

func TestSafeBankURL(t *testing.T) {
	v := New().Classify(Request{URL: "https://www.hdfcbank.com/personal/save"})
	if v.Label != "safe" {
		t.Errorf("trusted bank URL should be safe; got %+v", v)
	}
}

func TestBrandImpersonationURL(t *testing.T) {
	v := New().Classify(Request{URL: "https://hdfcbank-verify.com/login"})
	if v.Label == "safe" {
		t.Errorf("brand impersonation should not be safe; got %+v", v)
	}
}

func TestIPLiteralURL(t *testing.T) {
	v := New().Classify(Request{URL: "http://192.168.1.5/upi/refund"})
	if v.Label != "phishing" {
		t.Errorf("IP-literal URL should be phishing; got %+v", v)
	}
}

func TestOTPShareSMS(t *testing.T) {
	v := New().Classify(Request{Text: "Urgent! Share OTP to verify your KYC immediately."})
	if v.Label != "phishing" {
		t.Errorf("OTP-share + urgency + KYC should be phishing; got %+v", v)
	}
}

func TestLotterySpam(t *testing.T) {
	v := New().Classify(Request{Text: "You have won the lottery! Claim prize now."})
	if v.Label == "safe" {
		t.Errorf("lottery spam should not be safe; got %+v", v)
	}
}

func TestVPARefundScam(t *testing.T) {
	v := New().Classify(Request{VPA: "refund.support@random.com"})
	if v.Label == "safe" {
		t.Errorf("refund VPA should be flagged; got %+v", v)
	}
}

func TestNormalUPIVPA(t *testing.T) {
	v := New().Classify(Request{VPA: "alice@okhdfcbank"})
	if v.Label != "safe" {
		t.Errorf("normal VPA should be safe; got %+v", v)
	}
}

func TestDisclaimerPresent(t *testing.T) {
	if New().Classify(Request{URL: "https://example.com"}).Disclaimer == "" {
		t.Errorf("disclaimer missing")
	}
}
