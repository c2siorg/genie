package golden

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestLoadGoldenDir_ValidRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "settlement.json", `{
	  "domain": "settlement", "category": "correct", "total_cases": 2,
	  "cases": [
	    {"id":"SE-1","domain":"settlement","failure_mode":"none","scenario":"ok","input":{"order_id":"o1"},"expected":{"verdict":"PASS"}},
	    {"id":"SE-2","domain":"settlement","failure_mode":"FM-SE-001","scenario":"dup","input":{"order_id":"o2"},"expected":{"verdict":"FAIL"}}
	  ]
	}`)

	cases, datasets, err := LoadGoldenDir(dir)
	if err != nil {
		t.Fatalf("LoadGoldenDir: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("got %d cases, want 2", len(cases))
	}
	if len(datasets) != 1 || datasets[0].Domain != "settlement" {
		t.Fatalf("unexpected datasets: %+v", datasets)
	}
}

func TestLoadGoldenDir_RejectsPrebakedOutput(t *testing.T) {
	dir := t.TempDir()
	// This is the System-B shape that must be refused: a case carrying a
	// pre-baked actual_output means it was scored ahead of time.
	writeFile(t, dir, "bad.json", `{
	  "domain":"settlement","category":"correct","total_cases":1,
	  "cases":[{"id":"SE-X","domain":"settlement","input":{},"expected":{},"actual_output":{"settlement_result":"success"}}]
	}`)

	if _, _, err := LoadGoldenDir(dir); err == nil {
		t.Fatal("expected rejection of pre-baked actual_output")
	}
}

func TestLoadGoldenDir_RejectsCountMismatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "mismatch.json", `{
	  "domain":"settlement","category":"correct","total_cases":5,
	  "cases":[{"id":"SE-1","domain":"settlement","input":{},"expected":{"verdict":"PASS"}}]
	}`)
	if _, _, err := LoadGoldenDir(dir); err == nil {
		t.Fatal("expected total_cases mismatch error")
	}
}

func TestLoadGoldenDir_RejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.json", `{"domain":"settlement","category":"c","total_cases":1,"cases":[{"id":"DUP","domain":"settlement","input":{},"expected":{"verdict":"PASS"}}]}`)
	writeFile(t, dir, "b.json", `{"domain":"settlement","category":"d","total_cases":1,"cases":[{"id":"DUP","domain":"settlement","input":{},"expected":{"verdict":"PASS"}}]}`)
	if _, _, err := LoadGoldenDir(dir); err == nil {
		t.Fatal("expected duplicate case id error")
	}
}

func TestLoadGoldenDir_SkipsIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "INDEX.json", `{"anything":"is allowed here, even actual_output mentions"}`)
	writeFile(t, dir, "ok.json", `{"domain":"settlement","category":"c","total_cases":1,"cases":[{"id":"SE-1","domain":"settlement","input":{},"expected":{"verdict":"PASS"}}]}`)
	cases, _, err := LoadGoldenDir(dir)
	if err != nil {
		t.Fatalf("INDEX.json should be skipped, got: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("got %d cases, want 1", len(cases))
	}
}
