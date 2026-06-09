package security

import (
	"net/http"
	"strings"
)

// SecurityHeadersConfig holds CSP and frame protection settings.
// All directives follow OWASP Secure Headers Project recommendations.
type SecurityHeadersConfig struct {
	// Content-Security-Policy directives (OWASP A05: Security Misconfiguration).
	CSPDefaultSrc     []string // default: 'self'
	CSPScriptSrc      []string // default: 'self' (no unsafe-inline)
	CSPStyleSrc       []string // default: 'self' (no unsafe-inline)
	CSPFontSrc        []string // default: 'self', data:
	CSPImgSrc         []string // default: 'self', https:, data:
	CSPConnectSrc     []string // default: 'self' (XHR/WS/fetch)
	CSPFormAction     []string // default: 'self' (prevent form hijacking)
	CSPFrameAncestors []string // default: 'none' (prevent clickjacking)

	// X-Frame-Options (OWASP A04: Insecure Design).
	FrameOptions string // "DENY", "SAMEORIGIN", "ALLOW-FROM uri"

	// X-Content-Type-Options (prevent MIME sniffing).
	NoSniff bool // default: true

	// Referrer-Policy (prevent referrer leakage).
	ReferrerPolicy string // default: "strict-origin-when-cross-origin"

	// Strict-Transport-Security (enforce HTTPS).
	StrictTransportSecurity string // "max-age=31536000; includeSubDomains; preload"

	// Permissions-Policy (successor to Feature-Policy).
	PermissionsPolicy string
}

// DefaultSecurityHeadersConfig returns production-hardened settings.
// These headers provide defense-in-depth against XSS, clickjacking, and content injection.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		// CSP: strict self-only, no inline scripts/styles, no third-party resources.
		// Prevents XSS by disallowing inline code and restricting source domains.
		CSPDefaultSrc:     []string{"'self'"},
		CSPScriptSrc:      []string{"'self'"}, // No unsafe-inline, no external CDNs
		CSPStyleSrc:       []string{"'self'"}, // No unsafe-inline
		CSPFontSrc:        []string{"'self'", "data:"},
		CSPImgSrc:         []string{"'self'", "https:", "data:"},
		CSPConnectSrc:     []string{"'self'"}, // XHR/WS/fetch to same origin only
		CSPFormAction:     []string{"'self'"}, // Forms can only POST to same origin
		CSPFrameAncestors: []string{"'none'"}, // Cannot be embedded in iframes

		// Prevent clickjacking (OWASP A04).
		FrameOptions: "DENY",

		// Prevent MIME sniffing attacks (content type confusion).
		NoSniff: true,

		// Referrer policy: send referrer only for same-origin or strict-origin requests.
		ReferrerPolicy: "strict-origin-when-cross-origin",

		// HSTS: enforce HTTPS, preload list inclusion.
		StrictTransportSecurity: "max-age=31536000; includeSubDomains; preload",

		// Permissions policy: disable dangerous APIs (geolocation, microphone, camera).
		PermissionsPolicy: "geolocation=(), microphone=(), camera=()",
	}
}

// SecurityHeaders middleware applies security headers to all HTTP responses.
// Must be applied as the outermost middleware to wrap all requests.
//
// Headers applied:
// - Content-Security-Policy: Prevents XSS by controlling resource sources
// - X-Frame-Options: Prevents clickjacking (OWASP A04)
// - X-Content-Type-Options: Prevents MIME sniffing
// - Referrer-Policy: Prevents referrer leakage
// - Strict-Transport-Security: Enforces HTTPS (RBI Sutra 7: Safety, Resilience)
// - Permissions-Policy: Disables dangerous browser APIs
func SecurityHeaders(cfg SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Build CSP directive string.
			cspDirectives := []string{
				"default-src " + join(cfg.CSPDefaultSrc),
				"script-src " + join(cfg.CSPScriptSrc),
				"style-src " + join(cfg.CSPStyleSrc),
				"font-src " + join(cfg.CSPFontSrc),
				"img-src " + join(cfg.CSPImgSrc),
				"connect-src " + join(cfg.CSPConnectSrc),
				"form-action " + join(cfg.CSPFormAction),
				"frame-ancestors " + join(cfg.CSPFrameAncestors),
				"base-uri 'self'",              // Prevent <base href> hijacking
				"object-src 'none'",            // Block plugins (Flash, etc.)
				"block-all-mixed-content",      // Enforce HTTPS-only content
				"require-sri-for script style", // Require Subresource Integrity
			}

			w.Header().Set("Content-Security-Policy", strings.Join(cspDirectives, "; "))

			// Clickjacking protection (OWASP A04).
			if cfg.FrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.FrameOptions)
			}

			// MIME type sniffing protection.
			if cfg.NoSniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// Referrer policy.
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			// HSTS (only in production/HTTPS).
			if cfg.StrictTransportSecurity != "" && isHTTPS(r) {
				w.Header().Set("Strict-Transport-Security", cfg.StrictTransportSecurity)
			}

			// Permissions policy (disable dangerous APIs).
			if cfg.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
			}

			// Additional hardening headers.
			w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
			w.Header().Set("X-XSS-Protection", "0") // Disable legacy XSS filters (CSP sufficient)

			next.ServeHTTP(w, r)
		})
	}
}

// join concatenates strings with spaces.
func join(strs []string) string {
	return strings.Join(strs, " ")
}

// isHTTPS detects HTTPS context, accounting for proxy headers.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
