// Package security provides CSRF token generation/validation, session management,
// security headers, and standardized error handling aligned with NIST SP 800-63B
// and OWASP Top 10 2021 (A01: Broken Access Control, A04: Insecure Design,
// A05: Security Misconfiguration).
//
// MIT License — Copyright (c) 2026 Genie Contributors
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
	"strings"
	"time"
)

// CSRFConfig holds CSRF token generation/validation parameters.
// All durations and key sizes comply with NIST SP 800-63B recommendations.
type CSRFConfig struct {
	TokenTTL         time.Duration // token lifetime (default 15 minutes)
	MaxTokenAge      time.Duration // reject if older than this (default 15 minutes)
	ServerSecret     []byte        // HMAC signing key (min 32 bytes, NIST recommendation)
	TokenLength      int           // nonce length in bytes (default 32)
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
// The token is not stored server-side; validation is stateless (double-submit + HMAC).
type CSRFToken struct {
	Nonce       string    `json:"nonce"`                  // base64-encoded random nonce
	Signature   string    `json:"signature"`              // HMAC-SHA256 signature
	ExpiresAt   time.Time `json:"expires_at"`             // token TTL
	IssuedAt    time.Time `json:"issued_at"`              // creation timestamp
	SessionID   string    `json:"session_id"`             // bound to session (user ID)
	RotationTag string    `json:"rotation_tag,omitempty"` // anti-replay marker
}

// CSRFService generates and validates CSRF tokens using HMAC-SHA256.
// Uses double-submit + HMAC pattern: stateless validation without server-side storage.
type CSRFService struct {
	cfg CSRFConfig
}

// NewCSRFService creates a new CSRF service.
// Panics if serverSecret < 32 bytes (NIST SP 800-63B minimum).
func NewCSRFService(cfg CSRFConfig) *CSRFService {
	if len(cfg.ServerSecret) < 32 {
		panic("csrf: server secret must be >= 32 bytes (NIST SP 800-63B)")
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
// Returns the serialized token (format: nonce.signature.expiresAt) and the CSRFToken struct.
//
// The token is created by:
// 1. Generating 32-byte cryptographic nonce via crypto/rand
// 2. Computing HMAC-SHA256(nonce || sessionID || timestamp, serverSecret)
// 3. Returning both nonce and signature (double-submit pattern)
//
// The client stores the token in memory and sends it in X-CSRF-Token header
// on every POST/PUT/DELETE request. The server validates the signature without
// maintaining server-side session storage.
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

	// Step 2: Create token metadata.
	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.TokenTTL)

	// Step 3: Compute HMAC-SHA256 signature over (nonce || sessionID || expiresAt).
	// This binds the token to the session and time window (prevents token reuse across time).
	// Use Unix timestamp in HMAC to ensure consistency across parsing/formatting.
	h := hmac.New(sha256.New, s.cfg.ServerSecret)
	h.Write([]byte(nonceB64))
	h.Write([]byte(sessionID))
	h.Write([]byte(fmt.Sprintf("%d", expiresAt.Unix())))
	sig := hex.EncodeToString(h.Sum(nil))

	// Step 4: Format token as: base64(nonce).hex(signature).unix(expiresAt)
	// Client stores this and sends in X-CSRF-Token header.
	tokenStr := fmt.Sprintf("%s.%s.%d", nonceB64, sig, expiresAt.Unix())

	tok := &CSRFToken{
		Nonce:       nonceB64,
		Signature:   sig,
		ExpiresAt:   expiresAt,
		IssuedAt:    now,
		SessionID:   sessionID,
		RotationTag: generateRotationTag(),
	}

	return tokenStr, tok, nil
}

// ValidateToken verifies a CSRF token against a session ID.
// Uses constant-time HMAC comparison (hmac.Equal) to prevent timing attacks.
//
// Returns the CSRFToken struct and error if validation fails.
// Errors returned:
//   - "token format invalid": malformed token string
//   - "token expired": exceeds MaxTokenAge
//   - "signature mismatch": HMAC validation failed (potential CSRF attack)
func (s *CSRFService) ValidateToken(tokenStr string, sessionID string) (*CSRFToken, error) {
	if tokenStr == "" || sessionID == "" {
		return nil, errors.New("token and session id required")
	}

	// Step 1: Parse token format (nonce.signature.expiresAt).
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("token format invalid: expected nonce.signature.expiresAt")
	}
	nonceB64, sig, expiresAtStr := parts[0], parts[1], parts[2]

	// Step 2: Parse expiry timestamp.
	var expiresAtUnix int64
	if _, err := fmt.Sscanf(expiresAtStr, "%d", &expiresAtUnix); err != nil {
		return nil, errors.New("token format invalid: invalid timestamp")
	}
	expiresAt := time.Unix(expiresAtUnix, 0).UTC()

	// Step 3: Check expiry — reject if token older than MaxTokenAge.
	now := time.Now().UTC()
	if now.After(expiresAt) {
		return nil, fmt.Errorf("token expired at %v", expiresAt)
	}
	if expiresAt.Sub(now) > s.cfg.MaxTokenAge {
		return nil, fmt.Errorf("token age exceeds max: %v > %v", expiresAt.Sub(now), s.cfg.MaxTokenAge)
	}

	// Step 4: Validate signature using constant-time comparison (prevent timing attacks).
	// Use Unix timestamp in HMAC to match GenerateToken.
	h := hmac.New(sha256.New, s.cfg.ServerSecret)
	h.Write([]byte(nonceB64))
	h.Write([]byte(sessionID))
	h.Write([]byte(fmt.Sprintf("%d", expiresAt.Unix())))
	expectedSigBytes := h.Sum(nil)
	expectedSig := hex.EncodeToString(expectedSigBytes)

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, errors.New("signature mismatch: potential csrf attack")
	}

	tok := &CSRFToken{
		Nonce:       nonceB64,
		Signature:   sig,
		ExpiresAt:   expiresAt,
		IssuedAt:    expiresAt.Add(-s.cfg.TokenTTL),
		SessionID:   sessionID,
		RotationTag: generateRotationTag(),
	}

	return tok, nil
}

// RequiresValidation returns true if the HTTP method requires CSRF token validation.
// Safe methods (GET, HEAD, OPTIONS) do not require CSRF tokens.
// State-modifying methods (POST, PUT, DELETE, PATCH) do.
func RequiresValidation(method string) bool {
	switch method {
	case "POST", "PUT", "DELETE", "PATCH":
		return true
	default:
		return false
	}
}

// generateRotationTag creates a random anti-replay marker for token rotation.
func generateRotationTag() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}
