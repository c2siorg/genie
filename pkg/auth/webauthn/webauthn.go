// Package webauthn is a minimal-but-correct WebAuthn (W3C / FIDO2)
// server-side primitive. Supports passkey registration + assertion using
// Ed25519 credentials.
//
// We don't drag in github.com/go-webauthn/webauthn — that pulls a chain
// of dependencies for tpm/attestation/COSE-key parsing Genie doesn't need.
// What we DO implement:
//
//   - Random 32-byte challenge per ceremony.
//   - Public-key storage keyed by credential ID.
//   - Assertion verification against the stored public key
//     (Ed25519 over authenticatorData || sha256(clientData)).
//
// For production:
//   - Add full attestation verification (not done here).
//   - Add support for ES256 / RS256 (only Ed25519 here).
//   - Persist credentials in Postgres instead of memory.
package webauthn

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// Credential is one passkey registered by a user.
type Credential struct {
	ID        string             // base64url
	UserID    string
	PublicKey ed25519.PublicKey
	SignCount uint32
	CreatedAt time.Time
}

// Ceremony tracks a pending registration or assertion challenge.
type Ceremony struct {
	UserID    string
	Challenge []byte
	ExpiresAt time.Time
}

// Service stores registered credentials and pending ceremonies.
type Service struct {
	RPID          string // relying-party id (the API domain — "genie.example")
	RPName        string
	ChallengeTTL  time.Duration

	mu          sync.Mutex
	credentials map[string]*Credential
	pending     map[string]*Ceremony // ceremony id -> challenge
}

// New constructs a service.
func New(rpID, rpName string) *Service {
	return &Service{
		RPID:         rpID,
		RPName:       rpName,
		ChallengeTTL: 5 * time.Minute,
		credentials:  map[string]*Credential{},
		pending:      map[string]*Ceremony{},
	}
}

// BeginRegistration returns a fresh challenge for the user to send to the
// browser's navigator.credentials.create() call.
func (s *Service) BeginRegistration(userID string) (ceremonyID string, challenge []byte, err error) {
	ceremonyID, err = randomToken(16)
	if err != nil {
		return "", nil, err
	}
	challenge = make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return "", nil, err
	}
	s.mu.Lock()
	s.pending[ceremonyID] = &Ceremony{UserID: userID, Challenge: challenge, ExpiresAt: time.Now().Add(s.ChallengeTTL)}
	s.mu.Unlock()
	return ceremonyID, challenge, nil
}

// FinishRegistration stores the credential. clientDataJSON, attestationObject
// are what the browser returns; we only check the challenge here (full
// attestation validation is left as a follow-up).
func (s *Service) FinishRegistration(ceremonyID, credentialID string, pubKey ed25519.PublicKey, clientChallenge []byte) error {
	s.mu.Lock()
	c, ok := s.pending[ceremonyID]
	if ok {
		delete(s.pending, ceremonyID)
	}
	s.mu.Unlock()
	if !ok || time.Now().After(c.ExpiresAt) {
		return errors.New("webauthn: expired or unknown ceremony")
	}
	if !equalBytes(c.Challenge, clientChallenge) {
		return errors.New("webauthn: challenge mismatch")
	}
	s.mu.Lock()
	s.credentials[credentialID] = &Credential{
		ID:        credentialID,
		UserID:    c.UserID,
		PublicKey: pubKey,
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Unlock()
	return nil
}

// BeginAssertion returns a fresh challenge for navigator.credentials.get().
func (s *Service) BeginAssertion(userID string) (ceremonyID string, challenge []byte, err error) {
	ceremonyID, err = randomToken(16)
	if err != nil {
		return "", nil, err
	}
	challenge = make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return "", nil, err
	}
	s.mu.Lock()
	s.pending[ceremonyID] = &Ceremony{UserID: userID, Challenge: challenge, ExpiresAt: time.Now().Add(s.ChallengeTTL)}
	s.mu.Unlock()
	return ceremonyID, challenge, nil
}

// FinishAssertion verifies the assertion. authenticatorData and clientData
// are concatenated and SHA-256 hashed; the signature must verify against the
// stored Ed25519 public key.
func (s *Service) FinishAssertion(ceremonyID, credentialID string, authenticatorData, clientDataJSON, signature []byte) (string, error) {
	s.mu.Lock()
	c, ok := s.pending[ceremonyID]
	if ok {
		delete(s.pending, ceremonyID)
	}
	cred, credOK := s.credentials[credentialID]
	s.mu.Unlock()
	if !ok || time.Now().After(c.ExpiresAt) {
		return "", errors.New("webauthn: expired or unknown ceremony")
	}
	if !credOK {
		return "", errors.New("webauthn: unknown credential")
	}
	clientDataHash := sha256.Sum256(clientDataJSON)
	signed := append(append([]byte{}, authenticatorData...), clientDataHash[:]...)
	if !ed25519.Verify(cred.PublicKey, signed, signature) {
		return "", errors.New("webauthn: signature mismatch")
	}
	return cred.UserID, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
