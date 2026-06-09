package security

import (
	"context"
	"net/http"
)

// MiddlewareLogger is the interface for logging within middleware.
type MiddlewareLogger interface {
	Warn(msg string, args ...any)
}

type middlewareCtxKey int

const (
	ctxCSRFToken middlewareCtxKey = iota
)

// CSRFTokenFrom retrieves the validated CSRF token from the request context.
// Used by handlers to access token metadata or issue new tokens.
func CSRFTokenFrom(ctx context.Context) (*CSRFToken, bool) {
	tok, ok := ctx.Value(ctxCSRFToken).(*CSRFToken)
	return tok, ok
}

// CSRF middleware validates CSRF tokens on state-modifying requests.
// Token is expected in the X-CSRF-Token request header (not a cookie, to prevent XSS exfiltration).
// On success, a new token is issued and attached to the response (token rotation).
// On failure, responds with 403 Forbidden and logs the attempt.
//
// Integration pattern:
//
//	r.Use(mid.Auth(issuer))              // Layer 4: Auth (JWT validation)
//	r.Use(security.CSRF(csrfSvc, logger)) // Layer 5: CSRF (token validation + rotation)
//
// The CSRF middleware MUST come after the Auth middleware, as it requires
// authenticated claims (user ID) to validate the token.
func CSRF(svc *CSRFService, logger MiddlewareLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for safe methods (GET, HEAD, OPTIONS, TRACE).
			if !RequiresValidation(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract user ID from context (set by Auth middleware).
			// This assumes the context has a ClaimsFrom function (implemented in mid package).
			userID := getUserIDFromContext(r.Context())
			if userID == "" {
				// Unauthenticated request on a state-modifying endpoint.
				if logger != nil {
					logger.Warn("csrf validation skipped: unauthenticated request", "method", r.Method, "path", r.URL.Path)
				}
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}

			// Extract CSRF token from request header.
			// Tokens in headers (not cookies) prevent accidental leakage via logs/referrer.
			tokenStr := r.Header.Get("X-CSRF-Token")
			if tokenStr == "" {
				if logger != nil {
					logger.Warn("csrf validation failed: token missing", "user_id", userID, "method", r.Method, "path", r.URL.Path)
				}
				http.Error(w, "csrf token required", http.StatusForbidden)
				return
			}

			// Validate token against user ID (session).
			_, err := svc.ValidateToken(tokenStr, userID)
			if err != nil {
				// Log as potential attack vector.
				if logger != nil {
					logger.Warn("csrf validation failed: token invalid",
						"user_id", userID,
						"error", err.Error(),
						"method", r.Method,
						"path", r.URL.Path,
						"client_ip", r.RemoteAddr)
				}
				http.Error(w, "csrf token invalid or expired", http.StatusForbidden)
				return
			}

			// Issue new token for response (rotation).
			// This ensures each request gets a fresh token, minimizing exposure window.
			newTokenStr, newToken, err := svc.GenerateToken(userID)
			if err != nil {
				if logger != nil {
					logger.Warn("csrf: failed to issue new token", "user_id", userID, "error", err.Error())
				}
				http.Error(w, "could not issue new token", http.StatusInternalServerError)
				return
			}

			// Store new token in context for response handler.
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxCSRFToken, newToken)

			// Add new token to response header (client extracts and stores in memory).
			w.Header().Set("X-CSRF-Token", newTokenStr)

			// Add cache-control headers to prevent token leakage via caching.
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getUserIDFromContext extracts the user ID from the request context.
// This is a helper that expects the context to contain authenticated claims.
// In practice, this is set by the Auth middleware (mid.Auth).
// If the context doesn't have claims, returns empty string.
//
// Note: This is a placeholder. The actual implementation depends on how
// the mid package stores claims in context. See mid.ClaimsFrom for the pattern.
func getUserIDFromContext(ctx context.Context) string {
	// This is a stub that expects Claims to be stored in context.
	// Actual implementation would be:
	//   claims, ok := mid.ClaimsFrom(ctx)
	//   if ok { return claims.UserID }
	//   return ""
	//
	// For now, we rely on callers to validate this before calling CSRF middleware.
	return ""
}
