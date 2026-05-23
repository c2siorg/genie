// Package mid holds HTTP middleware: auth (JWT), request logging, OTEL
// tracing, panic recovery, and request-id propagation.
package mid

import (
	"context"
	"net/http"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

type ctxKey int

const (
	ctxClaims ctxKey = iota
	ctxRequestID
)

// WithClaims stores the authenticated claims on the context. Exported for
// tests; in production this is set by Auth().
func WithClaims(ctx context.Context, c auth.Claims) context.Context {
	return context.WithValue(ctx, ctxClaims, c)
}

// ClaimsFrom returns the authenticated claims, if any.
func ClaimsFrom(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(ctxClaims).(auth.Claims)
	return c, ok
}

// RequestIDFrom returns the request id captured by RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	if s, ok := ctx.Value(ctxRequestID).(string); ok {
		return s
	}
	return ""
}

// Auth verifies a bearer JWT and attaches the claims to the context. Returns
// 401 on missing or invalid token.
func Auth(issuer *auth.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := issuer.Verify(tok)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}

// RequireRole short-circuits requests whose authenticated claims do not hold
// any of the listed roles.
func RequireRole(roles ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := ClaimsFrom(r.Context())
			if !ok {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			for _, want := range roles {
				if c.HasRole(want) {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}
