package handlers

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// The console is a Next.js app statically exported into ui/. Its logic is compiled
// into ui/_next/static/chunks/*.js, so the UI<->handler contract is asserted against
// the compiled bundle (the hand-written app.js/styles.css no longer exist). These
// tests keep the same guarantee the old DOM tests gave: if a backend route, auth
// field, classification, SSE event, or storage key changes, a test fails until the
// UI is rebuilt (`make ui`).

// readUIFile pulls a file out of the embedded UI FS via the same root the
// production handler uses. If NewUI's embed path drifts, this starts failing.
func readUIFile(t *testing.T, name string) string {
	t.Helper()
	ui, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}
	b, err := fs.ReadFile(ui.root, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// bundleJS concatenates every emitted JS chunk of the export.
func bundleJS(t *testing.T) string {
	t.Helper()
	ui, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}
	var b strings.Builder
	err = fs.WalkDir(ui.root, "_next", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(p, ".js") {
			data, readErr := fs.ReadFile(ui.root, p)
			if readErr != nil {
				return readErr
			}
			b.Write(data)
			b.WriteByte('\n')
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk _next: %v", err)
	}
	if b.Len() == 0 {
		t.Fatal("no JS chunks in export — the ui/ copy from web-next/out is broken")
	}
	return b.String()
}

// TestUI_IndexIsPrerendered: the exported index.html is the SSG shell and must
// carry the brand + hero, so a bare load shows content before hydration.
func TestUI_IndexIsPrerendered(t *testing.T) {
	html := readUIFile(t, "index.html")
	for _, want := range []string{"Genie", "governed control room for money"} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
}

// TestUI_IndexReferencesBundlesThatExist: every /ui/_next/*.js|css the shell
// references must exist in the embedded FS — catches a half-copied export.
func TestUI_IndexReferencesBundlesThatExist(t *testing.T) {
	html := readUIFile(t, "index.html")
	ui, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}
	linkRE := regexp.MustCompile(`/ui/(_next/[^"]+?\.(?:js|css))`)
	ms := linkRE.FindAllStringSubmatch(html, -1)
	if len(ms) == 0 {
		t.Fatal("index.html references no _next bundles — export layout changed")
	}
	for _, m := range ms {
		p := m[1]
		t.Run(p, func(t *testing.T) {
			if _, err := fs.Stat(ui.root, p); err != nil {
				t.Errorf("index.html references %s but it's missing from the embedded FS: %v", p, err)
			}
		})
	}
}

// TestUI_BundleMatchesAPIContract: the compiled console must still call the exact
// API paths the Go handlers serve. Rename a route in the backend and this fails
// until the UI is rebuilt — the guarantee the old app.js test gave.
func TestUI_BundleMatchesAPIContract(t *testing.T) {
	js := bundleJS(t)
	for _, path := range []string{
		"/users/login", "/users", "/ask/stream", "/ask", "/documents",
		"/disclosures", "/ai-inventory", "/aibom", "/incidents", "/readyz",
	} {
		if !strings.Contains(js, path) {
			t.Errorf("compiled console never references %q — UI/handler contract drift", path)
		}
	}
}

// TestUI_BundleCoversAuthAndClassification: the auth field names the Users handler
// decodes and the classification labels the documents handler accepts. "secret" is
// intentionally absent — uploading secret-classified data via a browser is a policy
// violation, so it is not user-selectable.
func TestUI_BundleCoversAuthAndClassification(t *testing.T) {
	js := bundleJS(t)
	for _, want := range []string{"email", "password", "name"} {
		if !strings.Contains(js, want) {
			t.Errorf("auth field %q missing from compiled console — handler expects this JSON key", want)
		}
	}
	for _, v := range []string{"pii", "internal", "public"} {
		if !strings.Contains(js, v) {
			t.Errorf("classification %q missing from compiled console", v)
		}
	}
	if strings.Contains(js, `"secret"`) {
		t.Error("classification \"secret\" must not be user-selectable in the browser UI")
	}
}

// TestUI_BundleHandlesSSEContract: the streaming reader must understand the SSE
// event names /ask/stream emits.
func TestUI_BundleHandlesSSEContract(t *testing.T) {
	js := bundleJS(t)
	for _, want := range []string{"text/event-stream", "ai_disclosure", "agent.handle"} {
		if !strings.Contains(js, want) {
			t.Errorf("SSE contract token %q missing from compiled console", want)
		}
	}
}

// TestUI_LocalStorageKeysStable pins the persisted keys so a rename that would
// silently log every user out is caught.
func TestUI_LocalStorageKeysStable(t *testing.T) {
	js := bundleJS(t)
	for _, k := range []string{"genie.session.v1", "genie.apibase.v1"} {
		if !strings.Contains(js, k) {
			t.Errorf("localStorage key %q missing — a rename would drop every session", k)
		}
	}
}
