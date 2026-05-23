package handlers

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// readUIFile pulls a file out of the embedded UI FS via the same root the
// production handler uses. Keeps the test honest: if NewUI's embed.FS path
// ever drifts, this test starts failing.
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

// TestUI_JSReferencesExistInHTML asserts every #id the JS reaches for
// actually exists in index.html. The vanilla JS in app.js silently no-ops
// against a missing element — without this test a stray HTML rename would
// only surface as a runtime click that does nothing.
func TestUI_JSReferencesExistInHTML(t *testing.T) {
	html := readUIFile(t, "index.html")
	js := readUIFile(t, "app.js")

	// Pull every '#some-id' from JS strings.
	// $('#foo'), $$('#bar'), getElementById('baz'), querySelector('#qux .x')
	idRE := regexp.MustCompile(`['"]#([a-zA-Z][a-zA-Z0-9_-]*)`)
	idGetRE := regexp.MustCompile(`getElementById\(['"]([a-zA-Z][a-zA-Z0-9_-]*)['"]`)

	seen := map[string]bool{}
	for _, m := range idRE.FindAllStringSubmatch(js, -1) {
		seen[m[1]] = true
	}
	for _, m := range idGetRE.FindAllStringSubmatch(js, -1) {
		seen[m[1]] = true
	}
	if len(seen) == 0 {
		t.Fatal("regex pulled zero selectors — pattern probably broke")
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			needle := `id="` + id + `"`
			if !strings.Contains(html, needle) {
				t.Errorf("app.js references #%s but index.html has no element with id=%q", id, id)
			}
		})
	}
}

// TestUI_FormFieldsMatchHandlers asserts the auth form fields use exactly the
// names the Users handler decodes (signupRequest / loginRequest). If someone
// renames the JSON field in users.go we want the UI test to fail too — not
// for the user to discover at runtime that "name" went missing.
func TestUI_FormFieldsMatchHandlers(t *testing.T) {
	html := readUIFile(t, "index.html")

	signupBlock := sliceBetween(t, html, `id="form-signup"`, `</form>`)
	for _, want := range []string{`name="name"`, `name="email"`, `name="password"`} {
		if !strings.Contains(signupBlock, want) {
			t.Errorf("signup form missing %s — handler expects this JSON key", want)
		}
	}

	loginBlock := sliceBetween(t, html, `id="form-login"`, `</form>`)
	for _, want := range []string{`name="email"`, `name="password"`} {
		if !strings.Contains(loginBlock, want) {
			t.Errorf("login form missing %s — handler expects this JSON key", want)
		}
	}
}

// TestUI_ClassificationOptionsCoverHandlerEnum makes sure the upload form's
// classification dropdown lists every value the documents handler accepts.
// If protocol.Classification grows a new label, this test points at the UI
// gap before the new label silently becomes unreachable.
func TestUI_ClassificationOptionsCoverHandlerEnum(t *testing.T) {
	html := readUIFile(t, "index.html")
	uploadBlock := sliceBetween(t, html, `id="upload-class"`, `</select>`)

	// These mirror the constants in pkg/protocol/classification.go that the
	// upload handler accepts on the wire. "secret" is intentionally not in
	// the UI — uploading secret-classified data through a browser is a
	// policy violation, so we don't expose it.
	wantUserSelectable := []string{"pii", "internal", "public"}
	for _, v := range wantUserSelectable {
		if !strings.Contains(uploadBlock, `value="`+v+`"`) {
			t.Errorf("upload-class dropdown missing option value=%q", v)
		}
	}
}

// TestUI_ScriptAndStyleLinksResolve verifies index.html points at filenames
// that actually exist in the embedded FS. Catches a typo like src="app-v2.js"
// before deploy.
func TestUI_ScriptAndStyleLinksResolve(t *testing.T) {
	html := readUIFile(t, "index.html")
	linkRE := regexp.MustCompile(`(?:src|href)="([^"/][^"]*\.(?:js|css))"`)

	ui, err := NewUI()
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}
	for _, m := range linkRE.FindAllStringSubmatch(html, -1) {
		path := m[1]
		t.Run(path, func(t *testing.T) {
			if _, err := fs.Stat(ui.root, path); err != nil {
				t.Errorf("index.html references %s but file missing from embedded FS: %v", path, err)
			}
		})
	}
}

// sliceBetween returns the substring of s starting at the first occurrence of
// start and ending at the first occurrence of end after that. Used to bound
// per-element assertions to one form/select.
func sliceBetween(t *testing.T, s, start, end string) string {
	t.Helper()
	i := strings.Index(s, start)
	if i < 0 {
		t.Fatalf("anchor %q not found", start)
	}
	rest := s[i:]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("closing %q after %q not found", end, start)
	}
	return rest[:j]
}
