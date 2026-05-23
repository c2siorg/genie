package webauthn

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
)

func TestWebAuthn_RegisterAndAssert(t *testing.T) {
	svc := New("genie.example", "Genie")

	// Register.
	regID, challenge, err := svc.BeginRegistration("u-1")
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	if err := svc.FinishRegistration(regID, "cred-1", pub, challenge); err != nil {
		t.Fatal(err)
	}

	// Assert.
	asID, challenge, err := svc.BeginAssertion("u-1")
	if err != nil {
		t.Fatal(err)
	}
	authenticatorData := []byte("authn-data")
	clientDataJSON := []byte(`{"type":"webauthn.get","challenge":"…"}`)
	hash := sha256.Sum256(clientDataJSON)
	signed := append(append([]byte{}, authenticatorData...), hash[:]...)
	sig := ed25519.Sign(priv, signed)

	gotUser, err := svc.FinishAssertion(asID, "cred-1", authenticatorData, clientDataJSON, sig)
	if err != nil {
		t.Fatal(err)
	}
	if gotUser != "u-1" {
		t.Fatalf("expected u-1, got %q", gotUser)
	}
	// Acknowledge the registration challenge is consumed (vars used).
	_ = challenge
}

func TestWebAuthn_RejectsBadSignature(t *testing.T) {
	svc := New("genie.example", "Genie")
	regID, challenge, _ := svc.BeginRegistration("u-1")
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_ = svc.FinishRegistration(regID, "cred-1", pub, challenge)

	asID, _, _ := svc.BeginAssertion("u-1")
	if _, err := svc.FinishAssertion(asID, "cred-1", []byte("x"), []byte("y"), []byte("bad-sig")); err == nil {
		t.Fatal("expected signature mismatch")
	}
}
