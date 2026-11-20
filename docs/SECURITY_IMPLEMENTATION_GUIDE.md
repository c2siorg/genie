# Genie Phase 2: Security Implementation Guide

**Status**: Implementation Ready  
**Date**: June 4, 2026  
**Package**: `github.com/c2siorg/genie/pkg/security`  

---

## Quick Start

### 1. Enable CSRF Protection in Router

**File**: `pkg/web/router.go`

```go
import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/security"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Layer 1: Security Headers (outermost).
	r.Use(security.SecurityHeaders(security.DefaultSecurityHeadersConfig()))

	// Layer 2: Request context (trace ID, logging).
	r.Use(mid.RequestID)
	r.Use(mid.Recovery(d.Logger))
	r.Use(mid.AccessLog(d.Logger))

	// Public routes.
	r.Group(func(r chi.Router) {
		r.Get("/healthz", d.Health.Live)
		r.Post("/v1/users", d.Users.Signup)
		r.Post("/v1/users/login", d.Users.Login)
	})

	// Authenticated + CSRF-protected routes.
	r.Group(func(r chi.Router) {
		// Layer 3: Authentication (JWT from cookie or header).
		r.Use(mid.Auth(d.Issuer))

		// Layer 4: CSRF Protection (token validation + rotation).
		r.Use(security.CSRF(d.CSRFSvc, d.Logger))

		r.Route("/v1", func(r chi.Router) {
			// These routes are now protected against CSRF attacks.
			r.Post("/accounts", d.Accounts.Create)
			r.Post("/payment/initiate", d.Payment.InitiatePayment)
			r.Put("/merchant/{id}/limits", d.Merchant.UpdateLimits)
			r.Delete("/consent/{id}", d.Consent.RevokeConsent)
		})
	})

	return r
}
```

### 2. Initialize CSRF Service in main.go

**File**: `cmd/api/main.go`

```go
import (
	"encoding/hex"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/security"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web"
)

func run() error {
	// ... existing setup ...

	// Load CSRF service from environment.
	csrfSecretHex := os.Getenv("GENIE_CSRF_SECRET")
	if csrfSecretHex == "" {
		return errors.New("GENIE_CSRF_SECRET environment variable required")
	}

	csrfSecret, err := hex.DecodeString(csrfSecretHex)
	if err != nil {
		return fmt.Errorf("GENIE_CSRF_SECRET must be hex-encoded: %w", err)
	}

	csrfCfg := security.DefaultCSRFConfig()
	csrfCfg.ServerSecret = csrfSecret
	csrfSvc := security.NewCSRFService(csrfCfg)

	// Build dependencies.
	deps := web.Deps{
		Issuer:   issuer,
		Logger:   logger,
		CSRFSvc:  csrfSvc, // Add CSRF service to deps
		Users:    &handlers.Users{Repo: userRepo, Issuer: issuer, CSRFService: csrfSvc},
		// ... other handlers ...
	}

	// Create router.
	handler := web.NewRouter(deps)

	// Start server.
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

### 3. Update User Handler to Issue CSRF Token at Login

**File**: `pkg/web/handlers/users.go`

```go
import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/security"
)

type Users struct {
	Repo          postgres.UserRepo
	Issuer        *auth.Issuer
	CSRFService   *security.CSRFService // New field
	ErrSink       *security.ErrorSink    // New field
}

// Login validates credentials, issues JWT, and returns CSRF token.
func (h *Users) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.ErrSink.BadRequest(w, r, "invalid json")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		h.ErrSink.BadRequest(w, r, "email and 8+ char password required")
		return
	}

	user, err := h.Repo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		h.ErrSink.Unauthorized(w, r, "invalid credentials")
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		h.ErrSink.Unauthorized(w, r, "invalid credentials")
		return
	}

	// Issue JWT with extended claims.
	tok, claims, err := h.Issuer.Issue(user.ID, user.Email, user.Roles)
	if err != nil {
		h.ErrSink.ServerError(w, r, fmt.Errorf("token issuance: %w", err))
		return
	}

	// Set HttpOnly session cookie (JWT is not sent in response body).
	cookie := &http.Cookie{
		Name:     "genie-session",
		Value:    tok,
		MaxAge:   3600,           // 1 hour
		Path:     "/",
		HttpOnly: true,           // Prevent JavaScript access
		Secure:   true,           // HTTPS-only
		SameSite: http.SameSiteLax, // CSRF mitigation
	}
	http.SetCookie(w, cookie)

	// Issue first CSRF token (client will include in subsequent requests).
	csrfTokenStr, _, err := h.CSRFService.GenerateToken(user.ID)
	if err != nil {
		h.ErrSink.ServerError(w, r, fmt.Errorf("csrf token issuance: %w", err))
		return
	}

	// Return CSRF token in response header (client stores in memory).
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

// Logout destroys the session cookie.
func (h *Users) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		h.ErrSink.Unauthorized(w, r, "not authenticated")
		return
	}

	// Clear session cookie (MaxAge=-1 deletes it).
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
		"user_id": claims.UserID,
	})
}

// RefreshSession extends the session lifetime before expiry.
func (h *Users) RefreshSession(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		h.ErrSink.Unauthorized(w, r, "not authenticated")
		return
	}

	// Re-issue JWT with extended expiry.
	newTok, newClaims, err := h.Issuer.Issue(claims.UserID, claims.Email, claims.Roles)
	if err != nil {
		h.ErrSink.ServerError(w, r, fmt.Errorf("refresh failed: %w", err))
		return
	}

	// Update session cookie.
	cookie := &http.Cookie{
		Name:     "genie-session",
		Value:    newTok,
		MaxAge:   3600,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLax,
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

### 4. Environment Configuration

**File**: `.env` or deployment config

```bash
# CSRF Configuration
GENIE_CSRF_SECRET=<generate with: openssl rand -hex 32>

# Session Configuration
GENIE_SESSION_TTL=3600
GENIE_SESSION_SECURE=true
GENIE_SESSION_SAMESITE=Lax

# Existing configuration
GENIE_JWT_SECRET=<existing value>
GENIE_KEK_BASE64=<existing value>
GENIE_DB_DSN=<existing value>
```

**Generate CSRF Secret**:

```bash
# Generate a 32-byte hex-encoded secret
openssl rand -hex 32
# Output: a1b2c3d4e5f6...
```

---

## Client-Side Implementation

### 1. Login and Extract CSRF Token

```javascript
async function login(email, password) {
  const response = await fetch('/v1/users/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
    credentials: 'include', // Include session cookie
  });

  if (!response.ok) {
    throw new Error(`Login failed: ${response.status}`);
  }

  // Extract CSRF token from response header.
  const csrfToken = response.headers.get('X-CSRF-Token');
  if (!csrfToken) {
    throw new Error('CSRF token not provided');
  }

  // Store CSRF token in memory (NOT localStorage/sessionStorage).
  window.csrfToken = csrfToken;

  const data = await response.json();
  return data.user;
}
```

### 2. Include CSRF Token in State-Modifying Requests

```javascript
async function createAccount(accountData) {
  const response = await fetch('/v1/accounts', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': window.csrfToken, // Include CSRF token
    },
    body: JSON.stringify(accountData),
    credentials: 'include', // Include session cookie
  });

  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }

  // Extract new CSRF token from response header (token rotation).
  const newCsrfToken = response.headers.get('X-CSRF-Token');
  if (newCsrfToken) {
    window.csrfToken = newCsrfToken;
  }

  return await response.json();
}
```

### 3. Handle CSRF Token Expiry

```javascript
async function makeAuthorizedRequest(url, options = {}) {
  options.headers = options.headers || {};
  options.headers['X-CSRF-Token'] = window.csrfToken;
  options.credentials = 'include';

  let response = await fetch(url, options);

  if (response.status === 403) {
    // CSRF token expired or invalid.
    // Redirect to login (session likely expired too).
    window.location.href = '/login';
    return null;
  }

  if (response.ok) {
    // Update CSRF token from response (rotation).
    const newToken = response.headers.get('X-CSRF-Token');
    if (newToken) {
      window.csrfToken = newToken;
    }
  }

  return response;
}
```

### 4. Request Interceptor Pattern (Axios)

```javascript
import axios from 'axios';

const axiosClient = axios.create({
  baseURL: '/v1',
  withCredentials: true, // Include cookies
});

// Response interceptor: extract and update CSRF token.
axiosClient.interceptors.response.use(
  (response) => {
    const csrfToken = response.headers['x-csrf-token'];
    if (csrfToken) {
      window.csrfToken = csrfToken;
    }
    return response;
  },
  (error) => {
    if (error.response?.status === 403) {
      // CSRF or session failure.
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// Request interceptor: add CSRF token to state-modifying requests.
axiosClient.interceptors.request.use((config) => {
  if (['POST', 'PUT', 'DELETE', 'PATCH'].includes(config.method?.toUpperCase())) {
    config.headers['X-CSRF-Token'] = window.csrfToken;
  }
  return config;
});

export default axiosClient;
```

---

## Error Handling Integration

### 1. Wire ErrorSink in Handlers

```go
// In cmd/api/main.go
errSink := &security.ErrorSink{Logger: logger}

deps := web.Deps{
	Users: &handlers.Users{
		Repo:        userRepo,
		Issuer:      issuer,
		CSRFService: csrfSvc,
		ErrSink:     errSink, // Wire error sink
	},
	Accounts: &handlers.Accounts{
		Repo:    acctRepo,
		ErrSink: errSink,
	},
	// ... other handlers ...
}
```

### 2. Use ErrorSink in Handlers

```go
// In handlers/accounts.go
func (h *Accounts) Create(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Client sees: "invalid request" + trace ID.
		// Server logs: full JSON parse error + client IP + user agent.
		h.ErrSink.BadRequest(w, r, "invalid account data")
		return
	}

	// Validate business logic.
	if req.Balance < 0 {
		h.ErrSink.ValidationError(w, r, "balance must be non-negative")
		return
	}

	acct, err := h.Repo.Create(r.Context(), req.Name, req.Balance)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			// Duplicate account — 409 Conflict.
			h.ErrSink.ConflictError(w, r, "account name already exists")
		} else {
			// Unexpected database error — 500 Internal Server Error.
			// Client never sees the SQL error.
			h.ErrSink.ServerError(w, r, err)
		}
		return
	}

	respondJSON(w, http.StatusCreated, acct)
}
```

---

## Testing

### 1. Unit Tests

```bash
# Test CSRF service.
go test ./pkg/security -v -run TestCSRF

# Test error handling.
go test ./pkg/security -v -run TestError

# Test middleware integration.
go test ./pkg/web/mid -v
```

### 2. Integration Test

**File**: `pkg/security/integration_test.go`

```go
package security

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCSRFMiddleware_Integration tests end-to-end CSRF protection.
func TestCSRFMiddleware_Integration(t *testing.T) {
	// Setup: Create a simple handler that uses CSRF middleware.
	csrfCfg := DefaultCSRFConfig()
	csrfCfg.ServerSecret = []byte("test-secret-32-bytes-exactly!!!!")
	csrfSvc := NewCSRFService(csrfCfg)

	// Simulate authenticated request.
	userID := "user123"

	// Generate a valid token.
	validToken, _, err := csrfSvc.GenerateToken(userID)
	if err != nil {
		t.Fatalf("token generation failed: %v", err)
	}

	// Test 1: POST with valid CSRF token should succeed.
	req := httptest.NewRequest("POST", "/api/accounts", nil)
	req.Header.Set("X-CSRF-Token", validToken)
	// Simulate claims in context (would be set by Auth middleware).
	// ctx := mid.WithClaims(req.Context(), auth.Claims{UserID: userID})
	// req = req.WithContext(ctx)

	// ... validate request passes CSRF check ...

	// Test 2: POST without CSRF token should fail.
	req2 := httptest.NewRequest("POST", "/api/accounts", nil)
	// No X-CSRF-Token header.
	// ... validate request fails with 403 ...

	// Test 3: POST with invalid CSRF token should fail.
	req3 := httptest.NewRequest("POST", "/api/accounts", nil)
	req3.Header.Set("X-CSRF-Token", "invalid.token.123")
	// ... validate request fails with 403 ...

	// Test 4: GET request (safe method) should not require CSRF token.
	req4 := httptest.NewRequest("GET", "/api/accounts", nil)
	// No X-CSRF-Token header, but should succeed.
	// ... validate request passes ...
}
```

### 3. Manual Testing

```bash
# Step 1: Start the API server.
go run ./cmd/api

# Step 2: Login to get CSRF token.
curl -c cookies.txt -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}' \
  http://localhost:8080/v1/users/login

# The response header should contain X-CSRF-Token.
# The cookies.txt file should contain genie-session cookie.

# Step 3: Use CSRF token to create an account.
TOKEN=$(curl -b cookies.txt -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}' \
  http://localhost:8080/v1/users/login | jq -r '.headers."X-CSRF-Token"')

curl -b cookies.txt -H "Content-Type: application/json" \
  -H "X-CSRF-Token: $TOKEN" \
  -d '{"name":"My Account"}' \
  http://localhost:8080/v1/accounts

# Should respond with 201 Created.

# Step 4: Try without CSRF token (should fail).
curl -b cookies.txt -H "Content-Type: application/json" \
  -d '{"name":"My Account"}' \
  http://localhost:8080/v1/accounts

# Should respond with 403 Forbidden.
```

---

## Security Checklist

Before deploying to production:

- [ ] CSRF secret (GENIE_CSRF_SECRET) is 32+ bytes, random, loaded from env (NOT in code)
- [ ] Session cookie: HttpOnly=true, Secure=true, SameSite=Strict/Lax
- [ ] Auth middleware accepts JWT from cookie (preferred) AND Authorization header
- [ ] CSRF middleware is wired after Auth middleware in router
- [ ] CSRF token issued at login and rotated on every POST/PUT/DELETE
- [ ] Client stores CSRF token in memory (NOT localStorage/sessionStorage/cookies)
- [ ] Client includes CSRF token in X-CSRF-Token header on state-modifying requests
- [ ] Security headers (CSP, X-Frame-Options, etc.) are set on all responses
- [ ] Error responses are generic (no SQL errors, file paths, stack traces)
- [ ] Full error details are logged server-side with trace IDs
- [ ] HTTPS is enforced (Secure flag on cookies, HSTS headers)
- [ ] Rate limiting is enabled on login, payment, settlement endpoints
- [ ] Timeouts are hardened: ReadHeaderTimeout, WriteTimeout, IdleTimeout
- [ ] Tests pass: `go test ./pkg/security -v` and `go test ./pkg/web/mid -v`

---

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| CSRF token missing in response | CSRFService not initialized | Ensure CSRFService is in Deps, check router middleware order |
| 403 Forbidden on POST | CSRF token invalid/expired | Ensure token is passed in X-CSRF-Token header, not cookie |
| Token validation fails | Different user validates another's token | Tokens are user-specific; bind validation to correct user ID |
| HttpOnly cookie not set | Cookie serialization issue | Check Login handler, ensure MaxAge is int not Duration |
| Client can't read CSRF token | Token in cookie, not header | Move token from cookie to response header (X-CSRF-Token) |
| Tests fail with panic | Server secret < 32 bytes | Use openssl rand -hex 32 to generate proper secret |

---

## Performance Considerations

- **Token generation**: ~100µs per token (HMAC-SHA256)
- **Token validation**: ~50µs per validation (constant-time HMAC comparison)
- **Memory overhead**: ~500 bytes per token in flight
- **No database hits**: Tokens are stateless (no session storage required)

### Optimization Tips

1. **Use memory cache for server secret**: Avoid reading from env on every request
2. **Batch token generation**: Pre-generate tokens if expecting high concurrency
3. **Tune token TTL**: Shorter TTL = more rotations, longer TTL = larger exposure window (default 15 min is good)
4. **Monitor CSRF failures**: Track 403 responses to detect attacks or misconfiguration

---

## References

- **NIST SP 800-63B**: Authentication and Lifecycle Management
- **OWASP CSRF Prevention Cheat Sheet**: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html
- **OWASP Session Management**: https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
- **MDN: Set-Cookie**: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
- **RFC 6265**: HTTP State Management Mechanism

---

**Document Status**: Ready for Implementation  
**Last Updated**: June 4, 2026  
**License**: MIT
