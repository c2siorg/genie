package kyc

import (
	"context"

	"github.com/c2siorg/genie/pkg/compliance"
	"github.com/c2siorg/genie/pkg/kyc"
)

// DocumentProcessorAgent handles file ingestion, storage, OCR extraction, and format validation.
type DocumentProcessorAgent struct {
	ocr      OCRService
	store    DocumentStore
	auditLog compliance.AuditLog
}

// ProcessDocument receives raw file, validates format, stores securely,
// runs OCR, and returns extracted text with confidence.
func (a *DocumentProcessorAgent) ProcessDocument(
	ctx context.Context,
	docType string,
	rawFileBytes []byte,
) (*kyc.Document, error) {
	// TODO: implement
	panic("not implemented")
}

// OCRService defines the interface for document OCR extraction.
type OCRService interface {
	// ExtractText performs OCR on document bytes and returns extracted text with confidence.
	ExtractText(ctx context.Context, docType string, rawBytes []byte) (string, float64, error)
}

// DocumentStore defines the interface for secure document storage.
type DocumentStore interface {
	// Store persists raw document bytes and returns a reference ID.
	Store(ctx context.Context, docType string, rawBytes []byte, metadata map[string]any) (string, error)
	// Retrieve retrieves stored document bytes by reference ID.
	Retrieve(ctx context.Context, refID string) ([]byte, error)
}
