// xlsx.go — minimal .xlsx loader using only the stdlib.
//
// .xlsx is a zipped OOXML bundle. The plaintext content we care about
// lives in two parts:
//   - xl/sharedStrings.xml  — a string table
//   - xl/worksheets/sheet*.xml — cells reference into the string table by index
//
// This loader extracts each cell's text in row-major order and joins them
// with tabs/newlines, producing something close to what a CSV export would
// give us. It does not attempt formula evaluation, styles, or merged cells —
// for those, plug in a proper XLSX library (excelize/xuri) behind the same
// Loader interface.
//
// Stdlib-only on purpose: the loader package's contract is "no third-party
// runtime deps." Statement Excel exports and KYC questionnaires this
// minimal extractor handles.
package loader

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// XLSXLoader reads cells from each sheet in an xlsx workbook.
type XLSXLoader struct{}

// Load extracts text from the workbook at path.
func (XLSXLoader) Load(_ context.Context, path string) (Document, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Document{}, fmt.Errorf("loader: open xlsx %s: %w", path, err)
	}
	defer zr.Close()

	shared := []string{}
	sheets := map[string]string{} // filename in zip -> raw xml

	for _, f := range zr.File {
		name := f.Name
		switch {
		case name == "xl/sharedStrings.xml":
			shared, err = readSharedStrings(f)
			if err != nil {
				return Document{}, err
			}
		case strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml"):
			raw, err := readAll(f)
			if err != nil {
				return Document{}, err
			}
			sheets[name] = raw
		}
	}

	// Sort sheet keys so output is deterministic.
	keys := make([]string, 0, len(sheets))
	for k := range sheets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out strings.Builder
	for _, k := range keys {
		text, err := extractSheetText(sheets[k], shared)
		if err != nil {
			return Document{}, err
		}
		if text != "" {
			if out.Len() > 0 {
				out.WriteString("\n\n=== sheet boundary ===\n\n")
			}
			out.WriteString(text)
		}
	}
	return Document{Source: path, Title: filepath.Base(path), Text: out.String()}, nil
}

func readAll(f *zip.File) (string, error) {
	r, err := f.Open()
	if err != nil {
		return "", err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// readSharedStrings parses the <sst> entries.
type sst struct {
	Items []sstItem `xml:"si"`
}
type sstItem struct {
	T string `xml:"t"`
	R []struct {
		T string `xml:"t"`
	} `xml:"r"`
}

func readSharedStrings(f *zip.File) ([]string, error) {
	raw, err := readAll(f)
	if err != nil {
		return nil, err
	}
	var s sst
	if err := xml.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("loader: parse sharedStrings: %w", err)
	}
	out := make([]string, 0, len(s.Items))
	for _, it := range s.Items {
		if it.T != "" {
			out = append(out, it.T)
			continue
		}
		// rich text run — concatenate the parts.
		var b strings.Builder
		for _, r := range it.R {
			b.WriteString(r.T)
		}
		out = append(out, b.String())
	}
	return out, nil
}

// sheet cell structure for unmarshalling.
type sheet struct {
	Rows []sheetRow `xml:"sheetData>row"`
}
type sheetRow struct {
	Cells []sheetCell `xml:"c"`
}
type sheetCell struct {
	Type  string `xml:"t,attr"`
	Value string `xml:"v"`
	IS    *struct {
		T string `xml:"t"`
	} `xml:"is"`
}

func extractSheetText(raw string, shared []string) (string, error) {
	var s sheet
	if err := xml.Unmarshal([]byte(raw), &s); err != nil {
		return "", fmt.Errorf("loader: parse sheet: %w", err)
	}
	var out strings.Builder
	for _, row := range s.Rows {
		first := true
		for _, c := range row.Cells {
			if !first {
				out.WriteByte('\t')
			}
			first = false
			out.WriteString(cellText(c, shared))
		}
		out.WriteByte('\n')
	}
	return strings.TrimSpace(out.String()), nil
}

func cellText(c sheetCell, shared []string) string {
	switch c.Type {
	case "s":
		i, err := strconv.Atoi(c.Value)
		if err != nil || i < 0 || i >= len(shared) {
			return ""
		}
		return shared[i]
	case "inlineStr":
		if c.IS != nil {
			return c.IS.T
		}
		return ""
	case "b":
		if c.Value == "1" {
			return "true"
		}
		return "false"
	default:
		return c.Value
	}
}
