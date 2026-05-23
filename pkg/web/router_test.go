package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/handlers"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// nopLogger satisfies mid.Logger without printing during tests.
type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

func mustUI(t *testing.T) *handlers.UI {
	t.Helper()
	ui, err := handlers.NewUI()
	if err != nil {
		t.Fatalf("NewUI: %v", err)
	}
	return ui
}

// minimalDeps returns the smallest Deps that NewRouter can be invoked with
// safely — only Health is mandatory because /healthz and /readyz are wired
// unconditionally.
func minimalDeps() Deps {
	return Deps{
		Health: &handlers.Health{},
		Logger: nopLogger{},
	}
}

// TestNewRouter_PanicFreeWithRateLimitAndUI guards against a regression of the
// chi "all middlewares must be defined before routes on a mux" panic we hit
// when the rate-limit middleware was registered after the UI mount.
func TestNewRouter_PanicFreeWithRateLimitAndUI(t *testing.T) {
	d := minimalDeps()
	d.UI = mustUI(t)
	d.RateLimit = mid.NewRateLimit(100, 10)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewRouter panicked with RateLimit+UI: %v", r)
		}
	}()

	if NewRouter(d) == nil {
		t.Fatal("NewRouter returned nil")
	}
}

// TestNewRouter_DepsPermutations sweeps the optional fields and asserts that
// NewRouter never panics regardless of which combination is wired.
func TestNewRouter_DepsPermutations(t *testing.T) {
	ui := mustUI(t)
	cases := []struct {
		name string
		mut  func(*Deps)
	}{
		{"bare", func(*Deps) {}},
		{"+ui", func(d *Deps) { d.UI = ui }},
		{"+ratelimit", func(d *Deps) { d.RateLimit = mid.NewRateLimit(10, 1) }},
		{"+ui+ratelimit", func(d *Deps) {
			d.UI = ui
			d.RateLimit = mid.NewRateLimit(10, 1)
		}},
		{"+mcp", func(d *Deps) { d.MCPServer = http.NotFoundHandler() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := minimalDeps()
			tc.mut(&d)
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("NewRouter panicked: %v", r)
				}
			}()
			if NewRouter(d) == nil {
				t.Fatal("nil router")
			}
		})
	}
}

// TestRouter_PublicRoutesUnauthenticated proves /healthz, /readyz, and the
// UI redirect respond without an Authorization header — these endpoints
// must stay reachable for k8s probes and the SPA landing page.
func TestRouter_PublicRoutesUnauthenticated(t *testing.T) {
	d := minimalDeps()
	d.UI = mustUI(t)
	d.RateLimit = mid.NewRateLimit(100, 10)

	srv := httptest.NewServer(NewRouter(d))
	defer srv.Close()

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	cases := []struct {
		path string
		want int
	}{
		{"/healthz", http.StatusOK},
		{"/readyz", http.StatusOK},
		{"/", http.StatusFound}, // 302 → /ui/
		{"/ui/", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			resp, err := noFollow.Get(srv.URL + c.path)
			if err != nil {
				t.Fatalf("GET %s: %v", c.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != c.want {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("GET %s: got %d want %d (body: %s)", c.path, resp.StatusCode, c.want, body)
			}
		})
	}
}

// TestRouter_HealthzNotRateLimited verifies the rate-limit middleware doesn't
// throttle /healthz — k8s liveness/readiness probes must always succeed,
// even under hostile traffic that has drained the bucket.
func TestRouter_HealthzNotRateLimited(t *testing.T) {
	d := minimalDeps()
	d.RateLimit = mid.NewRateLimit(1, 0) // exactly one token, no refill

	srv := httptest.NewServer(NewRouter(d))
	defer srv.Close()

	for i := 0; i < 5; i++ {
		resp, err := http.Get(srv.URL + "/healthz")
		if err != nil {
			t.Fatalf("GET /healthz #%d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /healthz #%d: got %d, want 200 — rate limit leaked into public routes", i, resp.StatusCode)
		}
	}
}
