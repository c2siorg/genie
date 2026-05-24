package synthetic_identity

import "testing"

func TestCleanApplicationLow(t *testing.T) {
	v := New().Classify(Application{
		StatedAge: 35, BureauTenureMonths: 120,
		NameOnPAN: "Asha Kumar Singh", NameOnAadhaar: "Asha Singh",
		PANNumber: "ABCPS1234F", EmailDomain: "gmail.com",
		PhoneCreatedDays: 1500, AddressVelocity: 0,
	})
	if v.Label != "low" {
		t.Errorf("clean profile should be low; got %+v", v)
	}
}

func TestAddressFarmHighRisk(t *testing.T) {
	v := New().Classify(Application{
		StatedAge: 28, BureauTenureMonths: 4, AddressVelocity: 5,
		NameOnPAN: "X Y", NameOnAadhaar: "X Y", PANNumber: "ABCPY1234F",
	})
	if v.Label != "high" {
		t.Errorf("thin file + address farm should be high; got %+v", v)
	}
}

func TestPANSurnameMismatch(t *testing.T) {
	v := New().Classify(Application{
		StatedAge: 30, BureauTenureMonths: 24,
		NameOnPAN: "Ravi Sharma", NameOnAadhaar: "Ravi Sharma",
		PANNumber: "ABCPK1234F", // 5th char K but surname Sharma → S expected
	})
	hit := false
	for _, r := range v.Reasons {
		if contains(r, "PAN 5th character") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected PAN 5th-char mismatch reason; got %+v", v.Reasons)
	}
}

func TestNameMismatchAcrossDocuments(t *testing.T) {
	v := New().Classify(Application{
		StatedAge: 30, BureauTenureMonths: 24,
		NameOnPAN: "Alpha Bravo", NameOnAadhaar: "Charlie Delta",
		PANNumber: "ABCPB1234F",
	})
	hit := false
	for _, r := range v.Reasons {
		if contains(r, "PAN and Aadhaar") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected name-mismatch reason; got %+v", v.Reasons)
	}
}

func TestThrowawayEmail(t *testing.T) {
	v := New().Classify(Application{
		StatedAge: 30, BureauTenureMonths: 24,
		NameOnPAN: "A B", NameOnAadhaar: "A B", PANNumber: "ABCPB1234F",
		EmailDomain: "mailinator.com",
	})
	hit := false
	for _, r := range v.Reasons {
		if contains(r, "temp") {
			hit = true
		}
	}
	if !hit {
		t.Errorf("expected throwaway-domain reason")
	}
}

func TestDisclaimerPresent(t *testing.T) {
	if New().Classify(Application{}).Disclaimer == "" {
		t.Errorf("disclaimer missing")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
