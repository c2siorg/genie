package handlers

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestUI_TabsAreConsistentlyWired checks that every data-tab="X" button in
// the HTML has a matching `views.X` registration in the JS, and vice versa.
// Without this test, renaming a tab in HTML silently breaks navigation.
func TestUI_TabsAreConsistentlyWired(t *testing.T) {
	html := readUIFile(t, "index.html")
	js := readUIFile(t, "app.js")

	tabRE := regexp.MustCompile(`data-tab="([a-z_-]+)"`)
	viewsRE := regexp.MustCompile(`(?ms)const views = \{\s*([^}]+)}`)

	htmlTabs := map[string]bool{}
	for _, m := range tabRE.FindAllStringSubmatch(html, -1) {
		htmlTabs[m[1]] = true
	}
	jsViews := map[string]bool{}
	if m := viewsRE.FindStringSubmatch(js); len(m) > 1 {
		keyRE := regexp.MustCompile(`(?m)^\s*([a-z_-]+)\s*:`)
		for _, km := range keyRE.FindAllStringSubmatch(m[1], -1) {
			jsViews[km[1]] = true
		}
	}
	if len(htmlTabs) == 0 || len(jsViews) == 0 {
		t.Fatal("regex pulled nothing — both ends should declare tabs")
	}

	// Every HTML tab (except 'auth' which is the login view) must have a JS view.
	for tab := range htmlTabs {
		if !jsViews[tab] {
			t.Errorf("data-tab=%q in HTML has no matching views.%s in JS", tab, tab)
		}
	}
	// auth doesn't have a data-tab (it's the landing card) — exempt it from
	// the reverse check.
	for v := range jsViews {
		if v == "auth" {
			continue
		}
		if !htmlTabs[v] {
			t.Errorf("views.%s in JS has no <button data-tab=%q> in HTML", v, v)
		}
	}
}

// TestUI_AuthTabsConsistent does the same for the inline login/signup
// toggles (data-auth="login" / "signup").
func TestUI_AuthTabsConsistent(t *testing.T) {
	html := readUIFile(t, "index.html")
	js := readUIFile(t, "app.js")

	authRE := regexp.MustCompile(`data-auth="([a-z]+)"`)
	htmlAuth := map[string]bool{}
	for _, m := range authRE.FindAllStringSubmatch(html, -1) {
		htmlAuth[m[1]] = true
	}
	for want := range htmlAuth {
		// JS checks `which === '<value>'`.
		needle := `which !== '` + want + `'`
		if !strings.Contains(js, needle) {
			t.Errorf("data-auth=%q has no `which !== '%s'` toggle in JS", want, want)
		}
	}
}

// TestUI_SSEContractMatchesHandler asserts the SSE event names the JS
// switches on are the same ones the backend handler emits. Without this,
// renaming `report` → `final_report` server-side silently breaks the UI.
func TestUI_SSEContractMatchesHandler(t *testing.T) {
	js := readUIFile(t, "app.js")
	// Names the JS branches on inside parseSSE.
	jsEvents := []string{"ai_disclosure", "report", "agent.handle"}

	// The handler file lives in the same package; read it via os to avoid
	// importing internal types.
	handlerPath := "ask_stream.go"
	b, err := os.ReadFile(handlerPath)
	if err != nil {
		t.Fatalf("read %s: %v", handlerPath, err)
	}
	handler := string(b)

	for _, ev := range jsEvents {
		// app.js compares with single quotes, handler emits "event: <name>"
		jsNeedle := `'` + ev + `'`
		if !strings.Contains(js, jsNeedle) {
			t.Errorf("event %q not referenced in JS — test is wrong", ev)
			continue
		}
		// Handler should send the same string.
		serverNeedle := `"` + ev + `"`
		if !strings.Contains(handler, serverNeedle) {
			t.Errorf("backend handler does not emit event %q — UI will silently miss it", ev)
		}
	}
}

// TestUI_CSSReferencesValidClasses scans index.html for class names and
// checks each appears at least once in styles.css. Catches dead CSS
// classes referenced in HTML.
func TestUI_CSSReferencesValidClasses(t *testing.T) {
	html := readUIFile(t, "index.html")
	css := readUIFile(t, "styles.css")

	classRE := regexp.MustCompile(`class="([^"]+)"`)
	seen := map[string]bool{}
	for _, m := range classRE.FindAllStringSubmatch(html, -1) {
		for _, c := range strings.Fields(m[1]) {
			seen[c] = true
		}
	}
	names := make([]string, 0, len(seen))
	for c := range seen {
		names = append(names, c)
	}
	sort.Strings(names)
	for _, c := range names {
		// Skip semantic classes added at runtime by JS (event-error,
		// event-report, event-data, event-kind) — those don't need styles.
		if strings.HasPrefix(c, "event-") {
			continue
		}
		needle := "." + c
		if !strings.Contains(css, needle) {
			t.Errorf("HTML uses class %q but styles.css has no .%s rule", c, c)
		}
	}
}

// TestUI_NoAlertCallsInProductionPaths is a smoke test against leftover
// `alert(...)` calls in code paths that should fail gracefully via flash
// messages. Logins and signups currently use alert() intentionally — we
// pin that count so anyone adding new alerts has to justify it.
func TestUI_NoAlertCallsInProductionPaths(t *testing.T) {
	js := readUIFile(t, "app.js")
	got := strings.Count(js, "alert(")
	const max = 6 // login fail, signup fail, upload fail, ask validation, ask-stream validation, sanity
	if got > max {
		t.Errorf("alert() usage grew to %d (max %d) — prefer in-page flash messages", got, max)
	}
}

// TestUI_LocalStorageKeysStable pins the localStorage keys; the session
// upgrade path depends on stable names.
func TestUI_LocalStorageKeysStable(t *testing.T) {
	js := readUIFile(t, "app.js")
	for _, want := range []string{"genie.session.v1", "genie.apibase.v1"} {
		if !strings.Contains(js, want) {
			t.Errorf("localStorage key %q missing — would break session restore", want)
		}
	}
}

// TestUI_StylesheetNonEmpty guards against a corrupt build that ships an
// empty styles.css.
func TestUI_StylesheetNonEmpty(t *testing.T) {
	ui, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}
	info, err := fs.Stat(ui.root, "styles.css")
	if err != nil {
		t.Fatalf("stat styles.css: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("styles.css unexpectedly tiny (%d bytes) — corrupt build?", info.Size())
	}
}

// dummy reference to silence unused-import on filepath if we ever drop it.
var _ = filepath.Base
