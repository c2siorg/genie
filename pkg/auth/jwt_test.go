package auth

import (
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func TestIssueVerify_Roundtrip(t *testing.T) {
	iss := NewIssuer([]byte("test-secret"), "genie", []string{"genie-api"}, time.Minute)
	tok, claims, err := iss.Issue("u-1", "a@b.com", []Role{RoleUser, RoleAdvisor})
	if err != nil {
		t.Fatal(err)
	}
	if !claims.HasRole(RoleAdvisor) {
		t.Fatalf("claims missing role: %+v", claims)
	}

	got, err := iss.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "u-1" {
		t.Fatalf("subject mismatch: %q", got.Subject)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	good := NewIssuer([]byte("k1"), "genie", nil, time.Minute)
	bad := NewIssuer([]byte("k2"), "genie", nil, time.Minute)
	tok, _, _ := good.Issue("u-1", "a@b.com", []Role{RoleUser})
	if _, err := bad.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	iss := NewIssuer([]byte("k"), "genie", nil, -time.Second)
	tok, _, _ := iss.Issue("u-1", "a@b.com", nil)
	if _, err := iss.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(hash, "hunter2"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch")
	}
}

// ─── CSRF Secret Generation ────────────────────────────────────────────────

func TestGenerateCSRFSecret_Format(t *testing.T) {
	secret, err := GenerateCSRFSecret()
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	// Should be hex-encoded 32 bytes = 64 hex chars.
	if len(secret) != 64 {
		t.Fatalf("length = %d, want 64 hex chars", len(secret))
	}

	// Should be valid hex.
	_, err = hex.DecodeString(secret)
	if err != nil {
		t.Fatalf("not valid hex: %v", err)
	}
}

func TestGenerateCSRFSecret_Unique(t *testing.T) {
	// Generate a few secrets; they should all be different.
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		secret, err := GenerateCSRFSecret()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[secret] {
			t.Fatalf("duplicate secret generated")
		}
		seen[secret] = true
	}
}

// ─── JWT with CSRF Secret ─────────────────────────────────────────────────

func TestIssue_IncludesCSRFSecret(t *testing.T) {
	iss := NewIssuer([]byte("test-secret"), "genie", []string{"genie-api"}, time.Minute)
	tok, claims, err := iss.Issue("u-1", "a@b.com", []Role{RoleUser})
	if err != nil {
		t.Fatal(err)
	}

	// CSRFSecret should be populated.
	if claims.CSRFSecret == "" {
		t.Fatal("CSRFSecret is empty")
	}

	// Should be 64 hex characters (32 bytes encoded).
	if len(claims.CSRFSecret) != 64 {
		t.Fatalf("CSRFSecret length = %d, want 64", len(claims.CSRFSecret))
	}

	// Should decode as valid hex.
	_, err = hex.DecodeString(claims.CSRFSecret)
	if err != nil {
		t.Fatalf("CSRFSecret is not valid hex: %v", err)
	}

	// Verify roundtrip includes CSRF secret.
	got, err := iss.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.CSRFSecret != claims.CSRFSecret {
		t.Fatalf("CSRF mismatch: %q != %q", got.CSRFSecret, claims.CSRFSecret)
	}
}

func TestIssue_EachTokenHasFreshSecret(t *testing.T) {
	iss := NewIssuer([]byte("test-secret"), "genie", []string{"genie-api"}, time.Minute)

	// Issue two tokens for the same user.
	_, claims1, err := iss.Issue("u-1", "a@b.com", []Role{RoleUser})
	if err != nil {
		t.Fatal(err)
	}

	_, claims2, err := iss.Issue("u-1", "a@b.com", []Role{RoleUser})
	if err != nil {
		t.Fatal(err)
	}

	// Each token should have a unique CSRF secret.
	if claims1.CSRFSecret == claims2.CSRFSecret {
		t.Fatal("two tokens from same issuer have the same CSRF secret")
	}
}

func TestIssueWithActor_IncludesCSRFSecret(t *testing.T) {
	// Create an issuer with no audience requirement.
	iss := NewIssuer([]byte("test-secret"), "genie", nil, time.Minute)
	actor := &Actor{Subject: "mcp-service", Issuer: "genie"}
	tok, claims, err := iss.IssueWithActor(
		"u-1", "a@b.com", []Role{RoleUser},
		[]string{"downstream-api"}, actor,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Should have CSRF secret.
	if claims.CSRFSecret == "" {
		t.Fatal("CSRFSecret is empty in token-exchange flow")
	}

	// Should have actor.
	if claims.Actor == nil || claims.Actor.Subject != "mcp-service" {
		t.Fatalf("actor not set correctly: %+v", claims.Actor)
	}

	// Verify roundtrip. Use an issuer with matching audience.
	verifier := NewIssuer([]byte("test-secret"), "genie", []string{"downstream-api"}, time.Minute)
	got, err := verifier.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.CSRFSecret != claims.CSRFSecret {
		t.Fatalf("CSRF secret mismatch after verify")
	}
	if got.Actor.Subject != "mcp-service" {
		t.Fatalf("actor lost in roundtrip")
	}
}

// ─── Backward Compatibility ───────────────────────────────────────────────

func TestVerify_BackwardCompatibility_NoCSRFSecret(t *testing.T) {
	// Simulate a token issued by an older version (before CSRF secrets).
	// We'll manually construct and sign a token without csrf_secret.
	iss := NewIssuer([]byte("test-secret"), "genie", []string{"genie-api"}, time.Minute)

	// Create claims without CSRF secret (manually, not via Issue).
	now := time.Now().UTC()
	claimsNoCSRF := Claims{
		Subject:   "u-1",
		Email:     "a@b.com",
		Roles:     []Role{RoleUser},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(time.Minute).Unix(),
		Issuer:    "genie",
		Audience:  []string{"genie-api"},
		// CSRFSecret is intentionally empty
	}

	// Encode it manually.
	tok, err := encode(jwtHeader{Alg: "HS256", Typ: "JWT"}, claimsNoCSRF, iss.Secret)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Verify should succeed and CSRFSecret should be empty.
	got, err := iss.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.CSRFSecret != "" {
		t.Fatalf("old token should not have CSRFSecret, got %q", got.CSRFSecret)
	}
	if got.Subject != "u-1" {
		t.Fatalf("subject mismatch")
	}
}

func TestVerify_OldAndNewTokens_Coexist(t *testing.T) {
	iss := NewIssuer([]byte("test-secret"), "genie", []string{"genie-api"}, time.Minute)

	// Issue a new token (with CSRF).
	newTok, newClaims, err := iss.Issue("u-1", "a@b.com", []Role{RoleUser})
	if err != nil {
		t.Fatal(err)
	}

	// Manually create an old token (without CSRF).
	now := time.Now().UTC()
	oldClaims := Claims{
		Subject:   "u-1",
		Email:     "a@b.com",
		Roles:     []Role{RoleUser},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(time.Minute).Unix(),
		Issuer:    "genie",
		Audience:  []string{"genie-api"},
	}
	oldTok, err := encode(jwtHeader{Alg: "HS256", Typ: "JWT"}, oldClaims, iss.Secret)
	if err != nil {
		t.Fatal(err)
	}

	// Both should verify successfully.
	gotNew, err := iss.Verify(newTok)
	if err != nil {
		t.Fatal(err)
	}
	if gotNew.CSRFSecret == "" {
		t.Fatal("new token missing CSRF secret")
	}

	gotOld, err := iss.Verify(oldTok)
	if err != nil {
		t.Fatal(err)
	}
	if gotOld.CSRFSecret != "" {
		t.Fatalf("old token should have empty CSRF, got %q", gotOld.CSRFSecret)
	}

	// New token should have a different CSRF than the old token (which has none).
	if newClaims.CSRFSecret == gotOld.CSRFSecret {
		t.Fatal("new and old tokens should have different CSRF secrets")
	}
}

// ─── Integration: Full JWT Lifecycle with CSRF ────────────────────────────

func TestJWTLifecycle_WithCSRF(t *testing.T) {
	iss := NewIssuer([]byte("test-secret"), "genie", []string{"genie-api"}, time.Minute)

	// 1. Issue a token.
	tok, issuedClaims, err := iss.Issue("u-alice", "alice@example.com", []Role{RoleUser, RoleAdvisor})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Verify CSRF secret is present and valid.
	if issuedClaims.CSRFSecret == "" {
		t.Fatal("issued token missing CSRF secret")
	}
	if len(issuedClaims.CSRFSecret) != 64 {
		t.Fatalf("CSRF secret has wrong length: %d", len(issuedClaims.CSRFSecret))
	}

	// 3. Verify the token.
	verifiedClaims, err := iss.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Verify CSRF secret survived the round-trip.
	if verifiedClaims.CSRFSecret != issuedClaims.CSRFSecret {
		t.Fatal("CSRF secret mismatch after verify")
	}

	// 5. Verify other claims are intact.
	if verifiedClaims.Subject != "u-alice" {
		t.Fatalf("subject mismatch: %q", verifiedClaims.Subject)
	}
	if !verifiedClaims.HasRole(RoleAdvisor) {
		t.Fatal("advisor role lost")
	}
}

// ─── Error Handling ────────────────────────────────────────────────────────

func TestIssue_ErrorOnCSRFGeneration(t *testing.T) {
	// This is hard to test without mocking crypto/rand.
	// The function calls GenerateCSRFSecret which reads from rand.Reader.
	// A real test would need a mocked entropy source.
	// For now, this is a placeholder for documentation.
	t.Skip("requires mocking crypto/rand; documented in code review")
}
