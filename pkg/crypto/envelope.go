// Package crypto provides envelope encryption for documents Genie persists or
// transmits internally.
//
// Envelope model:
//   - A data encryption key (DEK) is generated per-payload.
//   - The DEK is wrapped (encrypted) by a key encryption key (KEK) resolved by
//     KeyResolver. The wrapped DEK is stored alongside the ciphertext.
//   - Ciphertext + nonce + wrapped DEK + KEK ID is the EncryptedPayload that
//     callers persist.
//
// In production the KEK lives in a KMS (AWS, GCP, HashiCorp Vault). Locally
// we use EnvKeyResolver which reads the KEK from an environment variable.
// The KMSKeyResolver is a stub that documents the production shape.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// EncryptedPayload is the on-disk / on-wire form. Marshal as JSON.
type EncryptedPayload struct {
	KEKID      string `json:"kek_id"`
	WrappedDEK string `json:"wrapped_dek"` // base64(WrappedDEK)
	Nonce      string `json:"nonce"`       // base64
	Ciphertext string `json:"ciphertext"`  // base64
	Algorithm  string `json:"algorithm"`   // "AES-256-GCM"
}

// Marshal returns the JSON wire form.
func (e EncryptedPayload) Marshal() ([]byte, error) { return json.Marshal(e) }

// Unmarshal parses the JSON wire form.
func Unmarshal(b []byte) (EncryptedPayload, error) {
	var p EncryptedPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return EncryptedPayload{}, err
	}
	return p, nil
}

// KeyResolver wraps and unwraps data encryption keys using a key encryption key.
// Implementations must be safe for concurrent use.
type KeyResolver interface {
	// ActiveKEKID returns the identifier of the KEK currently used for new wraps.
	// Stored alongside the ciphertext so the right KEK can be selected at decrypt time.
	ActiveKEKID() string
	// Wrap encrypts dek with the active KEK and returns the wrapped bytes.
	Wrap(dek []byte) ([]byte, error)
	// Unwrap decrypts wrappedDEK using the KEK identified by kekID.
	Unwrap(kekID string, wrappedDEK []byte) ([]byte, error)
}

// Encryptor encrypts arbitrary byte slices into EncryptedPayloads.
type Encryptor struct {
	Keys KeyResolver
}

// New constructs an Encryptor over the given KeyResolver.
func New(keys KeyResolver) *Encryptor { return &Encryptor{Keys: keys} }

// Encrypt seals plaintext with a freshly-generated DEK, wraps the DEK with
// the active KEK, and returns the envelope.
func (e *Encryptor) Encrypt(plaintext []byte) (EncryptedPayload, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return EncryptedPayload{}, fmt.Errorf("generate dek: %w", err)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return EncryptedPayload{}, fmt.Errorf("dek aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedPayload{}, fmt.Errorf("dek gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedPayload{}, fmt.Errorf("nonce: %w", err)
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)

	wrapped, err := e.Keys.Wrap(dek)
	if err != nil {
		return EncryptedPayload{}, fmt.Errorf("wrap dek: %w", err)
	}
	return EncryptedPayload{
		KEKID:      e.Keys.ActiveKEKID(),
		WrappedDEK: base64.StdEncoding.EncodeToString(wrapped),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ct),
		Algorithm:  "AES-256-GCM",
	}, nil
}

// Decrypt opens an EncryptedPayload and returns the plaintext.
func (e *Encryptor) Decrypt(p EncryptedPayload) ([]byte, error) {
	if p.Algorithm != "" && p.Algorithm != "AES-256-GCM" {
		return nil, fmt.Errorf("unsupported algorithm %q", p.Algorithm)
	}
	wrapped, err := base64.StdEncoding.DecodeString(p.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("wrapped dek b64: %w", err)
	}
	dek, err := e.Keys.Unwrap(p.KEKID, wrapped)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek: %w", err)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(p.Nonce)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(p.Ciphertext)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}
	return gcm.Open(nil, nonce, ct, nil)
}
