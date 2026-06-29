package golden

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
)

// LoadGoldenDir discovers and parses every *.json dataset file under dir on the
// real filesystem. It is a thin wrapper over LoadGoldenFS using os.DirFS, so the
// CLI (filesystem) and the embedded gate (embed.FS) share one code path.
func LoadGoldenDir(dir string) ([]Case, []Dataset, error) {
	return LoadGoldenFS(os.DirFS(dir), ".")
}

// LoadGoldenFS discovers and parses every *.json dataset file under root within
// fsys (recursively), returning all cases flattened across datasets plus the
// parsed datasets. It enforces the no-pre-baked-output invariant: any case that
// still carries actual_output or evaluation_results is a hard error, because
// those fields mean the case was scored ahead of time rather than at evaluation.
//
// Files named INDEX.json are treated as a manifest and skipped here.
func LoadGoldenFS(fsys fs.FS, root string) ([]Case, []Dataset, error) {
	var files []string
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if path.Ext(p) != ".json" {
			return nil
		}
		if path.Base(p) == "INDEX.json" {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Strings(files) // deterministic ordering

	var datasets []Dataset
	var allCases []Case
	seen := map[string]string{} // case ID -> file (duplicate detection)

	for _, f := range files {
		ds, err := loadDatasetFileFS(fsys, f)
		if err != nil {
			return nil, nil, err
		}
		if got, want := len(ds.Cases), ds.TotalCases; want != 0 && got != want {
			return nil, nil, fmt.Errorf("%s: total_cases=%d but file contains %d cases", f, want, got)
		}
		for _, c := range ds.Cases {
			if c.ID == "" {
				return nil, nil, fmt.Errorf("%s: a case is missing an id", f)
			}
			if prev, dup := seen[c.ID]; dup {
				return nil, nil, fmt.Errorf("duplicate case id %q in %s (already in %s)", c.ID, f, prev)
			}
			seen[c.ID] = f
		}
		datasets = append(datasets, *ds)
		allCases = append(allCases, ds.Cases...)
	}
	return allCases, datasets, nil
}

// loadDatasetFileFS parses one dataset file from fsys and rejects pre-baked output.
func loadDatasetFileFS(fsys fs.FS, name string) (*Dataset, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if err := rejectPrebaked(name, raw); err != nil {
		return nil, err
	}
	var ds Dataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return &ds, nil
}

// rejectPrebaked fails if the raw JSON contains any pre-baked scoring fields.
// We scan the decoded structure rather than the bytes so a field named
// "actual_output" nested anywhere is caught, while a substring in free text is
// not falsely flagged.
func rejectPrebaked(path string, raw []byte) error {
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	banned := map[string]bool{
		"actual_output":      true,
		"evaluation_results": true,
	}
	var walk func(v any) string
	walk = func(v any) string {
		switch t := v.(type) {
		case map[string]any:
			for k, child := range t {
				if banned[k] {
					return k
				}
				if hit := walk(child); hit != "" {
					return hit
				}
			}
		case []any:
			for _, child := range t {
				if hit := walk(child); hit != "" {
					return hit
				}
			}
		}
		return ""
	}
	if hit := walk(generic); hit != "" {
		return fmt.Errorf("%s: contains pre-baked field %q — golden cases must carry only input+expected, "+
			"the verdict is produced at evaluation time (see EVAL_EXPANSION_PLAN.md MF-4)", path, hit)
	}
	return nil
}
