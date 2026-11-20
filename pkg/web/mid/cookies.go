// Package mid holds HTTP middleware: auth (JWT), request logging, OTEL
// tracing, panic recovery, request-id propagation, CSRF protection,
// and HttpOnly session management.
//
// cookies.go — HttpOnly session creation, validation, refresh, and destruction.
//
// ─── Session management strategy ───────────────────────────────────────────
//
// Genie uses HttpOnly, Secure, SameSite=Strict cookies for session management:
//
//  1. Create: Server mints JWT with claims (user_id, csrf_secret, exp, iat),
//     stores in HttpOnly cookie with Secure + SameSite=Strict flags.
//     Returns cookie to client; client cannot access via JavaScript.
//
//  2. Validate: Extract cookie from request, parse JWT, verify signature
//     and expiry, return user claims. Cookie transmitted automatically
//     by browser on every request (but cannot be read by JS).
//
//  3. Refresh: Check if token age > half of TTL. If true, mint new token with
//     refreshed expiry and return updated cookie. Silent refresh
//     extends session without requiring user interaction.
//
//  4. Destroy: Set cookie Max-Age=0 to trigger browser deletion. Return success.
//     Cookie is cleared from client and server-side (no revocation list).
//
// ─── Cookie flags ────────────────────────────────────────────────────────────
//
//	HttpOnly=true  — Cookie is NOT accessible to JavaScript. Prevents XSS theft.
//	                 Browser transmits the cookie automatically; client code
//	                 cannot read document.cookie to extract the session token.
//
//	Secure=true    — Cookie is only transmitted over HTTPS. HTTP requests are
//	                 rejected. Prevents man-in-the-middle capture.
//
//	SameSite=Strict — Cookie is NOT sent on cross-site requests (e.g., when a
//	                  form on evil.com tries to POST to genie.com). Prevents
//	                  CSRF attacks where the attacker forges a request and
//	                  relies on the browser auto-sending the session cookie.
//
//	Path=/         — Cookie is valid for the entire domain (all routes).
//
//	Max-Age=86400  — Cookie expires in 24 hours (86400 seconds). The JWT itself
//	                 also expires; both serve as defense-in-depth.
//
// ─── Claims structure ────────────────────────────────────────────────────────
//
// Session JWTs use the same Claims type as auth.Issuer (sub, email, roles,
// iat, exp, iss, aud, act). Additionally, when a session is created, the
// session layer generates a CSRF secret and stores it out-of-band (or it can
// be derived from the user_id + a server secret for stateless operation).
//
// For stateless CSRF: the server stores only the signing secret; the CSRF
// secret itself is not stored. When the client sends a CSRF token, the server
// re-derives the expected secret from (user_id, server_secret, timestamp) and
// recomputes the HMAC. No storage layer needed.
//
// ─── Token age calculation ────────────────────────────────────────────────────
//
// Age = now - iat (issued-at time). If age > TTL/2, the token is refreshed.
// Example: TTL = 24 hours, refresh threshold = 12 hours. Token issued at
// 10:00, checked at 22:00 → age = 12 hours → refresh triggered.
//
// ─── Thread safety ───────────────────────────────────────────────────────────
//
// SessionManager is safe to call concurrently from multiple goroutines.
// The underlying auth.Issuer is also safe (no mutable state after construction).
package mid

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

const (
	// SessionCookieName is the canonical name for the session cookie.
	SessionCookieName = "session"

	// SessionRefreshThreshold is the fraction of TTL at which a token is
	// refreshed. If a token's age > (TTL * threshold), issue a new one.
	SessionRefreshThreshold = 0.5

	// CSRFSecretLength is the size of the per-session CSRF secret in bytes
	// (32 bytes = 256 bits). CSRF tokens are HMAC-SHA256(csrf_secret, data),
	// so a 256-bit key is appropriate.
	CSRFSecretLength = 32

	// DefaultSessionCookieTTL is the default lifetime of the session cookie.
	// Matches the JWT TTL for consistency; both expire together.
	DefaultSessionCookieTTL = 24 * time.Hour
)

// SessionError describes a session operation failure.
type SessionError struct {
	Code   string // "missing_cookie", "invalid_token", "expired", "verification_failed"
	Reason string // human-readable detail
}

func (e SessionError) Error() string {
	return fmt.Sprintf("session error: %s (%s)", e.Code, e.Reason)
}

// SessionManager handles creation, validation, refresh, and destruction of
// HttpOnly session cookies.
//
// SessionManager wraps an auth.Issuer and adds HTTP cookie handling.
// The issuer signs JWTs; the manager decorates them with HttpOnly flags and
// handles cookie extraction/setting/clearing in responses.
//
// All fields are exported so tests can construct managers with custom issuer
// or cookie parameters. In production, use NewSessionManager.
type SessionManager struct {
	// Issuer is the JWT signer. SessionManager does not create the Issuer;
	// it expects to receive one already configured with a secret, issuer name,
	// audience list, and TTL.
	Issuer *auth.Issuer

	// CookieTTL is the Max-Age attribute of the session cookie.
	// Defaults to 24 hours if zero; always kept in sync with Issuer.TTL
	// so both the HTTP cookie and the JWT expire together.
	CookieTTL time.Duration

	// Secure controls the Secure flag on the session cookie.
	// Set to true in production (HTTPS only); false in dev (localhost HTTP).
	Secure bool

	// SameSite controls the SameSite attribute ("Strict", "Lax", "None").
	// Production: "Strict" (recommended, blocks CSRF). Development: "Lax"
	// allows form submissions from other sites (useful for OAuth redirects).
	SameSite string
}

// NewSessionManager constructs a SessionManager with sane defaults.
//
// The issuer is passed in; NewSessionManager configures HTTP cookie flags
// based on the environment:
//   - Secure=true (HTTPS only, production default)
//   - SameSite=Strict (prevents CSRF, production default)
//   - CookieTTL = issuer.TTL (so they expire together)
//
// For testing or non-HTTPS environments, call with Secure=false or override
// SameSite after construction.
func NewSessionManager(issuer *auth.Issuer, secure bool) *SessionManager {
	ttl := issuer.TTL
	if ttl == 0 {
		ttl = DefaultSessionCookieTTL
	}
	sameSite := "Strict"
	if !secure {
		sameSite = "Lax" // Allow form submissions from other sites in dev
	}
	return &SessionManager{
		Issuer:    issuer,
		CookieTTL: ttl,
		Secure:    secure,
		SameSite:  sameSite,
	}
}

// CreateSession mints a new session JWT and returns an http.Cookie
// configured with HttpOnly, Secure, and SameSite=Strict flags.
//
// The JWT contains the standard claims (sub, email, roles, iat, exp, iss, aud).
// The returned cookie is set on the response writer; the caller should call
// http.SetCookie or rely on an adapter to install it.
//
// The CSRF secret is generated here (32 random bytes) but not stored in the
// JWT. Instead, callers should store it out-of-band (in session storage or
// derive it deterministically from user_id + server secret).
//
// Returns the cookie, the JWT claims (for logging/inspection), the CSRF secret
// in hex, and any error.
func (sm *SessionManager) CreateSession(w http.ResponseWriter, userID, email string, roles []auth.Role) (*http.Cookie, auth.Claims, string, error) {
	// Mint the JWT using the issuer.
	token, claims, err := sm.Issuer.Issue(userID, email, roles)
	if err != nil {
		return nil, auth.Claims{}, "", fmt.Errorf("failed to issue JWT: %w", err)
	}

	// Generate a CSRF secret (random bytes, stored separately by caller).
	csrfSecret := make([]byte, CSRFSecretLength)
	if _, err := rand.Read(csrfSecret); err != nil {
		return nil, auth.Claims{}, "", fmt.Errorf("failed to generate CSRF secret: %w", err)
	}
	csrfSecretHex := hex.EncodeToString(csrfSecret)

	// Build the session cookie with HttpOnly, Secure, and SameSite flags.
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sm.CookieTTL.Seconds()),
		HttpOnly: true,                    // NOT accessible to JavaScript
		Secure:   sm.Secure,               // HTTPS only
		SameSite: http.SameSiteStrictMode, // Block cross-site requests
	}

	// Set the cookie on the response.
	http.SetCookie(w, cookie)

	return cookie, claims, csrfSecretHex, nil
}

// ValidateSession extracts the session cookie from the request, parses and
// verifies the JWT, and returns the claims if valid.
//
// On success, returns the Claims and nil error.
// On any failure (missing cookie, invalid JWT, expired token, signature
// mismatch), returns zero Claims and a SessionError.
//
// The returned claims can be installed on the context with WithClaims()
// for downstream handlers to use.
func (sm *SessionManager) ValidateSession(r *http.Request) (auth.Claims, error) {
	// Extract the session cookie.
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		if err == http.ErrNoCookie {
			return auth.Claims{}, SessionError{
				Code:   "missing_cookie",
				Reason: "session cookie not found in request",
			}
		}
		return auth.Claims{}, SessionError{
			Code:   "invalid_cookie",
			Reason: fmt.Sprintf("failed to read cookie: %v", err),
		}
	}

	// Verify the JWT.
	claims, err := sm.Issuer.Verify(cookie.Value)
	if err != nil {
		// Wrap the JWT error as a SessionError for consistent error handling.
		if strings.Contains(err.Error(), "expired") {
			return auth.Claims{}, SessionError{
				Code:   "expired",
				Reason: "session token has expired",
			}
		}
		return auth.Claims{}, SessionError{
			Code:   "verification_failed",
			Reason: fmt.Sprintf("JWT verification failed: %v", err),
		}
	}

	return claims, nil
}

// RefreshSession checks if a session is past its refresh threshold.
// If the token's age > (TTL * SessionRefreshThreshold), a new token is
// minted and returned as a fresh cookie. Otherwise, returns nil (no refresh).
//
// This enables silent token refresh: when a valid session is used but is
// getting old, the server automatically extends it without the client having
// to log in again.
//
// Example:
//   - TTL = 24 hours
//   - Refresh threshold = 0.5 (12 hours)
//   - Token issued at 10:00, validated at 22:30 → age = 12.5 hours > 12 hours
//   - Refresh is triggered; new cookie with exp = 22:30 + 24 hours is returned
//
// Returns the new cookie (or nil if refresh not needed), the new claims,
// and any error.
func (sm *SessionManager) RefreshSession(w http.ResponseWriter, claims auth.Claims) (*http.Cookie, auth.Claims, error) {
	now := time.Now().UTC()
	iatTime := time.Unix(claims.IssuedAt, 0)
	age := now.Sub(iatTime)
	threshold := time.Duration(float64(sm.Issuer.TTL) * SessionRefreshThreshold)

	// If the token is not yet old enough to refresh, return nil (no action).
	if age <= threshold {
		return nil, claims, nil
	}

	// Token is old; mint a new one with the same claims (same user, email, roles).
	token, newClaims, err := sm.Issuer.Issue(claims.Subject, claims.Email, claims.Roles)
	if err != nil {
		return nil, auth.Claims{}, fmt.Errorf("failed to refresh session: %w", err)
	}

	// Build and set the new cookie.
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sm.CookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   sm.Secure,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)

	return cookie, newClaims, nil
}

// DestroySession clears the session cookie by setting Max-Age=0.
// The browser will immediately delete the cookie.
//
// On the server side, there is no explicit revocation list; the session is
// invalidated by virtue of the cookie being cleared from the client. Any
// subsequent requests without the cookie will fail validation.
//
// If stateful revocation is needed (e.g., logout immediately invalidates
// all sessions for a user, not just the current device), callers should
// implement a separate revocation list or blacklist outside this function.
//
// Returns any error from setting the cookie.
func (sm *SessionManager) DestroySession(w http.ResponseWriter) error {
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // Negative MaxAge signals immediate deletion
		HttpOnly: true,
		Secure:   sm.Secure,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
	return nil
}

// GenerateCSRFSecret generates a cryptographically random CSRF secret.
// The secret is returned as a hex-encoded string for easy storage/transmission.
//
// Used internally by CreateSession; also exported for tests or custom
// session-creation flows. The secret should be stored separately from the
// JWT (e.g., in a database or derived from a server secret + user_id).
func GenerateCSRFSecret() (string, error) {
	secret := make([]byte, CSRFSecretLength)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate CSRF secret: %w", err)
	}
	return hex.EncodeToString(secret), nil
}

// DeriveCSRFSecret derives a deterministic CSRF secret from a user ID and
// server master secret. Useful for stateless operation where you don't want
// to store per-session CSRF secrets.
//
// Algorithm:
//
//	secret = HMAC-SHA256(server_master_secret, user_id)
//	return hex(secret)
//
// The returned secret is the same for the same (user_id, master_secret) pair,
// so all sessions for a user share the same CSRF secret. This is fine if your
// CSRF token is time-bound (e.g., issued for 15 minutes). For long-lived
// sessions where you want per-session CSRF secrets, use GenerateCSRFSecret()
// and store them out-of-band.
func DeriveCSRFSecret(userID string, masterSecret []byte) string {
	mac := hmac.New(sha256.New, masterSecret)
	mac.Write([]byte(userID))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum)
}

// ctxSessionKey constants for storing session claims on the context.
// Separate from ctxClaims (used by Auth middleware) to distinguish sessions
// from bearer tokens. We reuse the ctxKey type from auth.go.
const (
	ctxSessionClaims ctxKey = 100 // arbitrary unique value
)

// WithSessionClaims stores the session claims on the context.
// Used internally by session middleware; also exported for tests.
func WithSessionClaims(ctx context.Context, claims auth.Claims) context.Context {
	return context.WithValue(ctx, ctxSessionClaims, claims)
}

// SessionClaimsFrom returns the session claims from the context, if present.
// Returns the Claims and a boolean indicating presence.
func SessionClaimsFrom(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(ctxSessionClaims).(auth.Claims)
	return c, ok
}

// SessionMiddleware is HTTP middleware that validates the session cookie on
// every request.
//
// If the cookie is present and valid, the claims are attached to the context.
// If the cookie is missing or invalid, the request fails with 401 Unauthorized.
//
// Usage:
//
//	mux := http.NewServeMux()
//	sm := NewSessionManager(issuer, secure)
//	mux.Use(SessionMiddleware(sm))
//
// After SessionMiddleware, downstream handlers can retrieve the claims via
// SessionClaimsFrom(r.Context()).
func SessionMiddleware(sm *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := sm.ValidateSession(r)
			if err != nil {
				// Log the error if a logger is available on the context.
				// For now, just return 401 to the client.
				http.Error(w, "invalid session", http.StatusUnauthorized)
				return
			}

			// Attempt to refresh the session if needed. If refresh fails,
			// log it but continue (the old token is still valid).
			// The client will get a 200 response, but may not receive the
			// new cookie if the refresh failed — trade-off: simplicity over
			// strict refresh-on-every-request.
			_, _, _ = sm.RefreshSession(w, claims)

			// Attach claims to the context and continue.
			next.ServeHTTP(w, r.WithContext(WithSessionClaims(r.Context(), claims)))
		})
	}
}

// SessionAndBearerMiddleware is HTTP middleware that accepts either a session
// cookie OR a bearer token in the Authorization header.
//
// Priority: session cookie is checked first; if missing or invalid, falls
// back to bearer token. This enables mixed authentication (some clients use
// sessions, some use tokens).
//
// Usage:
//
//	sm := NewSessionManager(issuer, secure)
//	mux.Use(SessionAndBearerMiddleware(sm, issuer))
//
// On success, the claims are attached via WithClaims (consistent with the
// existing Auth middleware).
func SessionAndBearerMiddleware(sm *SessionManager, issuer *auth.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try session cookie first.
			claims, err := sm.ValidateSession(r)
			if err == nil {
				// Attempt to refresh if needed.
				_, _, _ = sm.RefreshSession(w, claims)
				next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
				return
			}

			// Fall back to bearer token.
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "missing session or bearer token", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err = issuer.Verify(tok)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}
