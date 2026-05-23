package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUI_ServesIndexAtRoot(t *testing.T) {
	h, err := NewUI()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type: %q", ct)
	}
	if !strings.Contains(w.Body.String(), "Genie") {
		t.Fatal("missing Genie title in body")
	}
}

func TestUI_ServesNamedAssets(t *testing.T) {
	h, _ := NewUI()
	for _, name := range []string{"styles.css", "app.js"} {
		req := httptest.NewRequest(http.MethodGet, "/"+name, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", name, w.Code)
		}
		if w.Body.Len() == 0 {
			t.Fatalf("%s: empty body", name)
		}
	}
}

func TestUI_BlocksTraversal(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/../../etc/passwd", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on traversal, got %d", w.Code)
	}
}

func TestUI_IndexRedirect(t *testing.T) {
	h, _ := NewUI()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.IndexHTML(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/ui/" {
		t.Fatalf("expected redirect to /ui/, got %q", loc)
	}
}
