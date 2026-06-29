package mid

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

// captureLogger records Info calls so report-only mode can be asserted.
type captureLogger struct{ infos int }

func (c *captureLogger) Info(string, ...any)  { c.infos++ }
func (c *captureLogger) Error(string, ...any) {}

func newCSRFTestIssuer() *auth.Issuer {
	// 32-byte secret (HS256 minimum) + a non-zero TTL (zero TTL mints
	// already-expired tokens, which Verify rejects).
	return &auth.Issuer{Secret: []byte("0123456789abcdef0123456789abcdef"), TTL: time.Hour}
}

// sessionCookie mints a session JWT for userID and returns it as the session cookie.
func sessionCookie(t *testing.T, iss *auth.Issuer, userID string) *http.Cookie {
	t.Helper()
	token, _, err := iss.Issue(userID, userID+"@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return &http.Cookie{Name: SessionCookieName, Value: token}
}

// validCSRFToken generates a token bound to userID (matches Validator.Verify).
func validCSRFToken(t *testing.T, iss *auth.Issuer, userID string) string {
	t.Helper()
	gen := NewGenerator(iss.Secret)
	req := httptest.NewRequest("GET", "/", nil)
	claims := auth.Claims{Subject: userID}
	req = req.WithContext(WithClaims(req.Context(), claims))
	tok, err := gen.Generate(req)
	if err != nil {
		t.Fatalf("generate csrf: %v", err)
	}
	return tok
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestCookieScopedCSRF_SkipsSafeMethods(t *testing.T) {
	iss := newCSRFTestIssuer()
	h := CookieScopedCSRF(iss, true, nil)(okHandler())
	req := httptest.NewRequest("GET", "/x", nil)
	req.AddCookie(sessionCookie(t, iss, "u1")) // cookie present, but GET is safe
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("safe method blocked: %d", rec.Code)
	}
}

func TestCookieScopedCSRF_SkipsBearer(t *testing.T) {
	iss := newCSRFTestIssuer()
	h := CookieScopedCSRF(iss, true, nil)(okHandler())
	req := httptest.NewRequest("POST", "/x", nil)
	req.Header.Set("Authorization", "Bearer sometoken") // Bearer → CSRF-immune
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer request blocked by CSRF: %d", rec.Code)
	}
}

func TestCookieScopedCSRF_SkipsNoCookie(t *testing.T) {
	iss := newCSRFTestIssuer()
	h := CookieScopedCSRF(iss, true, nil)(okHandler())
	req := httptest.NewRequest("POST", "/x", nil) // no session cookie
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("no-cookie request blocked by CSRF: %d", rec.Code)
	}
}

func TestCookieScopedCSRF_EnforceRejectsMissingToken(t *testing.T) {
	iss := newCSRFTestIssuer()
	h := CookieScopedCSRF(iss, true, nil)(okHandler())
	req := httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(sessionCookie(t, iss, "u1")) // cookie-authed, no CSRF token
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("enforce mode: missing token got %d, want 403", rec.Code)
	}
}

func TestCookieScopedCSRF_ReportOnlyAllowsAndLogs(t *testing.T) {
	iss := newCSRFTestIssuer()
	log := &captureLogger{}
	h := CookieScopedCSRF(iss, false, log)(okHandler()) // enforce=false
	req := httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(sessionCookie(t, iss, "u1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("report-only blocked the request: %d", rec.Code)
	}
	if log.infos == 0 {
		t.Fatal("report-only did not log the would-be rejection")
	}
}

func TestCookieScopedCSRF_EnforceAllowsValidToken(t *testing.T) {
	iss := newCSRFTestIssuer()
	h := CookieScopedCSRF(iss, true, nil)(okHandler())
	req := httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(sessionCookie(t, iss, "u1"))
	req.Header.Set(CSRFHeaderName, validCSRFToken(t, iss, "u1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid token rejected: %d", rec.Code)
	}
}
