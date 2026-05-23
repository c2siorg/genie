package identity

import (
	"strings"
	"testing"
)

func TestVC_IssueAndVerify(t *testing.T) {
	issuer, err := NewDIDKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issuer.ID, "did:key:") {
		t.Fatalf("unexpected did: %q", issuer.ID)
	}
	vc := &VerifiableCredential{
		Type: []string{"VerifiableCredential", "GenieAgentManifest"},
		CredentialSubject: map[string]any{
			"agentId": "ingestor",
			"riskClass": "low",
		},
	}
	if _, err := IssueVC(issuer, vc); err != nil {
		t.Fatal(err)
	}
	if vc.Proof == nil || vc.Proof.ProofValue == "" {
		t.Fatal("expected proof to be attached")
	}
	if err := VerifyVC(vc, issuer.Public); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Tamper.
	vc.CredentialSubject["agentId"] = "tampered"
	if err := VerifyVC(vc, issuer.Public); err == nil {
		t.Fatal("expected verify to detect tamper")
	}
}
