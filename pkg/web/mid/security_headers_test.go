// Package mid holds HTTP middleware tests.
//
// security_headers_test.go — Unit and integration tests for security headers
// middleware, including CSP, X-Frame-Options, X-Content-Type-Options,
// X-XSS-Protection, Referrer-Policy, and Permissions-Policy validation.
package mid

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSecurityHeaders_DefaultHeaders verifies that SecurityHeaders middleware
// applies all default security headers to the response.
func TestSecurityHeaders_DefaultHeaders(t *testing.T) {
	// Create a simple handler that returns 200 OK.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with SecurityHeaders middleware.
	wrapped := SecurityHeaders()(handler)

	// Execute the request.
	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Verify all expected headers are present.
	err := VerifySecurityHeaders(rec.Result())
	if err != nil {
		t.Fatalf("VerifySecurityHeaders failed: %v", err)
	}

	// Verify the response status and body.
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "OK" {
		t.Errorf("expected body 'OK', got %q", body)
	}
}

// TestSecurityHeaders_HeaderValues verifies that each header has the correct value.
func TestSecurityHeaders_HeaderValues(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := SecurityHeaders()(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "X-Frame-Options",
			header:   "X-Frame-Options",
			expected: "DENY",
		},
		{
			name:     "X-Content-Type-Options",
			header:   "X-Content-Type-Options",
			expected: "nosniff",
		},
		{
			name:     "X-XSS-Protection",
			header:   "X-XSS-Protection",
			expected: "1; mode=block",
		},
		{
			name:     "Referrer-Policy",
			header:   "Referrer-Policy",
			expected: "strict-origin-when-cross-origin",
		},
		{
			name:     "Permissions-Policy",
			header:   "Permissions-Policy",
			expected: "geolocation=(), microphone=(), camera=()",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := resp.Header.Get(test.header)
			if actual != test.expected {
				t.Errorf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}

// TestSecurityHeaders_CSP verifies the Content-Security-Policy header format.
func TestSecurityHeaders_CSP(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := SecurityHeaders()(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header is empty")
	}

	// Verify required CSP directives are present.
	requiredDirectives := map[string]bool{
		"script-src 'self'":                false,
		"style-src 'self' 'unsafe-inline'": false,
		"img-src 'self' data:":             false,
		"font-src 'self'":                  false,
		"connect-src 'self'":               false,
	}

	for directive := range requiredDirectives {
		if !containsDirective(csp, directive) {
			t.Errorf("CSP missing directive: %s", directive)
		}
	}
}

// TestSecurityHeaders_EarlyWrite verifies that headers are applied even when
// Write is called before WriteHeader.
func TestSecurityHeaders_EarlyWrite(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Call Write without calling WriteHeader first.
		w.Write([]byte("content"))
	})

	wrapped := SecurityHeaders()(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Headers should still be present.
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options not set on early Write")
	}
}

// TestSecurityHeaders_MultipleWrites verifies that headers are not written twice.
func TestSecurityHeaders_MultipleWrites(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("first"))
		w.Write([]byte("second"))
	})

	wrapped := SecurityHeaders()(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Read body and verify content.
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "firstsecond" {
		t.Errorf("expected body 'firstsecond', got %q", string(body))
	}

	// Verify headers are present.
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options not set")
	}
}

// TestOverrideSecurityHeaders_CSPDirectives verifies that CSP directives can be overridden.
func TestOverrideSecurityHeaders_CSPDirectives(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	override := HeaderOverride{
		CSPDirectives: map[string]string{
			"script-src": "'self' https://cdn.example.com",
			"img-src":    "'self' data: https://images.example.com",
		},
	}

	wrapped := OverrideSecurityHeaders(override)(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	csp := resp.Header.Get("Content-Security-Policy")
	if !containsDirective(csp, "script-src 'self' https://cdn.example.com") {
		t.Errorf("CSP missing overridden script-src directive. Got: %s", csp)
	}
	if !containsDirective(csp, "img-src 'self' data: https://images.example.com") {
		t.Errorf("CSP missing overridden img-src directive. Got: %s", csp)
	}
}

// TestOverrideSecurityHeaders_CustomHeaders verifies that custom headers can be added.
func TestOverrideSecurityHeaders_CustomHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	override := HeaderOverride{
		CustomHeaders: map[string]string{
			"X-Custom-Header":  "custom-value",
			"X-Another-Header": "another-value",
		},
	}

	wrapped := OverrideSecurityHeaders(override)(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if actual := resp.Header.Get("X-Custom-Header"); actual != "custom-value" {
		t.Errorf("expected X-Custom-Header='custom-value', got %q", actual)
	}
	if actual := resp.Header.Get("X-Another-Header"); actual != "another-value" {
		t.Errorf("expected X-Another-Header='another-value', got %q", actual)
	}
}

// TestOverrideSecurityHeaders_RemoveHeaders verifies that headers can be removed.
func TestOverrideSecurityHeaders_RemoveHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	override := HeaderOverride{
		RemoveHeaders: []string{"X-XSS-Protection"},
	}

	wrapped := OverrideSecurityHeaders(override)(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if actual := resp.Header.Get("X-XSS-Protection"); actual != "" {
		t.Errorf("expected X-XSS-Protection to be removed, but got %q", actual)
	}

	// Verify other headers are still present.
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options should still be present")
	}
}

// TestOverrideSecurityHeaders_Combined verifies that CSP, custom headers,
// and removal can be combined.
func TestOverrideSecurityHeaders_Combined(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	override := HeaderOverride{
		CSPDirectives: map[string]string{
			"script-src": "'self' https://trusted.com",
		},
		CustomHeaders: map[string]string{
			"X-Custom": "value",
		},
		RemoveHeaders: []string{"X-XSS-Protection"},
	}

	wrapped := OverrideSecurityHeaders(override)(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Verify CSP override.
	if csp := resp.Header.Get("Content-Security-Policy"); !containsDirective(csp, "script-src 'self' https://trusted.com") {
		t.Errorf("CSP override not applied. Got: %s", csp)
	}

	// Verify custom header.
	if resp.Header.Get("X-Custom") != "value" {
		t.Error("Custom header not set")
	}

	// Verify removal.
	if resp.Header.Get("X-XSS-Protection") != "" {
		t.Error("Header should have been removed")
	}
}

// TestListSecurityHeaders verifies that ListSecurityHeaders returns all
// security headers present in the response.
func TestListSecurityHeaders(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := SecurityHeaders()(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	headers := ListSecurityHeaders(resp)

	expectedCount := 6 // CSP, X-Frame-Options, X-Content-Type-Options, X-XSS-Protection, Referrer-Policy, Permissions-Policy
	if len(headers) < expectedCount {
		t.Errorf("expected at least %d security headers, got %d", expectedCount, len(headers))
	}

	if _, ok := headers["Content-Security-Policy"]; !ok {
		t.Error("Content-Security-Policy not in headers list")
	}
	if _, ok := headers["X-Frame-Options"]; !ok {
		t.Error("X-Frame-Options not in headers list")
	}
}

// TestListSecurityHeaders_NoHeaders verifies that ListSecurityHeaders returns
// empty map when no security headers are present.
func TestListSecurityHeaders_NoHeaders(t *testing.T) {
	// Create a response without security headers.
	rec := httptest.NewRecorder()
	resp := rec.Result()
	defer resp.Body.Close()

	headers := ListSecurityHeaders(resp)

	if len(headers) != 0 {
		t.Errorf("expected empty headers, got %d", len(headers))
	}
}

// TestDefaultSecurityHeaders verifies that DefaultSecurityHeaders returns
// the expected default set.
func TestDefaultSecurityHeaders(t *testing.T) {
	defaults := DefaultSecurityHeaders()

	expectedKeys := []string{
		"Content-Security-Policy",
		"X-Frame-Options",
		"X-Content-Type-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Permissions-Policy",
	}

	for _, key := range expectedKeys {
		if _, ok := defaults[key]; !ok {
			t.Errorf("expected header %s not in defaults", key)
		}
	}

	// Verify values are non-empty.
	for key, value := range defaults {
		if value == "" {
			t.Errorf("header %s has empty value", key)
		}
	}
}

// TestBuildCSP verifies that buildCSP correctly formats CSP directives.
func TestBuildCSP(t *testing.T) {
	tests := []struct {
		name       string
		directives map[string]string
		check      func(string) bool
	}{
		{
			name: "single directive",
			directives: map[string]string{
				"script-src": "'self'",
			},
			check: func(s string) bool { return containsDirective(s, "script-src 'self'") },
		},
		{
			name: "multiple directives",
			directives: map[string]string{
				"script-src": "'self'",
				"img-src":    "'self' data:",
			},
			check: func(s string) bool {
				return containsDirective(s, "script-src 'self'") &&
					containsDirective(s, "img-src 'self' data:")
			},
		},
		{
			name:       "empty directives",
			directives: map[string]string{},
			check:      func(s string) bool { return s == "" },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csp := buildCSP(test.directives)
			if !test.check(csp) {
				t.Errorf("buildCSP failed for %q", test.name)
			}
		})
	}
}

// TestAllSecurityHeaderNames verifies that AllSecurityHeaderNames returns
// the expected list of header names.
func TestAllSecurityHeaderNames(t *testing.T) {
	names := AllSecurityHeaderNames()

	if len(names) == 0 {
		t.Fatal("AllSecurityHeaderNames returned empty list")
	}

	expectedHeaders := map[string]bool{
		"Content-Security-Policy": false,
		"X-Frame-Options":         false,
		"X-Content-Type-Options":  false,
		"X-XSS-Protection":        false,
		"Referrer-Policy":         false,
		"Permissions-Policy":      false,
	}

	for _, name := range names {
		if _, ok := expectedHeaders[name]; ok {
			expectedHeaders[name] = true
		}
	}

	for name, found := range expectedHeaders {
		if !found {
			t.Errorf("expected header %s not in AllSecurityHeaderNames", name)
		}
	}
}

// TestVerifySecurityHeaders_Valid verifies that VerifySecurityHeaders returns
// nil for valid security headers.
func TestVerifySecurityHeaders_Valid(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := SecurityHeaders()(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	err := VerifySecurityHeaders(resp)
	if err != nil {
		t.Fatalf("VerifySecurityHeaders should not error for valid headers: %v", err)
	}
}

// TestVerifySecurityHeaders_MissingHeaders verifies that VerifySecurityHeaders
// returns an error when expected headers are missing.
func TestVerifySecurityHeaders_MissingHeaders(t *testing.T) {
	// Create a response without security headers.
	rec := httptest.NewRecorder()
	resp := rec.Result()
	defer resp.Body.Close()

	err := VerifySecurityHeaders(resp)
	if err == nil {
		t.Error("VerifySecurityHeaders should return error for missing headers")
	}
}

// TestVerifySecurityHeaders_IncorrectValue verifies that VerifySecurityHeaders
// returns an error when a header has an incorrect value.
func TestVerifySecurityHeaders_IncorrectValue(t *testing.T) {
	rec := httptest.NewRecorder()
	// Set an incorrect value for X-Frame-Options.
	rec.Header().Set("X-Frame-Options", "SAMEORIGIN")
	rec.Header().Set("Content-Security-Policy", "default-src 'self'")
	rec.Header().Set("X-Content-Type-Options", "nosniff")
	rec.Header().Set("X-XSS-Protection", "1; mode=block")
	rec.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	rec.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

	resp := rec.Result()
	defer resp.Body.Close()

	err := VerifySecurityHeaders(resp)
	if err == nil {
		t.Error("VerifySecurityHeaders should return error for incorrect header value")
	}
}

// containsDirective is a helper that checks if a CSP header contains a directive.
// It handles the fact that CSP directives are separated by semicolons and may
// have trailing spaces.
func containsDirective(csp, directive string) bool {
	// Split by semicolon and check each part.
	parts := splitCSPDirectives(csp)
	for _, part := range parts {
		if part == directive {
			return true
		}
	}
	return false
}

// splitCSPDirectives splits a CSP header into individual directives.
func splitCSPDirectives(csp string) []string {
	var directives []string
	var current string

	for _, ch := range csp {
		if ch == ';' {
			if current != "" {
				// Trim whitespace and add to list.
				trimmed := trimSpace(current)
				if trimmed != "" {
					directives = append(directives, trimmed)
				}
			}
			current = ""
		} else {
			current += string(ch)
		}
	}

	// Don't forget the last directive.
	if current != "" {
		trimmed := trimSpace(current)
		if trimmed != "" {
			directives = append(directives, trimmed)
		}
	}

	return directives
}

// trimSpace is a simple string trimming function.
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}

	return s[start:end]
}

// TestSecurityHeaders_ErrorHandler verifies that security headers are applied
// even when the handler returns an error.
func TestSecurityHeaders_ErrorHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	})

	wrapped := SecurityHeaders()(handler)

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Status should be 500.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	// Headers should still be present.
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options not set on error response")
	}
}

// TestSecurityHeaders_Chaining verifies that SecurityHeaders can be chained
// with other middleware.
func TestSecurityHeaders_Chaining(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create a custom middleware that adds a header.
	customMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "value")
			next.ServeHTTP(w, r)
		})
	}

	// Chain middleware: custom first, then security headers.
	wrapped := customMiddleware(SecurityHeaders()(handler))

	req := httptest.NewRequest("GET", "http://localhost:8080/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Both headers should be present.
	if resp.Header.Get("X-Custom") != "value" {
		t.Error("custom header not set")
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options not set when chaining middleware")
	}
}
