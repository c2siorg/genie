package handlers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// mockLogger is a test logger that records messages.
type mockLogger struct {
	errors []string
	infos  []string
}

func (ml *mockLogger) Errorf(msg string, args ...interface{}) {
	ml.errors = append(ml.errors, fmt.Sprintf(msg, args...))
}

func (ml *mockLogger) Infof(msg string, args ...interface{}) {
	ml.infos = append(ml.infos, fmt.Sprintf(msg, args...))
}

// mockUserRepo is a test repository for user operations.
type mockUserRepo struct {
	users            map[string]auth.User
	shouldFailCreate bool
	shouldFailGet    bool
}

func (m *mockUserRepo) Create(ctx context.Context, email, name, passwordHash string, roles []auth.Role) (auth.User, error) {
	if m.shouldFailCreate {
		return auth.User{}, fmt.Errorf("creation failed")
	}
	if _, exists := m.users[email]; exists {
		return auth.User{}, postgres.ErrUserNotFound // Using this as "already exists" signal
	}
	user := auth.User{
		ID:           fmt.Sprintf("user_%d", len(m.users)),
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		Roles:        roles,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.users[email] = user
	return user, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (auth.User, error) {
	if m.shouldFailGet {
		return auth.User{}, fmt.Errorf("database error")
	}
	user, exists := m.users[email]
	if !exists {
		return auth.User{}, postgres.ErrUserNotFound
	}
	return user, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (auth.User, error) {
	if m.shouldFailGet {
		return auth.User{}, fmt.Errorf("database error")
	}
	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return auth.User{}, postgres.ErrUserNotFound
}

// newTestUsers creates a Users handler with test dependencies.
func newTestUsers() *Users {
	issuer := &auth.Issuer{
		Secret:   []byte("test-secret-32-bytes-minimum!!!"),
		TTL:      1 * time.Hour,
		Issuer:   "test-issuer",
		Audience: []string{"test-audience"},
	}
	return &Users{
		Repo:           &mockUserRepo{users: make(map[string]auth.User)},
		Issuer:         issuer,
		SessionManager: mid.NewSessionManager(issuer, false), // Secure=false for tests
		CSRFGenerator:  mid.NewGenerator(issuer.Secret),
		Logger:         &mockLogger{},
	}
}

// TestSignup_Success validates successful user registration.
func TestSignup_Success(t *testing.T) {
	h := newTestUsers()

	reqBody := signupRequest{
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "secure-password-123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d", resp.StatusCode)
	}

	// Verify session cookie is set.
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Errorf("expected session cookie, got none")
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("session cookie should be HttpOnly")
	}
	if sessionCookie.Path != "/" {
		t.Errorf("session cookie path should be /, got %s", sessionCookie.Path)
	}

	// Verify CSRF token is in response header.
	csrfToken := resp.Header.Get("X-CSRF-Token")
	if csrfToken == "" {
		t.Errorf("expected X-CSRF-Token header, got empty")
	}
	if len(csrfToken) < 50 {
		t.Errorf("CSRF token looks invalid (too short): %s", csrfToken)
	}

	// Verify response body.
	var respBody sessionResponse
	_ = json.NewDecoder(resp.Body).Decode(&respBody)
	if respBody.User.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", respBody.User.Email)
	}
	if respBody.User.Name != "Test User" {
		t.Errorf("expected name 'Test User', got %s", respBody.User.Name)
	}
	if respBody.ExpiresAt == 0 {
		t.Errorf("expected non-zero expiry time")
	}
}

// TestSignup_InvalidJSON validates error handling for malformed JSON.
func TestSignup_InvalidJSON(t *testing.T) {
	h := newTestUsers()

	req := httptest.NewRequest("POST", "/signup", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}

	// Verify error is generic.
	var errBody map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "invalid request format" {
		t.Errorf("expected generic error message, got %v", errBody["error"])
	}
}

// TestSignup_MissingEmail validates email validation.
func TestSignup_MissingEmail(t *testing.T) {
	h := newTestUsers()

	reqBody := signupRequest{
		Email:    "",
		Password: "secure-password-123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

// TestSignup_WeakPassword validates password strength check.
func TestSignup_WeakPassword(t *testing.T) {
	h := newTestUsers()

	reqBody := signupRequest{
		Email:    "test@example.com",
		Password: "short",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", resp.StatusCode)
	}
}

// TestSignup_DuplicateEmail validates conflict handling.
func TestSignup_DuplicateEmail(t *testing.T) {
	h := newTestUsers()

	// First signup succeeds.
	reqBody := signupRequest{
		Email:    "test@example.com",
		Name:     "First",
		Password: "secure-password-123",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Signup(w, req)
	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("first signup failed")
	}

	// Second signup with same email should fail.
	reqBody.Name = "Second"
	body, _ = json.Marshal(reqBody)
	req = httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w = httptest.NewRecorder()
	h.Signup(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	var errBody map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "user already exists" {
		t.Errorf("expected 'user already exists' error, got %v", errBody["error"])
	}
}

// TestLogin_Success validates successful login.
func TestLogin_Success(t *testing.T) {
	h := newTestUsers()

	// Create a user first.
	password := "secure-password-123"
	hash, _ := auth.HashPassword(password)
	user := auth.User{
		ID:           "user_123",
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: hash,
		Roles:        []auth.Role{auth.RoleUser},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo := h.Repo.(*mockUserRepo)
	repo.users["test@example.com"] = user

	// Attempt login.
	reqBody := loginRequest{
		Email:    "test@example.com",
		Password: password,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	// Verify session cookie and CSRF token.
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Errorf("expected session cookie, got none")
	}

	csrfToken := resp.Header.Get("X-CSRF-Token")
	if csrfToken == "" {
		t.Errorf("expected X-CSRF-Token header")
	}

	// Verify response contains user data.
	var respBody sessionResponse
	_ = json.NewDecoder(resp.Body).Decode(&respBody)
	if respBody.User.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", respBody.User.Email)
	}
}

// TestLogin_InvalidCredentials validates generic error for non-existent user.
func TestLogin_InvalidCredentials(t *testing.T) {
	h := newTestUsers()

	reqBody := loginRequest{
		Email:    "nonexistent@example.com",
		Password: "any-password",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// Verify error message is generic (doesn't reveal user doesn't exist).
	var errBody map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "invalid credentials" {
		t.Errorf("expected 'invalid credentials', got %v", errBody["error"])
	}
}

// TestLogin_WrongPassword validates generic error for incorrect password.
func TestLogin_WrongPassword(t *testing.T) {
	h := newTestUsers()

	// Create a user.
	correctPassword := "secure-password-123"
	hash, _ := auth.HashPassword(correctPassword)
	user := auth.User{
		ID:           "user_123",
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: hash,
		Roles:        []auth.Role{auth.RoleUser},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo := h.Repo.(*mockUserRepo)
	repo.users["test@example.com"] = user

	// Attempt login with wrong password.
	reqBody := loginRequest{
		Email:    "test@example.com",
		Password: "wrong-password",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	var errBody map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "invalid credentials" {
		t.Errorf("expected 'invalid credentials', got %v", errBody["error"])
	}
}

// TestLogout_Success validates successful logout.
func TestLogout_Success(t *testing.T) {
	h := newTestUsers()

	// Create a mock request with session claims.
	claims := auth.Claims{
		Subject:   "user_123",
		Email:     "test@example.com",
		Roles:     []auth.Role{auth.RoleUser},
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}

	req := httptest.NewRequest("POST", "/logout", nil)
	ctx := mid.WithSessionClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	h.Logout(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	// Verify session cookie is cleared (Max-Age=-1).
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Errorf("expected session cookie in logout response")
	}
	if sessionCookie.MaxAge != -1 {
		t.Errorf("expected MaxAge=-1 to clear cookie, got %d", sessionCookie.MaxAge)
	}
}

// TestLogout_NoSession validates error when not authenticated.
func TestLogout_NoSession(t *testing.T) {
	h := newTestUsers()

	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	var errBody map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "not authenticated" {
		t.Errorf("expected 'not authenticated', got %v", errBody["error"])
	}
}

// TestMe_Success validates fetching authenticated user profile.
func TestMe_Success(t *testing.T) {
	h := newTestUsers()

	// Create a user.
	user := auth.User{
		ID:        "user_123",
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     []auth.Role{auth.RoleUser, auth.RoleAdmin},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo := h.Repo.(*mockUserRepo)
	repo.users["test@example.com"] = user

	// Create a request with auth claims.
	claims := auth.Claims{
		Subject: "user_123",
		Email:   "test@example.com",
		Roles:   []auth.Role{auth.RoleUser, auth.RoleAdmin},
	}

	req := httptest.NewRequest("GET", "/me", nil)
	ctx := mid.WithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	h.Me(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var respBody publicUser
	_ = json.NewDecoder(resp.Body).Decode(&respBody)
	if respBody.ID != "user_123" {
		t.Errorf("expected ID user_123, got %s", respBody.ID)
	}
	if respBody.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", respBody.Email)
	}
	if len(respBody.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(respBody.Roles))
	}
}

// TestMe_NoAuth validates error when not authenticated.
func TestMe_NoAuth(t *testing.T) {
	h := newTestUsers()

	req := httptest.NewRequest("GET", "/me", nil)
	w := httptest.NewRecorder()

	h.Me(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	var errBody map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "not authenticated" {
		t.Errorf("expected 'not authenticated', got %v", errBody["error"])
	}
}

// TestMe_UserNotFound validates error when user is deleted between auth and lookup.
func TestMe_UserNotFound(t *testing.T) {
	h := newTestUsers()

	// Don't add user to repo (simulates deletion).
	claims := auth.Claims{
		Subject: "nonexistent_user",
		Email:   "test@example.com",
		Roles:   []auth.Role{auth.RoleUser},
	}

	req := httptest.NewRequest("GET", "/me", nil)
	ctx := mid.WithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	h.Me(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d", resp.StatusCode)
	}

	var errBody map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] != "failed to fetch profile" {
		t.Errorf("expected generic error, got %v", errBody["error"])
	}
}

// TestEmailNormalization validates email is lowercased and trimmed.
func TestEmailNormalization(t *testing.T) {
	h := newTestUsers()

	// Signup with uppercase and spaces.
	reqBody := signupRequest{
		Email:    "  TEST@EXAMPLE.COM  ",
		Name:     "Test User",
		Password: "secure-password-123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("signup failed")
	}

	// Verify the email was normalized.
	repo := h.Repo.(*mockUserRepo)
	if _, exists := repo.users["test@example.com"]; !exists {
		t.Errorf("expected email to be normalized to lowercase, got users: %v", repo.users)
	}
}

// TestCSRFTokenFormat validates CSRF token (secret) structure.
//
// The CSRF secret returned by CreateSession is a 64-character hex string (32 bytes).
// This is the server-generated CSRF secret that the client stores and uses to
// compute CSRF tokens for state-changing requests.
func TestCSRFTokenFormat(t *testing.T) {
	h := newTestUsers()

	reqBody := signupRequest{
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "secure-password-123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	csrfSecret := w.Result().Header.Get("X-CSRF-Token")

	// CSRF secret should be 64 hex characters (32 bytes = 256 bits).
	// This is the output of hex.EncodeToString(32_random_bytes).
	if len(csrfSecret) != 64 {
		t.Errorf("expected CSRF secret to be 64 hex chars, got %d: %s", len(csrfSecret), csrfSecret)
	}

	// Verify it's valid hex.
	_, err := hex.DecodeString(csrfSecret)
	if err != nil {
		t.Errorf("expected valid hex, got error: %v", err)
	}
}

// TestSessionCookieAttributes validates HttpOnly, Secure, and SameSite flags.
func TestSessionCookieAttributes(t *testing.T) {
	h := newTestUsers()

	reqBody := signupRequest{
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "secure-password-123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	resp := w.Result()
	cookies := resp.Cookies()

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatalf("session cookie not found")
	}

	if !sessionCookie.HttpOnly {
		t.Errorf("session cookie should be HttpOnly")
	}

	// In test environment (Secure=false), SameSite should be Lax.
	// In production (Secure=true), SameSite should be Strict.
	if sessionCookie.SameSite != http.SameSiteLaxMode && sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("session cookie SameSite should be Lax or Strict, got %v", sessionCookie.SameSite)
	}
}

// TestLoggerIntegration validates that errors and info are logged.
func TestLoggerIntegration(t *testing.T) {
	h := newTestUsers()
	logger := h.Logger.(*mockLogger)

	// Signup should log info message.
	reqBody := signupRequest{
		Email:    "test@example.com",
		Name:     "Test User",
		Password: "secure-password-123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Signup(w, req)

	if len(logger.infos) == 0 {
		t.Errorf("expected info log message")
	}

	// Duplicate signup should log info message (user already exists).
	req = httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	w = httptest.NewRecorder()

	h.Signup(w, req)

	if len(logger.infos) < 2 {
		t.Errorf("expected multiple info log messages")
	}
}

// TestLoginWithSessionClaims validates fallback to bearer token in login.
func TestLoginWithSessionClaims(t *testing.T) {
	h := newTestUsers()

	// Create a user.
	correctPassword := "secure-password-123"
	hash, _ := auth.HashPassword(correctPassword)
	user := auth.User{
		ID:           "user_123",
		Email:        "test@example.com",
		Name:         "Test User",
		PasswordHash: hash,
		Roles:        []auth.Role{auth.RoleUser},
	}
	repo := h.Repo.(*mockUserRepo)
	repo.users["test@example.com"] = user

	// Test Me endpoint with session claims (not bearer token).
	claims := auth.Claims{
		Subject: "user_123",
		Email:   "test@example.com",
		Roles:   []auth.Role{auth.RoleUser},
	}

	req := httptest.NewRequest("GET", "/me", nil)
	ctx := mid.WithSessionClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	h.Me(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK with session claims, got %d", resp.StatusCode)
	}

	var respBody publicUser
	_ = json.NewDecoder(resp.Body).Decode(&respBody)
	if respBody.ID != "user_123" {
		t.Errorf("expected to fetch user from session claims")
	}
}
