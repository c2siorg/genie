// Package privacy provides utilities for PII tokenisation and differential-
// privacy noise injection. The goal is making aggregate analytics safe to
// surface without leaking individual records.
//
// Two primitives:
//
//   - Tokenise: deterministic, keyed HMAC-SHA256 mapping from sensitive
//     value to opaque token. Same input → same token within a key epoch,
//     so joins still work across systems; different keys per tenant.
//   - LaplaceNoise / GaussianNoise: add calibrated noise to numeric
//     aggregates so single-record contribution is statistically hidden.
package privacy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"math/rand"
	"sync"
)

// Tokeniser maps sensitive string values (account numbers, phone, email)
// to opaque, deterministic tokens keyed by a tenant secret. Pseudonymise
// once, join across systems with the token, never carry the raw value.
type Tokeniser struct {
	mu     sync.Mutex
	secret []byte
}

// NewTokeniser builds a tokeniser from a 32-byte secret.
func NewTokeniser(secret []byte) (*Tokeniser, error) {
	if len(secret) < 16 {
		return nil, errors.New("privacy: tokeniser secret must be >= 16 bytes")
	}
	return &Tokeniser{secret: secret}, nil
}

// Tokenise returns a hex token prefixed with "tok_" so it's obvious in
// logs that the value has been pseudonymised.
func (t *Tokeniser) Tokenise(value string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(value))
	return "tok_" + hex.EncodeToString(mac.Sum(nil))[:32]
}

// LaplaceNoise returns Laplace(0, scale) noise. scale = sensitivity / epsilon
// per the standard (ε,0)-differential-privacy mechanism.
//
// rng is exposed so deterministic tests can pass a seeded source.
func LaplaceNoise(rng *rand.Rand, scale float64) float64 {
	if scale <= 0 {
		return 0
	}
	u := rng.Float64() - 0.5
	sign := 1.0
	if u < 0 {
		sign = -1.0
		u = -u
	}
	return -sign * scale * math.Log(1-2*u)
}

// GaussianNoise returns Gaussian(0, sigma) noise for (ε,δ)-DP mechanisms.
func GaussianNoise(rng *rand.Rand, sigma float64) float64 {
	if sigma <= 0 {
		return 0
	}
	return rng.NormFloat64() * sigma
}

// AggregateWithDP applies Laplace noise to a numeric aggregate.
//
// sensitivity is the worst-case change in the aggregate from adding /
// removing a single record (e.g. 1 for a count). epsilon is the privacy
// budget — lower = noisier, more private.
func AggregateWithDP(rng *rand.Rand, value, sensitivity, epsilon float64) float64 {
	if epsilon <= 0 || sensitivity <= 0 {
		return value
	}
	return value + LaplaceNoise(rng, sensitivity/epsilon)
}
