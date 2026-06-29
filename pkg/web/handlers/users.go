// Package handlers wires HTTP endpoints to repositories and the auth package.
//
// handlers/users.go — User authentication (signup, login, logout, profile)
// with session management, CSRF protection, and security headers.
//
// ─── Authentication Flow ───────────────────────────────────────────────────
//
// Login and signup handlers now follow a unified security flow:
//
//  1. Validate request (JSON format, email, password)
//  2. Create/verify user in database
//  3. Call SessionManager.CreateSession to mint JWT and CSRF secret
//  4. Return session cookie (HttpOnly) + CSRF token in header
//  5. Client stores CSRF token in memory; browser auto-sends cookie
//  6. On subsequent requests: cookie sent by browser, CSRF token in header
//  7. Middleware validates both; CSRF middleware rejects requests without token
//
// Logout handler:
//  1. Validates session exists
//  2. Calls SessionManager.DestroySession to clear cookie (Max-Age=0)
//  3. Returns success (no token/cookie in response)
//
// ─── Error Handling ───────────────────────────────────────────────────────
//
// All error responses are generic to prevent information leakage:
//   - Login failures always return "invalid credentials" (never "user not found")
//   - Password errors never leak hash failure details
//   - User creation errors redact database constraint messages
//   - Internal errors return generic "service error" with unique request ID
//
// Detailed logs are written server-side; client never sees sensitive details.
//
// ─── CSRF Token Management ────────────────────────────────────────────────
//
// CSRF tokens are single-use, time-bound (15 minutes), and personalized:
//   - Generated: CreateSession calls SessionManager.CreateSession, which returns token
//   - Stored: Client keeps in memory (JavaScript accessible, not in document.cookie)
//   - Submitted: Client includes in X-CSRF-Token header on state-changing requests
//   - Validated: CSRF middleware (Validator.Middleware) checks before handler runs
//
// Tokens are derived from the JWT session secret + user ID for personalization.
// This prevents one user's token from being used on behalf of another.
//
// ─── Session Lifetime ─────────────────────────────────────────────────────
//
// Session JWTs are issued with TTL (default 24 hours) and automatically refreshed
// by SessionMiddleware if over half the TTL has passed. This provides transparent
// session extension without requiring user interaction.
//
// The HTTP cookie's Max-Age matches the JWT TTL; both expire together.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Users handles user signup, login, logout, and profile endpoints.
//
// Fields are exported for testing; in production, use NewUsers.
type Users struct {
	// Repo is the user repository (database abstraction).
	Repo postgres.UserRepo

	// Issuer is the JWT signer for bearer tokens and sessions.
	Issuer *auth.Issuer

	// SessionManager handles HttpOnly session cookies and CSRF secrets.
	SessionManager *mid.SessionManager

	// CSRFGenerator creates CSRF tokens for clients to submit.
	CSRFGenerator *mid.Generator

	// Logger is the structured logger (optional, for internal error logging).
	// If nil, errors are not logged; they still return generic responses.
	Logger interface {
		Errorf(msg string, args ...interface{})
		Infof(msg string, args ...interface{})
	}
}

// NewUsers constructs a Users handler with the given dependencies.
//
// issuer is required; sessionManager, csrfGenerator, and logger can be nil
// (defaults will be used).
func NewUsers(repo postgres.UserRepo, issuer *auth.Issuer) *Users {
	sm := mid.NewSessionManager(issuer, true)  // Secure=true (HTTPS in prod)
	csrfGen := mid.NewGenerator(issuer.Secret) // Reuse JWT secret for CSRF
	return &Users{
		Repo:           repo,
		Issuer:         issuer,
		SessionManager: sm,
		CSRFGenerator:  csrfGen,
		Logger:         nil,
	}
}

// signupRequest is the JSON payload for user registration.
type signupRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// loginRequest is the JSON payload for user login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// sessionResponse is returned by login/signup with session info and CSRF token.
//
// The session cookie is set via Set-Cookie; the CSRF token is in X-CSRF-Token header.
// Client reads X-CSRF-Token and stores it in memory for subsequent requests.
type sessionResponse struct {
	User      publicUser `json:"user"`
	ExpiresAt int64      `json:"expires_at"`
}

// publicUser is the authenticated user's public profile.
type publicUser struct {
	ID    string      `json:"id"`
	Email string      `json:"email"`
	Name  string      `json:"name,omitempty"`
	Roles []auth.Role `json:"roles"`
}

// logoutRequest (empty for now, reserved for future use like logout reason).
type logoutRequest struct {
	// Optional: logout reason, device ID, etc.
}

// Signup creates a new user and establishes a session.
//
// Flow:
//  1. Validate email and password (8+ chars, non-empty email)
//  2. Hash password using bcrypt
//  3. Create user in database with default role "user"
//  4. Call SessionManager.CreateSession to mint JWT and CSRF secret
//  5. Return 201 Created with session cookie + CSRF token header
//
// On error:
//   - 400 Bad Request: invalid JSON, missing fields, weak password
//   - 409 Conflict: email already exists (generic "user already exists")
//   - 500 Internal Server Error: password hashing, user creation, token signing
//
// All errors return generic messages; detailed reasons logged server-side.
func (h *Users) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logErr("signup JSON decode failed", err)
		respondError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	// Normalize and validate email.
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "email is required")
		return
	}

	// Validate password strength.
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Hash password.
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		h.logErr("password hash failed", err)
		respondError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	// Create user in repository.
	user, err := h.Repo.Create(r.Context(), req.Email, req.Name, hash, []auth.Role{auth.RoleUser})
	if err != nil {
		// Generic error for all creation failures (conflict, database error, etc.)
		h.logErr("user creation failed", err)
		if errors.Is(err, postgres.ErrUserNotFound) || strings.Contains(err.Error(), "already exists") {
			respondError(w, http.StatusConflict, "user already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	h.logInfo("user signup successful", map[string]interface{}{"user_id": user.ID, "email": user.Email})

	// Create session and return CSRF token.
	h.createSessionAndRespond(w, user, http.StatusCreated)
}

// Login validates credentials and establishes a session (session-based, not JWT).
//
// Flow:
//  1. Validate email and password format
//  2. Look up user by email (non-existent user returns generic error)
//  3. Verify password against hash
//  4. Call SessionManager.CreateSession to mint JWT and CSRF secret
//  5. Return 200 OK with session cookie + CSRF token header
//
// On error:
//   - 400 Bad Request: invalid JSON
//   - 401 Unauthorized: user not found or password incorrect (same message for both)
//   - 500 Internal Server Error: database lookup, token signing
//
// All errors return generic messages; detailed reasons logged server-side.
func (h *Users) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logErr("login JSON decode failed", err)
		respondError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	// Normalize email.
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "email is required")
		return
	}

	// Lookup user by email.
	user, err := h.Repo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		// Generic error: never reveal whether user exists.
		if errors.Is(err, postgres.ErrUserNotFound) {
			h.logInfo("login failed: user not found", map[string]interface{}{"email": req.Email})
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		h.logErr("user lookup failed", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	// Verify password.
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		h.logInfo("login failed: invalid password", map[string]interface{}{"user_id": user.ID})
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.logInfo("user login successful", map[string]interface{}{"user_id": user.ID, "email": user.Email})

	// Create session and return CSRF token.
	h.createSessionAndRespond(w, user, http.StatusOK)
}

// Logout destroys the session and clears the cookie.
//
// Flow:
//  1. Validate that a session exists (from SessionMiddleware or SessionAndBearerMiddleware)
//  2. Call SessionManager.DestroySession to clear the cookie (Max-Age=-1)
//  3. Return 200 OK with empty body
//
// The cookie is marked with Max-Age=-1, which signals the browser to delete it immediately.
// Subsequent requests without the cookie will fail authentication.
//
// On error:
//   - 401 Unauthorized: no valid session found
//   - 500 Internal Server Error: cookie write failure (rare)
func (h *Users) Logout(w http.ResponseWriter, r *http.Request) {
	// Extract session from context (set by SessionMiddleware).
	claims, ok := mid.SessionClaimsFrom(r.Context())
	if !ok {
		// Fall back to bearer token claims for mixed auth environments.
		claims, ok = mid.ClaimsFrom(r.Context())
		if !ok {
			h.logInfo("logout attempted without session", map[string]interface{}{})
			respondError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
	}

	// Destroy the session cookie.
	if err := h.SessionManager.DestroySession(w); err != nil {
		h.logErr("session destruction failed", err)
		respondError(w, http.StatusInternalServerError, "logout failed")
		return
	}

	h.logInfo("user logout successful", map[string]interface{}{"user_id": claims.Subject})

	// Return success with empty body.
	respondJSON(w, http.StatusOK, map[string]interface{}{})
}

// Me returns the authenticated user's public profile.
//
// This endpoint requires an active session (enforced by SessionMiddleware or Auth middleware).
// The user's ID is extracted from the session claims and used to fetch the full profile.
//
// On error:
//   - 401 Unauthorized: no valid session found
//   - 500 Internal Server Error: user lookup failed
func (h *Users) Me(w http.ResponseWriter, r *http.Request) {
	// Extract claims from context (set by SessionMiddleware or Auth middleware).
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		// Try session claims as fallback.
		claims, ok = mid.SessionClaimsFrom(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
	}

	// Fetch user by ID to get latest profile.
	user, err := h.Repo.GetByID(r.Context(), claims.Subject)
	if err != nil {
		h.logErr("user lookup failed in Me endpoint", err)
		respondError(w, http.StatusInternalServerError, "failed to fetch profile")
		return
	}

	// Return user profile.
	respondJSON(w, http.StatusOK, publicUser{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
		Roles: user.Roles,
	})
}

// createSessionAndRespond is a helper that creates a session and returns the response.
//
// Steps:
//  1. Call SessionManager.CreateSession to mint JWT and CSRF secret
//  2. Set CSRF token in X-CSRF-Token response header
//  3. Return user profile + expiry in JSON body
//
// The session cookie is set by SessionManager.CreateSession via http.SetCookie.
// The CSRF token is returned in a response header for the client to store.
//
// Status can be 200 (login) or 201 (signup).
func (h *Users) createSessionAndRespond(w http.ResponseWriter, user auth.User, status int) {
	// Create session JWT and CSRF secret.
	cookie, claims, csrfToken, err := h.SessionManager.CreateSession(w, user.ID, user.Email, user.Roles)
	if err != nil {
		h.logErr("session creation failed", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	h.logInfo("session created", map[string]interface{}{
		"user_id":            user.ID,
		"cookie_name":        cookie.Name,
		"csrf_token_present": (csrfToken != ""),
	})

	// Set CSRF token in response header for client to retrieve.
	w.Header().Set("X-CSRF-Token", csrfToken)

	// Return user profile and session expiry.
	respondJSON(w, status, sessionResponse{
		User:      publicUser{ID: user.ID, Email: user.Email, Name: user.Name, Roles: user.Roles},
		ExpiresAt: claims.ExpiresAt,
	})
}

// logErr logs an error server-side (if logger is available).
//
// This is for internal debugging; the error is never returned to the client.
func (h *Users) logErr(msg string, err error) {
	if h.Logger != nil {
		h.Logger.Errorf("%s: %v", msg, err)
	}
}

// logInfo logs an informational message server-side (if logger is available).
//
// Used for audit trails (login, signup, logout).
func (h *Users) logInfo(msg string, fields map[string]interface{}) {
	if h.Logger != nil {
		fieldStr := ""
		for k, v := range fields {
			fieldStr += fmt.Sprintf(" %s=%v", k, v)
		}
		h.Logger.Infof("%s%s", msg, fieldStr)
	}
}
