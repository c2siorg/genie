package afg

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSingleConstructionDoor enforces §6 of docs/migration-agent-framework.md: the
// governance gate is only unbypassable if every framework agent is built through
// afg.NewGoverned* (this package). The framework exposes no way to inspect an
// agent's middleware chain after construction, so we enforce by construction
// discipline: NO framework agent constructor may appear in a framework-importing
// file OUTSIDE pkg/afg. CI fails otherwise.
func TestSingleConstructionDoor(t *testing.T) {
	const frameworkImport = "github.com/microsoft/agent-framework-go"

	// Word-boundary on agent.New so legacy calls like `alm_agent.New()` (where the
	// char before "agent" is '_', a word char, so no boundary) do NOT match.
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`\bagent\.New\(`),
		regexp.MustCompile(`\w*provider\.New\w*Agent\(`),
	}

	repoRoot := "../.." // pkg/afg -> pkg -> repo root
	var violations []string

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "geniepython", "microsoftagentframeworklearning", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		slash := filepath.ToSlash(path)
		if strings.Contains(slash, "pkg/afg/") { // the factory package is the door itself
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		src := string(b)
		if !strings.Contains(src, frameworkImport) { // only files that touch the framework
			return nil
		}
		for _, re := range forbidden {
			if re.MatchString(src) {
				violations = append(violations, slash+"  ::  "+re.String())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("framework agents must be built via afg.NewGoverned* (the single door), "+
			"not raw framework constructors:\n  %s", strings.Join(violations, "\n  "))
	}
}
