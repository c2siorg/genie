package opa

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Middleware returns an HTTP middleware that evaluates every request against
// the OPA HTTP-level authz policy (data.genie.http_authz).
//
// If OPA denies the request the middleware returns 403 with a JSON body:
//
//	{"error": "governance: <reason>"}
//
// Requests that pass OPA proceed to the next handler unchanged.
// If the Engine has no HTTP query loaded (http_authz module was not given),
// every request is passed through — the middleware becomes a no-op.
func Middleware(engine *Engine) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			input := buildHTTPInput(r)

			allowed, reason, err := engine.EvaluateHTTP(r.Context(), input)
			if err != nil {
				// OPA evaluation error — fail closed.
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error": "governance: opa evaluation error",
				})
				return
			}
			if !allowed {
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error": "governance: " + reason,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// buildHTTPInput extracts the OPA HTTP input document from an HTTP request.
// The JWT claims are read from the request context (set by the auth middleware).
func buildHTTPInput(r *http.Request) HTTPAuthzInput {
	// Path as string slice: "/v1/governance/audit" → ["v1","governance","audit"]
	raw := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 1 && parts[0] == "" {
		parts = []string{}
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	// Extract authenticated user from context (populated by JWT middleware).
	user := extractUser(r)

	return HTTPAuthzInput{
		Method:  r.Method,
		Path:    parts,
		Headers: headers,
		User:    user,
	}
}

// extractUser reads the authenticated principal from the request context.
// Falls back to empty values if no claims are present (unauthenticated).
func extractUser(r *http.Request) HTTPAuthzUser {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		return HTTPAuthzUser{}
	}
	roles := make([]string, len(claims.Roles))
	for i, role := range claims.Roles {
		roles[i] = string(role)
	}
	return HTTPAuthzUser{
		ID:    claims.Subject,
		Roles: roles,
	}
}

// writeJSON is a minimal JSON response helper for the middleware.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
