# Genie Phase 2: Security Architecture & Implementation Spec

**Status**: Implementation Design Document  
**Target**: Genie Phase 2 (e-Rupee Commerce)  
**Author**: Security Architecture Team  
**Date**: June 4, 2026  
**Governance**: RBI FREE-AI Aligned + OWASP Top 10  

---

## Executive Summary

This document specifies the complete security architecture for Genie Phase 2, addressing OWASP A01:2021 (Broken Access Control), A05:2021 (Security Misconfiguration), and A04:2021 (Insecure Design) through four integrated subsystems:

1. **CSRF Protection System** — Token generation (HMAC-SHA256), validation, and rotation
2. **HttpOnly Session Management** — Secure cookies with JWT payload mutations
3. **Security Headers Middleware** — CSP, X-Frame-Options, X-Content-Type-Options
4. **Error Handling Standards** — Generic responses with sensitive data scrubbing

---

## 1. CSRF Protection System

### 1.1 Architecture Overview

The CSRF protection system prevents Cross-Site Request Forgery attacks by:

- **Issuing unique, cryptographically secure tokens** per user session
- **Validating tokens on all state-modifying requests** (POST, PUT, DELETE, PATCH)
- **Binding tokens to sessions** via HMAC signing (double-submit + storage)
- **Rotating tokens** on every successful POST/PUT/DELETE to minimize exposure window
- **TTL enforcement** — tokens expire in 15 minutes; requests with stale tokens are rejected

### 1.2 Token Generation Algorithm

The token uses **HMAC-SHA256** over a 32-byte random nonce, signed with a server secret:

```
CSRF Token = base64(HMAC-SHA256(nonce, serverSecret))
Token Signature = HMAC-SHA256(token || sessionID || timestamp, serverSecret)
Storage = token || "." || signature || "." || expiresAt
```

### 1.3 Go Implementation: CSRF Service

```go
// Package security provides CSRF, session, and error-handling primitives.
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
)

// CSRFConfig holds CSRF token generation/validation parameters.
type CSRFConfig struct {
	TokenTTL         time.Duration // default 15 minutes
	MaxTokenAge      time.Duration // reject if older than this
	ServerSecret     []byte        // HMAC signing key (min 32 bytes)
	TokenLength      int           // nonce length (default 32 bytes)
	RotateOnValidate bool          // issue new token after validation (default true)
}

// DefaultCSRFConfig returns sensible defaults aligned with NIST SP 800-63B.
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		TokenTTL:         15 * time.Minute,
		MaxTokenAge:      15 * time.Minute,
		ServerSecret:     make([]byte, 32), // MUST be overridden at init
		TokenLength:      32,
		RotateOnValidate: true,
	}
}

// CSRFToken represents a parsed, validated CSRF token bound to a session.
type CSRFToken struct {
	Nonce       string    `json:"nonce"`                    // base64-encoded random nonce
	Signature   string    `json:"signature"`                // HMAC signature
	ExpiresAt   time.Time `json:"expires_at"`               // token TTL
	IssuedAt    time.Time `json:"issued_at"`                // for age validation
	SessionID   string    `json:"session_id"`               // bound to session
	RotationTag string    `json:"rotation_tag,omitempty"`   // anti-replay marker
}

// CSRFService generates and validates CSRF tokens.
type CSRFService struct {
	cfg CSRFConfig
}

// NewCSRFService creates a new CSRF service. Panics if serverSecret < 32 bytes.
func NewCSRFService(cfg CSRFConfig) *CSRFService {
	if len(cfg.ServerSecret) < 32 {
		panic("csrf: server secret must be >= 32 bytes")
	}
	if cfg.TokenLength < 16 {
		cfg.TokenLength = 32
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 15 * time.Minute
	}
	return &CSRFService{cfg: cfg}
}

// GenerateToken creates a new CSRF token bound to a session ID.
// Returns the token string (format: nonce.signature.expiresAt) and the CSRFToken struct.
func (s *CSRFService) GenerateToken(sessionID string) (string, *CSRFToken, error) {
	if sessionID == "" {
		return "", nil, errors.New("session id required")
	}

	// Step 1: Generate 32-byte cryptographic nonce.
	nonce := make([]byte, s.cfg.TokenLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", nil, fmt.Errorf("generate nonce: %w", err)
	}
	nonceB64 := base64.RawURLEncoding.EncodeToString(nonce)

	// Step 2: Create token metadata (issued-at + expiry).
	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.TokenTTL)

	// Step 3: Compute HMAC-SHA256 signature over (nonce || sessionID || timestamp).
	// This binds the token to the session and a time window (prevents token reuse across time).
	h := hmac.New(sha256.New, s.cfg.ServerSecret)
	h.Write([]byte(nonceB64))
	h.Write([]byte(sessionID))
	h.Write([]byte(expiresAt.Format(time.RFC3339Nano)))
	sig := hex.EncodeToString(h.Sum(nil))

	// Step 4: Format token as: base64(nonce).hex(signature).unix(expiresAt)
	// This prevents timing attacks on signature comparison (uses constant-time HMAC).
	tokenStr := fmt.Sprintf("%s.%s.%d", nonceB64, sig, expiresAt.Unix())

	tok := &CSRFToken{
		Nonce:     nonceB64,
		Signature: sig,
		ExpiresAt: expiresAt,
		IssuedAt:  now,
		SessionID: sessionID,
	}

	return tokenStr, tok, nil
}

// ValidateToken verifies a CSRF token against a session ID.
// Returns the CSRFToken struct and a new token (if RotateOnValidate=true).
// Errors returned:
//   - "token format invalid": malformed token string
//   - "token expired": exceeds MaxTokenAge
//   - "signature mismatch": HMAC validation failed (potential CSRF attack)
//   - "session mismatch": token bound to different session
func (s *CSRFService) ValidateToken(tokenStr string, sessionID string) (*CSRFToken, error) {
	if tokenStr == "" || sessionID == "" {
		return nil, errors.New("token and session id required")
	}

	// Step 1: Parse token format (nonce.signature.expiresAt).
	parts := splitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, errors.New("token format invalid: expected nonce.signature.expiresAt")
	}
	nonceB64, sig, expiresAtStr := parts[0], parts[1], parts[2]

	// Step 2: Parse expiry timestamp.
	var expiresAt time.Time
	if _, err := fmt.Sscanf(expiresAtStr, "%d", func(unixSecs int64) {
		expiresAt = time.Unix(unixSecs, 0).UTC()
	}); err != nil {
		return nil, errors.New("token format invalid: invalid timestamp")
	}

	// Step 3: Check expiry (reject if token older than TTL).
	now := time.Now().UTC()
	if now.After(expiresAt) || expiresAt.Sub(now) > s.cfg.MaxTokenAge {
		return nil, fmt.Errorf("token expired at %v", expiresAt)
	}

	// Step 4: Validate signature using constant-time comparison.
	h := hmac.New(sha256.New, s.cfg.ServerSecret)
	h.Write([]byte(nonceB64))
	h.Write([]byte(sessionID))
	h.Write([]byte(expiresAt.Format(time.RFC3339Nano)))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, errors.New("signature mismatch: potential csrf attack")
	}

	tok := &CSRFToken{
		Nonce:     nonceB64,
		Signature: sig,
		ExpiresAt: expiresAt,
		IssuedAt:  expiresAt.Add(-s.cfg.TokenTTL),
		SessionID: sessionID,
	}

	return tok, nil
}

// RequiresValidation returns true if the HTTP method requires CSRF token validation.
func RequiresValidation(method string) bool {
	switch method {
	case "POST", "PUT", "DELETE", "PATCH":
		return true
	default:
		return false
	}
}

// splitN splits string on delimiter, max n parts.
func splitN(s, sep string, n int) []string {
	parts := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := len(s)
		if j := len(s) - 1; j >= 0 {
			for k := j; k >= 0; k-- {
				if s[k:k+len(sep)] == sep {
					idx = k
					break
				}
			}
		}
		if idx < len(s) {
			parts = append(parts, s[idx+len(sep):])
			s = s[:idx]
		} else {
			parts = append(parts, s)
			break
		}
	}
	parts = append(parts, s)
	// Reverse because we split from the end.
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return parts
}
```

### 1.4 CSRF Middleware

The CSRF middleware:
- Extracts the token from **X-CSRF-Token header** (not cookie, to prevent XSS exfiltration)
- Validates the token against the session ID
- Issues a new token for the next request (rotation)
- Returns 403 Forbidden if validation fails

```go
package mid

import (
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/security"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

type ctxKey int

const (
	ctxCSRFToken ctxKey = iota + 10 // avoid collision with existing context keys
)

// CSRFTokenFrom retrieves the validated CSRF token from the context.
func CSRFTokenFrom(ctx context.Context) (*security.CSRFToken, bool) {
	tok, ok := ctx.Value(ctxCSRFToken).(*security.CSRFToken)
	return tok, ok
}

// CSRF validates CSRF tokens on state-modifying requests.
// Token is expected in X-CSRF-Token header (not cookie).
// On success, a new token is issued and attached to context for response.
func CSRF(svc *security.CSRFService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for safe methods (GET, HEAD, OPTIONS).
			if !security.RequiresValidation(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract session ID from context (set by Auth middleware).
			claims, ok := ClaimsFrom(r.Context())
			if !ok {
				// Unauthenticated requests don't have CSRF tokens.
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}

			// Extract token from request header.
			tokenStr := r.Header.Get("X-CSRF-Token")
			if tokenStr == "" {
				http.Error(w, "csrf token required", http.StatusForbidden)
				return
			}

			// Validate token against user ID (session).
			validatedToken, err := svc.ValidateToken(tokenStr, claims.UserID)
			if err != nil {
				// Log this as a potential attack vector.
				logger := LoggerFrom(r.Context())
				logger.Warn("csrf validation failed", "error", err.Error(), "user_id", claims.UserID)
				http.Error(w, "csrf token invalid or expired", http.StatusForbidden)
				return
			}

			// Issue new token for response (rotation).
			newTokenStr, newToken, err := svc.GenerateToken(claims.UserID)
			if err != nil {
				http.Error(w, "could not issue new token", http.StatusInternalServerError)
				return
			}

			// Store new token in context for response handler to set in header.
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxCSRFToken, newToken)

			// Add new token to response header before calling handler.
			w.Header().Set("X-CSRF-Token", newTokenStr)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

### 1.5 CSRF Token Issuance at Login

After successful login, issue the first CSRF token:

```go
// In handlers/users.go Login method:
func (h *Users) Login(w http.ResponseWriter, r *http.Request) {
	// ... existing validation ...

	tok, claims, err := h.Issuer.Issue(user.ID, user.Email, user.Roles)
	if err != nil {
		http.Error(w, "could not sign token", http.StatusInternalServerError)
		return
	}

	// NEW: Issue CSRF token bound to user ID
	csrfTokenStr, _, err := h.CSRFService.GenerateToken(user.ID)
	if err != nil {
		http.Error(w, "could not issue csrf token", http.StatusInternalServerError)
		return
	}

	// Set CSRF token in response header (client stores in memory, sends in X-CSRF-Token).
	w.Header().Set("X-CSRF-Token", csrfTokenStr)

	respondJSON(w, http.StatusOK, tokenResponse{
		Token:     tok,
		ExpiresAt: claims.ExpiresAt,
		User:      publicUser{ID: user.ID, Email: user.Email, Name: user.Name, Roles: user.Roles},
	})
}
```

---

## 2. HttpOnly Session Management

### 2.1 Architecture Overview

Session management uses **JWT in an HttpOnly, Secure, SameSite=Strict cookie** combined with:

- **JWT payload mutations** — add `csrf_secret` to JWT for CSRF token rotation
- **Cookie flags**:
  - `HttpOnly=true` — prevents JavaScript access (XSS mitigation)
  - `Secure=true` — HTTPS-only transmission
  - `SameSite=Strict` — blocks cross-site cookie sending
  - `Max-Age=3600` — 1-hour session lifetime
- **Session lifecycle** — create on login, refresh before expiry, destroy on logout
- **Token refresh endpoint** — extends session without re-authentication

### 2.2 JWT Payload with CSRF Secret

Extend the JWT claims to include a CSRF secret per session:

```go
// In pkg/auth/auth.go
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"time"
)

// Claims represents JWT payload with CSRF binding.
type Claims struct {
	UserID       string    `json:"sub"`                     // user id
	Email        string    `json:"email"`                   // user email
	Roles        []Role    `json:"roles"`                   // RBAC roles
	Issued       time.Time `json:"iat"`                     // issued at
	ExpiresAt    time.Time `json:"exp"`                     // expiry
	CSRFSecret   string    `json:"csrf_secret,omitempty"`   // CSRF token binding (new)
	SessionID    string    `json:"sid,omitempty"`           // session identifier (new)
	RotationTag  string    `json:"rotation_tag,omitempty"`  // replay protection (new)
}

// Issuer signs and verifies JWTs with CSRF binding.
type Issuer struct {
	secret       []byte
	issuer       string
	audience     []string
	sessionTTL   time.Duration
	csrfSvcBound *CSRFService // CSRF service reference (optional)
}

// Issue creates a new JWT with CSRF secret embedded.
// The CSRF secret is a random 32-byte value used for server-side token validation.
func (i *Issuer) Issue(userID string, email string, roles []Role) (string, *Claims, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(i.sessionTTL)

	// Generate CSRF secret (used by CSRF service for token signing).
	csrfSecret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, csrfSecret); err != nil {
		return "", nil, err
	}

	sessionID := generateSessionID() // e.g., UUID

	claims := &Claims{
		UserID:      userID,
		Email:       email,
		Roles:       roles,
		Issued:      now,
		ExpiresAt:   expiresAt,
		CSRFSecret:  base64.RawURLEncoding.EncodeToString(csrfSecret),
		SessionID:   sessionID,
		RotationTag: generateRotationTag(), // anti-replay marker
	}

	// Sign JWT (existing logic, just with enriched claims).
	// ...

	return tokenStr, claims, nil
}

// generateSessionID creates a random session ID for this JWT.
func generateSessionID() string {
	b := make([]byte, 16)
	io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateRotationTag creates an anti-replay marker.
func generateRotationTag() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}
```

### 2.3 HttpOnly Cookie Handler

Set the JWT in an HttpOnly cookie after login:

```go
// In pkg/web/handlers/users.go

import (
	"net/http"
	"time"
)

// SessionCookieConfig holds HttpOnly cookie parameters.
type SessionCookieConfig struct {
	Name     string        // default: "genie-session"
	MaxAge   time.Duration // default: 1 hour
	Secure   bool          // default: true (HTTPS-only)
	SameSite http.SameSite // default: Strict
	Path     string        // default: "/"
}

// DefaultSessionCookieConfig returns NIST SP 800-63B compliant settings.
func DefaultSessionCookieConfig() SessionCookieConfig {
	return SessionCookieConfig{
		Name:     "genie-session",
		MaxAge:   1 * time.Hour,
		Secure:   true,
		SameSite: http.SameSiteLax, // Lax allows same-site form submissions
		Path:     "/",
	}
}

// In the Login handler, set HttpOnly cookie:
func (h *Users) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	// ... authentication checks ...

	// Issue JWT with CSRF secret embedded.
	tok, claims, err := h.Issuer.Issue(user.ID, user.Email, user.Roles)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "token issue failed")
		return
	}

	// Set HttpOnly cookie for session persistence.
	cookieCfg := DefaultSessionCookieConfig()
	cookie := &http.Cookie{
		Name:     cookieCfg.Name,
		Value:    tok,
		MaxAge:   int(cookieCfg.MaxAge.Seconds()),
		Path:     cookieCfg.Path,
		HttpOnly: true,          // Prevent JavaScript access (XSS mitigation)
		Secure:   cookieCfg.Secure,       // HTTPS-only
		SameSite: cookieCfg.SameSite,     // CSRF mitigation
	}
	http.SetCookie(w, cookie)

	// Also issue CSRF token in response header.
	csrfTokenStr, _, err := h.CSRFService.GenerateToken(user.ID)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "csrf token issue failed")
		return
	}
	w.Header().Set("X-CSRF-Token", csrfTokenStr)

	// Return user info (NOT the JWT, already in cookie).
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "login successful",
		"user": publicUser{
			ID:    user.ID,
			Email: user.Email,
			Name:  user.Name,
			Roles: user.Roles,
		},
	})
}
```

### 2.4 Session Refresh Endpoint

Refresh the JWT before expiry (extends session):

```go
// In handlers/users.go
func (h *Users) RefreshSession(w http.ResponseWriter, r *http.Request) {
	// Extract session from cookie (already validated by Auth middleware).
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	// Re-issue JWT with extended expiry.
	newTok, newClaims, err := h.Issuer.Issue(claims.UserID, claims.Email, claims.Roles)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "could not refresh session")
		return
	}

	// Update cookie.
	cookieCfg := DefaultSessionCookieConfig()
	cookie := &http.Cookie{
		Name:     cookieCfg.Name,
		Value:    newTok,
		MaxAge:   int(cookieCfg.MaxAge.Seconds()),
		Path:     cookieCfg.Path,
		HttpOnly: true,
		Secure:   cookieCfg.Secure,
		SameSite: cookieCfg.SameSite,
	}
	http.SetCookie(w, cookie)

	// Issue new CSRF token.
	csrfTokenStr, _, _ := h.CSRFService.GenerateToken(claims.UserID)
	w.Header().Set("X-CSRF-Token", csrfTokenStr)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "session refreshed",
		"expires_at": newClaims.ExpiresAt.Unix(),
	})
}
```

### 2.5 Logout (Cookie Destruction)

Destroy the session cookie:

```go
// In handlers/users.go
func (h *Users) Logout(w http.ResponseWriter, r *http.Request) {
	// Verify authenticated.
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		writeErrorJSON(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	// Clear session cookie (set MaxAge to -1).
	cookie := &http.Cookie{
		Name:     "genie-session",
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLax,
	}
	http.SetCookie(w, cookie)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "logged out successfully",
	})
}
```

### 2.6 Updated Auth Middleware

Accept JWT from cookie **or** Authorization header (backwards compatibility):

```go
// In pkg/web/mid/auth.go
import (
	"net/http"
	"strings"
)

// Auth verifies JWT from cookie (preferred) or Authorization header.
func Auth(issuer *auth.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tok string

			// Step 1: Try to extract from HttpOnly cookie (preferred).
			cookie, err := r.Cookie("genie-session")
			if err == nil {
				tok = cookie.Value
			} else {
				// Step 2: Fall back to Authorization header (backward compat).
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					tok = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tok == "" {
				http.Error(w, "missing authentication", http.StatusUnauthorized)
				return
			}

			// Verify JWT.
			claims, err := issuer.Verify(tok)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}
```

---

## 3. Security Headers Middleware

### 3.1 Architecture Overview

Security headers prevent:

- **XSS attacks** — Content-Security-Policy
- **Clickjacking** — X-Frame-Options
- **MIME type sniffing** — X-Content-Type-Options
- **Referrer leakage** — Referrer-Policy
- **Insecure redirects** — Strict-Transport-Security

### 3.2 Security Headers Service

```go
// In pkg/security/headers.go
package security

import (
	"net/http"
)

// SecurityHeadersConfig holds CSP and frame protection settings.
type SecurityHeadersConfig struct {
	// Content-Security-Policy directives.
	CSPDefaultSrc   []string // typically: self
	CSPScriptSrc    []string // 'self' 'unsafe-inline' is dangerous
	CSPStyleSrc     []string // 'self' 'unsafe-inline'
	CSPFontSrc      []string // 'self' data:
	CSPImgSrc       []string // 'self' https: data:
	CSPConnectSrc   []string // 'self' for XHR/WS
	CSPFormAction   []string // 'self' (prevent form hijacking)
	CSPFrameAncestors []string // 'none' (prevent clickjacking)

	// Frame protection.
	FrameOptions string // "DENY", "SAMEORIGIN", "ALLOW-FROM uri"

	// MIME type sniffing protection.
	NoSniff bool

	// Referrer policy.
	ReferrerPolicy string // "no-referrer", "same-origin", "strict-origin"

	// HSTS (only enable in production with HTTPS).
	StrictTransportSecurity string // "max-age=31536000; includeSubDomains"

	// Permissions policy (Feature-Policy successor).
	PermissionsPolicy string
}

// DefaultSecurityHeadersConfig returns production-hardened settings.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		// CSP: self-only, no inline scripts/styles, no third-party resources.
		CSPDefaultSrc:     []string{"'self'"},
		CSPScriptSrc:      []string{"'self'"},    // No unsafe-inline
		CSPStyleSrc:       []string{"'self'"},    // No unsafe-inline
		CSPFontSrc:        []string{"'self'", "data:"},
		CSPImgSrc:         []string{"'self'", "https:", "data:"},
		CSPConnectSrc:     []string{"'self'"},    // XHR/WS to same origin
		CSPFormAction:     []string{"'self'"},    // Forms to same origin
		CSPFrameAncestors: []string{"'none'"},    // Prevent clickjacking

		FrameOptions:                "DENY",
		NoSniff:                     true,
		ReferrerPolicy:              "strict-origin-when-cross-origin",
		StrictTransportSecurity:     "max-age=31536000; includeSubDomains; preload",
		PermissionsPolicy:           "geolocation=(), microphone=(), camera=()",
	}
}

// SecurityHeaders middleware applies security headers to all responses.
func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Build CSP directive.
			cspDirectives := []string{
				"default-src " + join(cfg.CSPDefaultSrc),
				"script-src " + join(cfg.CSPScriptSrc),
				"style-src " + join(cfg.CSPStyleSrc),
				"font-src " + join(cfg.CSPFontSrc),
				"img-src " + join(cfg.CSPImgSrc),
				"connect-src " + join(cfg.CSPConnectSrc),
				"form-action " + join(cfg.CSPFormAction),
				"frame-ancestors " + join(cfg.CSPFrameAncestors),
				"base-uri 'self'",           // Prevent <base href> hijacking
				"object-src 'none'",         // Block plugins
				"block-all-mixed-content",   // HTTPS-only
				"require-sri-for script style", // Require Subresource Integrity
			}

			w.Header().Set("Content-Security-Policy", join(cspDirectives, "; "))

			// Clickjacking protection.
			if cfg.FrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.FrameOptions)
			}

			// MIME type sniffing protection.
			if cfg.NoSniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// Referrer policy.
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			// HSTS (only in production/HTTPS).
			if cfg.StrictTransportSecurity != "" && isHTTPS(r) {
				w.Header().Set("Strict-Transport-Security", cfg.StrictTransportSecurity)
			}

			// Permissions policy.
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}

			// Additional hardening.
			w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
			w.Header().Set("X-XSS-Protection", "0")  // Disable legacy XSS filters (modern CSP sufficient)

			next.ServeHTTP(w, r)
		})
	}
}

func join(strs []string, sep ...string) string {
	if len(sep) == 0 {
		sep = []string{" "}
	}
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep[0]
		}
		result += s
	}
	return result
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
```

### 3.3 Integration into Router

```go
// In pkg/web/router.go
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Apply security headers as the outermost middleware.
	r.Use(security.SecurityHeaders(security.DefaultSecurityHeadersConfig()))

	// Then apply other middleware...
	r.Use(mid.RequestID)
	r.Use(mid.Recovery(d.Logger))
	r.Use(mid.AccessLog(d.Logger))

	// ... rest of router ...
}
```

---

## 4. Error Handling Standards

### 4.1 Architecture Overview

Error responses must:

- **Be generic** — Hide implementation details (no stack traces, file paths, SQL errors)
- **Include context** — Trace IDs for debugging, but opaque to clients
- **Distinguish layers** — Client errors (4xx), server errors (5xx)
- **Log internally** — Full details to structured logs, minimal to clients
- **Never expose** — Database schemas, API keys, internal IPs, algorithm names

### 4.2 Error Response Types

```go
// In pkg/web/errors.go
package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// ErrorResponse is the generic JSON error response sent to clients.
// All sensitive details are removed; only safe information is included.
type ErrorResponse struct {
	Status  int    `json:"status"`               // HTTP status code
	Message string `json:"message"`              // Generic, user-safe message
	TraceID string `json:"trace_id,omitempty"`   // For support to correlate logs
	Path    string `json:"path,omitempty"`       // Request path (for debugging)
}

// InternalError represents an internal error with full context (NOT sent to client).
type InternalError struct {
	Status     int
	Message    string
	Details    string // Internal context (SQL error, file path, etc.)
	Err        error  // Original error
	TraceID    string
	StatusText string
}

// ErrorSink encapsulates error handling: logging internally, responding generically.
type ErrorSink struct {
	Logger Logger // structured logger
}

// Logger is the logging interface expected by ErrorSink.
type Logger interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
}

// Handle logs the full error internally and sends a generic response to the client.
// The client sees only status, generic message, and trace ID.
// The server log contains full details: underlying error, SQL query, file path, etc.
func (es *ErrorSink) Handle(w http.ResponseWriter, r *http.Request, status int, userMsg string, internalErr error) {
	traceID := extractTraceID(r)

	// Log full context internally (accessible only to ops/support).
	es.Logger.Error("http error",
		"status", status,
		"method", r.Method,
		"path", r.URL.Path,
		"trace_id", traceID,
		"message", userMsg,
		"error", internalErr.Error(),
		"user_agent", r.Header.Get("User-Agent"),
		"client_ip", clientIP(r),
	)

	// Send generic response to client (no internal details).
	resp := ErrorResponse{
		Status:  status,
		Message: userMsg,
		TraceID: traceID,
		Path:    r.URL.Path,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// Forbidden logs and responds with 403 Forbidden.
func (es *ErrorSink) Forbidden(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusForbidden, "access denied", fmt.Errorf("forbidden: %s", reason))
}

// Unauthorized logs and responds with 401 Unauthorized.
func (es *ErrorSink) Unauthorized(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusUnauthorized, "authentication required", fmt.Errorf("unauthorized: %s", reason))
}

// BadRequest logs and responds with 400 Bad Request.
func (es *ErrorSink) BadRequest(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusBadRequest, "invalid request", fmt.Errorf("bad request: %s", reason))
}

// NotFound logs and responds with 404 Not Found.
func (es *ErrorSink) NotFound(w http.ResponseWriter, r *http.Request) {
	es.Handle(w, r, http.StatusNotFound, "resource not found", fmt.Errorf("not found: %s", r.URL.Path))
}

// ServerError logs and responds with 500 Internal Server Error.
// Never exposes the underlying error to the client.
func (es *ErrorSink) ServerError(w http.ResponseWriter, r *http.Request, err error) {
	es.Handle(w, r, http.StatusInternalServerError, "an error occurred processing your request", err)
}

// ConflictError logs and responds with 409 Conflict (e.g., duplicate resource).
func (es *ErrorSink) ConflictError(w http.ResponseWriter, r *http.Request, reason string) {
	es.Handle(w, r, http.StatusConflict, "resource already exists", fmt.Errorf("conflict: %s", reason))
}

// ValidationError logs and responds with 422 Unprocessable Entity.
func (es *ErrorSink) ValidationError(w http.ResponseWriter, r *http.Request, reason string) {
	resp := ErrorResponse{
		Status:  http.StatusUnprocessableEntity,
		Message: "validation failed: " + reason,
		TraceID: extractTraceID(r),
		Path:    r.URL.Path,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	json.NewEncoder(w).Encode(resp)
}

// ClientError returns true if the status code is in the 4xx range (client fault).
func ClientError(status int) bool {
	return status >= 400 && status < 500
}

// ServerErr returns true if the status code is in the 5xx range (server fault).
func ServerErr(status int) bool {
	return status >= 500 && status < 600
}

// extractTraceID retrieves the trace ID from the request context.
// Falls back to request ID if trace ID unavailable.
func extractTraceID(r *http.Request) string {
	// Try X-Trace-ID header (set by OpenTelemetry).
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		return traceID
	}
	// Fall back to X-Request-ID (set by RequestID middleware).
	return mid.RequestIDFrom(r.Context())
}

// clientIP extracts client IP from request (accounting for proxies).
func clientIP(r *http.Request) string {
	// Check X-Forwarded-For (set by load balancer).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Fall back to remote address.
	return r.RemoteAddr
}

// Sanitize removes sensitive fields from error messages.
// Examples: database URLs, API keys, file paths, SQL queries.
func Sanitize(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	// Remove common sensitive patterns.
	sanitizers := []struct {
		pattern string
		replace string
	}{
		{"postgres://[^@]*@[^/]*", "postgres://***"},        // DB URL
		{"mysql://[^@]*@[^/]*", "mysql://***"},              // DB URL
		{"/var/www/.*", "/***"},                             // File paths
		{`"password":"[^"]*"`, `"password":"***"`},          // JSON passwords
		{`password=\w+`, "password=***"},                    // URL params
		{"GENIE_[A-Z_]*_.*", "GENIE_***"},                   // Env vars
		{"-H.*api.key.*", "-H ***"},                         // API keys
	}

	for _, s := range sanitizers {
		// Simple pattern matching (regex would be better).
		if strings.Contains(msg, s.pattern) {
			msg = strings.ReplaceAll(msg, s.pattern, s.replace)
		}
	}

	return msg
}
```

### 4.3 Error Handling in Handlers

```go
// Example: Using ErrorSink in a handler.
// In handlers/users.go

type Users struct {
	Repo   postgres.UserRepo
	Issuer *auth.Issuer
	ErrSink *web.ErrorSink
}

func (h *Users) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Invalid JSON — 400 Bad Request, generic response to client.
		h.ErrSink.BadRequest(w, r, "invalid json request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	user, err := h.Repo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		// Database error — 500 Internal Server Error, hide DB details from client.
		// Full error logged internally with trace ID.
		h.ErrSink.ServerError(w, r, fmt.Errorf("user lookup failed: %w", err))
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		// Password mismatch — 401 Unauthorized (generic, don't reveal user doesn't exist).
		// This prevents user enumeration attacks.
		h.ErrSink.Unauthorized(w, r, "invalid credentials")
		return
	}

	tok, claims, err := h.Issuer.Issue(user.ID, user.Email, user.Roles)
	if err != nil {
		h.ErrSink.ServerError(w, r, fmt.Errorf("token issuance failed: %w", err))
		return
	}

	// Success — set session cookie and respond.
	// ... (existing cookie/CSRF logic) ...
}
```

### 4.4 Sensitive Data Patterns

**NEVER include these in client responses:**

- SQL query text, table schemas
- File paths (`/var/www/...`, `/home/user/...`)
- Internal IP addresses, hostnames
- API keys, secrets, tokens
- Stack traces, panic details
- Environment variable names/values
- Database connection strings
- Algorithm names (e.g., "RSA-2048", "AES-256")
- User lists, internal user identifiers
- System information (`uname`, kernel version, etc.)

**OK to include:**

- HTTP status codes and standard messages
- Request path (non-sensitive)
- Trace ID (opaque UUID for correlation)
- Generic error messages ("validation failed", "not found")
- Timestamps (request received, response sent)

---

## 5. Integration Checklist

### 5.1 Router Wiring

```go
// In cmd/api/main.go, wrap NewRouter call:
func run() error {
	// ... existing setup ...

	// Load CSRF service.
	csrfCfg := security.DefaultCSRFConfig()
	csrfCfg.ServerSecret = []byte(mustEnv("GENIE_CSRF_SECRET"))
	csrfSvc := security.NewCSRFService(csrfCfg)

	// Build handler dependencies.
	deps := web.Deps{
		Issuer:     issuer,
		Logger:     logger,
		Users:      &handlers.Users{Repo: userRepo, Issuer: issuer, CSRFService: csrfSvc},
		CSRFSvc:    csrfSvc,
		// ... other handlers ...
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           web.NewRouter(deps),
		ReadHeaderTimeout: 5 * time.Second,
		// Add security-hardened timeouts.
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// ... rest of startup ...
}
```

### 5.2 Router Middleware Chain

```go
// In pkg/web/router.go
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Layer 1: Security headers (outermost).
	r.Use(security.SecurityHeaders(security.DefaultSecurityHeadersConfig()))

	// Layer 2: Request context (trace ID, logger).
	r.Use(mid.RequestID)
	r.Use(mid.Recovery(d.Logger))
	r.Use(mid.AccessLog(d.Logger))
	r.Use(mid.Trace("github.com/c2siorg/genie/pkg/web"))

	// Layer 3: Rate limiting (protect against DoS).
	if d.RateLimit != nil {
		r.Use(d.RateLimit.Middleware)
	}

	// Public routes — no auth, no CSRF.
	r.Group(func(r chi.Router) {
		r.Get("/healthz", d.Health.Live)
		r.Get("/readyz", d.Health.Readiness)
		r.Post("/v1/users", d.Users.Signup)
		r.Post("/v1/users/login", d.Users.Login)
	})

	// Authenticated routes — CSRF protected.
	r.Group(func(r chi.Router) {
		// Layer 4: Auth (validates JWT from cookie or header).
		r.Use(mid.Auth(d.Issuer))

		// Layer 5: CSRF (validates X-CSRF-Token header on POST/PUT/DELETE).
		r.Use(mid.CSRF(d.CSRFSvc))

		// Routes requiring authentication + CSRF.
		r.Route("/v1", func(r chi.Router) {
			r.Get("/users/me", d.Users.Me)
			r.Post("/accounts", d.Accounts.Create)         // CSRF protected
			r.Post("/payment/initiate", d.Payment.InitiatePayment) // CSRF protected
			r.Post("/commerce/order", d.Commerce.CreateOrder)      // CSRF protected
			// ... more routes ...
		})
	})

	return r
}
```

### 5.3 Environment Variables

Add to `.env` / deployment config:

```bash
# CSRF token configuration
GENIE_CSRF_SECRET=<32-byte hex string>  # Must be >= 32 bytes, random

# Session cookie configuration
GENIE_SESSION_TTL=3600              # 1 hour in seconds
GENIE_SESSION_COOKIE_NAME=genie-session
GENIE_SESSION_SECURE=true           # HTTPS-only
GENIE_SESSION_SAMESITE=Lax          # CSRF mitigation

# Security headers (optional, use defaults if not set)
GENIE_CSP_SCRIPT_SRC='self'         # Customize CSP if needed
GENIE_HSTS_MAX_AGE=31536000         # 1 year

# Error handling (optional)
GENIE_LOG_LEVEL=info                # Don't use debug in production
GENIE_TRACE_SAMPLE_RATE=0.1         # 10% sampling to reduce noise
```

### 5.4 Testing

```go
// In pkg/security/csrf_test.go
package security

import (
	"testing"
	"time"
)

func TestCSRFService_GenerateAndValidate(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly!") // 32 bytes
	svc := NewCSRFService(cfg)

	// Generate token for session.
	tokenStr, tok, err := svc.GenerateToken("user123")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if tok == nil || tok.Nonce == "" {
		t.Fatal("token struct invalid")
	}

	// Validate token — should succeed.
	validatedTok, err := svc.ValidateToken(tokenStr, "user123")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if validatedTok.SessionID != "user123" {
		t.Errorf("session id mismatch")
	}

	// Validate with wrong session — should fail.
	_, err = svc.ValidateToken(tokenStr, "user456")
	if err == nil {
		t.Fatal("validate should reject token for different session")
	}

	// Validate expired token — should fail.
	time.Sleep(cfg.TokenTTL + 1*time.Second)
	_, err = svc.ValidateToken(tokenStr, "user123")
	if err == nil {
		t.Fatal("validate should reject expired token")
	}
}

func TestCSRFService_TokenRotation(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly!!")
	svc := NewCSRFService(cfg)

	// Generate token 1.
	tokenStr1, _, _ := svc.GenerateToken("user123")

	// Validate token 1 and generate token 2.
	_, _ = svc.ValidateToken(tokenStr1, "user123")
	tokenStr2, _, _ := svc.GenerateToken("user123")

	// Token 1 and 2 should be different (rotation).
	if tokenStr1 == tokenStr2 {
		t.Error("tokens should be different after rotation")
	}

	// Both should still be valid (old tokens don't expire immediately).
	// This allows for clock skew tolerance.
}
```

---

## 6. Security Checklist

### Before Deploying to Production

- [ ] **CSRF Service**: ServerSecret is 32+ bytes, loaded from secure env var (NOT in code)
- [ ] **Session Cookies**: HttpOnly=true, Secure=true, SameSite=Strict or Lax
- [ ] **JWT Payload**: Includes `csrf_secret` and `session_id` fields
- [ ] **Auth Middleware**: Accepts JWT from cookie (preferred) AND header (backward compat)
- [ ] **CSRF Middleware**: Validates X-CSRF-Token header on POST/PUT/DELETE
- [ ] **Security Headers**: CSP, X-Frame-Options, X-Content-Type-Options all set
- [ ] **Error Responses**: No SQL errors, file paths, stack traces in client responses
- [ ] **Logging**: Full error details in structured logs (server-side only)
- [ ] **Trace IDs**: All errors include trace ID for support correlation
- [ ] **HTTPS Enforced**: Secure flag on cookies, HSTS headers active
- [ ] **Rate Limiting**: Enabled on login, payment, settlement endpoints
- [ ] **Timeout Hardening**: ReadHeaderTimeout, WriteTimeout, IdleTimeout all set

### Testing in Staging

- [ ] Run `go test ./pkg/security -v` — all tests pass
- [ ] Run `go test ./pkg/web/mid -v` — middleware tests pass
- [ ] Manual test: POST without CSRF token → 403 Forbidden
- [ ] Manual test: CSRF token from user A on request for user B → 403 Forbidden
- [ ] Manual test: Invalid JSON → 400 Bad Request (no stack trace in response)
- [ ] Manual test: Database error → 500 Internal Server Error (no SQL in response)
- [ ] Check logs: Full error context logged internally
- [ ] Check response headers: CSP, X-Frame-Options, etc. all present

---

## 7. Threat Model & Mitigations

| Threat | CVSS | Mitigation |
|--------|------|-----------|
| CSRF (cross-site form hijacking) | 8.1 | CSRF tokens + SameSite cookies |
| XSS (script injection) | 9.3 | HttpOnly cookies + CSP + no inline scripts |
| Clickjacking (UI redressing) | 5.4 | X-Frame-Options: DENY + CSP frame-ancestors |
| MIME sniffing (content type confusion) | 4.3 | X-Content-Type-Options: nosniff |
| Information disclosure (error details leak) | 5.3 | Generic error responses + internal logging |
| Session fixation | 6.2 | Rotate CSRF tokens, bind to user ID + session ID |
| Timing attacks (signature comparison) | 7.4 | Use hmac.Equal() for constant-time comparison |
| Referrer leakage | 3.1 | Referrer-Policy: strict-origin-when-cross-origin |

---

## 8. References

- **NIST SP 800-63B**: Authentication and Lifecycle Management
- **OWASP Top 10 2021**:
  - A01: Broken Access Control
  - A04: Insecure Design
  - A05: Security Misconfiguration
- **OWASP CSRF Prevention Cheat Sheet**
- **OWASP Session Management Cheat Sheet**
- **OWASP Secure Headers Project**
- **RBI FREE-AI**: Sutras 7 (Safety, Resilience, Sustainability)
- **RFC 6265**: HTTP State Management Mechanism (Cookies)
- **RFC 7234**: HTTP Caching

---

## 9. Appendix: Configuration Examples

### 9.1 Docker Environment

```dockerfile
# In Dockerfile or docker-compose.yml
ENV GENIE_CSRF_SECRET=<generate with: openssl rand -hex 32>
ENV GENIE_SESSION_TTL=3600
ENV GENIE_SESSION_SECURE=true
ENV GENIE_JWT_SECRET=<keep existing>
ENV GENIE_KEK_BASE64=<keep existing>
```

### 9.2 Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: genie-security
type: Opaque
stringData:
  csrf-secret: $(openssl rand -hex 32)
  jwt-secret: "existing-jwt-secret"
  kek-base64: "existing-kek"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: genie-security-config
data:
  session-ttl: "3600"
  session-secure: "true"
  session-samesite: "Lax"
  csp-default-src: "'self'"
  csp-script-src: "'self'"
```

### 9.3 Go Application Setup

```go
// In cmd/api/main.go
func run() error {
	// ... existing code ...

	// Load CSRF configuration from environment.
	csrfSecret, err := hex.DecodeString(os.Getenv("GENIE_CSRF_SECRET"))
	if err != nil {
		return fmt.Errorf("GENIE_CSRF_SECRET must be hex-encoded: %w", err)
	}

	csrfCfg := security.DefaultCSRFConfig()
	csrfCfg.ServerSecret = csrfSecret
	csrfSvc := security.NewCSRFService(csrfCfg)

	// Wire handlers with error sink.
	errSink := &web.ErrorSink{Logger: logger}

	deps := web.Deps{
		Issuer:     issuer,
		Logger:     logger,
		CSRFSvc:    csrfSvc,
		ErrorSink:  errSink,
		Users:      &handlers.Users{Repo: userRepo, Issuer: issuer, CSRFService: csrfSvc, ErrSink: errSink},
		// ... other handlers ...
	}

	// Build router with all middleware.
	handler := web.NewRouter(deps)

	// Start server with hardened timeouts.
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// ... rest of startup ...
}
```

---

**Document Status**: Final (Ready for Implementation)  
**Last Updated**: June 4, 2026  
**Phase**: 2 (e-Rupee Commerce Integration)  
**License**: MIT  
**Governance**: RBI FREE-AI Aligned
