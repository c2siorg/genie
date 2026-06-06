package handlers

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// appBundleBuilt reports whether the React app bundle has been built into the
// embedded FS (pkg/web/handlers/ui/app/index.html). When it hasn't (a fresh
// checkout where `make ui-build` hasn't run), the SPA tests skip rather than
// fail — the legacy UI and the rest of the suite stay green regardless.
func appBundleBuilt(t *testing.T) *UI {
	t.Helper()
	h, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}
	if _, err := fs.ReadFile(h.root, "app/index.html"); err != nil {
		t.Skip("react app bundle not built (run `make ui-build`); skipping SPA serving test")
	}
	return h
}

func TestUIApp_ServesIndexAtAppRoot(t *testing.T) {
	h := appBundleBuilt(t)
	// chi strips the /ui/ mount prefix, so the app root arrives as "/app/".
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /app/ = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Errorf("app index missing React root mount point")
	}
}

func TestUIApp_ClientRouteFallsBackToIndex(t *testing.T) {
	h := appBundleBuilt(t)
	// A client-side route with no backing file must still serve the SPA shell.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/commerce/orders", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /app/commerce/orders = %d, want 200 (SPA fallback)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="root"`) {
		t.Errorf("SPA fallback did not serve the app shell")
	}
}

func TestUIApp_MissingAssetStill404s(t *testing.T) {
	h := appBundleBuilt(t)
	// A missing asset (has an extension) must NOT be masked by the SPA fallback.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/assets/does-not-exist.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing asset = %d, want 404 (must not mask broken builds)", rec.Code)
	}
}

func TestIsAppRoute(t *testing.T) {
	cases := map[string]bool{
		"app":                    true,
		"app/":                   true,
		"app/commerce":           true,
		"app/commerce/orders":    true,
		"app/assets/index-x.js":  false, // has extension → asset, not a route
		"app/index.html":         false,
		"index.html":             false,
		"styles.css":             false,
		"":                       false,
	}
	for rel, want := range cases {
		if got := isAppRoute(rel); got != want {
			t.Errorf("isAppRoute(%q) = %v, want %v", rel, got, want)
		}
	}
}
