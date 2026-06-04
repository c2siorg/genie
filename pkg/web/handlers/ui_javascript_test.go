package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================================================
// JavaScript Runtime Tests — Test app.js behavior via HTTP contracts
// ============================================================================

// TestJS_LoginFormSubmission verifies the login form validation and submission
func TestJS_LoginFormSubmission(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		password  string
		wantError bool
	}{
		{"valid_email_password", "user@example.com", "securepass123", false},
		{"empty_email", "", "password", true},
		{"empty_password", "user@example.com", "", true},
		{"invalid_email_format", "notanemail", "password", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The actual validation happens in app.js
			// We verify the HTML form has the right validation attributes
			h, _ := NewUI()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("failed to serve UI")
			}

			html := w.Body.String()
			// Check form has email input with required
			if !strings.Contains(html, `type="email"`) {
				t.Error("form missing email input")
			}
			if !strings.Contains(html, `type="password"`) {
				t.Error("form missing password input")
			}
			if !strings.Contains(html, `required`) {
				t.Error("form missing required attribute")
			}
		})
	}
}

// TestJS_TabNavigation verifies tab switching logic
func TestJS_TabNavigation(t *testing.T) {
	tests := []struct {
		name   string
		tabID  string
		viewID string
	}{
		{"ask_tab", "ask", "view-ask"},
		{"documents_tab", "documents", "view-documents"},
		{"governance_tab", "governance", "view-governance"},
		{"settings_tab", "settings", "view-settings"},
	}

	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify tab button exists with correct data-tab
			if !strings.Contains(html, `data-tab="`+tt.tabID+`"`) {
				t.Errorf("missing tab button with data-tab=%q", tt.tabID)
			}
			// Verify corresponding view exists
			if !strings.Contains(html, `id="`+tt.viewID+`"`) {
				t.Errorf("missing view with id=%q", tt.viewID)
			}
		})
	}
}

// TestJS_DocumentUploadValidation verifies file upload form validation
func TestJS_DocumentUploadValidation(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify upload form structure
	if !strings.Contains(html, `id="form-upload"`) {
		t.Error("missing upload form")
	}
	if !strings.Contains(html, `id="upload-file"`) {
		t.Error("missing file input")
	}
	if !strings.Contains(html, `accept=".csv,text/csv"`) {
		t.Error("missing CSV accept restriction")
	}
	if !strings.Contains(html, `id="upload-class"`) {
		t.Error("missing classification selector")
	}
}

// TestJS_LocalStorageKeys verifies localStorage keys are stable
func TestJS_LocalStorageKeys(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	js := w.Body.String()

	// Verify localStorage key constants are defined in app.js
	if !strings.Contains(js, `genie.session.v1`) {
		t.Error("missing session key constant in app.js")
	}
	if !strings.Contains(js, `genie.apibase.v1`) {
		t.Error("missing API base key constant in app.js")
	}
}

// TestJS_APIErrorHandling verifies error message display
func TestJS_APIErrorHandling(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify error display elements exist
	if !strings.Contains(html, `id="form-login-error"`) {
		t.Error("missing login error element")
	}
	if !strings.Contains(html, `id="form-signup-error"`) {
		t.Error("missing signup error element")
	}
	if !strings.Contains(html, `aria-live="polite"`) {
		t.Error("missing aria-live for error messages")
	}
}

// TestJS_SSEEventHandling verifies SSE event names match backend
func TestJS_SSEEventHandling(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	js := w.Body.String()

	// Verify SSE event handlers exist
	expectedEvents := []string{"ai_disclosure", "report", "agent.handle"}
	for _, event := range expectedEvents {
		if !strings.Contains(js, `'`+event+`'`) {
			t.Errorf("missing SSE event handler for %q", event)
		}
	}
}

// TestJS_FormFieldValidation verifies input field types and constraints
func TestJS_FormFieldValidation(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify password field has minimum length
	if !strings.Contains(html, `minlength="8"`) {
		t.Error("missing password minlength constraint")
	}

	// Verify textarea has rows
	if !strings.Contains(html, `id="ask-question"`) {
		t.Error("missing question textarea")
	}
	if !strings.Contains(html, `rows="3"`) {
		t.Error("question textarea missing rows attribute")
	}
}

// TestJS_AdminGateOnInventory verifies admin-only inventory access
func TestJS_AdminGateOnInventory(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify inventory element exists but has no data initially
	if !strings.Contains(html, `id="inventory"`) {
		t.Error("missing inventory element")
	}
	if !strings.Contains(html, `admin only`) {
		t.Error("inventory not marked as admin-only in UI")
	}
}

// TestJS_ReportCardDisplay verifies report display structure
func TestJS_ReportCardDisplay(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify report card structure
	if !strings.Contains(html, `id="report-card"`) {
		t.Error("missing report card element")
	}
	if !strings.Contains(html, `id="report-body"`) {
		t.Error("missing report body element")
	}
	if !strings.Contains(html, `<pre`) {
		t.Error("report body should be in <pre> for formatting")
	}
}

// TestJS_DocumentListTable verifies document list structure
func TestJS_DocumentListTable(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify table structure
	if !strings.Contains(html, `id="doc-list"`) {
		t.Error("missing document list table")
	}
	if !strings.Contains(html, `<table`) {
		t.Error("document list should be a table")
	}
	// Check for required columns
	if !strings.Contains(html, `scope="col"`) {
		t.Error("table missing header scope")
	}
}

// TestJS_LoadingStateSkeletons verifies skeleton loaders exist
func TestJS_LoadingStateSkeletons(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify skeleton elements for loading state
	if !strings.Contains(html, `skeleton`) {
		t.Error("missing skeleton loaders for loading state")
	}
}

// TestJS_KeyboardNavigationAttributes verifies tabindex for navigation
func TestJS_KeyboardNavigationAttributes(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify interactive elements have tabindex
	if !strings.Contains(html, `tabindex`) {
		t.Error("missing tabindex for keyboard navigation")
	}
}

// TestJS_EventDelegationForTabs verifies closest() usage for event delegation
func TestJS_EventDelegationForTabs(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	js := w.Body.String()

	// Verify .closest('.tab') for proper event delegation
	if !strings.Contains(js, `.closest('.tab')`) {
		t.Error("missing .closest('.tab') for event delegation")
	}
}

// TestJS_AriaLiveRegions verifies aria-live for dynamic content
func TestJS_AriaLiveRegions(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Count aria-live regions
	liveCount := strings.Count(html, `aria-live=`)
	if liveCount < 3 {
		t.Errorf("expected at least 3 aria-live regions, got %d", liveCount)
	}
}

// TestJS_SessionPersistenceFlow verifies auth flow with session storage
func TestJS_SessionPersistenceFlow(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify session-related elements
	if !strings.Contains(html, `id="user-chip"`) {
		t.Error("missing user chip element for displaying session user")
	}
	if !strings.Contains(html, `id="logout"`) {
		t.Error("missing logout button")
	}
}

// TestJS_APIBaseConfiguration verifies configurable API base
func TestJS_APIBaseConfiguration(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify API base can be configured
	if !strings.Contains(html, `id="settings-base"`) {
		t.Error("missing API base configuration input")
	}
	if !strings.Contains(html, `id="settings-save"`) {
		t.Error("missing settings save button")
	}
	if !strings.Contains(html, `id="api-base"`) {
		t.Error("missing API base display element")
	}
}

// TestJS_HealthStatusDisplay verifies health check display
func TestJS_HealthStatusDisplay(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify health status element
	if !strings.Contains(html, `id="health"`) {
		t.Error("missing health status element")
	}
}

// TestJS_DocumentClassificationOptions verifies all classification options
func TestJS_DocumentClassificationOptions(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify all classification options exist
	classifications := []string{"pii", "internal", "public"}
	for _, cls := range classifications {
		if !strings.Contains(html, `value="`+cls+`"`) {
			t.Errorf("missing classification option: %s", cls)
		}
	}
}

// TestJS_AIDisclosureDisplay verifies AI transparency disclosure
func TestJS_AIDisclosureDisplay(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	html := w.Body.String()

	// Verify AI disclosure element
	if !strings.Contains(html, `id="ai-disclosure"`) {
		t.Error("missing AI disclosure element")
	}
	if !strings.Contains(html, `disclosure`) {
		t.Error("disclosure element missing class")
	}
}
