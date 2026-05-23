package aibom

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
)

// Sigstore-compatible signing surface — Genie ships a plain Ed25519 signer
// instead of a full cosign integration so the binary stays self-contained.
// The signature shape is intentionally compatible with cosign's "raw
// signature" mode (base64 sha256 of payload).
//
// Production: replace the in-memory keys with a KMS-backed Signer that
// implements this interface (which already exists at sigstore/cosign).

// Signer signs an arbitrary blob.
type Signer interface {
	Sign(data []byte) (string, error) // base64 signature
	PublicKey() ed25519.PublicKey
}

// Ed25519Signer is the demo Signer.
type Ed25519Signer struct {
	private ed25519.PrivateKey
}

// NewEd25519Signer generates a fresh keypair. Persist the private key
// before restarts; the in-memory variant is for local dev.
func NewEd25519Signer() (*Ed25519Signer, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Ed25519Signer{private: priv}, nil
}

// Sign returns the base64-encoded Ed25519 signature of sha256(data).
func (s *Ed25519Signer) Sign(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	sig := ed25519.Sign(s.private, sum[:])
	return base64.StdEncoding.EncodeToString(sig), nil
}

// PublicKey returns the public verifier.
func (s *Ed25519Signer) PublicKey() ed25519.PublicKey {
	return s.private.Public().(ed25519.PublicKey)
}

// SignedDocument bundles the AIBOM with a detached signature.
type SignedDocument struct {
	Document  Document `json:"document"`
	Signature string   `json:"signature"`     // base64
	PublicKey string   `json:"public_key"`    // base64
	Algorithm string   `json:"algorithm"`     // "ed25519-sha256"
}

// Sign returns a SignedDocument over the canonical JSON of d.
func Sign(d Document, signer Signer) (SignedDocument, error) {
	body, err := json.Marshal(d)
	if err != nil {
		return SignedDocument{}, err
	}
	sig, err := signer.Sign(body)
	if err != nil {
		return SignedDocument{}, err
	}
	return SignedDocument{
		Document:  d,
		Signature: sig,
		PublicKey: base64.StdEncoding.EncodeToString(signer.PublicKey()),
		Algorithm: "ed25519-sha256",
	}, nil
}

// Verify checks that the SignedDocument's signature matches its document.
func Verify(sd SignedDocument) error {
	pubBytes, err := base64.StdEncoding.DecodeString(sd.PublicKey)
	if err != nil {
		return err
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return errors.New("aibom: invalid public key size")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sd.Signature)
	if err != nil {
		return err
	}
	body, err := json.Marshal(sd.Document)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if !ed25519.Verify(ed25519.PublicKey(pubBytes), sum[:], sigBytes) {
		return errors.New("aibom: signature verification failed")
	}
	return nil
}
