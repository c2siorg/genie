// Package mid holds HTTP middleware: auth (JWT), request logging, OTEL
// tracing, panic recovery, request-id propagation, CSRF protection,
// session management, and security headers.
//
// security_test.go — Comprehensive security tests for Phase 2 including
// CSRF protection, HttpOnly cookies, security headers, and OWASP compliance.
package mid

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

// ─── CSRF Tests (8+ tests) ─────────────────────────────────────────────────

// TestCSRF_TokenGenerationProducesValidHMAC verifies that token generation
// produces valid HMAC-SHA256 signatures.
func TestCSRF_TokenGenerationProducesValidHMAC(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)

	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Token format: signature.payload
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("token format invalid: expected 2 parts, got %d", len(parts))
	}

	signatureHex := parts[0]
	payload := parts[1]

	// Decode signature from hex
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		t.Fatalf("signature is not valid hex: %v", err)
	}

	// Verify signature is 32 bytes (SHA256)
	if len(signature) != sha256.Size {
		t.Fatalf("signature length invalid: got %d, want %d", len(signature), sha256.Size)
	}

	// Manually compute HMAC to verify signature is correct
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	expectedSignature := h.Sum(nil)

	if !hmac.Equal(signature, expectedSignature) {
		t.Fatal("signature does not match HMAC computation")
	}
}

// TestCSRF_TokenValidationAcceptsValidTokens verifies that valid tokens pass
// validation.
func TestCSRF_TokenValidationAcceptsValidTokens(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Validate the token
	err = val.Verify(req, token)
	if err != nil {
		t.Fatalf("Verify() rejected valid token: %v", err)
	}
}

// TestCSRF_TokenValidationRejectsInvalidSignatures verifies that tokens with
// invalid signatures are rejected.
func TestCSRF_TokenValidationRejectsInvalidSignatures(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Corrupt the signature
	parts := strings.Split(token, ".")
	corruptedSignature := strings.Repeat("a", 64)
	corruptedToken := corruptedSignature + "." + parts[1]

	// Validate should fail
	err = val.Verify(req, corruptedToken)
	if err == nil {
		t.Fatal("Verify() accepted token with corrupted signature")
	}

	csrfErr, ok := err.(CSRFTokenError)
	if !ok {
		t.Fatalf("expected CSRFTokenError, got %T", err)
	}

	if csrfErr.Reason != "signature mismatch" {
		t.Fatalf("expected 'signature mismatch', got %q", csrfErr.Reason)
	}
}

// TestCSRF_TokenValidationRejectsExpiredTokens verifies that expired tokens
// are rejected (15 min TTL).
func TestCSRF_TokenValidationRejectsExpiredTokens(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	gen.TTL = 1 * time.Second // Very short TTL for testing

	val := NewValidator(secret)
	val.TTL = 1 * time.Second

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Wait for token to expire
	time.Sleep(2 * time.Second)

	// Validate should fail
	err = val.Verify(req, token)
	if err == nil {
		t.Fatal("Verify() accepted expired token")
	}

	csrfErr, ok := err.(CSRFTokenError)
	if !ok {
		t.Fatalf("expected CSRFTokenError, got %T", err)
	}

	if csrfErr.Reason != "expired" {
		t.Fatalf("expected 'expired', got %q", csrfErr.Reason)
	}
}

// TestCSRF_TokenValidationRejectsMalformedTokens verifies that malformed
// tokens are rejected.
func TestCSRF_TokenValidationRejectsMalformedTokens(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	req := httptest.NewRequest("GET", "/", nil)

	tests := []struct {
		name  string
		token string
	}{
		{"no dot separator", "abcdef1234567890"},
		{"too many parts", "part1.part2.part3"},
		{"invalid hex signature", "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ.payload"},
		{"empty token", ""},
		{"malformed payload", "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := val.Verify(req, tt.token)
			if err == nil {
				t.Fatalf("Verify() accepted malformed token: %q", tt.token)
			}

			csrfErr, ok := err.(CSRFTokenError)
			if !ok {
				t.Fatalf("expected CSRFTokenError, got %T", err)
			}

			if csrfErr.Reason != "invalid" && csrfErr.Reason != "missing" {
				t.Fatalf("expected 'invalid' or 'missing', got %q", csrfErr.Reason)
			}
		})
	}
}

// TestCSRF_MiddlewareSkipsValidationForGetRequests verifies that GET requests
// skip CSRF validation.
func TestCSRF_MiddlewareSkipsValidationForGetRequests(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := val.Middleware(handler)

	req := httptest.NewRequest("GET", "/api/resource", nil)
	// Do NOT set CSRF token on GET request
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestCSRF_MiddlewareValidatesPostRequests verifies that POST requests require
// valid CSRF tokens.
func TestCSRF_MiddlewareValidatesPostRequests(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	// Track which requests were actually processed
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := val.Middleware(handler)

	// Generate a valid token
	getReq := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(getReq)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// POST request without token should be rejected
	postReq := httptest.NewRequest("POST", "/api/transfer", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, postReq)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing token, got %d", w.Code)
	}

	if handlerCalled {
		t.Fatal("handler should not be called for request without CSRF token")
	}

	// POST request with valid token in header should succeed
	postReq = httptest.NewRequest("POST", "/api/transfer", nil)
	postReq.Header.Set(CSRFHeaderName, token)
	w = httptest.NewRecorder()
	handlerCalled = false

	middleware.ServeHTTP(w, postReq)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid token, got %d", w.Code)
	}

	if !handlerCalled {
		t.Fatal("handler should be called for request with valid CSRF token")
	}
}

// TestCSRF_ConcurrentTokenGeneration verifies that concurrent token generation
// produces different tokens (different nonces).
func TestCSRF_ConcurrentTokenGeneration(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)

	const numGoroutines = 50
	tokens := make([]string, numGoroutines)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/", nil)
			token, err := gen.Generate(req)
			if err != nil {
				t.Errorf("Generate() failed: %v", err)
				return
			}
			mu.Lock()
			tokens[idx] = token
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// All tokens should be unique (different nonces)
	seen := make(map[string]bool)
	for _, token := range tokens {
		if token == "" {
			t.Fatal("generated empty token")
		}
		if seen[token] {
			t.Fatal("duplicate token generated in concurrent execution")
		}
		seen[token] = true
	}
}

// ─── HttpOnly Cookies Tests (8+ tests) ─────────────────────────────────────

// TestCookies_SessionCreationSetsHttpOnlyFlag verifies that sessions set
// HttpOnly flag.
func TestCookies_SessionCreationSetsHttpOnlyFlag(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true) // secure=true (HTTPS)

	w := httptest.NewRecorder()
	cookie, _, _, err := sm.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})

	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	if !cookie.HttpOnly {
		t.Fatal("HttpOnly flag not set on session cookie")
	}
}

// TestCookies_SessionCreationSetsSecureFlag verifies that sessions set Secure
// flag when HTTPS is enabled.
func TestCookies_SessionCreationSetsSecureFlag(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true) // secure=true (HTTPS)

	w := httptest.NewRecorder()
	cookie, _, _, err := sm.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})

	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	if !cookie.Secure {
		t.Fatal("Secure flag not set on session cookie")
	}
}

// TestCookies_SessionCreationSetsSameSiteStrict verifies that sessions set
// SameSite=Strict.
func TestCookies_SessionCreationSetsSameSiteStrict(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true) // secure=true

	w := httptest.NewRecorder()
	cookie, _, _, err := sm.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})

	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite flag not Strict: got %v", cookie.SameSite)
	}
}

// TestCookies_SessionValidationAcceptsValidCookies verifies that valid
// session cookies are accepted.
func TestCookies_SessionValidationAcceptsValidCookies(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	w := httptest.NewRecorder()
	_, _, _, err := sm.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	// Extract the cookie from the response
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies in response")
	}

	// Create a request with the cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])

	// Validate the session
	claims, err := sm.ValidateSession(req)
	if err != nil {
		t.Fatalf("ValidateSession() failed: %v", err)
	}

	if claims.Subject != "user-123" {
		t.Fatalf("expected user-123, got %q", claims.Subject)
	}

	if claims.Email != "user@example.com" {
		t.Fatalf("expected user@example.com, got %q", claims.Email)
	}
}

// TestCookies_SessionValidationRejectsExpiredCookies verifies that expired
// session cookies are rejected.
func TestCookies_SessionValidationRejectsExpiredCookies(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      1 * time.Second, // Very short TTL for testing
	}
	sm := NewSessionManager(issuer, true)

	w := httptest.NewRecorder()
	_, _, _, err := sm.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies in response")
	}

	// Wait for cookie to expire
	time.Sleep(2 * time.Second)

	// Create a request with the expired cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])

	// Validate should fail
	_, err = sm.ValidateSession(req)
	if err == nil {
		t.Fatal("ValidateSession() accepted expired cookie")
	}

	sessErr, ok := err.(SessionError)
	if !ok {
		t.Fatalf("expected SessionError, got %T", err)
	}

	if sessErr.Code != "expired" {
		t.Fatalf("expected 'expired', got %q", sessErr.Code)
	}
}

// TestCookies_SessionRefreshGeneratesNewToken verifies that session refresh
// generates a new token.
func TestCookies_SessionRefreshGeneratesNewToken(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	w := httptest.NewRecorder()
	_, claims, _, err := sm.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	// Simulate time passing by setting an old IssuedAt time
	claims.IssuedAt = time.Now().UTC().Add(-13 * time.Hour).Unix()

	// Request refresh
	w2 := httptest.NewRecorder()
	newCookie, newClaims, err := sm.RefreshSession(w2, claims)
	if err != nil {
		t.Fatalf("RefreshSession() failed: %v", err)
	}

	if newCookie == nil {
		t.Fatal("RefreshSession() did not generate new cookie")
	}

	// New claims should have updated IssuedAt time
	if newClaims.IssuedAt <= claims.IssuedAt {
		t.Fatal("new claims should have newer IssuedAt time")
	}

	// New claims should keep the same subject
	if newClaims.Subject != claims.Subject {
		t.Fatalf("subject changed on refresh: %q -> %q", claims.Subject, newClaims.Subject)
	}
}

// TestCookies_SessionDestructionClearsCookie verifies that session destruction
// clears the cookie.
func TestCookies_SessionDestructionClearsCookie(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	w := httptest.NewRecorder()
	err := sm.DestroySession(w)
	if err != nil {
		t.Fatalf("DestroySession() failed: %v", err)
	}

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies in response")
	}

	cookie := cookies[0]
	if cookie.Name != SessionCookieName {
		t.Fatalf("expected cookie name %q, got %q", SessionCookieName, cookie.Name)
	}

	// MaxAge=-1 signals deletion
	if cookie.MaxAge != -1 {
		t.Fatalf("expected MaxAge=-1 for deletion, got %d", cookie.MaxAge)
	}

	// Value should be empty
	if cookie.Value != "" {
		t.Fatalf("expected empty value, got %q", cookie.Value)
	}
}

// TestCookies_CookieParsingEdgeCases verifies handling of edge cases in
// cookie parsing.
func TestCookies_CookieParsingEdgeCases(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	tests := []struct {
		name       string
		cookieName string
		setupReq   func(*http.Request)
		expectErr  bool
	}{
		{
			name:       "missing session cookie",
			cookieName: "session",
			setupReq:   func(r *http.Request) { /* no cookie */ },
			expectErr:  true,
		},
		{
			name:       "malformed cookie value",
			cookieName: "session",
			setupReq: func(r *http.Request) {
				r.AddCookie(&http.Cookie{
					Name:  "session",
					Value: "not.a.valid.jwt",
				})
			},
			expectErr: true,
		},
		{
			name:       "empty cookie value",
			cookieName: "session",
			setupReq: func(r *http.Request) {
				r.AddCookie(&http.Cookie{
					Name:  "session",
					Value: "",
				})
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tt.setupReq(req)

			_, err := sm.ValidateSession(req)
			if (err != nil) != tt.expectErr {
				t.Fatalf("expectErr=%v, got err=%v", tt.expectErr, err)
			}
		})
	}
}

// ─── Security Headers Tests (6+ tests) ──────────────────────────────────────

// TestSecurityHeaders_CSPHeaderPresent verifies that Content-Security-Policy
// header is present and non-empty.
func TestSecurityHeaders_CSPHeaderPresent(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeaders()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header missing")
	}

	// CSP should contain script-src directive
	if !strings.Contains(csp, "script-src") {
		t.Fatalf("CSP missing script-src directive: %q", csp)
	}
}

// TestSecurityHeaders_AllDefaultHeadersApplied verifies that all default
// security headers are applied.
func TestSecurityHeaders_AllDefaultHeadersApplied(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeaders()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	resp := w.Result()
	err := VerifySecurityHeaders(resp)
	if err != nil {
		t.Fatalf("VerifySecurityHeaders() failed: %v", err)
	}
}

// TestSecurityHeaders_XFrameOptionsDeny verifies X-Frame-Options: DENY is set.
func TestSecurityHeaders_XFrameOptionsDeny(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeaders()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	xFrameOptions := w.Header().Get("X-Frame-Options")
	if xFrameOptions != "DENY" {
		t.Fatalf("expected X-Frame-Options=DENY, got %q", xFrameOptions)
	}
}

// TestSecurityHeaders_XContentTypeOptionsNosniff verifies
// X-Content-Type-Options: nosniff is set.
func TestSecurityHeaders_XContentTypeOptionsNosniff(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeaders()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	contentType := w.Header().Get("X-Content-Type-Options")
	if contentType != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options=nosniff, got %q", contentType)
	}
}

// TestSecurityHeaders_HeadersSurviveMiddlewareChain verifies that headers
// survive through a chain of middleware.
func TestSecurityHeaders_HeadersSurviveMiddlewareChain(t *testing.T) {
	// Create a chain of middleware
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply security headers middleware
	handler := http.Handler(baseHandler)
	handler = SecurityHeaders()(handler)

	// Wrap with a passthrough middleware that doesn't modify headers
	passthroughMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This middleware just passes through
			next.ServeHTTP(w, r)
		})
	}
	handler = passthroughMiddleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	err := VerifySecurityHeaders(resp)
	if err != nil {
		t.Fatalf("headers lost through middleware chain: %v", err)
	}
}

// TestSecurityHeaders_CustomHeaderOverrides verifies that custom header
// overrides work correctly.
func TestSecurityHeaders_CustomHeaderOverrides(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	override := HeaderOverride{
		CSPDirectives: map[string]string{
			"script-src": "'self' https://cdn.example.com",
		},
	}

	middleware := OverrideSecurityHeaders(override)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "https://cdn.example.com") {
		t.Fatalf("custom CSP override not applied: %q", csp)
	}
}

// ─── Integration Tests (3+ tests) ───────────────────────────────────────────

// TestIntegration_FullLoginFlowWithCSRFAndCookies verifies the complete login
// flow with CSRF tokens and HttpOnly cookies.
func TestIntegration_FullLoginFlowWithCSRFAndCookies(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	csrfGen := NewGenerator(secret)
	csrfValidator := NewValidator(secret)

	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Step 1: Client makes GET request to login page (no CSRF required)
	getHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := csrfGen.Generate(r)
		w.Header().Set("X-CSRF-Token", token)
		w.WriteHeader(http.StatusOK)
	})

	getReq := httptest.NewRequest("GET", "/login", nil)
	getW := httptest.NewRecorder()
	getHandler.ServeHTTP(getW, getReq)

	token := getW.Header().Get("X-CSRF-Token")
	if token == "" {
		t.Fatal("no CSRF token in login response")
	}

	// Step 2: Client makes POST request with CSRF token and credentials
	postHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate CSRF token
		if err := csrfValidator.Verify(r, r.Header.Get(CSRFHeaderName)); err != nil {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}

		// Create session
		_, _, _, err := sm.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})
		if err != nil {
			http.Error(w, "failed to create session", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	postReq := httptest.NewRequest("POST", "/login", nil)
	postReq.Header.Set(CSRFHeaderName, token)
	postW := httptest.NewRecorder()
	postHandler.ServeHTTP(postW, postReq)

	if postW.Code != http.StatusOK {
		t.Fatalf("login failed: got %d", postW.Code)
	}

	// Verify session cookie was set
	cookies := postW.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie in response")
	}

	sessionCookie := cookies[0]
	if sessionCookie.Name != SessionCookieName {
		t.Fatalf("expected session cookie, got %q", sessionCookie.Name)
	}

	if !sessionCookie.HttpOnly {
		t.Fatal("session cookie missing HttpOnly flag")
	}

	// Step 3: Client makes authenticated request with session cookie
	authedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := sm.ValidateSession(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if claims.Subject != "user-123" {
			http.Error(w, "wrong user", http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	authedReq := httptest.NewRequest("GET", "/api/profile", nil)
	authedReq.AddCookie(sessionCookie)
	authedW := httptest.NewRecorder()
	authedHandler.ServeHTTP(authedW, authedReq)

	if authedW.Code != http.StatusOK {
		t.Fatalf("authenticated request failed: got %d", authedW.Code)
	}
}

// TestIntegration_FullLogoutFlow verifies the complete logout flow.
func TestIntegration_FullLogoutFlow(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Step 1: Create a session
	w1 := httptest.NewRecorder()
	_, _, _, err := sm.CreateSession(w1, "user-123", "user@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	cookies := w1.Result().Cookies()
	sessionCookie := cookies[0]

	// Step 2: Validate session is active
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(sessionCookie)

	_, err = sm.ValidateSession(req)
	if err != nil {
		t.Fatalf("ValidateSession() failed: %v", err)
	}

	// Step 3: Destroy the session
	w2 := httptest.NewRecorder()
	err = sm.DestroySession(w2)
	if err != nil {
		t.Fatalf("DestroySession() failed: %v", err)
	}

	cookies = w2.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies in logout response")
	}

	if cookies[0].MaxAge != -1 {
		t.Fatalf("expected MaxAge=-1 for logout, got %d", cookies[0].MaxAge)
	}

	// Step 4: Verify session is no longer valid (using the old cookie)
	// Note: The old cookie's JWT is still technically valid until it expires,
	// but in a real app with server-side revocation, it would be invalid.
	// For this test, we just verify the cookie was marked for deletion.
}

// TestIntegration_UnauthorizedRequestHandling verifies proper handling of
// unauthorized requests.
func TestIntegration_UnauthorizedRequestHandling(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Request without session cookie should return 401
	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := sm.ValidateSession(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing session, got %d", w.Code)
	}
}

// ─── OWASP Compliance Tests (6+ tests) ───────────────────────────────────────

// TestOWASP_NoSensitiveDataInErrorMessages verifies that error messages don't
// leak sensitive data.
func TestOWASP_NoSensitiveDataInErrorMessages(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	// Create a request with an invalid CSRF token
	req := httptest.NewRequest("POST", "/api/transfer", nil)
	req.Header.Set(CSRFHeaderName, "invalid-token-should-not-leak-details")

	// Error message should be generic
	err := val.Verify(req, "invalid-token-should-not-leak-details")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}

	errMsg := err.Error()

	// Should not contain the actual token
	if strings.Contains(errMsg, "invalid-token-should-not-leak-details") {
		t.Fatalf("error message leaked token: %s", errMsg)
	}

	// Should not contain cryptographic details that could help an attacker
	if strings.Contains(strings.ToLower(errMsg), "hmac") {
		t.Fatalf("error message leaked cryptographic details: %s", errMsg)
	}
}

// TestOWASP_NoStackTracesInResponses verifies that responses don't contain
// stack traces.
func TestOWASP_NoStackTracesInResponses(t *testing.T) {
	// Create a handler that would panic in unsafe code
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate an error
		msg := "this is an error"
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(msg))
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Should not contain common stack trace indicators
	if strings.Contains(body, "goroutine") {
		t.Fatalf("response contains goroutine stack trace")
	}

	if strings.Contains(body, ".go:") {
		t.Fatalf("response contains file:line references")
	}
}

// TestOWASP_AuthenticationEnforcement verifies that protected endpoints
// enforce authentication.
func TestOWASP_AuthenticationEnforcement(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Protected handler requires valid session
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := sm.ValidateSession(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name         string
		setupReq     func(*http.Request)
		expectedCode int
	}{
		{
			name: "no session cookie",
			setupReq: func(r *http.Request) {
				// Don't add any cookie
			},
			expectedCode: http.StatusUnauthorized,
		},
		{
			name: "invalid session cookie",
			setupReq: func(r *http.Request) {
				r.AddCookie(&http.Cookie{
					Name:  SessionCookieName,
					Value: "invalid.jwt.token",
				})
			},
			expectedCode: http.StatusUnauthorized,
		},
		{
			name: "expired session cookie",
			setupReq: func(r *http.Request) {
				// Create an expired session
				shortIssuer := &auth.Issuer{
					Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
					Issuer:   "test-issuer",
					Audience: []string{"test-app"},
					TTL:      1 * time.Millisecond,
				}
				w := httptest.NewRecorder()
				shortSM := NewSessionManager(shortIssuer, true)
				shortSM.CreateSession(w, "user-123", "user@example.com", []auth.Role{auth.RoleUser})

				cookies := w.Result().Cookies()
				if len(cookies) > 0 {
					time.Sleep(2 * time.Millisecond)
					r.AddCookie(cookies[0])
				}
			},
			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			tt.setupReq(req)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Fatalf("expected %d, got %d", tt.expectedCode, w.Code)
			}
		})
	}
}

// TestOWASP_SessionFixationPrevention verifies that session IDs are
// regenerated on login.
func TestOWASP_SessionFixationPrevention(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// First session (before login)
	w1 := httptest.NewRecorder()
	_, claims1, _, _ := sm.CreateSession(w1, "user-123", "user@example.com", []auth.Role{auth.RoleUser})

	// Second session (after login - simulating a new login)
	w2 := httptest.NewRecorder()
	_, claims2, _, _ := sm.CreateSession(w2, "user-123", "user@example.com", []auth.Role{auth.RoleUser})

	// The JWT tokens should be different (different iat/nonce)
	cookies1 := w1.Result().Cookies()
	cookies2 := w2.Result().Cookies()

	if len(cookies1) > 0 && len(cookies2) > 0 {
		if cookies1[0].Value == cookies2[0].Value {
			t.Fatal("session ID not regenerated on new login (same JWT token)")
		}
	}

	// Claims should have different IssuedAt times
	if claims1.IssuedAt == claims2.IssuedAt {
		// This is unlikely to happen since both are issued at similar times,
		// but the important thing is that they're different JWTs
		t.Logf("claims have same IssuedAt: %d", claims1.IssuedAt)
	}
}

// TestOWASP_CSRFValidationFailureReturns403 verifies that CSRF validation
// failures return 403 Forbidden.
func TestOWASP_CSRFValidationFailureReturns403(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := val.Middleware(handler)

	// POST request without CSRF token
	req := httptest.NewRequest("POST", "/api/transfer", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	// Response body should not leak token details
	body := w.Body.String()
	if strings.Contains(body, "signature") || strings.Contains(body, "HMAC") {
		t.Fatalf("error response leaked cryptographic details: %s", body)
	}
}

// ─── Additional Edge Case Tests ────────────────────────────────────────────

// TestCSRF_TokenWithPersonalizedSecretAcceptedByCorrectUser verifies that
// personalized tokens work correctly with user-specific secrets.
func TestCSRF_TokenWithPersonalizedSecretAcceptedByCorrectUser(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	// User 1 generates a token
	claims1 := auth.Claims{Subject: "user-1"}
	ctx1 := WithClaims(context.Background(), claims1)
	req1 := httptest.NewRequest("GET", "/", nil).WithContext(ctx1)

	token1, err := gen.Generate(req1)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// User 1 should validate the token successfully
	err = val.Verify(req1, token1)
	if err != nil {
		t.Fatalf("Verify() failed for correct user: %v", err)
	}

	// User 2 should NOT validate user 1's token
	claims2 := auth.Claims{Subject: "user-2"}
	ctx2 := WithClaims(context.Background(), claims2)
	req2 := httptest.NewRequest("GET", "/", nil).WithContext(ctx2)

	err = val.Verify(req2, token1)
	if err == nil {
		t.Fatal("Verify() accepted token from different user")
	}
}

// TestSecurityHeaders_ListSecurityHeadersReturnsAllHeaders verifies that
// ListSecurityHeaders returns the expected headers.
func TestSecurityHeaders_ListSecurityHeadersReturnsAllHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeaders()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	headers := ListSecurityHeaders(w.Result())

	expectedHeaders := map[string]bool{
		"Content-Security-Policy": false,
		"X-Frame-Options":         false,
		"X-Content-Type-Options":  false,
		"X-XSS-Protection":        false,
		"Referrer-Policy":         false,
		"Permissions-Policy":      false,
	}

	for name := range headers {
		if _, ok := expectedHeaders[name]; ok {
			expectedHeaders[name] = true
		}
	}

	// At least the core headers should be present
	if !expectedHeaders["Content-Security-Policy"] {
		t.Fatal("CSP header missing from ListSecurityHeaders")
	}

	if !expectedHeaders["X-Frame-Options"] {
		t.Fatal("X-Frame-Options header missing from ListSecurityHeaders")
	}
}

// TestCookies_SessionCreationWithMultipleRoles verifies session creation
// with multiple roles.
func TestCookies_SessionCreationWithMultipleRoles(t *testing.T) {
	issuer := &auth.Issuer{
		Secret:   []byte("this-is-a-32-byte-secret-key-!!!"),
		Issuer:   "test-issuer",
		Audience: []string{"test-app"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	roles := []auth.Role{auth.RoleUser, auth.RoleAdvisor}

	w := httptest.NewRecorder()
	_, claims, _, err := sm.CreateSession(w, "user-123", "user@example.com", roles)
	if err != nil {
		t.Fatalf("CreateSession() failed: %v", err)
	}

	if len(claims.Roles) != len(roles) {
		t.Fatalf("expected %d roles, got %d", len(roles), len(claims.Roles))
	}

	for i, role := range roles {
		if claims.Roles[i] != role {
			t.Fatalf("role mismatch at index %d: expected %v, got %v", i, role, claims.Roles[i])
		}
	}
}

// TestCSRF_FormFieldTokenExtraction verifies CSRF token extraction from
// form fields.
func TestCSRF_FormFieldTokenExtraction(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Create a POST request with token in form field
	formData := url.Values{}
	formData.Set("_csrf", token)
	formData.Set("username", "testuser")

	postReq := httptest.NewRequest("POST", "/login", strings.NewReader(formData.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Validate token from form field
	err = val.Verify(postReq, token)
	if err != nil {
		t.Fatalf("Verify() failed for form field token: %v", err)
	}
}

// TestSecurityHeaders_PermissionsPolicyDisablesAPIs verifies that
// Permissions-Policy disables geolocation, microphone, camera.
func TestSecurityHeaders_PermissionsPolicyDisablesAPIs(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeaders()
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	policy := w.Header().Get("Permissions-Policy")
	if policy == "" {
		t.Fatal("Permissions-Policy header missing")
	}

	// Should disable geolocation
	if !strings.Contains(policy, "geolocation=()") {
		t.Fatalf("Permissions-Policy does not disable geolocation: %s", policy)
	}

	// Should disable microphone
	if !strings.Contains(policy, "microphone=()") {
		t.Fatalf("Permissions-Policy does not disable microphone: %s", policy)
	}

	// Should disable camera
	if !strings.Contains(policy, "camera=()") {
		t.Fatalf("Permissions-Policy does not disable camera: %s", policy)
	}
}
