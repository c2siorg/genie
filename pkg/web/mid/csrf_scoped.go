package mid

import (
	"net/http"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

// CookieScopedCSRF returns middleware that enforces CSRF protection ONLY for
// state-changing requests that are authenticated via the session cookie. It is
// a deliberate no-op for:
//
//   - safe methods (GET/HEAD/OPTIONS/TRACE) — they don't change state;
//   - requests carrying an "Authorization: Bearer" header — token auth is
//     immune to CSRF because an attacker cannot set that header cross-site;
//   - requests without a valid session cookie — there's no ambient credential
//     for an attacker to ride.
//
// This self-scoping is what makes the middleware safe to install globally: the
// current Bearer-token API traffic passes straight through untouched. For a
// cookie-authenticated unsafe request it verifies the session JWT itself (to
// derive the user), then validates the double-submit CSRF token (the
// X-CSRF-Token header or the _csrf form field) bound to that user — the same
// check as Validator.Verify.
//
// Rollout safety: `enforce` defaults to false (report-only). In report-only mode
// a missing/invalid token is LOGGED but the request proceeds, so the protection
// can be deployed before the frontend is updated to send the token. Flip
// `enforce` to true (GENIE_CSRF_ENFORCE=true) once the client sends X-CSRF-Token.
func CookieScopedCSRF(issuer *auth.Issuer, enforce bool, logger Logger) func(http.Handler) http.Handler {
	validator := NewValidator(issuer.Secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Not a CSRF-relevant request → pass through.
			if csrfSafeMethod(r.Method) || csrfHasBearer(r) {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r) // no session cookie → no CSRF surface
				return
			}
			claims, err := issuer.Verify(cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r) // invalid session → downstream auth rejects it
				return
			}
			// Bind the verified claims so the validator derives the per-user secret.
			r = r.WithContext(WithClaims(r.Context(), claims))

			token := r.Header.Get(CSRFHeaderName)
			if token == "" {
				token = r.FormValue(CSRFFormFieldName)
			}
			if verr := validator.Verify(r, token); verr != nil {
				if enforce {
					http.Error(w, "invalid CSRF token", http.StatusForbidden)
					return
				}
				if logger != nil {
					logger.Info("csrf report-only: cookie-authed request would be rejected",
						"method", r.Method, "path", r.URL.Path, "reason", verr.Error())
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func csrfHasBearer(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
}
