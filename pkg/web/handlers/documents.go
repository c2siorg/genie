package handlers

import (
	"io"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/kyc"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Documents handles encrypted document uploads (transaction CSVs, Aadhaar
// offline-KYC XML, PAN, ...). The plaintext only lives in memory long enough to
// be encrypted; the DB only ever sees ciphertext plus non-sensitive masked
// metadata (e.g. an Aadhaar last-4 — never the full number).
type Documents struct {
	Repo      postgres.DocumentRepo
	Encryptor *crypto.Encryptor
}

type uploadResponse struct {
	ID             string                  `json:"id"`
	Type           string                  `json:"type"`
	Classification protocol.Classification `json:"classification"`
	Description    string                  `json:"description"`
	KEKID          string                  `json:"kek_id"`
	// Masked holds only non-sensitive derived values (e.g. {"aadhaar_last4":
	// "2346"}) — never a full Aadhaar number or any decrypted content.
	Masked map[string]string `json:"masked,omitempty"`
}

// Upload accepts an octet-stream / text body of CSV, encrypts it, and
// persists the envelope. The decryption side is exercised by the Ask handler.
func (h *Documents) Upload(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	const maxBytes = 5 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > maxBytes {
		http.Error(w, "payload too large (5MB max)", http.StatusRequestEntityTooLarge)
		return
	}

	desc := r.URL.Query().Get("description")

	// Document type drives validation + the classification floor. Default to a
	// transaction CSV (the /v1/ask flow) for backward compatibility.
	docType := kyc.DocType(r.URL.Query().Get("type"))
	if docType == "" {
		docType = kyc.DocCSV
	}
	if !kyc.ValidDocType(docType) {
		http.Error(w, "unknown document type: "+string(docType), http.StatusBadRequest)
		return
	}

	// Type-specific validation + masked-metadata extraction. Aadhaar comes as a
	// UIDAI offline-KYC XML: validate its shape and pull ONLY the last-4; the
	// full number is never present, stored, logged, or returned.
	masked := map[string]string{}
	if docType == kyc.DocAadhaarOfflineKYC {
		last4, perr := kyc.ParseOfflineKYCLast4(body)
		if perr != nil {
			http.Error(w, "invalid Aadhaar offline e-KYC: "+perr.Error(), http.StatusBadRequest)
			return
		}
		masked["aadhaar_last4"] = last4
	}

	// A sensitive type can be raised but never downgraded below its floor
	// (Aadhaar/passport -> secret; PAN/statement/CSV -> pii).
	classification := kyc.AtLeast(
		protocol.Classification(r.URL.Query().Get("classification")),
		kyc.DefaultClassification(docType),
	)

	ep, err := h.Encryptor.Encrypt(body)
	if err != nil {
		http.Error(w, "encrypt failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	d, err := h.Repo.Create(r.Context(), postgres.Document{
		UserID:         claims.Subject,
		DocType:        string(docType),
		Classification: classification,
		Description:    desc,
		Payload:        ep,
		MaskedMeta:     masked,
	})
	if err != nil {
		http.Error(w, "persist failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, uploadResponse{
		ID: d.ID, Type: d.DocType, Classification: d.Classification,
		Description: d.Description, KEKID: ep.KEKID, Masked: d.MaskedMeta,
	})
}

// Get returns the document metadata (never the decrypted body — the Ask flow
// decrypts only for the agent pipeline). Only the owner may read it.
func (h *Documents) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	d, err := h.Repo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Ownership check: a document is only readable by the user who uploaded it.
	// Return 404 (not 403) so we don't confirm the existence of others' docs.
	if d.UserID != claims.Subject {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, struct {
		ID             string                  `json:"id"`
		Type           string                  `json:"type"`
		Classification protocol.Classification `json:"classification"`
		Description    string                  `json:"description"`
		KEKID          string                  `json:"kek_id"`
		Masked         map[string]string       `json:"masked,omitempty"`
	}{ID: d.ID, Type: d.DocType, Classification: d.Classification, Description: d.Description, KEKID: d.Payload.KEKID, Masked: d.MaskedMeta})
}
