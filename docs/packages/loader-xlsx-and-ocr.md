# pkg/loader — XLSX and Scanned-PDF loaders

> **Where:** `pkg/loader/xlsx.go`, `pkg/loader/scanned_pdf.go`
> **Lines of code:** ~290 combined · **Tests:** 2 + integration via existing harness
> **Inspired by:** Google ADK `multiformat-hybrid-rag`

---

## Overview

Extends `pkg/loader` with two formats that AA-driven onboarding,
KYC packets, and SME current accounts hit constantly:

1. **`XLSXLoader`** — extracts cell text from `.xlsx` workbooks using
   only the Go standard library.
2. **`ScannedPDFLoader`** — falls back to OCR for PDFs whose text
   extraction returns nothing (i.e. scanned documents).

Both implement the existing `loader.Loader` interface, so callers using
`loader.Auto(ctx, path)` get them for free. The existing
`PDFLoader`/`HTMLLoader`/`DOCXLoader`/`TextLoader` unchanged.

---

## XLSXLoader

### Why stdlib-only

`.xlsx` is a zipped OOXML bundle. The plaintext we care about lives in:

- `xl/sharedStrings.xml` — the string table
- `xl/worksheets/sheet*.xml` — cells, often referencing the string table by index

`archive/zip` and `encoding/xml` are all we need. We don't pull in
`excelize/xuri` or `tealeg/xlsx` — the dependency graph stays small.

### What it does

For each sheet (sorted alphabetically by filename in the zip):

- Parse `<row><c>...</c></row>` structure.
- Resolve shared-string indices for cells of type `s`.
- Handle inline strings (`t="inlineStr"`).
- Booleans, numbers, formulas as cell-value strings.

Output: rows tab-separated, lines newline-separated, sheets joined by
`\n\n=== sheet boundary ===\n\n`.

### What it does NOT do

- **Formulas**: returned as the cached value if present, otherwise empty.
- **Styles / formatting**: ignored.
- **Merged cells**: each cell stands alone.
- **Charts / images**: skipped.
- **Pivot tables**: extracts the source table only.

For deep XLSX surgery (chart edits, formula re-eval, complex pivots),
plug `excelize` behind the same `Loader` interface — the contract
doesn't change.

### Auto dispatch

`pkg/loader/loader.go` `Auto()` now includes `.xlsx`:

```go
case ".xlsx":
    return XLSXLoader{}.Load(ctx, path)
```

Callers using `loader.Auto(ctx, "statement.xlsx")` get XLSX
extraction transparently.

---

## ScannedPDFLoader

### The problem

`PDFLoader` shells out to `pdftotext` (from poppler-utils). For text-PDFs
this works great; for **scanned PDFs** (image-only) the output is empty.
KYC packets and older bank statements are usually scanned.

### What ScannedPDFLoader does

1. Verify `pdftoppm` and `tesseract` are on `PATH`. Return a clear error if not.
2. Render each PDF page to PNG via `pdftoppm -r <DPI> -png <path>`.
3. OCR each PNG via `tesseract <png> stdout -l eng`.
4. Concatenate the OCR text per page, joined by blank lines.
5. Cleanup the temp directory.

### Default DPI

300 — the Tesseract sweet spot. Override via the struct field:

```go
ScannedPDFLoader{DPI: 600}.Load(ctx, "kyc.pdf")
```

### AutoOCR fallback

`AutoOCR(ctx, path)` is a convenience that:

- Calls `Auto(ctx, path)` first.
- If the result is a PDF with empty text, falls back to
  `ScannedPDFLoader{}.Load(ctx, path)`.
- Otherwise returns the original.

Useful for AA-fetched statements where the bank may return either a
text-PDF or a scanned-PDF depending on the channel.

### Production swap-out

Google Document AI / AWS Textract / Azure Document Intelligence usually
produce higher-fidelity text than local Tesseract. Wire them behind the
same `Loader` interface; the rest of the system doesn't change.

---

## Example

```go
// Excel statement
doc, err := loader.Auto(ctx, "downloads/jan-statement.xlsx")
if err != nil { ... }
fmt.Println(doc.Text) // tab-separated, newline-separated rows

// Scanned PDF — automatic fallback
doc, err := loader.AutoOCR(ctx, "kyc/aadhaar-back.pdf")
```

---

## What it does NOT do

- **Encrypted XLSX or PDFs** — both loaders require unencrypted input.
- **Image PDFs without Tesseract installed** — `ScannedPDFLoader` returns a clear "install tesseract-ocr" error rather than silently producing empty text.
- **Mixed text+image PDFs**: text path returns text, image path is silently dropped. Run `ScannedPDFLoader` explicitly if you suspect mixed content.

---

## FREE-AI alignment

- **Rec 15 (Data Lifecycle Governance)** — XLSX/PDF content is plaintext only after envelope-decrypt; loaders never see the raw at-rest bytes.
- **Rec 18 (Disclosure)** — when OCR produces low-confidence output, the agent consuming the text should disclose "this was OCR-extracted; verify."

---

## Anti-patterns

1. **Trusting XLSX numbers as exact financial values.** Cell types like `n` (number) are floats — apply your own rounding.
2. **Calling Tesseract synchronously inside the bus hot path.** OCR is slow (seconds per page). Run it as a job, cache the output.
3. **Bumping DPI above 600 without measuring.** Tesseract accuracy plateaus around 300 dpi; higher just costs CPU.

---

## Testing

`pkg/loader/xlsx_test.go` builds a minimal valid `.xlsx` in memory and
asserts:

- Cell text is extracted (header + body)
- Numeric cell values come through
- `Auto()` dispatches to `XLSXLoader` for `.xlsx`

The scanned-PDF loader is exercised manually because pdftoppm +
tesseract are heavyweight to require for CI. Add a tagged integration
test (`//go:build ocr`) if your CI host has them installed.

Run:

```bash
go test ./pkg/loader/ -v
```

---

## References

- [OOXML spec](https://learn.microsoft.com/en-us/openspecs/office_standards/ms-xlsx/) — the XLSX file format
- [poppler-utils](https://poppler.freedesktop.org/) — for `pdftotext` and `pdftoppm`
- [Tesseract OCR](https://github.com/tesseract-ocr/tesseract) — the OCR engine
- [Google Document AI](https://cloud.google.com/document-ai) — production-grade swap-out
