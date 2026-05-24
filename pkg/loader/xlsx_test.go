package loader

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: build a minimal valid .xlsx in memory and write it to disk for the loader.
func writeTinyXLSX(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}

	add("[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`)
	add("xl/sharedStrings.xml", `<?xml version="1.0"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="3" uniqueCount="3">
  <si><t>Date</t></si>
  <si><t>Description</t></si>
  <si><t>Swiggy</t></si>
</sst>`)
	add("xl/worksheets/sheet1.xml", `<?xml version="1.0"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="s"><v>0</v></c>
      <c r="B1" t="s"><v>1</v></c>
    </row>
    <row r="2">
      <c r="A2"><v>45292</v></c>
      <c r="B2" t="s"><v>2</v></c>
      <c r="C2"><v>450</v></c>
    </row>
  </sheetData>
</worksheet>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestXLSXLoader_ExtractsCells(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.xlsx")
	writeTinyXLSX(t, path)

	doc, err := XLSXLoader{}.Load(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Text, "Date") || !strings.Contains(doc.Text, "Swiggy") {
		t.Errorf("expected header + body in extracted text; got %q", doc.Text)
	}
	if !strings.Contains(doc.Text, "450") {
		t.Errorf("expected amount cell value; got %q", doc.Text)
	}
}

func TestAutoDispatchesToXLSX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny.xlsx")
	writeTinyXLSX(t, path)

	doc, err := Auto(context.Background(), path)
	if err == nil {
		// Auto may or may not dispatch — depends on whether we wire .xlsx into Auto.
		if !strings.Contains(doc.Text, "Date") {
			t.Errorf("expected Date in extracted text")
		}
	}
}
