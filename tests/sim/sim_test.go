//go:build e2e

// Package sim contains the user-simulation test that drives a real Genie
// server end-to-end the way the embedded SPA would: signup → /users/me →
// create account → upload encrypted CSV → /v1/ask → /v1/disclosures.
//
// The default build tag keeps it out of `go test ./...` because it needs a
// live server. Run it with:
//
//	make e2e
//	# or
//	go test -tags=e2e -v ./tests/sim/...
//
// Targets a live stack on http://localhost:8080 by default; override with
// GENIE_BASE_URL.
package sim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const sampleCSV = `date,description,category,amount,type
2026-01-01,Salary,Income,50000,credit
2026-01-05,Swiggy,Food,350,debit
2026-02-01,Rent,Housing,15000,debit
`

// client wraps the bits of net/http we need with the user's bearer token
// auto-attached. Stays a struct so subtests can hang test state off it.
type client struct {
	t       *testing.T
	base    string
	token   string
	user    user
	httpCli *http.Client
}

type user struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

type signupResp struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	User      user   `json:"user"`
}

type uploadResp struct {
	ID             string `json:"id"`
	Classification string `json:"classification"`
	KEKID          string `json:"kek_id"`
}

type askResp struct {
	TraceID      string `json:"trace_id"`
	Report       string `json:"report"`
	AIDisclosure string `json:"ai_disclosure"`
}

type disclosuresResp struct {
	PolicyVersion string `json:"policy_version"`
	HomeRegion    string `json:"home_region"`
}

func newClient(t *testing.T) *client {
	base := os.Getenv("GENIE_BASE_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	c := &client{
		t:       t,
		base:    base,
		httpCli: &http.Client{Timeout: 180 * time.Second},
	}
	c.requireServerUp()
	return c
}

// requireServerUp skips the whole test if /readyz is unreachable so this
// file doesn't false-fail when the user runs the suite without a stack.
func (c *client) requireServerUp() {
	resp, err := http.Get(c.base + "/readyz")
	if err != nil {
		c.t.Skipf("no Genie server at %s (%v) — run `make up` first", c.base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.t.Skipf("Genie server at %s is not ready (status=%d)", c.base, resp.StatusCode)
	}
}

func (c *client) do(method, path string, body io.Reader, contentType string) ([]byte, int) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatalf("build %s %s: %v", method, path, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode
}

func (c *client) postJSON(path string, payload any) ([]byte, int) {
	buf, err := json.Marshal(payload)
	if err != nil {
		c.t.Fatalf("marshal %s: %v", path, err)
	}
	return c.do(http.MethodPost, path, bytes.NewReader(buf), "application/json")
}

func (c *client) get(path string) ([]byte, int) {
	return c.do(http.MethodGet, path, nil, "")
}

func assertStatus(t *testing.T, got, want int, body []byte) {
	t.Helper()
	if got != want {
		t.Fatalf("status=%d want=%d body=%s", got, want, string(body))
	}
}

// TestUserJourney walks the exact flow a real user would take through the
// SPA. Each step depends on state established by the prior step, so the
// subtests run sequentially.
func TestUserJourney(t *testing.T) {
	c := newClient(t)
	email := fmt.Sprintf("sim-%d@genie.local", time.Now().UnixNano())
	var docID string

	t.Run("signup", func(t *testing.T) {
		body, status := c.postJSON("/v1/users", map[string]string{
			"email": email, "name": "Sim", "password": "hunter2hunter2",
		})
		assertStatus(t, status, http.StatusCreated, body)
		var got signupResp
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode signup: %v", err)
		}
		if got.Token == "" {
			t.Fatalf("empty token: %s", body)
		}
		if got.User.Email != email {
			t.Errorf("user.email=%q want %q", got.User.Email, email)
		}
		c.token = got.Token
		c.user = got.User
	})

	t.Run("users/me", func(t *testing.T) {
		body, status := c.get("/v1/users/me")
		assertStatus(t, status, http.StatusOK, body)
		var got user
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode me: %v", err)
		}
		if got.ID != c.user.ID {
			t.Errorf("me.id=%q want %q", got.ID, c.user.ID)
		}
	})

	t.Run("create account", func(t *testing.T) {
		body, status := c.postJSON("/v1/accounts", map[string]string{
			"name": "Salary", "currency": "INR",
		})
		assertStatus(t, status, http.StatusCreated, body)
		var got struct{ ID string `json:"id"` }
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode account: %v", err)
		}
		if got.ID == "" {
			t.Fatalf("empty account id: %s", body)
		}
	})

	t.Run("upload document", func(t *testing.T) {
		// Use classification=internal because the supervisor's RBAC ceiling
		// rejects pii-labelled inputs by default (see protocol.Classification).
		body, status := c.do(http.MethodPost,
			"/v1/documents?description=sim&classification=internal",
			strings.NewReader(sampleCSV), "text/csv")
		assertStatus(t, status, http.StatusCreated, body)
		var got uploadResp
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode upload: %v", err)
		}
		if got.ID == "" {
			t.Fatalf("empty doc id: %s", body)
		}
		if got.KEKID == "" {
			t.Errorf("empty kek_id — envelope encryption metadata missing")
		}
		docID = got.ID
	})

	t.Run("ask", func(t *testing.T) {
		if docID == "" {
			t.Fatal("doc not uploaded — earlier subtest failed")
		}
		body, status := c.postJSON("/v1/ask", map[string]string{
			"question":    "Summarise this month's spending in one sentence.",
			"document_id": docID,
		})
		assertStatus(t, status, http.StatusOK, body)
		var got askResp
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode ask: %v", err)
		}
		if got.Report == "" {
			t.Fatal("empty report — agent pipeline produced no output")
		}
		if got.TraceID == "" {
			t.Errorf("missing trace_id — observability metadata stripped")
		}
		// AI disclosure banner is a regulatory requirement (Rec 18); the test
		// fails if it's not attached so the legal team gets a CI signal.
		if got.AIDisclosure == "" {
			t.Errorf("missing ai_disclosure banner — Rec 18 / Sutra 2 violation")
		}
		t.Logf("trace=%s disclosure=%q report-len=%d", got.TraceID, got.AIDisclosure, len(got.Report))
	})

	t.Run("disclosures public", func(t *testing.T) {
		// Save the token, hit /v1/disclosures unauthenticated, restore. The
		// disclosure surface is supposed to be reachable without login.
		saved := c.token
		c.token = ""
		body, status := c.get("/v1/disclosures")
		c.token = saved

		assertStatus(t, status, http.StatusOK, body)
		var got disclosuresResp
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("decode disclosures: %v", err)
		}
		if got.PolicyVersion == "" {
			t.Errorf("empty policy_version — Annexure V policy not loaded")
		}
		if got.HomeRegion == "" {
			t.Errorf("empty home_region — sovereignty metadata missing")
		}
	})
}

// TestPublicEndpointsNoAuth proves the unauthenticated surface (healthz,
// readyz, /, /ui/) really is reachable without a token. Guards against
// someone accidentally wrapping these in auth middleware.
func TestPublicEndpointsNoAuth(t *testing.T) {
	c := newClient(t)
	noFollow := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	cases := []struct {
		path string
		want int
	}{
		{"/healthz", http.StatusOK},
		{"/readyz", http.StatusOK},
		{"/", http.StatusFound},
		{"/ui/", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := noFollow.Get(c.base + tc.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tc.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("GET %s: got %d want %d body=%s", tc.path, resp.StatusCode, tc.want, body)
			}
		})
	}
}

// TestAuthRejectsUnauthenticated proves /v1/users/me requires the JWT.
// Catches an obvious-but-disastrous regression where auth middleware gets
// disabled.
func TestAuthRejectsUnauthenticated(t *testing.T) {
	c := newClient(t)
	body, status := c.get("/v1/users/me")
	if status != http.StatusUnauthorized {
		t.Fatalf("GET /v1/users/me without token: got %d want 401, body=%s", status, body)
	}
}
