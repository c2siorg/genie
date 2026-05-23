// Package identity implements minimal W3C DID + Verifiable Credentials so
// Genie agents can prove who they are and what they did.
//
// Spec subsets covered:
//   - did:key (Ed25519) for the agent identifier.
//   - W3C VC 1.1 "verifiableCredential" with a JWS-like signature
//     (Ed25519 over the canonical-JSON payload).
//
// Why minimal: a full ssi-sdk dependency is large; the demo just needs
// signed identity + claim primitives. Swap in github.com/TBD54566975/ssi-sdk
// behind these types for production interop.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// DID is a decentralised identifier in the did:key method.
type DID struct {
	ID      string             // did:key:<multibase-encoded-pubkey>
	Private ed25519.PrivateKey // present only on the issuer side
	Public  ed25519.PublicKey
}

// NewDIDKey generates a fresh did:key.
func NewDIDKey() (*DID, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	// did:key uses multibase base58btc + a 0xed01 multicodec prefix for
	// Ed25519. To keep this dependency-free we substitute base64-url for the
	// suffix; verifiers must use the matching encoding. Document this in
	// the agent profile.
	suffix := base64.RawURLEncoding.EncodeToString(pub)
	return &DID{
		ID:      "did:key:z" + suffix,
		Private: priv,
		Public:  pub,
	}, nil
}

// VerifiableCredential is a W3C VC 1.1 envelope.
type VerifiableCredential struct {
	Context           []string       `json:"@context"`
	ID                string         `json:"id,omitempty"`
	Type              []string       `json:"type"`
	Issuer            string         `json:"issuer"`            // DID
	IssuanceDate      time.Time      `json:"issuanceDate"`
	CredentialSubject map[string]any `json:"credentialSubject"` // free-form claims
	Proof             *Proof         `json:"proof,omitempty"`
}

// Proof is the JWS-shaped signature block.
type Proof struct {
	Type               string    `json:"type"`
	Created            time.Time `json:"created"`
	VerificationMethod string    `json:"verificationMethod"` // DID#key-1
	ProofPurpose       string    `json:"proofPurpose"`       // "assertionMethod"
	ProofValue         string    `json:"proofValue"`         // base64 Ed25519 signature
}

// IssueVC signs a credential with the issuer's DID. Mutates the credential
// in place (adds the Proof) and returns it for convenience.
func IssueVC(issuer *DID, vc *VerifiableCredential) (*VerifiableCredential, error) {
	if issuer == nil || issuer.Private == nil {
		return nil, errors.New("identity: issuer without private key")
	}
	if vc == nil {
		return nil, errors.New("identity: nil credential")
	}
	if len(vc.Context) == 0 {
		vc.Context = []string{"https://www.w3.org/2018/credentials/v1"}
	}
	if len(vc.Type) == 0 {
		vc.Type = []string{"VerifiableCredential"}
	}
	if vc.IssuanceDate.IsZero() {
		vc.IssuanceDate = time.Now().UTC()
	}
	vc.Issuer = issuer.ID
	// Hash a stable JSON form (proof stripped) for signing.
	cp := *vc
	cp.Proof = nil
	body, err := json.Marshal(cp)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(issuer.Private, body)
	vc.Proof = &Proof{
		Type:               "Ed25519Signature2020",
		Created:            time.Now().UTC(),
		VerificationMethod: issuer.ID + "#key-1",
		ProofPurpose:       "assertionMethod",
		ProofValue:         base64.StdEncoding.EncodeToString(sig),
	}
	return vc, nil
}

// VerifyVC checks the signature against the issuer's public key.
func VerifyVC(vc *VerifiableCredential, pub ed25519.PublicKey) error {
	if vc == nil || vc.Proof == nil {
		return errors.New("identity: missing proof")
	}
	cp := *vc
	cp.Proof = nil
	body, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(vc.Proof.ProofValue)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, body, sig) {
		return errors.New("identity: signature mismatch")
	}
	return nil
}
