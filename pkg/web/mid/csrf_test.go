package mid

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

// TestGenerator_Generate verifies token generation produces valid tokens.
func TestGenerator_Generate(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)

	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if token == "" {
		t.Fatal("token is empty")
	}

	if len(token) < CSRFTokenLength {
		t.Fatalf("token too short: got %d, want min %d", len(token), CSRFTokenLength)
	}

	// Token must contain exactly one dot separator.
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("token format invalid: expected signature.payload, got %q", token)
	}

	// Signature must be 64 hex characters (32 bytes * 2).
	if len(parts[0]) != 64 {
		t.Fatalf("signature length invalid: got %d, want 64", len(parts[0]))
	}

	// Validate hex encoding of signature.
	for _, ch := range parts[0] {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Fatalf("signature contains non-hex character: %c", ch)
		}
	}
}

// TestGenerator_GenerateWithClaims verifies personalized tokens with user ID.
func TestGenerator_GenerateWithClaims(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)

	claims := auth.Claims{Subject: "user-123"}
	ctx := WithClaims(context.Background(), claims)
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	token1, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Generate another token for a different user.
	claims2 := auth.Claims{Subject: "user-456"}
	ctx2 := WithClaims(context.Background(), claims2)
	req2 := httptest.NewRequest("GET", "/", nil).WithContext(ctx2)
	token2, err := gen.Generate(req2)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Tokens must be different (different user IDs).
	if token1 == token2 {
		t.Fatal("tokens must differ for different users")
	}
}

// TestGenerator_GenerateShortSecret verifies error for short secret.
func TestGenerator_GenerateShortSecret(t *testing.T) {
	gen := &Generator{
		Secret: []byte("short"),
		TTL:    15 * time.Minute,
	}

	req := httptest.NewRequest("GET", "/", nil)
	_, err := gen.Generate(req)

	if err == nil {
		t.Fatal("expected error for short secret")
	}

	if !strings.Contains(err.Error(), "secret too short") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidator_Verify verifies token validation with correct secret.
func TestValidator_Verify(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	err = val.Verify(req, token)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}
}

// TestValidator_VerifyWithClaims verifies personalized tokens.
func TestValidator_VerifyWithClaims(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	claims := auth.Claims{Subject: "user-123"}
	ctx := WithClaims(context.Background(), claims)

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify with same claims should succeed.
	err = val.Verify(req, token)
	if err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}

	// Verify with different claims should fail (wrong user).
	claims2 := auth.Claims{Subject: "user-456"}
	ctx2 := WithClaims(context.Background(), claims2)
	req2 := httptest.NewRequest("GET", "/", nil).WithContext(ctx2)

	err = val.Verify(req2, token)
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

// TestValidator_VerifyInvalidFormat verifies format validation.
func TestValidator_VerifyInvalidFormat(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	req := httptest.NewRequest("GET", "/", nil)

	tests := []struct {
		name  string
		token string
	}{
		{"no dot", "abcd1234"},
		{"no payload", "abc123."},
		{"invalid hex", "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ.1234567890:nonce"},
		{"too many parts", "abc.def.ghi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := val.Verify(req, tt.token)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "invalid") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidator_VerifyMissingToken verifies missing token rejection.
func TestValidator_VerifyMissingToken(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	req := httptest.NewRequest("GET", "/", nil)
	err := val.Verify(req, "")

	if err == nil {
		t.Fatal("expected error for missing token")
	}

	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidator_VerifyWrongSecret verifies rejection with wrong secret.
func TestValidator_VerifyWrongSecret(t *testing.T) {
	secret1 := []byte("this-is-a-32-byte-secret-key-!!!")
	secret2 := []byte("different-32-byte-secret-key-###")

	gen := NewGenerator(secret1)
	val := NewValidator(secret2)

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	err = val.Verify(req, token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}

	if !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidator_VerifyExpiredToken verifies rejection of expired tokens.
func TestValidator_VerifyExpiredToken(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := &Validator{
		Secret: secret,
		TTL:    1 * time.Millisecond, // Very short TTL for testing
	}

	req := httptest.NewRequest("GET", "/", nil)
	token, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Wait for token to expire.
	time.Sleep(10 * time.Millisecond)

	err = val.Verify(req, token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}

	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestMiddleware_SkipSafeMethods verifies CSRF validation is skipped for safe methods.
func TestMiddleware_SkipSafeMethods(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	safeMethods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodTrace,
	}

	for _, method := range safeMethods {
		t.Run(method, func(t *testing.T) {
			called := false
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			val.Middleware(handler).ServeHTTP(w, req)

			if !called {
				t.Fatal("handler should have been called")
			}

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
		})
	}
}

// TestMiddleware_ValidateUnsafeMethods verifies CSRF validation for unsafe methods.
func TestMiddleware_ValidateUnsafeMethods(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	unsafeMethods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, method := range unsafeMethods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			token, _ := gen.Generate(req)
			req.Header.Set("X-CSRF-Token", token)

			w := httptest.NewRecorder()
			val.Middleware(handler).ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
		})
	}
}

// TestMiddleware_RejectMissingToken verifies rejection of missing tokens.
func TestMiddleware_RejectMissingToken(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", nil)
	w := httptest.NewRecorder()

	val.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "invalid CSRF token") {
		t.Fatalf("unexpected response: %s", w.Body.String())
	}
}

// TestMiddleware_ExtractFromHeader verifies token extraction from header.
func TestMiddleware_ExtractFromHeader(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", nil)
	token, _ := gen.Generate(req)
	req.Header.Set("X-CSRF-Token", token)

	w := httptest.NewRecorder()
	val.Middleware(handler).ServeHTTP(w, req)

	if !called {
		t.Fatal("handler should have been called")
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestMiddleware_ExtractFromForm verifies token extraction from form field.
func TestMiddleware_ExtractFromForm(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", nil)
	token, _ := gen.Generate(req)

	// Add token as form field.
	req.PostForm = url.Values{"_csrf": {token}}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	val.Middleware(handler).ServeHTTP(w, req)

	if !called {
		t.Fatal("handler should have been called")
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestMiddleware_PreferHeaderOverForm verifies header is checked before form.
func TestMiddleware_PreferHeaderOverForm(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", nil)
	validToken, _ := gen.Generate(req)

	// Add valid token in header.
	req.Header.Set("X-CSRF-Token", validToken)

	// Add invalid token in form field (should be ignored).
	req.PostForm = url.Values{"_csrf": {"invalid-token"}}

	w := httptest.NewRecorder()
	val.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (header token used), got %d", w.Code)
	}
}

// TestMiddleware_RejectInvalidToken verifies rejection of invalid tokens.
func TestMiddleware_RejectInvalidToken(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	val := NewValidator(secret)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-CSRF-Token", "invalid-token")

	w := httptest.NewRecorder()
	val.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// TestMiddleware_IntegrationFlow verifies a complete request flow.
func TestMiddleware_IntegrationFlow(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	gen := NewGenerator(secret)
	val := NewValidator(secret)

	// Step 1: Client GET request (no CSRF validation).
	getReq := httptest.NewRequest("GET", "/form", nil)
	w := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate token for the response.
		token, _ := gen.Generate(r)
		fmt.Fprintf(w, `<form method="POST"><input name="_csrf" value="%s"></form>`, token)
	})
	val.Middleware(handler).ServeHTTP(w, getReq)

	if w.Code != http.StatusOK {
		t.Fatalf("GET failed: %d", w.Code)
	}

	// Extract token from response.
	body := w.Body.String()
	tokenStart := strings.Index(body, `value="`) + 7
	tokenEnd := strings.Index(body[tokenStart:], `"`) + tokenStart
	token := body[tokenStart:tokenEnd]

	// Step 2: Client POST request with token.
	postReq := httptest.NewRequest("POST", "/submit", nil)
	postReq.PostForm = url.Values{"_csrf": {token}}

	w = httptest.NewRecorder()
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
	val.Middleware(handler2).ServeHTTP(w, postReq)

	if w.Code != http.StatusOK {
		t.Fatalf("POST failed: %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "success") {
		t.Fatalf("unexpected response: %s", w.Body.String())
	}
}

// TestSecretFromJWT verifies secret extraction from JWT issuer.
func TestSecretFromJWT(t *testing.T) {
	secret := []byte("this-is-a-32-byte-secret-key-!!!")
	issuer := &auth.Issuer{Secret: secret}

	extracted := SecretFromJWT(issuer)

	if len(extracted) != 32 {
		t.Fatalf("extracted secret length: got %d, want 32", len(extracted))
	}

	for i := range secret {
		if secret[i] != extracted[i] {
			t.Fatalf("extracted secret mismatch at byte %d", i)
		}
	}
}

// TestSecretFromJWT_Panic verifies panic for short secret.
func TestSecretFromJWT_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for short secret")
		}
	}()

	issuer := &auth.Issuer{Secret: []byte("short")}
	SecretFromJWT(issuer)
}
