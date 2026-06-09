package mid

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

func TestCreateSession_Happy(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)
	w := httptest.NewRecorder()

	userID := "user-123"
	email := "alice@example.com"
	roles := []auth.Role{auth.RoleUser}

	// Act
	cookie, claims, csrfSecret, err := sm.CreateSession(w, userID, email, roles)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookie == nil {
		t.Fatal("expected cookie, got nil")
	}
	if cookie.Name != SessionCookieName {
		t.Errorf("expected cookie name %s, got %s", SessionCookieName, cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
	if !cookie.Secure {
		t.Error("expected Secure=true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSite=Strict, got %v", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Errorf("expected Path=/, got %s", cookie.Path)
	}
	if cookie.MaxAge != int(24*time.Hour.Seconds()) {
		t.Errorf("expected MaxAge=%d, got %d", int(24*time.Hour.Seconds()), cookie.MaxAge)
	}

	// Check claims
	if claims.Subject != userID {
		t.Errorf("expected Subject=%s, got %s", userID, claims.Subject)
	}
	if claims.Email != email {
		t.Errorf("expected Email=%s, got %s", email, claims.Email)
	}
	if !claims.HasRole(auth.RoleUser) {
		t.Error("expected role RoleUser in claims")
	}

	// Check CSRF secret is non-empty hex
	if csrfSecret == "" {
		t.Fatal("expected non-empty CSRF secret")
	}
	if len(csrfSecret) != 64 { // 32 bytes * 2 hex chars
		t.Errorf("expected CSRF secret length 64, got %d", len(csrfSecret))
	}

	// Verify response has Set-Cookie header
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, SessionCookieName) {
		t.Errorf("expected Set-Cookie header to contain %s", SessionCookieName)
	}
	if !strings.Contains(setCookie, "HttpOnly") {
		t.Error("expected Set-Cookie to contain HttpOnly")
	}
	if !strings.Contains(setCookie, "Secure") {
		t.Error("expected Set-Cookie to contain Secure")
	}
}

func TestValidateSession_Happy(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Create a session to get a valid token
	w := httptest.NewRecorder()
	_, _, _, err := sm.CreateSession(w, "user-123", "alice@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Extract the token from the Set-Cookie header
	setCookie := w.Header().Get("Set-Cookie")
	cookieStart := strings.Index(setCookie, "session=") + len("session=")
	cookieEnd := strings.Index(setCookie[cookieStart:], ";")
	if cookieEnd == -1 {
		cookieEnd = len(setCookie)
	} else {
		cookieEnd = cookieStart + cookieEnd
	}
	token := setCookie[cookieStart:cookieEnd]

	// Create a request with the session cookie
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: token,
	})

	// Act
	claims, err := sm.ValidateSession(req)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("expected Subject=user-123, got %s", claims.Subject)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("expected Email=alice@example.com, got %s", claims.Email)
	}
}

func TestValidateSession_MissingCookie(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Request without session cookie
	req := httptest.NewRequest("GET", "/api/test", nil)

	// Act
	claims, err := sm.ValidateSession(req)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	sessErr, ok := err.(SessionError)
	if !ok {
		t.Fatalf("expected SessionError, got %T", err)
	}
	if sessErr.Code != "missing_cookie" {
		t.Errorf("expected Code=missing_cookie, got %s", sessErr.Code)
	}
	if claims.Subject != "" {
		t.Errorf("expected zero Claims, got %v", claims)
	}
}

func TestValidateSession_InvalidToken(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Request with malformed token
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: "invalid.token.here",
	})

	// Act
	_, err := sm.ValidateSession(req)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	sessErr, ok := err.(SessionError)
	if !ok {
		t.Fatalf("expected SessionError, got %T", err)
	}
	if sessErr.Code != "verification_failed" {
		t.Errorf("expected Code=verification_failed, got %s", sessErr.Code)
	}
}

func TestValidateSession_ExpiredToken(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	// Create issuer with very short TTL (1 nanosecond)
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      1 * time.Nanosecond,
	}
	sm := NewSessionManager(issuer, true)

	// Create a session
	w := httptest.NewRecorder()
	_, _, _, err := sm.CreateSession(w, "user-123", "alice@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Extract token
	setCookie := w.Header().Get("Set-Cookie")
	cookieStart := strings.Index(setCookie, "session=") + len("session=")
	cookieEnd := strings.Index(setCookie[cookieStart:], ";")
	if cookieEnd == -1 {
		cookieEnd = len(setCookie)
	} else {
		cookieEnd = cookieStart + cookieEnd
	}
	token := setCookie[cookieStart:cookieEnd]

	// Wait for token to expire
	time.Sleep(2 * time.Millisecond)

	// Create request with expired token
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: token,
	})

	// Act
	_, err = sm.ValidateSession(req)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	sessErr, ok := err.(SessionError)
	if !ok {
		t.Fatalf("expected SessionError, got %T", err)
	}
	if sessErr.Code != "expired" {
		t.Errorf("expected Code=expired, got %s", sessErr.Code)
	}
}

func TestRefreshSession_NoRefreshNeeded(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Create claims that are fresh (just issued)
	now := time.Now().UTC()
	freshClaims := auth.Claims{
		Subject:   "user-123",
		Email:     "alice@example.com",
		Roles:     []auth.Role{auth.RoleUser},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(24 * time.Hour).Unix(),
		Issuer:    "test-issuer",
		Audience:  []string{"test-api"},
	}

	w := httptest.NewRecorder()

	// Act
	cookie, newClaims, err := sm.RefreshSession(w, freshClaims)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookie != nil {
		t.Error("expected no refresh (cookie=nil), but got a cookie")
	}
	// Claims should be unchanged
	if newClaims.Subject != freshClaims.Subject {
		t.Errorf("claims changed unexpectedly")
	}
}

func TestRefreshSession_RefreshTriggered(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Create claims that are old (issued 13 hours ago)
	now := time.Now().UTC()
	thirteenHoursAgo := now.Add(-13 * time.Hour)
	oldClaims := auth.Claims{
		Subject:   "user-123",
		Email:     "alice@example.com",
		Roles:     []auth.Role{auth.RoleUser},
		IssuedAt:  thirteenHoursAgo.Unix(),
		ExpiresAt: thirteenHoursAgo.Add(24 * time.Hour).Unix(),
		Issuer:    "test-issuer",
		Audience:  []string{"test-api"},
	}

	w := httptest.NewRecorder()

	// Act
	cookie, newClaims, err := sm.RefreshSession(w, oldClaims)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cookie == nil {
		t.Fatal("expected cookie from refresh, got nil")
	}
	if cookie.Name != SessionCookieName {
		t.Errorf("expected cookie name %s, got %s", SessionCookieName, cookie.Name)
	}
	// New claims should have updated timestamps
	if newClaims.IssuedAt == oldClaims.IssuedAt {
		t.Error("expected new IssuedAt, got same as old")
	}
	if newClaims.ExpiresAt == oldClaims.ExpiresAt {
		t.Error("expected new ExpiresAt, got same as old")
	}
	if newClaims.ExpiresAt <= newClaims.IssuedAt {
		t.Error("expected ExpiresAt > IssuedAt")
	}
}

func TestDestroySession(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)
	w := httptest.NewRecorder()

	// Act
	err := sm.DestroySession(w)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check Set-Cookie header
	// Note: Go's http.Cookie serialization converts MaxAge=-1 to "Max-Age=0"
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, SessionCookieName) {
		t.Errorf("expected Set-Cookie to contain %s", SessionCookieName)
	}
	if !strings.Contains(setCookie, "Max-Age=0") {
		t.Error("expected Max-Age=0 for cookie deletion")
	}
}

func TestGenerateCSRFSecret(t *testing.T) {
	// Act
	secret, err := GenerateCSRFSecret()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if len(secret) != 64 { // 32 bytes * 2 hex chars
		t.Errorf("expected length 64, got %d", len(secret))
	}

	// Generate another and verify they're different (random)
	secret2, _ := GenerateCSRFSecret()
	if secret == secret2 {
		t.Error("expected different secrets, got same")
	}
}

func TestDeriveCSRFSecret_Deterministic(t *testing.T) {
	// Arrange
	masterSecret := make([]byte, 32)
	copy(masterSecret, "master-secret-32-bytes-long-----")
	userID := "user-123"

	// Act
	secret1 := DeriveCSRFSecret(userID, masterSecret)
	secret2 := DeriveCSRFSecret(userID, masterSecret)

	// Assert
	if secret1 != secret2 {
		t.Error("expected same secret for same inputs, got different")
	}
	if len(secret1) != 64 {
		t.Errorf("expected length 64, got %d", len(secret1))
	}

	// Different user ID should produce different secret
	secret3 := DeriveCSRFSecret("user-456", masterSecret)
	if secret1 == secret3 {
		t.Error("expected different secret for different user, got same")
	}
}

func TestSessionMiddleware_Valid(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Create session
	w := httptest.NewRecorder()
	sm.CreateSession(w, "user-123", "alice@example.com", []auth.Role{auth.RoleUser})

	// Extract token
	setCookie := w.Header().Get("Set-Cookie")
	cookieStart := strings.Index(setCookie, "session=") + len("session=")
	cookieEnd := strings.Index(setCookie[cookieStart:], ";")
	if cookieEnd == -1 {
		cookieEnd = len(setCookie)
	} else {
		cookieEnd = cookieStart + cookieEnd
	}
	token := setCookie[cookieStart:cookieEnd]

	// Create handler that checks context
	nextCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		claims, ok := SessionClaimsFrom(r.Context())
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims.Subject != "user-123" {
			t.Errorf("expected Subject=user-123, got %s", claims.Subject)
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := SessionMiddleware(sm)
	handler := middleware(testHandler)

	// Create request with session cookie
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: token,
	})
	w = httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSessionMiddleware_MissingCookie(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	nextCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	middleware := SessionMiddleware(sm)
	handler := middleware(testHandler)

	// Request without session cookie
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if nextCalled {
		t.Fatal("expected next handler NOT to be called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSessionAndBearerMiddleware_SessionValid(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Create session
	w := httptest.NewRecorder()
	sm.CreateSession(w, "user-123", "alice@example.com", []auth.Role{auth.RoleUser})

	// Extract token
	setCookie := w.Header().Get("Set-Cookie")
	cookieStart := strings.Index(setCookie, "session=") + len("session=")
	cookieEnd := strings.Index(setCookie[cookieStart:], ";")
	if cookieEnd == -1 {
		cookieEnd = len(setCookie)
	} else {
		cookieEnd = cookieStart + cookieEnd
	}
	token := setCookie[cookieStart:cookieEnd]

	nextCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		claims, ok := ClaimsFrom(r.Context())
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims.Subject != "user-123" {
			t.Errorf("expected Subject=user-123, got %s", claims.Subject)
		}
	})

	middleware := SessionAndBearerMiddleware(sm, issuer)
	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: token,
	})
	w = httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestSessionAndBearerMiddleware_BearerFallback(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}
	sm := NewSessionManager(issuer, true)

	// Create a bearer token
	bearerToken, _, _ := issuer.Issue("user-456", "bob@example.com", []auth.Role{auth.RoleUser})

	nextCalled := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		claims, ok := ClaimsFrom(r.Context())
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims.Subject != "user-456" {
			t.Errorf("expected Subject=user-456, got %s", claims.Subject)
		}
	})

	middleware := SessionAndBearerMiddleware(sm, issuer)
	handler := middleware(testHandler)

	// Request with bearer token, no session cookie
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestNewSessionManager_Defaults(t *testing.T) {
	// Arrange
	secret := make([]byte, 32)
	copy(secret, "test-secret-32-bytes-long------")
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "test-issuer",
		Audience: []string{"test-api"},
		TTL:      24 * time.Hour,
	}

	// Act
	sm := NewSessionManager(issuer, true)

	// Assert
	if sm.Secure != true {
		t.Error("expected Secure=true")
	}
	if sm.SameSite != "Strict" {
		t.Errorf("expected SameSite=Strict, got %s", sm.SameSite)
	}
	if sm.CookieTTL != 24*time.Hour {
		t.Errorf("expected CookieTTL=24h, got %v", sm.CookieTTL)
	}

	// Test insecure mode
	sm2 := NewSessionManager(issuer, false)
	if sm2.Secure != false {
		t.Error("expected Secure=false")
	}
	if sm2.SameSite != "Lax" {
		t.Errorf("expected SameSite=Lax, got %s", sm2.SameSite)
	}
}

func TestWithSessionClaims_Roundtrip(t *testing.T) {
	// Arrange
	claims := auth.Claims{
		Subject:   "user-123",
		Email:     "alice@example.com",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}
	ctx := context.Background()

	// Act
	ctx = WithSessionClaims(ctx, claims)
	retrieved, ok := SessionClaimsFrom(ctx)

	// Assert
	if !ok {
		t.Fatal("expected claims to be present in context")
	}
	if retrieved.Subject != claims.Subject {
		t.Errorf("expected Subject=%s, got %s", claims.Subject, retrieved.Subject)
	}
	if retrieved.Email != claims.Email {
		t.Errorf("expected Email=%s, got %s", claims.Email, retrieved.Email)
	}
}

func TestSessionError_String(t *testing.T) {
	// Arrange
	err := SessionError{
		Code:   "missing_cookie",
		Reason: "session cookie not found",
	}

	// Act
	msg := err.Error()

	// Assert
	if !strings.Contains(msg, "missing_cookie") {
		t.Errorf("expected error to contain code, got %s", msg)
	}
	if !strings.Contains(msg, "session cookie not found") {
		t.Errorf("expected error to contain reason, got %s", msg)
	}
}
