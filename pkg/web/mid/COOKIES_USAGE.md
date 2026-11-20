# HttpOnly Session Management — Usage Guide

This document explains how to use the `SessionManager` in `pkg/web/mid/cookies.go` for production-ready session management with HttpOnly, Secure, and SameSite=Strict cookies.

---

## Overview

**What it does:**
- Create JWT-based sessions in HttpOnly cookies (JavaScript cannot access)
- Validate sessions on every request
- Automatically refresh sessions when they near expiry (silent refresh)
- Destroy sessions with a single call
- Support mixed authentication (sessions + bearer tokens)

**Security features:**
- `HttpOnly=true` — Prevents XSS attacks from stealing the session cookie
- `Secure=true` — HTTPS-only transmission (prevents man-in-the-middle)
- `SameSite=Strict` — Prevents CSRF attacks via cross-site form submission
- `Max-Age=24h` — Automatic expiry after 24 hours (with JWT-level expiry as backup)

**Zero external dependencies:**
- Built on Go's stdlib `net/http` and the existing `auth.Issuer`
- No third-party cookie libraries required
- Thread-safe for concurrent goroutines

---

## Quick Start

### 1. Initialize the SessionManager

```go
package main

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// At startup, create an Issuer and SessionManager
func setupSessionManager() *mid.SessionManager {
	secret := []byte(os.Getenv("GENIE_JWT_SECRET")) // 32+ bytes
	issuer := &auth.Issuer{
		Secret:   secret,
		Issuer:   "genie-api",
		Audience: []string{"genie-web"},
		TTL:      24 * time.Hour, // Must be ≥ CookieTTL
	}
	
	// Secure=true for production (HTTPS)
	// Secure=false for development (localhost HTTP)
	secure := !isDevelopment()
	sm := mid.NewSessionManager(issuer, secure)
	return sm
}
```

### 2. Create a Session (Login)

```go
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	// ... validate credentials ...
	userID := "user-123"
	email := "alice@example.com"
	roles := []auth.Role{auth.RoleUser}
	
	cookie, claims, csrfSecret, err := sm.CreateSession(w, userID, email, roles)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	
	// Store CSRF secret (e.g., in session storage or return to client)
	// For stateless operation, derive it from user_id + server_secret
	storeCSRFSecret(userID, csrfSecret)
	
	// Return success; cookie is already set on response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "logged in",
		"user":   claims.Email,
	})
}
```

### 3. Validate a Session (Middleware)

```go
// Add SessionMiddleware to your router
func setupRouter(sm *mid.SessionManager) http.Handler {
	mux := http.NewServeMux()
	
	// Protected routes: session validation required
	mux.HandleFunc("/api/profile", func(w http.ResponseWriter, r *http.Request) {
		// SessionMiddleware has already validated and attached claims
		claims, ok := mid.SessionClaimsFrom(r.Context())
		if !ok {
			http.Error(w, "no claims", http.StatusUnauthorized)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"user":  claims.Email,
			"roles": fmt.Sprintf("%v", claims.Roles),
		})
	})
	
	// Wrap with SessionMiddleware
	return mid.SessionMiddleware(sm)(mux)
}
```

### 4. Destroy a Session (Logout)

```go
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	sm.DestroySession(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}
```

---

## Detailed Usage

### Session Creation

```go
cookie, claims, csrfSecret, err := sm.CreateSession(w, userID, email, roles)
```

**Returns:**
- `cookie` — The `*http.Cookie` (already set on response)
- `claims` — The JWT claims (for inspection, logging)
- `csrfSecret` — Hex-encoded CSRF secret (store separately)
- `err` — Any error during JWT signing

**Cookie flags set automatically:**
- `HttpOnly=true` — JavaScript cannot access `document.cookie`
- `Secure=true` (if `NewSessionManager(..., true)`)
- `SameSite=Strict` — No cross-site transmission
- `Path=/` — Valid for entire domain
- `Max-Age=86400` (24 hours)

### Session Validation

```go
claims, err := sm.ValidateSession(r)
if err != nil {
	// Session is invalid, expired, or missing
	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return
}

// Use claims downstream
fmt.Printf("User: %s, Roles: %v\n", claims.Email, claims.Roles)
```

**Error types:**
- `"missing_cookie"` — No session cookie in request
- `"verification_failed"` — JWT signature or format invalid
- `"expired"` — Token's expiry time has passed

### Session Refresh (Automatic)

```go
// Check if token needs refresh and issue new one if needed
cookie, newClaims, err := sm.RefreshSession(w, claims)
if err != nil {
	// Log error, but request continues (old token still valid)
	log.Printf("refresh failed: %v", err)
}
if cookie != nil {
	// New cookie was set; client will use updated session
}
```

**How refresh works:**
- Compare token age (`now - iat`) to refresh threshold (TTL × 0.5)
- If age > threshold, mint a new token with fresh expiry
- Cookie is automatically set on response
- No client action required (silent refresh)

**Example:**
- Token issued at 10:00
- TTL = 24 hours, threshold = 12 hours
- Checked at 22:30 (age = 12.5 hours)
- Refresh triggered → new token issued with exp = next day 22:30

### Session Destruction

```go
sm.DestroySession(w)
```

Sets the session cookie with `Max-Age=0`, triggering immediate browser deletion.

---

## Middleware Integration

### Single Authentication Method (Sessions Only)

```go
router := chi.NewRouter()
router.Use(mid.SessionMiddleware(sm))

// All routes now require valid session
router.Get("/api/data", handleGetData)
```

### Mixed Authentication (Sessions + Bearer Tokens)

```go
router := chi.NewRouter()
router.Use(mid.SessionAndBearerMiddleware(sm, issuer))

// Routes accept either:
// 1. Session cookie, OR
// 2. Authorization: Bearer <JWT>
router.Get("/api/data", handleGetData)
```

---

## CSRF Protection Integration

HttpOnly cookies prevent XSS theft, but CSRF still requires a separate token (double-submit pattern).

### Generate CSRF Token

```go
// Option 1: Random (new CSRF secret per session)
csrfSecret := mid.GenerateCSRFSecret()

// Option 2: Deterministic (derive from user + master secret)
masterSecret := []byte(os.Getenv("GENIE_CSRF_MASTER_SECRET"))
csrfSecret := mid.DeriveCSRFSecret(userID, masterSecret)
```

### Return CSRF Token to Client

```go
// In login response
json.NewEncoder(w).Encode(map[string]string{
	"status":       "logged in",
	"csrf_token":   csrfSecret, // Send to client
})
```

### Validate CSRF Token on Mutation

The CSRF validation is handled by `mid.Validator` (in `csrf.go`). Use it alongside SessionMiddleware:

```go
router := chi.NewRouter()
router.Use(mid.SessionMiddleware(sm))
router.Use(mid.CSRFProtection(
	csrfValidator,
	mid.CSRFFormFieldName,
))

// Now all POST/PUT/DELETE require valid CSRF token
router.Post("/api/data", handleCreateData)
```

---

## Context Helpers

### Extract Session Claims

```go
// Inside a handler after SessionMiddleware
claims, ok := mid.SessionClaimsFrom(r.Context())
if !ok {
	// No session (shouldn't happen if middleware is configured)
}
fmt.Printf("User: %s\n", claims.Email)
```

### Store Custom Values on Context

```go
// SessionMiddleware calls this internally:
ctx := mid.WithSessionClaims(context.Background(), claims)

// Retrieve later:
claims, _ := mid.SessionClaimsFrom(ctx)
```

---

## Cookie Behavior

### Browser Auto-Transmission

The browser **automatically includes the session cookie** on every request to the same domain:

```javascript
// Browser automatic (no JavaScript code needed):
fetch('/api/data', { credentials: 'same-origin' })
  // Browser adds: Cookie: session=<jwt>
```

### JavaScript Cannot Read It

The `HttpOnly` flag prevents JavaScript from reading the cookie:

```javascript
// This will NOT include the session token (empty)
console.log(document.cookie)

// This WILL fail (HttpOnly prevents access)
const token = document.cookie.split('session=')[1]
```

This is a **security feature** — XSS attacks cannot steal the token.

### Cross-Site Requests Blocked

The `SameSite=Strict` flag blocks cookie transmission on cross-site requests:

```html
<!-- evil.com -->
<form action="https://genie.com/api/data" method="POST">
  <!-- Browser will NOT include session cookie -->
</form>
```

This prevents **CSRF attacks** where an attacker tricks your browser into making unauthorized requests.

---

## Security Considerations

### 1. Always Use Secure=true in Production

```go
secure := os.Getenv("ENVIRONMENT") == "production"
sm := mid.NewSessionManager(issuer, secure)
```

Without `Secure=true`, the cookie can be transmitted over plain HTTP and intercepted.

### 2. Rotate CSRF Secrets

CSRF secrets should be short-lived (15-minute TTL recommended):

```go
// In session validation middleware
csrfExpiry := time.Now().Add(-15 * time.Minute)
if csrfGeneratedAt.Before(csrfExpiry) {
	// Regenerate CSRF secret
	newCSRFSecret := mid.GenerateCSRFSecret()
	// Return new secret to client
}
```

### 3. Implement Logout Revocation (Optional)

Without explicit logout revocation, a stolen session cookie remains valid until expiry (24 hours). For high-security applications, maintain a revocation list:

```go
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	claims, _ := mid.SessionClaimsFrom(r.Context())
	
	// Add to revocation list (Redis, DB, in-memory set)
	revokeSession(claims.Subject, claims.IssuedAt)
	
	// Clear cookie
	sm.DestroySession(w)
}

// In middleware, check revocation:
claims, _ := sm.ValidateSession(r)
if isRevoked(claims.Subject, claims.IssuedAt) {
	http.Error(w, "session revoked", http.StatusUnauthorized)
	return
}
```

### 4. Validate Issuer and Audience

The `SessionManager` reuses `auth.Issuer` which verifies:
- JWT signature (HS256-SHA256)
- Expiry time (`exp` claim)
- Issuer name (`iss` claim) — optional but recommended
- Audience list (`aud` claim) — optional but recommended

Ensure your issuer is configured with the correct audience:

```go
issuer := &auth.Issuer{
	Secret:   secret,
	Issuer:   "genie-api",        // Set this
	Audience: []string{"genie-web"}, // Set this
	TTL:      24 * time.Hour,
}
```

---

## Troubleshooting

### Session Cookie Not Set

**Problem:** `Set-Cookie` header is missing.

**Causes:**
1. `CreateSession` returned an error (check logs)
2. Response was already written before `CreateSession` was called
3. Middleware is stripping Set-Cookie headers

**Fix:** Ensure `CreateSession` is called before `w.WriteHeader()`.

### Session Validates, But User Info Is Empty

**Problem:** Claims have no email/roles.

**Cause:** The issuer was created with incorrect fields.

**Fix:** Check that `CreateSession` received valid user email and roles:

```go
// Debug: print what was issued
log.Printf("Issued: %#v", claims)
```

### Cookies Sent But Not Stored

**Problem:** Cookie is set in response but browser doesn't store it.

**Causes:**
1. Browser is in strict privacy mode (SameSite=Strict + cross-site)
2. Secure flag set but URL is `http://` (not `https://`)
3. Cookie domain mismatch

**Fix:**
- For local development with `http://localhost`, use `Secure=false`
- Ensure domain matches (no `subdomain.example.com` for `example.com` cookie)

---

## Testing

### Unit Tests

All functionality is unit-tested in `cookies_test.go`:

```bash
go test ./pkg/web/mid -v -run TestCreateSession
go test ./pkg/web/mid -v -run TestValidateSession
go test ./pkg/web/mid -v -run TestRefreshSession
go test ./pkg/web/mid -v -run TestSessionMiddleware
```

### Integration Test Example

```go
func TestLoginFlow(t *testing.T) {
	// Setup
	sm := mid.NewSessionManager(issuer, false) // false = insecure for test
	
	// 1. Create session
	w := httptest.NewRecorder()
	sm.CreateSession(w, "user-1", "alice@example.com", []auth.Role{auth.RoleUser})
	
	// Extract cookie
	cookie := extractCookie(w)
	
	// 2. Use cookie in request
	req := httptest.NewRequest("GET", "/api/data", nil)
	req.AddCookie(cookie)
	
	// 3. Validate session
	claims, err := sm.ValidateSession(req)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	
	// Assert
	if claims.Email != "alice@example.com" {
		t.Errorf("wrong email: %s", claims.Email)
	}
}
```

---

## References

- [MDN: HttpOnly Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Cookies#restrict_access_to_cookies)
- [RFC 6265: HTTP State Management](https://tools.ietf.org/html/rfc6265)
- [RFC 7818: JSON Web Token (JWT)](https://tools.ietf.org/html/rfc7818)
- [SameSite Cookies Explained](https://web.dev/same-site-cookies-explained/)
- [OWASP: Cross-Site Request Forgery (CSRF)](https://owasp.org/www-community/attacks/csrf)
- [OWASP: Cross-Site Scripting (XSS)](https://owasp.org/www-community/attacks/xss/)

---

## FAQ

**Q: Why HttpOnly instead of storing the JWT in localStorage?**
A: localStorage is vulnerable to XSS attacks (JavaScript can read it). HttpOnly cookies are inaccessible to JavaScript, significantly raising the bar for attackers.

**Q: Can I use sessions with fetch()?**
A: Yes, with `credentials: 'same-origin'` or `'include'`:
```javascript
fetch('/api/data', { credentials: 'same-origin' })
```

**Q: Why SameSite=Strict instead of Lax?**
A: Strict provides maximum protection against CSRF. Lax allows some legitimate cross-site requests (e.g., from the app's own OAuth redirect). Use Lax only if needed for OAuth flows.

**Q: How do I implement "remember me" (longer-lived sessions)?**
A: Override the TTL when creating the SessionManager:
```go
issuer.TTL = 30 * 24 * time.Hour // 30 days
sm := mid.NewSessionManager(issuer, true)
```
Note: Longer TTL = larger security window. Trade-off: convenience vs. risk.

**Q: Can I use sessions across multiple domains?**
A: No — cookies are domain-specific. For federation, use bearer tokens or implement cross-domain session storage.

---

**Last Updated:** June 4, 2026  
**License:** MIT  
**Maintained by:** Genie Project
