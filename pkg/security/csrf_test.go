package security

import (
	"strings"
	"testing"
	"time"
)

// TestNewCSRFService validates initialization with proper secret length.
func TestNewCSRFService_Init(t *testing.T) {
	// Should panic if serverSecret < 32 bytes.
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for short server secret")
		}
	}()

	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("short") // Only 5 bytes
	_ = NewCSRFService(cfg)
}

// TestCSRFService_GenerateToken validates token generation.
func TestCSRFService_GenerateToken(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!") // 34 bytes to be safe
	svc := NewCSRFService(cfg)

	// Generate token for user session.
	tokenStr, tok, err := svc.GenerateToken("user123")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	// Validate token structure.
	if tok == nil {
		t.Fatal("token struct is nil")
	}
	if tok.Nonce == "" {
		t.Error("nonce is empty")
	}
	if tok.Signature == "" {
		t.Error("signature is empty")
	}
	if tok.SessionID != "user123" {
		t.Errorf("session id mismatch: got %s, want user123", tok.SessionID)
	}
	if tok.ExpiresAt.IsZero() {
		t.Error("expires_at not set")
	}

	// Validate token format (nonce.signature.expiresAt).
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Errorf("token format invalid: got %d parts, want 3", len(parts))
	}
}

// TestCSRFService_ValidateToken validates token validation.
func TestCSRFService_ValidateToken(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	// Generate and validate token.
	tokenStr, _, _ := svc.GenerateToken("user123")
	validatedTok, err := svc.ValidateToken(tokenStr, "user123")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	if validatedTok.SessionID != "user123" {
		t.Errorf("session id mismatch: got %s, want user123", validatedTok.SessionID)
	}
}

// TestCSRFService_ValidateToken_WrongSession validates rejection of token for different session.
func TestCSRFService_ValidateToken_WrongSession(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	tokenStr, _, _ := svc.GenerateToken("user123")

	// Try to validate token for different user (CSRF attack).
	_, err := svc.ValidateToken(tokenStr, "user456")
	if err == nil {
		t.Fatal("validate should reject token for different session")
	}
	if !strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("expected 'signature mismatch', got: %v", err)
	}
}

// TestCSRFService_ValidateToken_Expired validates rejection of expired tokens.
func TestCSRFService_ValidateToken_Expired(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.TokenTTL = 1 * time.Millisecond // Very short TTL for testing
	cfg.MaxTokenAge = 1 * time.Millisecond
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	tokenStr, _, _ := svc.GenerateToken("user123")

	// Wait for token to expire.
	time.Sleep(10 * time.Millisecond)

	// Try to validate expired token.
	_, err := svc.ValidateToken(tokenStr, "user123")
	if err == nil {
		t.Fatal("validate should reject expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected 'expired', got: %v", err)
	}
}

// TestCSRFService_ValidateToken_InvalidFormat validates rejection of malformed tokens.
func TestCSRFService_ValidateToken_InvalidFormat(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	tests := []string{
		"",               // empty token
		"invalid",        // single part
		"a.b",            // two parts
		"a.b.c.d",        // four parts
		"a.b.notanumber", // invalid timestamp
	}

	for _, tokenStr := range tests {
		_, err := svc.ValidateToken(tokenStr, "user123")
		if err == nil {
			t.Errorf("validate should reject malformed token: %q", tokenStr)
		}
	}
}

// TestCSRFService_ValidateToken_EmptySessionID validates rejection when session ID empty.
func TestCSRFService_ValidateToken_EmptySessionID(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	tokenStr, _, _ := svc.GenerateToken("user123")

	_, err := svc.ValidateToken(tokenStr, "")
	if err == nil {
		t.Fatal("validate should reject empty session id")
	}
}

// TestRequiresValidation validates CSRF requirement logic.
func TestRequiresValidation(t *testing.T) {
	tests := []struct {
		method   string
		required bool
	}{
		{"GET", false},
		{"HEAD", false},
		{"OPTIONS", false},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"PATCH", true},
	}

	for _, tt := range tests {
		result := RequiresValidation(tt.method)
		if result != tt.required {
			t.Errorf("RequiresValidation(%s) = %v, want %v", tt.method, result, tt.required)
		}
	}
}

// TestCSRFService_TokenRotation validates that consecutive tokens are different.
func TestCSRFService_TokenRotation(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	// Generate multiple tokens.
	token1, _, _ := svc.GenerateToken("user123")
	token2, _, _ := svc.GenerateToken("user123")
	token3, _, _ := svc.GenerateToken("user123")

	// Tokens should be different (due to random nonce).
	if token1 == token2 {
		t.Error("token1 and token2 should be different")
	}
	if token2 == token3 {
		t.Error("token2 and token3 should be different")
	}
	if token1 == token3 {
		t.Error("token1 and token3 should be different")
	}
}

// TestCSRFService_TimingAttackResistance validates constant-time HMAC comparison.
// This is a basic sanity check; proper timing attack validation requires specialized tools.
func TestCSRFService_TimingAttackResistance(t *testing.T) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	tokenStr, _, _ := svc.GenerateToken("user123")

	// Corrupt the signature by flipping its first byte to a GUARANTEED-different
	// value. (The old code hard-coded "0", which was a no-op — and thus a flaky
	// false pass — whenever the signature already began with '0'.)
	parts := strings.Split(tokenStr, ".")
	repl := byte('0')
	if parts[1][0] == repl {
		repl = '1'
	}
	corruptedToken := parts[0] + "." + string(repl) + parts[1][1:] + "." + parts[2]

	// Should fail with signature mismatch.
	_, err := svc.ValidateToken(corruptedToken, "user123")
	if err == nil {
		t.Fatal("validate should reject token with invalid signature")
	}
	if !strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("expected 'signature mismatch', got: %v", err)
	}
}

// BenchmarkCSRFService_GenerateToken benchmarks token generation.
func BenchmarkCSRFService_GenerateToken(b *testing.B) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GenerateToken("user123")
	}
}

// BenchmarkCSRFService_ValidateToken benchmarks token validation.
func BenchmarkCSRFService_ValidateToken(b *testing.B) {
	cfg := DefaultCSRFConfig()
	cfg.ServerSecret = []byte("test-secret-32-bytes-exactly1234!!")
	svc := NewCSRFService(cfg)

	tokenStr, _, _ := svc.GenerateToken("user123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.ValidateToken(tokenStr, "user123")
	}
}
