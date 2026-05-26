package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

// ui_security_test.go pins the UI ↔ handler contract for the security
// primitives shipped in the Q1 hardening pass:
//
//   - The /ai-inventory route is admin-only and the UI must NOT fetch it
//     for non-admin sessions (would expose tier + risk metadata).
//   - The InventoryItem JSON contract surfaces the tier field so the risk
//     team can spot non-production agents serving customer traffic.
//   - HTML never embeds admin-only data; admin fields are populated by JS
//     only after isAdmin is true.
//   - The router must protect every admin route — these tests assert the
//     UI side, the integration tests in pkg/web/router_test.go (if any)
//     would cover the server side.

// ---------------------------------------------------------------------------
// 1. UI fetch is gated on isAdmin
// ---------------------------------------------------------------------------

// TestUI_InventoryFetchGatedByAdmin asserts the /ai-inventory request
// is wrapped in an `if (isAdmin)` so a non-admin session never even
// attempts the call. Otherwise even a 403 from the server leaks
// schema info via the error path.
func TestUI_InventoryFetchGatedByAdmin(t *testing.T) {
	js := readUIFile(t, "app.js")
	// Pull the admin-only block. The block starts with the isAdmin
	// computation and runs through the closing brace of the if.
	const anchor = "const isAdmin ="
	idx := strings.Index(js, anchor)
	if idx < 0 {
		t.Fatal("isAdmin computation missing — admin gating regressed")
	}
	block := js[idx:]
	end := strings.Index(block, "// -----")
	if end < 0 {
		end = 1500
		if end > len(block) {
			end = len(block)
		}
	}
	block = block[:end]

	// /ai-inventory must appear inside this block (not before it).
	if !strings.Contains(block, "/ai-inventory") {
		t.Error("/ai-inventory fetch must live under the isAdmin gate — a non-admin session must never even attempt the call")
	}
	if !strings.Contains(block, "/aibom") {
		t.Error("/aibom fetch must live under the isAdmin gate")
	}
	if !strings.Contains(block, "/incidents") {
		t.Error("/incidents fetch must live under the isAdmin gate")
	}

	// And the pre-isAdmin section must NOT call /ai-inventory.
	pre := js[:idx]
	if strings.Contains(pre, "/ai-inventory") {
		t.Error("/ai-inventory fetched before isAdmin check — admin-only data would leak")
	}
}

// TestUI_AdminPanelsNotPopulatedInHTML asserts the admin-only panels
// (#inventory, #aibom, #incidents) are present as empty containers but
// never pre-populated with admin data. If a future build inlined the
// inventory JSON server-side, it would leak even when the UI's JS
// later refuses to fetch.
func TestUI_AdminPanelsNotPopulatedInHTML(t *testing.T) {
	html := readUIFile(t, "index.html")
	for _, id := range []string{"inventory", "aibom", "incidents"} {
		anchor := `id="` + id + `"`
		i := strings.Index(html, anchor)
		if i < 0 {
			t.Errorf("admin panel #%s missing from HTML", id)
			continue
		}
		// Walk to the closing tag of this element. The panel is a <pre>
		// in current markup.
		end := strings.Index(html[i:], "</pre>")
		if end < 0 {
			t.Errorf("admin panel #%s has no closing </pre>", id)
			continue
		}
		body := html[i : i+end]
		// Strip the opening tag.
		gt := strings.Index(body, ">")
		if gt < 0 {
			continue
		}
		inner := strings.TrimSpace(body[gt+1:])
		// A pre-populated panel would have substantive content. Empty,
		// or a single placeholder word, is fine.
		if len(inner) > 80 {
			t.Errorf("admin panel #%s appears to be pre-populated (%d chars) — admin data must not leak in HTML", id, len(inner))
		}
	}
}

// ---------------------------------------------------------------------------
// 2. /ai-inventory handler exposes the tier field
// ---------------------------------------------------------------------------

// fixtureAgent is a tier-aware, risk-aware agent for the inventory test.
type fixtureAgent struct {
	id, name string
	tier     agent.Tier
	risk     agent.RiskClass
	caps     []string
}

func (f *fixtureAgent) ID() string                 { return f.id }
func (f *fixtureAgent) Name() string               { return f.name }
func (f *fixtureAgent) Capabilities() []string     { return f.caps }
func (f *fixtureAgent) Tier() agent.Tier           { return f.tier }
func (f *fixtureAgent) RiskLevel() agent.RiskClass { return f.risk }
func (f *fixtureAgent) HandleMessage(_ context.Context, _ protocol.Message, _ agent.Environment) ([]protocol.Message, error) {
	return nil, nil
}

// TestInventory_ListIncludesTier confirms the inventory handler emits
// the tier field. Risk reviewers read this column on the inventory
// page to spot non-production agents that are receiving customer
// traffic — without the field, a sketch agent on the bus would be
// invisible to the dashboard.
func TestInventory_ListIncludesTier(t *testing.T) {
	reg := registry.NewInMemory()
	_ = reg.Register(context.Background(), &fixtureAgent{
		id: "prod", name: "Production Agent",
		tier: agent.TierProduction, risk: agent.RiskMedium,
		caps: []string{"finance.read"},
	})
	_ = reg.Register(context.Background(), &fixtureAgent{
		id: "sketch", name: "Sketch Agent",
		tier: agent.TierSketch, risk: agent.RiskLow,
		caps: []string{"finance.scratch"},
	})

	h := &Inventory{Reg: reg, Fallbacks: map[string]string{"prod": "prod_fallback"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/ai-inventory", nil)
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	var items []InventoryItem
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, body)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 inventory items; got %d", len(items))
	}

	tiers := map[string]agent.Tier{}
	for _, it := range items {
		tiers[it.ID] = it.Tier
	}
	if tiers["prod"] != agent.TierProduction {
		t.Errorf("prod agent tier should be production; got %q", tiers["prod"])
	}
	if tiers["sketch"] != agent.TierSketch {
		t.Errorf("sketch agent tier should be sketch; got %q", tiers["sketch"])
	}
	// JSON field name must be exactly "tier" — the dashboard column
	// selector depends on it.
	if !strings.Contains(string(body), `"tier":`) {
		t.Errorf(`response missing "tier" field — UI column will be empty; body: %s`, body)
	}
}

// TestInventory_TierFieldStableJSONName pins the JSON field name to
// `tier` against accidental renames. The UI table column header reads
// this name; rename → silent UI breakage.
func TestInventory_TierFieldStableJSONName(t *testing.T) {
	it := InventoryItem{ID: "x", Name: "x", Tier: agent.TierBeta}
	b, _ := json.Marshal(it)
	// The exact key plus value avoids matching a stringified inner
	// value that happens to contain "tier".
	want := `"tier":"beta"`
	if !strings.Contains(string(b), want) {
		t.Errorf("InventoryItem JSON must contain %q; got %s", want, b)
	}
}

// ---------------------------------------------------------------------------
// 3. No admin field leaks into the public DOM
// ---------------------------------------------------------------------------

// TestUI_NoAdminFieldsInPublicHTML asserts strings that should only
// arrive over the wire from /ai-inventory (tier names, the sentinel
// admin tenant id, RBI agent IDs known to be admin-only) are not
// embedded in the static HTML or CSS. Otherwise a logged-out user
// viewing the page source would see them.
func TestUI_NoAdminFieldsInPublicHTML(t *testing.T) {
	html := readUIFile(t, "index.html")
	css := readUIFile(t, "styles.css")
	combined := html + "\n" + css

	// __admin__ is the RLS sentinel — must never appear in the UI.
	if strings.Contains(combined, "__admin__") {
		t.Error("UI must not embed the RLS admin sentinel '__admin__' — would leak the bypass keyword to the public source")
	}
	// Internal tenant config keys must not be hard-coded into the UI.
	for _, bad := range []string{"app.current_tenant", "BYPASSRLS", "set_config"} {
		if strings.Contains(combined, bad) {
			t.Errorf("UI must not embed Postgres RLS implementation detail %q", bad)
		}
	}
}

// TestUI_AdminGuardChecksRolesArray pins the JS shape used to decide
// admin status: `(state.user && state.user.roles || []).includes('admin')`.
// If a refactor changed this to `state.user.role === 'admin'` it would
// miss multi-role users and grant or revoke admin access incorrectly.
func TestUI_AdminGuardChecksRolesArray(t *testing.T) {
	js := readUIFile(t, "app.js")
	// Allow whitespace and quote variation around `.includes('admin')`.
	guardRE := regexp.MustCompile(`state\.user\s*&&\s*state\.user\.roles[^.]*\.includes\(['"]admin['"]\)`)
	if !guardRE.MatchString(js) {
		t.Error("admin guard shape changed — must remain `(state.user && state.user.roles || []).includes('admin')` so multi-role users are recognised")
	}
}

// ---------------------------------------------------------------------------
// 4. The session storage shape includes roles (so the UI can compute isAdmin)
// ---------------------------------------------------------------------------

// TestUI_PersistSessionStoresRoles asserts the session persisted to
// localStorage carries the roles array, not just the token. Without
// roles, isAdmin always evaluates false after a page reload and the
// admin sees an empty governance tab.
func TestUI_PersistSessionStoresRoles(t *testing.T) {
	js := readUIFile(t, "app.js")
	// persistSession() body must reference state.user when writing.
	const anchor = "function persistSession"
	i := strings.Index(js, anchor)
	if i < 0 {
		t.Fatal("persistSession() missing")
	}
	end := i + 400
	if end > len(js) {
		end = len(js)
	}
	body := js[i:end]
	if !strings.Contains(body, "state.user") {
		t.Error("persistSession() must include state.user (with roles) — otherwise admin status is lost across reloads")
	}
}
