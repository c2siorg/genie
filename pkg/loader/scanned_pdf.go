// scanned_pdf.go — OCR-backed loader for image-rich (scanned) PDFs.
//
// Genie's existing PDFLoader shells out to `pdftotext`, which returns
// empty text for scanned documents. KYC packets, cancelled cheques, and
// older bank statements often come as scanned PDFs — we want a graceful
// degradation rather than silent emptiness.
//
// The ScannedPDFLoader detects an empty-text result, then shells out to
// Tesseract via the `pdftoppm` → `tesseract` pipe. Both binaries are
// optional; if either is missing the loader returns a clear error so the
// caller can surface "OCR not available" to the user instead of pretending
// the document was empty.
//
// Production deployments often replace this with Google Vision / AWS
// Textract / Azure Document Intelligence — the Loader interface stays
// the same.
package loader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ScannedPDFLoader OCRs a scanned PDF.
type ScannedPDFLoader struct {
	DPI int // rendering DPI; 300 is the Tesseract sweet spot
}

// Load runs pdftoppm → tesseract on the document.
func (s ScannedPDFLoader) Load(ctx context.Context, path string) (Document, error) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return Document{}, fmt.Errorf("loader: pdftoppm not on PATH (install poppler-utils): %w", err)
	}
	if _, err := exec.LookPath("tesseract"); err != nil {
		return Document{}, fmt.Errorf("loader: tesseract not on PATH (install tesseract-ocr): %w", err)
	}

	dpi := s.DPI
	if dpi <= 0 {
		dpi = 300
	}

	tmp, err := os.MkdirTemp("", "scanocr-")
	if err != nil {
		return Document{}, err
	}
	defer os.RemoveAll(tmp)

	prefix := filepath.Join(tmp, "page")
	cmd := exec.CommandContext(ctx, "pdftoppm", "-r", itoa(dpi), "-png", path, prefix)
	if err := cmd.Run(); err != nil {
		return Document{}, fmt.Errorf("loader: pdftoppm %s: %w", path, err)
	}

	pages, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return Document{}, err
	}
	if len(pages) == 0 {
		return Document{}, errors.New("loader: pdftoppm produced no pages")
	}

	var allText bytes.Buffer
	for _, page := range pages {
		ocr := exec.CommandContext(ctx, "tesseract", page, "stdout", "-l", "eng")
		var out bytes.Buffer
		ocr.Stdout = &out
		if err := ocr.Run(); err != nil {
			return Document{}, fmt.Errorf("loader: tesseract %s: %w", page, err)
		}
		allText.Write(bytes.TrimSpace(out.Bytes()))
		allText.WriteString("\n\n")
	}

	return Document{
		Source: path,
		Title:  filepath.Base(path),
		Text:   strings.TrimSpace(allText.String()),
	}, nil
}

// AutoOCR is like Auto but falls back to OCR for PDFs that come back empty.
// Useful for AA-fetched statements where the bank may return either a
// text-PDF or a scanned-PDF depending on the channel.
func AutoOCR(ctx context.Context, path string) (Document, error) {
	doc, err := Auto(ctx, path)
	if err != nil {
		return doc, err
	}
	if strings.ToLower(filepath.Ext(path)) == ".pdf" && strings.TrimSpace(doc.Text) == "" {
		return ScannedPDFLoader{}.Load(ctx, path)
	}
	return doc, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
