package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidToken is returned by Verify for any structural or signature error.
// Callers should treat this as "401 Unauthorized" without leaking details.
var ErrInvalidToken = errors.New("invalid token")

// Issuer signs JWTs (HS256) for users.
//
// Genie keeps its own minimal implementation to avoid a third-party JWT
// dependency: the surface area we need is small (HS256, exp, iat, aud), and
// audit risk goes up with each dependency we pull in for security code.
type Issuer struct {
	Secret   []byte
	Issuer   string
	Audience []string
	TTL      time.Duration
}

// NewIssuer builds an Issuer with sane defaults.
func NewIssuer(secret []byte, issuer string, audience []string, ttl time.Duration) *Issuer {
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	return &Issuer{Secret: secret, Issuer: issuer, Audience: audience, TTL: ttl}
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Issue returns a signed JWT for the given user identity.
func (i *Issuer) Issue(userID, email string, roles []Role) (string, Claims, error) {
	now := time.Now().UTC()
	claims := Claims{
		Subject:   userID,
		Email:     email,
		Roles:     roles,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(i.TTL).Unix(),
		Issuer:    i.Issuer,
		Audience:  i.Audience,
	}
	tok, err := encode(jwtHeader{Alg: "HS256", Typ: "JWT"}, claims, i.Secret)
	return tok, claims, err
}

// Verify parses, validates the signature, expiration, and audience of a token.
func (i *Issuer) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	headerB, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var h jwtHeader
	if err := json.Unmarshal(headerB, &h); err != nil || h.Alg != "HS256" {
		return Claims{}, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	expected := sign(signingInput, i.Secret)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	if !hmac.Equal(expected, got) {
		return Claims{}, ErrInvalidToken
	}

	claimsB, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(claimsB, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if time.Now().UTC().Unix() >= c.ExpiresAt {
		return Claims{}, fmt.Errorf("%w: expired", ErrInvalidToken)
	}
	if i.Issuer != "" && c.Issuer != i.Issuer {
		return Claims{}, fmt.Errorf("%w: issuer mismatch", ErrInvalidToken)
	}
	if len(i.Audience) > 0 && !audienceContains(c.Audience, i.Audience) {
		return Claims{}, fmt.Errorf("%w: audience mismatch", ErrInvalidToken)
	}
	return c, nil
}

func audienceContains(have, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if h == w {
				return true
			}
		}
	}
	return false
}

func encode(h jwtHeader, c Claims, secret []byte) (string, error) {
	hb, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	sig := sign(signingInput, secret)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func sign(input string, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(input))
	return mac.Sum(nil)
}
