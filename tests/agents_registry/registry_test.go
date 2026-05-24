// Package agents_registry holds cross-cutting tests that aren't tied to a
// single agent package. The Go test runner picks them up via the normal
// `go test ./...` sweep.
//
// What we check here:
//
//   * Every agent package under agents/ declares a stable string ID.
//   * No two agents share the same ID — the registry would otherwise
//     silently overwrite handlers.
//   * The set of declared IDs matches the directory list (no orphaned
//     directories, no missing IDs).
//   * Every agent has a unit test file.
//
// Implementation note: rather than import every agent package (≥40
// imports, brittle), we scan the source tree for the canonical
// `const (... ID = "..." ...)` declaration. This is what the README
// "agent contract" requires, so the grep doubles as a contract test.
package agents_registry

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// idDecl matches `ID = "agent_id"` inside an agent .go file.
var idDecl = regexp.MustCompile(`(?m)^\s*ID\s*=\s*"([a-zA-Z][a-zA-Z0-9_]*)"`)

func agentsRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test's CWD until we find an `agents/` sibling.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for d := wd; d != "/" && d != "."; d = filepath.Dir(d) {
		candidate := filepath.Join(d, "agents")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	t.Fatalf("agents/ root not found from %s", wd)
	return ""
}

// agentDirs returns every direct subdirectory of agents/ that contains at
// least one non-test .go file.
func agentDirs(t *testing.T) []string {
	root := agentsRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read agents/: %v", err)
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(root, e.Name()))
		hasGo := false
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
				continue
			}
			hasGo = true
			break
		}
		if hasGo {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// extractIDs returns the agent IDs declared inside a directory's .go files.
func extractIDs(t *testing.T, dir string) []string {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	ids := []string{}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".go") || strings.HasSuffix(f.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}
		for _, m := range idDecl.FindAllStringSubmatch(string(b), -1) {
			ids = append(ids, m[1])
		}
	}
	return ids
}

// TestAgentRegistry_IDsUnique asserts no two agents declare the same ID.
func TestAgentRegistry_IDsUnique(t *testing.T) {
	seen := map[string]string{} // id → first-seen directory
	for _, dir := range agentDirs(t) {
		for _, id := range extractIDs(t, dir) {
			if prev, dup := seen[id]; dup {
				t.Errorf("duplicate agent id %q in %s (already in %s)", id, dir, prev)
			}
			seen[id] = dir
		}
	}
}

// TestAgentRegistry_EveryDirHasID guarantees every agent directory
// declares at least one ID (catches missing-const drift). A small set of
// factory packages are exempted — they parameterise the ID at runtime.
func TestAgentRegistry_EveryDirHasID(t *testing.T) {
	factoryExempt := map[string]bool{
		"fallback": true, // NewFor("<primary>") → "<primary>_fallback"
	}
	for _, dir := range agentDirs(t) {
		name := filepath.Base(dir)
		if factoryExempt[name] {
			continue
		}
		ids := extractIDs(t, dir)
		if len(ids) == 0 {
			t.Errorf("%s: no ID constant found", name)
		}
	}
}

// TestAgentRegistry_HasMinimumCount sanity check — Genie ships 40+ agents.
// If this count drops the README is out of date or an agent went missing.
func TestAgentRegistry_HasMinimumCount(t *testing.T) {
	const want = 40
	got := len(agentDirs(t))
	if got < want {
		t.Errorf("agent count regressed: got %d want ≥%d — did a directory move?", got, want)
	}
}

// TestAgentRegistry_EveryAgentHasTests asserts each non-trivial agent
// directory ships a *_test.go. Catches new agents shipping without
// coverage.
func TestAgentRegistry_EveryAgentHasTests(t *testing.T) {
	// Some early agents are too small for a dedicated test file (single
	// constructor wrapper). Exempt them by name to keep the test honest.
	exempt := map[string]bool{
		"voice":             true, // Bhashini stub — no logic
		"aa_fetcher":        true, // AA stub
		"portfolio_advisor": true, // MCP-wrapped, tested via mcp_client tests
		"h_supervisor":      true, // hierarchical router, exercised via supervisor tests
	}
	for _, dir := range agentDirs(t) {
		name := filepath.Base(dir)
		if exempt[name] {
			continue
		}
		files, _ := os.ReadDir(dir)
		hasTest := false
		for _, f := range files {
			if strings.HasSuffix(f.Name(), "_test.go") {
				hasTest = true
				break
			}
		}
		if !hasTest {
			t.Errorf("%s: no *_test.go found", name)
		}
	}
}
