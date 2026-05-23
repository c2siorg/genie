// Package loader extracts plain text from PDF, HTML, and DOCX so Genie can
// feed real documents into pkg/rag.
//
// We keep zero hard dependencies: PDF text extraction shells out to
// `pdftotext` (poppler) when present; HTML uses a tiny regex stripper; DOCX
// reads the .docx zip and pulls text from word/document.xml.
//
// For more robust extraction add github.com/ledongthuc/pdf or unidoc later
// behind the same Loader interface.
package loader

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Document is the loaded plaintext + provenance metadata.
type Document struct {
	Source string
	Title  string
	Text   string
}

// Loader returns a Document from a file path.
type Loader interface {
	Load(ctx context.Context, path string) (Document, error)
}

// Auto picks a loader based on file extension.
func Auto(ctx context.Context, path string) (Document, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return PDFLoader{}.Load(ctx, path)
	case ".html", ".htm":
		return HTMLLoader{}.Load(ctx, path)
	case ".docx":
		return DOCXLoader{}.Load(ctx, path)
	case ".txt", ".md":
		return TextLoader{}.Load(ctx, path)
	}
	return Document{}, fmt.Errorf("loader: unsupported extension %q", filepath.Ext(path))
}

// PDFLoader shells out to `pdftotext`. Returns an explanatory error if the
// binary isn't installed.
type PDFLoader struct{}

func (PDFLoader) Load(ctx context.Context, path string) (Document, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return Document{}, fmt.Errorf("loader: pdftotext not on PATH (install poppler-utils): %w", err)
	}
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", path, "-")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Document{}, fmt.Errorf("loader: pdftotext %s: %w", path, err)
	}
	return Document{Source: path, Title: filepath.Base(path), Text: strings.TrimSpace(out.String())}, nil
}

// HTMLLoader strips tags with a regex. Crude but stdlib-only.
type HTMLLoader struct{}

var (
	// Go's RE2 lacks backreferences — script and style get separate patterns.
	htmlScript    = regexp.MustCompile(`(?is)<script.*?</script>`)
	htmlStyle     = regexp.MustCompile(`(?is)<style.*?</style>`)
	htmlTags      = regexp.MustCompile(`<[^>]+>`)
	htmlEntities  = regexp.MustCompile(`&[a-zA-Z]+;`)
	whitespaceRun = regexp.MustCompile(`\s+`)
)

func (HTMLLoader) Load(_ context.Context, path string) (Document, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	s := htmlScript.ReplaceAllString(string(body), " ")
	s = htmlStyle.ReplaceAllString(s, " ")
	s = htmlTags.ReplaceAllString(s, " ")
	s = htmlEntities.ReplaceAllString(s, " ")
	s = whitespaceRun.ReplaceAllString(s, " ")
	return Document{Source: path, Title: filepath.Base(path), Text: strings.TrimSpace(s)}, nil
}

// TextLoader returns the file verbatim.
type TextLoader struct{}

func (TextLoader) Load(_ context.Context, path string) (Document, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	return Document{Source: path, Title: filepath.Base(path), Text: string(body)}, nil
}

// DOCXLoader extracts the body text from a DOCX zip.
type DOCXLoader struct{}

type docxBody struct {
	XMLName xml.Name `xml:"document"`
	Body    struct {
		Paragraphs []struct {
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
	} `xml:"body"`
}

func (DOCXLoader) Load(_ context.Context, path string) (Document, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Document{}, fmt.Errorf("loader: open docx: %w", err)
	}
	defer zr.Close()
	var docXML []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return Document{}, err
			}
			docXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return Document{}, err
			}
			break
		}
	}
	if len(docXML) == 0 {
		return Document{}, fmt.Errorf("loader: docx missing word/document.xml")
	}
	var b docxBody
	if err := xml.Unmarshal(docXML, &b); err != nil {
		return Document{}, fmt.Errorf("loader: parse docx: %w", err)
	}
	var sb strings.Builder
	for _, p := range b.Body.Paragraphs {
		for _, r := range p.Runs {
			sb.WriteString(r.Text)
		}
		sb.WriteString("\n")
	}
	return Document{Source: path, Title: filepath.Base(path), Text: strings.TrimSpace(sb.String())}, nil
}
