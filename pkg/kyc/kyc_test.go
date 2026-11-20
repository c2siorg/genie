package kyc

import (
	"fmt"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// validAadhaar builds a Verhoeff-valid 12-digit Aadhaar from an 11-digit
// payload, so tests never hard-code a "known" number.
func validAadhaar(t *testing.T, first11 string) string {
	t.Helper()
	cd, err := AadhaarCheckDigit(first11)
	if err != nil {
		t.Fatalf("check digit: %v", err)
	}
	return fmt.Sprintf("%s%d", first11, cd)
}

func TestValidateAadhaar_RoundTrip(t *testing.T) {
	full := validAadhaar(t, "23412341234")
	if !ValidateAadhaar(full) {
		t.Fatalf("generated Aadhaar %q should be Verhoeff-valid", full)
	}
	// Spaces/hyphens tolerated.
	spaced := full[:4] + " " + full[4:8] + " " + full[8:]
	if !ValidateAadhaar(spaced) {
		t.Fatalf("spaced Aadhaar %q should validate", spaced)
	}
	// Flip the last digit -> checksum must fail.
	bad := full[:11] + string('0'+byte((int(full[11]-'0')+1)%10))
	if ValidateAadhaar(bad) {
		t.Fatalf("single-digit corruption %q must fail Verhoeff", bad)
	}
	// Transpose the last two payload digits -> Verhoeff catches most transpositions.
	if full[9] != full[10] {
		trans := full[:9] + string(full[10]) + string(full[9]) + string(full[11])
		if ValidateAadhaar(trans) {
			t.Fatalf("adjacent transposition %q should fail", trans)
		}
	}
}

func TestValidateAadhaar_Rejects(t *testing.T) {
	cases := []string{
		"",
		"1234",              // too short
		"2341234123",        // 10 digits
		"023412341234",      // starts with 0
		"123412341234",      // starts with 1
		"23412341234a",      // non-digit
		"234123412340",      // wrong check digit (very likely)
	}
	for _, c := range cases {
		if ValidateAadhaar(c) {
			t.Errorf("ValidateAadhaar(%q) = true, want false", c)
		}
	}
}

func TestMaskAadhaar(t *testing.T) {
	if got := MaskAadhaar("234123412346"); got != "XXXX XXXX 2346" {
		t.Errorf("MaskAadhaar = %q, want %q", got, "XXXX XXXX 2346")
	}
	if got := MaskAadhaar("2341 2341 2346"); got != "XXXX XXXX 2346" {
		t.Errorf("MaskAadhaar(spaced) = %q", got)
	}
	if got := MaskAadhaar("12"); got != "XXXX XXXX XXXX" {
		t.Errorf("MaskAadhaar(short) = %q", got)
	}
	// The mask must never contain the leading eight digits.
	if MaskAadhaar("234123412346") == "234123412346" {
		t.Fatal("mask leaked the full number")
	}
}

func TestValidatePAN(t *testing.T) {
	ok := []string{"ABCDE1234F", "abcde1234f", " ABCDE1234F "}
	for _, s := range ok {
		if !ValidatePAN(s) {
			t.Errorf("ValidatePAN(%q) = false, want true", s)
		}
	}
	bad := []string{"", "ABCD1234F", "ABCDE12345", "1BCDE1234F", "ABCDE1234"}
	for _, s := range bad {
		if ValidatePAN(s) {
			t.Errorf("ValidatePAN(%q) = true, want false", s)
		}
	}
}

func TestParseOfflineKYCLast4(t *testing.T) {
	good := []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<OfflineKyc referenceId="2346202401011200000000">` +
		`<UidData><Poi name="Test User" dob="01-01-1990" gender="M"/></UidData>` +
		`</OfflineKyc>`)
	last4, err := ParseOfflineKYCLast4(good)
	if err != nil {
		t.Fatalf("valid offline KYC: unexpected error %v", err)
	}
	if last4 != "2346" {
		t.Fatalf("last4 = %q, want 2346", last4)
	}

	// The parser must extract ONLY the last-4 — never a full number
	// (offline KYC XML has none), and nothing else this iteration.
	if len(last4) != 4 {
		t.Fatalf("extracted more than the last-4: %q", last4)
	}

	bad := map[string][]byte{
		"not xml":        []byte("just some bytes {not xml"),
		"wrong root":     []byte(`<Something referenceId="2346x"/>`),
		"no referenceId": []byte(`<OfflineKyc><UidData/></OfflineKyc>`),
	}
	for name, raw := range bad {
		if _, err := ParseOfflineKYCLast4(raw); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

func TestDefaultClassificationAndDocType(t *testing.T) {
	if DefaultClassification(DocAadhaarOfflineKYC) != protocol.ClassSecret {
		t.Error("Aadhaar must default to secret")
	}
	if DefaultClassification(DocPassport) != protocol.ClassSecret {
		t.Error("passport must default to secret")
	}
	if DefaultClassification(DocPAN) != protocol.ClassPII {
		t.Error("PAN must default to pii")
	}
	if !ValidDocType(DocAadhaarOfflineKYC) || ValidDocType(DocType("nonsense")) {
		t.Error("ValidDocType wrong")
	}
}
