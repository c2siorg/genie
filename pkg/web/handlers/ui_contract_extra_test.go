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

// TestUI_HiddenAttributeOverridesViewDisplay pins the CSS rule that lets the
// `hidden` HTML attribute actually hide `.view` sections. Without an explicit
// `.view[hidden]` (or equivalent) rule, the author rule `.view { display:
// block }` ties on specificity with the UA `[hidden]` rule and wins the
// cascade — so `el.hidden = true` from JS sets the attribute but the section
// stays painted. That regression manifested as the Welcome card sitting on
// top of every tab after login.
func TestUI_HiddenAttributeOverridesViewDisplay(t *testing.T) {
	css := readUIFile(t, "styles.css")
	// Strip whitespace inside braces so we tolerate `{display:none}` or
	// `{ display: none; }`.
	flat := regexp.MustCompile(`\s+`).ReplaceAllString(css, "")
	if !strings.Contains(flat, ".view[hidden]{display:none") {
		t.Errorf("styles.css missing `.view[hidden] { display: none }` — `.view { display: block }` will override the UA [hidden] rule and leave hidden views painted")
	}
}

// TestUI_HideHelperSetsInlineDisplay backs up the CSS test above: even if a
// future stylesheet edit removes the `.view[hidden]` rule, the JS helpers
// should still hide via inline `style.display = 'none'`, which beats any
// class-based rule on specificity.
func TestUI_HideHelperSetsInlineDisplay(t *testing.T) {
	js := readUIFile(t, "app.js")
	// Match `function hide(el) { ... el.style.display = 'none' ... }` with
	// flexible whitespace and quote style.
	hideRE := regexp.MustCompile(`function\s+hide\s*\([^)]*\)\s*\{[^}]*style\.display\s*=\s*['"]none['"]`)
	if !hideRE.MatchString(js) {
		t.Error("hide() helper must set el.style.display = 'none' so hiding works even if CSS regresses")
	}
	showRE := regexp.MustCompile(`function\s+show\s*\([^)]*\)\s*\{[^}]*style\.display\s*=\s*['"]['"]`)
	if !showRE.MatchString(js) {
		t.Error("show() helper must clear el.style.display so previously hidden elements become visible again")
	}
}

// TestUI_LoginSuccessEntersApp asserts both login and signup success paths
// (a) persist the session and (b) call enterApp(), which is what triggers
// hide(views.auth). If either branch ever stops calling enterApp(), the
// Welcome card would stay on screen.
func TestUI_LoginSuccessEntersApp(t *testing.T) {
	js := readUIFile(t, "app.js")

	// sliceBetween isn't quote/brace-aware, and the handler bodies contain
	// `});` from `api(..., {...})` calls, so we walk a fixed window after
	// each anchor instead.
	mustContainNear := func(label, anchor string, want []string) {
		i := strings.Index(js, anchor)
		if i < 0 {
			t.Fatalf("%s: anchor %q not found", label, anchor)
		}
		end := i + 800
		if end > len(js) {
			end = len(js)
		}
		body := js[i:end]
		for _, w := range want {
			if !strings.Contains(body, w) {
				t.Errorf("%s missing %q — needed so the Welcome card hides after sign-in", label, w)
			}
		}
	}

	successCalls := []string{"state.token = out.token", "persistSession()", "enterApp()"}
	mustContainNear("login success path", "$('#form-login').addEventListener", successCalls)
	mustContainNear("signup success path", "$('#form-signup').addEventListener", successCalls)

	// And enterApp itself must hide views.auth — otherwise calling it after
	// login wouldn't move the user out of the auth section.
	mustContainNear("enterApp()", "function enterApp()", []string{"hide(views.auth)"})
}

// TestUI_TabHandlerUsesClosest pins the delegated `#tabs` listener to
// `closest('.tab')` rather than `matches('.tab')`. The matches() variant
// silently misses clicks that land on a descendant node (e.g. a future icon
// inside the button), which surfaced as the Governance tab highlighting
// without ever calling refreshGovernance().
func TestUI_TabHandlerUsesClosest(t *testing.T) {
	js := readUIFile(t, "app.js")
	tabsBlock := sliceBetween(t, js, "$('#tabs').addEventListener", "});")

	if !strings.Contains(tabsBlock, ".closest('.tab')") {
		t.Error("#tabs click handler must use closest('.tab') so clicks on descendant nodes still resolve to the tab button")
	}
	if strings.Contains(tabsBlock, ".matches('.tab')") {
		t.Error("#tabs click handler still uses matches('.tab') — clicks on a descendant node will be ignored")
	}
}

// dummy reference to silence unused-import on filepath if we ever drop it.
var _ = filepath.Base
