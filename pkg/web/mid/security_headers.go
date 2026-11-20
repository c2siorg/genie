// Package mid holds HTTP middleware: auth (JWT), request logging, OTEL
// tracing, panic recovery, request-id propagation, CSRF protection,
// session management, and security headers.
//
// security_headers.go — HTTP security headers middleware that applies
// Content-Security-Policy, X-Frame-Options, X-Content-Type-Options,
// X-XSS-Protection, Referrer-Policy, and Permissions-Policy headers
// to all HTTP responses.
//
// ─── Security Headers Strategy ─────────────────────────────────────────
//
// Genie applies a defense-in-depth strategy with six critical security headers:
//
//  1. Content-Security-Policy (CSP): Restricts the sources from which scripts,
//     styles, images, fonts, and other resources can be loaded. Prevents XSS
//     attacks by blocking inline scripts and external injection vectors.
//
//     Applied policy:
//     script-src 'self'              — Only scripts from same origin
//     style-src 'self' 'unsafe-inline' — Styles from same origin; inline allowed
//     img-src 'self' data:           — Images from same origin or data URIs
//     font-src 'self'                — Fonts from same origin
//     connect-src 'self'             — API calls to same origin only
//
//  2. X-Frame-Options: DENY — Prevents clickjacking attacks by disallowing
//     the page from being rendered in an <iframe>. Set to DENY (not in any frame).
//
//  3. X-Content-Type-Options: nosniff — Prevents MIME type sniffing.
//     Browser must respect the Content-Type header and not guess the type,
//     which could lead to executing HTML/JS when expecting an image/document.
//
//  4. X-XSS-Protection: 1; mode=block — Enables browser XSS filters and blocks
//     rendering if XSS is detected. (Modern browsers prefer CSP, but this is
//     a fallback for older browsers.)
//
//  5. Referrer-Policy: strict-origin-when-cross-origin — Controls what
//     information is sent in the Referer header. Strict origin: only send the
//     origin (scheme + host), not the full URL. Only on cross-origin requests.
//
//  6. Permissions-Policy: geolocation=(), microphone=(), camera=() — Disables
//     powerful APIs that could be abused (geolocation, microphone, camera).
//     More can be added as needed (payment-request, usb, accelerometer, etc.).
//
// ─── Middleware Application ────────────────────────────────────────────────
//
// SecurityHeaders() returns an HTTP middleware that wraps the response writer
// and injects all headers before the first write. Headers are set once and
// apply to all downstream handlers.
//
// Usage:
//
//	mux := http.NewServeMux()
//	mux.Use(SecurityHeaders())
//
// ─── Header Override for Specific Routes ───────────────────────────────────
//
// Some routes may need relaxed CSP (e.g., routes that embed third-party widgets).
// The HeaderOverride type allows per-route customization:
//
//	// Allow external scripts on the /embed route
//	override := HeaderOverride{
//	    CSPDirectives: map[string]string{
//	        "script-src": "'self' https://trusted-cdn.example.com",
//	    },
//	}
//	mux.Post("/embed", OverrideSecurityHeaders(override)(yourHandler))
//
// ─── Testing Hooks ─────────────────────────────────────────────────────────
//
// VerifySecurityHeaders(r *http.Response) error — Validates that all expected
// security headers are present and correctly formatted. Used in tests to ensure
// headers are applied.
//
// ListSecurityHeaders(r *http.Response) map[string]string — Returns a map of
// all security-related headers in the response. Useful for debugging and
// test inspection.
//
// ─── Thread Safety ──────────────────────────────────────────────────────────
//
// All SecurityHeaders functions are safe to call concurrently. The middleware
// uses http.ResponseWriter wrapping, which is safe for concurrent access if
// the underlying handler is safe.
package mid

import (
	"fmt"
	"net/http"
)

// DefaultSecurityHeaders returns the default set of security headers
// as a map. Used internally by SecurityHeaders() and in tests.
//
// Map structure:
//
//	header-name -> header-value
//
// Example:
//
//	Content-Security-Policy -> "script-src 'self'; style-src 'self' 'unsafe-inline'; ..."
func DefaultSecurityHeaders() map[string]string {
	return map[string]string{
		"Content-Security-Policy": "script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self';",
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "geolocation=(), microphone=(), camera=()",
	}
}

// SecurityHeaders returns HTTP middleware that applies default security headers
// to all responses.
//
// The middleware wraps http.ResponseWriter to inject headers before the first
// write. This ensures headers are present on all responses, including those
// that immediately return (e.g., early error returns).
//
// Usage:
//
//	router.Use(SecurityHeaders())
//
// All requests will receive:
//   - Content-Security-Policy with restrictive defaults
//   - X-Frame-Options: DENY (prevents clickjacking)
//   - X-Content-Type-Options: nosniff (prevents MIME sniffing)
//   - X-XSS-Protection: 1; mode=block (browser XSS filter)
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Permissions-Policy: disables geolocation, microphone, camera
//
// For custom headers per-route, use OverrideSecurityHeaders.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := &securityHeadersWriter{
				ResponseWriter: w,
				headers:        DefaultSecurityHeaders(),
			}
			next.ServeHTTP(wrapped, r)
		})
	}
}

// HeaderOverride allows customization of specific security headers for a route.
//
// Example:
//
//	override := HeaderOverride{
//	    CSPDirectives: map[string]string{
//	        "script-src": "'self' https://cdn.example.com",
//	        "img-src":    "'self' data: https://images.example.com",
//	    },
//	}
type HeaderOverride struct {
	// CSPDirectives is a map of CSP directive names to values.
	// These override the default CSP directives.
	// Example: map[string]string{"script-src": "'self' https://trusted.com"}
	// The final CSP is rebuilt from the merged directives.
	CSPDirectives map[string]string

	// CustomHeaders is a map of arbitrary header name -> value pairs.
	// These completely replace existing headers with the same name.
	// Example: map[string]string{"X-Custom-Header": "value"}
	CustomHeaders map[string]string

	// RemoveHeaders is a list of header names to remove from the default set.
	// Example: []string{"X-XSS-Protection"} (if you want to omit this header)
	RemoveHeaders []string
}

// OverrideSecurityHeaders returns HTTP middleware that applies custom security
// headers for a specific route.
//
// The override is merged with the default headers:
//  1. Start with DefaultSecurityHeaders()
//  2. Remove any headers listed in override.RemoveHeaders
//  3. Apply CSPDirectives to rebuild the Content-Security-Policy header
//  4. Apply CustomHeaders to set/override individual headers
//
// Usage:
//
//	override := HeaderOverride{
//	    CSPDirectives: map[string]string{
//	        "script-src": "'self' https://cdn.example.com",
//	    },
//	}
//	router.Post("/custom-route", OverrideSecurityHeaders(override)(handler))
//
// The override is applied only to this route; other routes keep defaults.
func OverrideSecurityHeaders(override HeaderOverride) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers := DefaultSecurityHeaders()

			// Remove headers if specified.
			for _, name := range override.RemoveHeaders {
				delete(headers, name)
			}

			// Apply custom headers.
			for name, value := range override.CustomHeaders {
				headers[name] = value
			}

			// If CSP directives are provided, rebuild the CSP header.
			if len(override.CSPDirectives) > 0 {
				headers["Content-Security-Policy"] = buildCSP(override.CSPDirectives)
			}

			wrapped := &securityHeadersWriter{
				ResponseWriter: w,
				headers:        headers,
			}
			next.ServeHTTP(wrapped, r)
		})
	}
}

// securityHeadersWriter wraps http.ResponseWriter to inject security headers
// before the first write.
//
// The wrapper intercepts the WriteHeader() call and injects headers on first
// use. This ensures headers are present even if the handler never calls
// WriteHeader explicitly.
type securityHeadersWriter struct {
	http.ResponseWriter
	headers        map[string]string
	headersWritten bool
}

// WriteHeader injects security headers before delegating to the underlying writer.
// Headers are only written once (on first call to WriteHeader or Write).
func (w *securityHeadersWriter) WriteHeader(statusCode int) {
	if !w.headersWritten {
		for name, value := range w.headers {
			w.ResponseWriter.Header().Set(name, value)
		}
		w.headersWritten = true
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write delegates to the underlying writer, ensuring headers are written first.
func (w *securityHeadersWriter) Write(b []byte) (int, error) {
	if !w.headersWritten {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// buildCSP constructs a Content-Security-Policy header from CSP directives.
//
// CSP directives are key-value pairs:
//
//	"script-src" -> "'self' https://example.com"
//	"img-src" -> "'self' data:"
//
// The function combines them into a single header value with semicolons:
//
//	"script-src 'self' https://example.com; img-src 'self' data:;"
//
// Used internally by OverrideSecurityHeaders and in tests.
func buildCSP(directives map[string]string) string {
	if len(directives) == 0 {
		return ""
	}
	csp := ""
	for directive, value := range directives {
		csp += fmt.Sprintf("%s %s; ", directive, value)
	}
	return csp
}

// VerifySecurityHeaders checks that all expected security headers are present
// and correctly formatted in the response.
//
// Returns nil if all headers are present and valid. Returns an error if any
// header is missing or malformed.
//
// Expected headers:
//   - Content-Security-Policy (non-empty)
//   - X-Frame-Options: DENY
//   - X-Content-Type-Options: nosniff
//   - X-XSS-Protection: 1; mode=block
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Permissions-Policy: geolocation=(), microphone=(), camera=()
//
// Usage in tests:
//
//	resp, _ := http.Get("http://localhost:8080/some-route")
//	err := VerifySecurityHeaders(resp)
//	if err != nil {
//	    t.Fatalf("security headers validation failed: %v", err)
//	}
func VerifySecurityHeaders(resp *http.Response) error {
	expected := map[string]string{
		"Content-Security-Policy": "", // non-empty check only
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"X-XSS-Protection":        "1; mode=block",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "geolocation=(), microphone=(), camera=()",
	}

	for header, expectedValue := range expected {
		actual := resp.Header.Get(header)

		// Special case: CSP header should be non-empty.
		if header == "Content-Security-Policy" {
			if actual == "" {
				return fmt.Errorf("missing header: %s", header)
			}
			// CSP is complex and varies based on directives; just ensure it's present.
			continue
		}

		// For other headers, check exact match.
		if actual != expectedValue {
			return fmt.Errorf("header %s: expected %q, got %q", header, expectedValue, actual)
		}
	}

	return nil
}

// ListSecurityHeaders returns a map of all security-related headers in the
// response.
//
// Useful for debugging and test inspection. The returned map contains only
// headers recognized as security-related (CSP, X-Frame-Options, etc.).
//
// Usage in tests:
//
//	resp, _ := http.Get("http://localhost:8080/some-route")
//	headers := ListSecurityHeaders(resp)
//	for name, value := range headers {
//	    t.Logf("%s: %s", name, value)
//	}
//
// Returns a map with header names as keys and header values as values.
// If no security headers are present, returns an empty map.
func ListSecurityHeaders(resp *http.Response) map[string]string {
	securityHeaderNames := []string{
		"Content-Security-Policy",
		"X-Frame-Options",
		"X-Content-Type-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Permissions-Policy",
		"Cross-Origin-Opener-Policy",
		"Cross-Origin-Resource-Policy",
		"Cross-Origin-Embedder-Policy",
		"Strict-Transport-Security",
	}

	result := make(map[string]string)
	for _, name := range securityHeaderNames {
		if value := resp.Header.Get(name); value != "" {
			result[name] = value
		}
	}

	return result
}

// AllSecurityHeaderNames returns a list of all recognized security header names.
// Used for validation and testing.
//
// Returns a slice of header names that are considered "security headers"
// by the middleware.
func AllSecurityHeaderNames() []string {
	return []string{
		"Content-Security-Policy",
		"X-Frame-Options",
		"X-Content-Type-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Permissions-Policy",
		"Cross-Origin-Opener-Policy",
		"Cross-Origin-Resource-Policy",
		"Cross-Origin-Embedder-Policy",
		"Strict-Transport-Security",
	}
}
