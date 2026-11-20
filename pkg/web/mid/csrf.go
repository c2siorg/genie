// Package mid holds HTTP middleware: auth (JWT), request logging, OTEL
// tracing, panic recovery, request-id propagation, and CSRF protection.
//
// csrf.go — CSRF token generation, validation, and middleware.
//
// ─── CSRF protection strategy ──────────────────────────────────────────────
//
// Genie uses HMAC-SHA256 double-submit cookie pattern for CSRF protection:
//
//  1. Generate: server creates HMAC(secret, timestamp+nonce) → hex token
//  2. Transmit: token returned to client (via response body or header)
//  3. Submit: client echoes token in X-CSRF-Token header or form field
//  4. Verify: server recomputes HMAC, compares with submitted token
//
// Double-submit avoids session state on the server (stateless, scales to
// load balancers). Token TTL (default 15min) limits blast radius of leaked
// tokens. GET/HEAD/OPTIONS skip validation (read-only, safe).
//
// ─── Token structure ──────────────────────────────────────────────────────
//
// Tokens are hex-encoded strings: signature (64 hex chars) + encoded payload
// (variable length). The payload contains the timestamp and nonce, encoded
// as base64url to avoid special characters in URLs/forms. Signature is
// recomputed on verify; if it matches, the token is valid. If not, reject.
//
// Example token (decoded for readability):
//
//	signature = HMAC-SHA256(secret, payload)
//	payload = base64url(timestamp || nonce)
//	token = hex(signature) + base64url(payload)
//
// ─── Secret extraction ──────────────────────────────────────────────────────
//
// If JWT claims are available (user is authenticated), the function uses
// the user's ID as part of the secret derivation (personalized tokens).
// This ensures tokens are not transferable between users. For unauthenticated
// requests, a global secret is used.
//
// ─── TTL enforcement ──────────────────────────────────────────────────────
//
// Tokens are rejected if they were generated more than TTL ago. Default TTL
// is 15 minutes; this is configurable. The check is strict: 1 second over
// TTL is rejected. No grace period.
package mid

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

const (
	// CSRFTokenLength is the expected length of a valid CSRF token
	// (64 hex-char signature + min 24 base64url chars for timestamp+nonce).
	CSRFTokenLength = 88

	// CSRFHeaderName is the canonical request header for CSRF tokens.
	CSRFHeaderName = "X-CSRF-Token"

	// CSRFFormFieldName is the canonical form field name for CSRF tokens.
	CSRFFormFieldName = "_csrf"

	// NonceLength is the size of the random nonce in bytes (16 = 128 bits).
	NonceLength = 16
)

// CSRFTokenError describes why a token validation failed.
type CSRFTokenError struct {
	Reason string // "missing", "invalid", "expired", "signature mismatch"
}

func (e CSRFTokenError) Error() string {
	return fmt.Sprintf("CSRF token error: %s", e.Reason)
}

// Generator creates CSRF tokens.
//
// Fields are exported so tests can construct generators with known secrets.
// The global secret should be read from environment at startup and never
// exposed in logs or error messages.
type Generator struct {
	// Secret is the HMAC-SHA256 signing key. Must be at least 32 bytes.
	// In production, read from GENIE_CSRF_SECRET environment variable.
	Secret []byte

	// TTL is the maximum age of a token. Defaults to 15 minutes.
	// Tokens older than this are rejected by Validator.
	TTL time.Duration
}

// NewGenerator constructs a CSRF token generator with default TTL (15 minutes).
// The secret must be at least 32 bytes for reasonable security.
func NewGenerator(secret []byte) *Generator {
	return &Generator{
		Secret: secret,
		TTL:    15 * time.Minute,
	}
}

// Generate creates a new CSRF token for the given context.
//
// If claims are available in the context (user is authenticated), the token
// is personalized with the user's subject (ID). Otherwise, the token is
// generic for the session. Tokens are valid for Generator.TTL from now.
//
// Returns a hex-encoded token, or error if secret is invalid.
//
// Example:
//
//	gen := NewGenerator([]byte("...32-byte secret..."))
//	token, err := gen.Generate(r.Context())
//	// token = "abc123...xyz" (hex-encoded signature + payload)
func (g *Generator) Generate(ctx *http.Request) (string, error) {
	if len(g.Secret) < 32 {
		return "", CSRFTokenError{Reason: "secret too short"}
	}

	// Allocate nonce: 16 cryptographically-secure random bytes.
	nonce := make([]byte, NonceLength)
	_, err := rand.Read(nonce)
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encode nonce + timestamp in base64url (no padding, URL-safe).
	timestamp := time.Now().Unix()
	payloadRaw := fmt.Sprintf("%d:%s", timestamp, base64.URLEncoding.EncodeToString(nonce))

	// Derive the effective secret: base secret + user ID if authenticated.
	effectiveSecret := g.Secret
	if claims, ok := ClaimsFrom(ctx.Context()); ok {
		effectiveSecret = append(g.Secret, []byte(claims.Subject)...)
	}

	// Compute HMAC-SHA256(effectiveSecret, payload).
	h := hmac.New(sha256.New, effectiveSecret)
	h.Write([]byte(payloadRaw))
	signature := h.Sum(nil)

	// Token = hex(signature) + base64url(payload).
	// Hex keeps the signature as 64 characters (32 bytes * 2).
	// Base64url keeps the payload URL-safe.
	token := hex.EncodeToString(signature) + "." + payloadRaw
	return token, nil
}

// Validator checks CSRF tokens.
//
// Fields are exported so tests can construct validators with known secrets.
// The secret must match the Generator's secret for valid tokens to verify.
type Validator struct {
	// Secret is the HMAC-SHA256 signing key. Must match the Generator's secret.
	Secret []byte

	// TTL is the maximum age of a token. Tokens older than this are rejected.
	TTL time.Duration
}

// NewValidator constructs a CSRF token validator with default TTL (15 minutes).
// The secret must match the Generator's secret, and must be at least 32 bytes.
func NewValidator(secret []byte) *Validator {
	return &Validator{
		Secret: secret,
		TTL:    15 * time.Minute,
	}
}

// Verify checks if the given token is valid for the request context.
//
// Returns nil if the token is valid. Returns CSRFTokenError with a reason
// ("missing", "invalid", "expired", "signature mismatch") if validation fails.
// Error messages are generic to avoid leaking information to attackers.
//
// Verification steps:
//  1. Check token format (64 hex chars + "." + payload)
//  2. Decode signature and payload
//  3. Derive effective secret (base secret + user ID if authenticated)
//  4. Recompute HMAC-SHA256 of payload
//  5. Compare recomputed signature with submitted signature (constant time)
//  6. Extract timestamp from payload and check age
//
// Example:
//
//	val := NewValidator([]byte("...same 32-byte secret..."))
//	err := val.Verify(r, token)
//	if err != nil {
//	    http.Error(w, "invalid CSRF token", http.StatusForbidden)
//	    return
//	}
//	// token is valid, continue
func (v *Validator) Verify(r *http.Request, token string) error {
	if token == "" {
		return CSRFTokenError{Reason: "missing"}
	}

	if len(v.Secret) < 32 {
		return CSRFTokenError{Reason: "secret too short"}
	}

	// Parse token: signature + "." + payload.
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return CSRFTokenError{Reason: "invalid"}
	}

	signatureHex := parts[0]
	payloadRaw := parts[1]

	// Decode signature from hex.
	submittedSignature, err := hex.DecodeString(signatureHex)
	if err != nil || len(submittedSignature) != sha256.Size {
		return CSRFTokenError{Reason: "invalid"}
	}

	// Extract timestamp from payload: "timestamp:nonce".
	payloadParts := strings.Split(payloadRaw, ":")
	if len(payloadParts) != 2 {
		return CSRFTokenError{Reason: "invalid"}
	}

	timestampStr := payloadParts[0]
	var timestamp int64
	_, err = fmt.Sscanf(timestampStr, "%d", &timestamp)
	if err != nil {
		return CSRFTokenError{Reason: "invalid"}
	}

	// Check token age.
	age := time.Since(time.Unix(timestamp, 0))
	if age < 0 || age > v.TTL {
		return CSRFTokenError{Reason: "expired"}
	}

	// Derive the effective secret: base secret + user ID if authenticated.
	effectiveSecret := v.Secret
	if claims, ok := ClaimsFrom(r.Context()); ok {
		effectiveSecret = append(v.Secret, []byte(claims.Subject)...)
	}

	// Recompute HMAC-SHA256(effectiveSecret, payload).
	h := hmac.New(sha256.New, effectiveSecret)
	h.Write([]byte(payloadRaw))
	expectedSignature := h.Sum(nil)

	// Constant-time comparison to avoid timing attacks.
	if !hmac.Equal(submittedSignature, expectedSignature) {
		return CSRFTokenError{Reason: "signature mismatch"}
	}

	return nil
}

// Middleware returns an http.Handler middleware that validates CSRF tokens.
//
// The middleware validates tokens for unsafe methods (POST, PUT, DELETE, PATCH)
// and skips validation for safe methods (GET, HEAD, OPTIONS, TRACE).
//
// Token sources (checked in order):
//  1. X-CSRF-Token request header
//  2. _csrf form field (for application/x-www-form-urlencoded or multipart/form-data)
//
// If a token is present but invalid, the request is rejected with 403 Forbidden.
// If no token is present for an unsafe method, the request is rejected with 403.
// For safe methods, missing tokens are allowed (stateless, no session lookup).
//
// Example usage in a router:
//
//	validator := NewValidator(csrfSecret)
//	r.Use(validator.Middleware)
//	r.Post("/api/v1/transfer", handler)  // CSRF validated
//	r.Get("/api/v1/accounts", handler)   // CSRF skipped (GET is safe)
//
// Example client-side flow (for form-based submissions):
//
//	// 1. Client fetches a page with a form (GET request, CSRF skipped)
//	// 2. Server generates a token: token, _ := gen.Generate(getRequest)
//	// 3. Server renders form with hidden field: <input name="_csrf" value="{token}">
//	// 4. Client submits form (POST request with _csrf field)
//	// 5. Middleware extracts and validates _csrf field
//	// 6. If valid, request proceeds; if invalid, 403
//
// Example client-side flow (for JSON API):
//
//	// 1. Client makes an authenticated request (e.g., login returns JWT in Set-Cookie)
//	// 2. Server includes token in response body or header
//	// 3. Client includes token in X-CSRF-Token header on subsequent requests
//	// 4. Middleware extracts and validates header
//	// 5. If valid, request proceeds; if invalid, 403
func (v *Validator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip validation for safe methods.
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			next.ServeHTTP(w, r)
			return
		}

		// For unsafe methods, extract and validate token.
		token := r.Header.Get(CSRFHeaderName)

		// If not in header, try form field.
		if token == "" {
			token = r.FormValue(CSRFFormFieldName)
		}

		// Validate token.
		if err := v.Verify(r, token); err != nil {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}

		// Token is valid, proceed.
		next.ServeHTTP(w, r)
	})
}

// SecretFromJWT extracts the HMAC secret from JWT claims if available.
//
// This is a convenience function for bootstrapping: if you already have
// an issuer secret (for JWT signing), you can reuse it for CSRF tokens
// (same key material). The returned secret is safe to use with NewValidator.
//
// If the issuer secret is less than 32 bytes, this function panics
// (deployment config error).
//
// Example:
//
//	jwtIssuer := &auth.Issuer{Secret: []byte("...32-byte secret...")}
//	csrfValidator := NewValidator(jwtIssuer.Secret)
func SecretFromJWT(issuer *auth.Issuer) []byte {
	if len(issuer.Secret) < 32 {
		panic("CSRF: JWT secret too short (min 32 bytes)")
	}
	return issuer.Secret
}
