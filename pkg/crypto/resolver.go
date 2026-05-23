package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// EnvKeyResolver loads the KEK from an environment variable. The variable
// holds a base64-encoded 32-byte key. Suitable for local dev and CI only.
//
// In production prefer KMSKeyResolver (or your platform's equivalent).
type EnvKeyResolver struct {
	KEKID string
	kek   []byte
	once  sync.Once
	err   error

	// EnvVar is the environment variable name. Defaults to GENIE_KEK_BASE64.
	EnvVar string
}

// NewEnvKeyResolver constructs a resolver. Set GENIE_KEK_BASE64 before use.
func NewEnvKeyResolver(kekID string) *EnvKeyResolver {
	return &EnvKeyResolver{KEKID: kekID, EnvVar: "GENIE_KEK_BASE64"}
}

func (r *EnvKeyResolver) load() {
	r.once.Do(func() {
		name := r.EnvVar
		if name == "" {
			name = "GENIE_KEK_BASE64"
		}
		v := os.Getenv(name)
		if v == "" {
			r.err = fmt.Errorf("env %s not set", name)
			return
		}
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			r.err = fmt.Errorf("env %s base64: %w", name, err)
			return
		}
		if len(b) != 32 {
			r.err = fmt.Errorf("env %s must decode to 32 bytes, got %d", name, len(b))
			return
		}
		r.kek = b
	})
}

func (r *EnvKeyResolver) ActiveKEKID() string {
	if r.KEKID == "" {
		return "local-env-v1"
	}
	return r.KEKID
}

// Wrap encrypts the DEK using AES-GCM under the KEK.
func (r *EnvKeyResolver) Wrap(dek []byte) ([]byte, error) {
	r.load()
	if r.err != nil {
		return nil, r.err
	}
	return aesGCMSeal(r.kek, dek)
}

func (r *EnvKeyResolver) Unwrap(kekID string, wrapped []byte) ([]byte, error) {
	r.load()
	if r.err != nil {
		return nil, r.err
	}
	if kekID != "" && kekID != r.ActiveKEKID() {
		return nil, fmt.Errorf("kek id %q not known to env resolver", kekID)
	}
	return aesGCMOpen(r.kek, wrapped)
}

// KMSKeyResolver documents the production shape; the actual KMS calls are
// left for downstream integrators. The struct is exported to make the
// interface surface visible without forcing a cloud SDK dependency on every
// build.
type KMSKeyResolver struct {
	KEKID  string
	Client KMSClient
}

// KMSClient is the minimal surface a real KMS adapter needs to implement.
type KMSClient interface {
	Encrypt(kekID string, plaintext []byte) ([]byte, error)
	Decrypt(kekID string, ciphertext []byte) ([]byte, error)
}

func (r *KMSKeyResolver) ActiveKEKID() string { return r.KEKID }

func (r *KMSKeyResolver) Wrap(dek []byte) ([]byte, error) {
	if r.Client == nil {
		return nil, errors.New("kms client not configured")
	}
	return r.Client.Encrypt(r.KEKID, dek)
}

func (r *KMSKeyResolver) Unwrap(kekID string, wrapped []byte) ([]byte, error) {
	if r.Client == nil {
		return nil, errors.New("kms client not configured")
	}
	return r.Client.Decrypt(kekID, wrapped)
}

// aesGCMSeal seals plaintext with key using AES-GCM, prefixing the nonce.
func aesGCMSeal(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ct...), nil
}

// aesGCMOpen reverses aesGCMSeal.
func aesGCMOpen(key, payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(payload) < gcm.NonceSize() {
		return nil, errors.New("payload too short")
	}
	nonce, ct := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
