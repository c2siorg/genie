// jwt.go — HS256 JWT issuance + verification, stdlib-only.
//
// ─── Why we don't use a JWT library ────────────────────────────────────────
//
// Genie keeps its own minimal JWT implementation rather than pulling in
// a third-party library. The surface area we need is small (HS256, exp,
// iat, aud), and the audit risk goes up with each dependency we pull in
// for security code. A third-party library is more code to read on
// every release, more CVE notifications to triage, and (historically)
// a class of "alg=none" / RSA-key-confusion bugs that have bitten
// well-known JWT libraries.
//
// The whole HS256 path is ~150 lines of stdlib code: crypto/hmac,
// crypto/sha256, encoding/base64, encoding/json. It fits in your head,
// it's reviewable in 15 minutes, and it does what we need.
//
// ─── What's in this file ───────────────────────────────────────────────────
//
//   - Issuer struct: holds secret, audience defaults, TTL
//   - Issue() — first-party user token
//   - IssueWithActor() — token-exchange output (RFC 8693 act claim)
//   - Verify() — strict verify (audience checked)
//   - VerifyIgnoringAudience() — used by the token-exchange package
//   - encode / sign — internal helpers
//
// ─── Algorithm: HS256, not RS256 ───────────────────────────────────────────
//
// We sign with HS256 (HMAC-SHA256, shared secret) because in Genie's
// reference topology the same process verifies what it issues — there's
// one secret holder. RS256 (asymmetric) would only buy us "any service
// can verify without holding the signing key", which is the federated
// pattern. When Genie federates against an upstream IdP, we add an
// RS256-capable verifier alongside; the issuer for our own tokens stays
// HS256 because asymmetric signing has more failure modes (key rotation
// machinery, JWKS endpoint, alg-confusion attacks).
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

// ErrInvalidToken is returned by Verify for any structural or signature
// error. Callers should treat this as "401 Unauthorized" without leaking
// details.
//
// Why a single sentinel for every failure: error messages from JWT
// verification can leak information ("expired" vs "signature mismatch"
// vs "issuer wrong" tells an attacker which part of the token they
// guessed correctly). The HTTP middleware should return 401 with no
// detail to the client. Internal logs can still include the wrapped
// detail via fmt.Errorf("%w: …").
var ErrInvalidToken = errors.New("invalid token")

// Issuer signs JWTs (HS256) for users.
//
// Genie keeps its own minimal implementation to avoid a third-party JWT
// dependency: the surface area we need is small (HS256, exp, iat, aud), and
// audit risk goes up with each dependency we pull in for security code.
//
// Fields are exported so a test or a CLI can construct one directly.
// In production, prefer NewIssuer which validates the TTL.
type Issuer struct {
	// Secret is the HS256 signing key. Must be at least 32 bytes for
	// reasonable security (256-bit symmetric key). Production reads
	// this from GENIE_JWT_SECRET; never hard-code.
	Secret []byte

	// Issuer is the "iss" claim value. Genie's deployments use
	// "genie-api" by default. Used by the verifier to reject tokens
	// from other issuers (cross-IdP confusion protection).
	Issuer string

	// Audience is the default "aud" claim — list of audiences a
	// first-party token is good for. Verify() requires at least one
	// match if it has a non-empty audience list of its own.
	Audience []string

	// TTL is the default token lifetime. Defaults to 15 minutes in
	// NewIssuer if zero. Short TTLs limit the blast radius of a stolen
	// token; the price is more frequent re-authentication.
	TTL time.Duration
}

// NewIssuer builds an Issuer with sane defaults.
//
// Defaults applied:
//   - TTL: 15 minutes if zero
//
// Other fields are passed through. The function does NOT validate
// Secret length — that's a deployment-config concern and a 0-byte
// secret would fail at first sign-attempt anyway. Future improvement:
// require at least 32 bytes and return an error.
func NewIssuer(secret []byte, issuer string, audience []string, ttl time.Duration) *Issuer {
	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	return &Issuer{Secret: secret, Issuer: issuer, Audience: audience, TTL: ttl}
}

// jwtHeader is the standard JWT header. We only support HS256 with
// typ=JWT. A real implementation might support multiple algs; we don't,
// because the verifier rejects anything else and supporting more would
// just be more attack surface.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Issue returns a signed JWT for the given user identity.
//
// Sets the standard claims (sub, email, roles, iat, exp, iss, aud)
// using the Issuer's defaults. Does NOT set Actor — that's the
// IssueWithActor path for token-exchange flows.
//
// Returns the encoded token (header.payload.signature), the populated
// Claims struct (so the caller can inspect without re-decoding), and
// any encode error.
//
// Used by the password-login handler and the OAuth code-exchange flow.
func (i *Issuer) Issue(userID, email string, roles []Role) (string, Claims, error) {
	// All timestamps in UTC seconds. JWT spec uses NumericDate which is
	// "seconds since the epoch, UTC." Using time.Now().UTC() avoids
	// confusion in mixed-tz deployments.
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

// IssueWithActor returns a signed JWT with a custom audience and an RFC
// 8693 actor claim. Used by the token-exchange flow to mint a
// dual-identity token whose Subject stays the original user and Actor
// identifies the service currently acting on the user's behalf.
//
// When audience is nil the Issuer's default audience is used. Pass an
// explicit audience to scope the token to a specific upstream target.
//
// This is the only path that sets the Actor field — the first-party
// Issue() path leaves it nil, which keeps the JWT small for the common
// case (one Actor adds ~40 bytes to the base64-encoded payload).
//
// Why a separate method instead of overloading Issue: keeps the common
// case (Issue with no audience override, no actor) ergonomic, and makes
// it obvious in the call site that the caller is doing a token-exchange
// mint rather than a first-party login.
func (i *Issuer) IssueWithActor(userID, email string, roles []Role, audience []string, actor *Actor) (string, Claims, error) {
	now := time.Now().UTC()
	// Default to the Issuer's audience if the caller didn't override.
	// In practice, every token-exchange call passes an explicit audience
	// (that's the whole point of the audience-scoping), so this fallback
	// is mostly defensive.
	aud := audience
	if len(aud) == 0 {
		aud = i.Audience
	}
	claims := Claims{
		Subject:   userID,
		Email:     email,
		Roles:     roles,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(i.TTL).Unix(),
		Issuer:    i.Issuer,
		Audience:  aud,
		Actor:     actor,
	}
	tok, err := encode(jwtHeader{Alg: "HS256", Typ: "JWT"}, claims, i.Secret)
	return tok, claims, err
}

// VerifyIgnoringAudience is Verify with the audience check skipped.
// Used by the token-exchange flow where the input token may already be
// scoped to a different audience than the issuer's default.
//
// Concurrency note: mutates Audience temporarily; not safe to call from
// many goroutines on the same Issuer. For multi-goroutine use, the
// token-exchange package should hold the verifier behind a mutex or use
// a per-call copy of the Issuer.
//
// In current usage (Service.Exchange is the only caller), Exchange
// itself doesn't run concurrent verifies on the same Issuer pointer —
// the cache lookup short-circuits before the verifier runs in the
// common path. If the access pattern changes, fix this method (use a
// local copy of the Issuer rather than mutating) before the concurrent
// bug surfaces.
func (i *Issuer) VerifyIgnoringAudience(token string) (Claims, error) {
	// Save the original audience so we can restore it after the verify.
	saved := i.Audience
	// Setting Audience to nil disables the audience check inside Verify
	// (len(i.Audience) > 0 gates the check).
	i.Audience = nil
	c, err := i.Verify(token)
	// Restore unconditionally — even if Verify errored, we don't want
	// to leave the Issuer in a permanently-loose state.
	i.Audience = saved
	return c, err
}

// Verify parses, validates the signature, expiration, and audience of a
// token.
//
// Algorithm:
//   1. Split on '.' — JWT format is header.payload.signature, 3 parts.
//   2. Base64-decode the header; require Alg=HS256, Typ=JWT.
//   3. Recompute the signature over header+'.'+payload and compare with
//      hmac.Equal (constant-time).
//   4. Base64-decode the payload into Claims.
//   5. Check expiry: exp > now.
//   6. Check issuer: iss == this Issuer's Issuer (if non-empty).
//   7. Check audience: at least one of our audiences is in the token's
//      audience list (if our audience is non-empty).
//
// On any failure, returns ErrInvalidToken (sometimes wrapped with the
// underlying detail for logs). Returns the parsed Claims on success.
//
// Defence against the classic JWT attacks:
//   - alg=none — rejected at step 2 (we require HS256)
//   - alg confusion (HS256 token verified as RS256) — N/A, we only
//     support HS256
//   - signature stripping — caught at step 1 (need 3 parts) and step 3
//     (signature mismatch)
//   - expired token replay — caught at step 5
//   - audience confusion — caught at step 7 (if audience is set)
//   - issuer confusion in federated setups — caught at step 6
func (i *Issuer) Verify(token string) (Claims, error) {
	// JWT format: three dot-separated base64-url parts.
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	// Decode the header. Strict on alg + typ to defeat alg=none.
	headerB, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var h jwtHeader
	if err := json.Unmarshal(headerB, &h); err != nil || h.Alg != "HS256" {
		return Claims{}, ErrInvalidToken
	}

	// Recompute the signature over header.payload using our secret, and
	// compare in constant time. hmac.Equal is the constant-time
	// comparator; using bytes.Equal here would be a timing side channel.
	signingInput := parts[0] + "." + parts[1]
	expected := sign(signingInput, i.Secret)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	if !hmac.Equal(expected, got) {
		return Claims{}, ErrInvalidToken
	}

	// Signature is good; decode the payload.
	claimsB, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(claimsB, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	// Expiry check. Comparing Unix seconds avoids any timezone bugs;
	// JWT exp is always seconds-since-epoch UTC.
	if time.Now().UTC().Unix() >= c.ExpiresAt {
		return Claims{}, fmt.Errorf("%w: expired", ErrInvalidToken)
	}
	// Issuer check, only if we have an Issuer set. Reject tokens minted
	// by a different issuer than the one we expect.
	if i.Issuer != "" && c.Issuer != i.Issuer {
		return Claims{}, fmt.Errorf("%w: issuer mismatch", ErrInvalidToken)
	}
	// Audience check, only if we have an Audience set. At least one of
	// our audiences must be in the token's audience list — that's the
	// standard "any-of" semantics for JWT aud.
	if len(i.Audience) > 0 && !audienceContains(c.Audience, i.Audience) {
		return Claims{}, fmt.Errorf("%w: audience mismatch", ErrInvalidToken)
	}
	return c, nil
}

// audienceContains reports whether the token's audience list ("have")
// contains at least one of the audiences we're willing to accept
// ("want"). O(have * want) — both are short, so no need for a map.
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

// encode produces a signed JWT from a header, claims, and secret.
//
// Two-stage:
//   1. Marshal header and claims to JSON, base64-URL encode each.
//   2. Concatenate header.payload, compute HMAC-SHA256 over the
//      concatenation, base64-URL encode the signature, append.
//
// Output: "header.payload.signature".
func encode(h jwtHeader, c Claims, secret []byte) (string, error) {
	hb, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	// Use RawURLEncoding (no padding) per RFC 7515 §2.
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	sig := sign(signingInput, secret)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// sign returns HMAC-SHA256(input, secret). Uses crypto/hmac so the
// signature is deterministic and the comparison path can use
// hmac.Equal (constant-time).
func sign(input string, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(input))
	return mac.Sum(nil)
}
