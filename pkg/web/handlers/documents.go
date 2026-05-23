package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Documents handles encrypted CSV uploads. The plaintext only lives in
// memory long enough to be encrypted; the DB only ever sees ciphertext.
type Documents struct {
	Repo      postgres.DocumentRepo
	Encryptor *crypto.Encryptor
}

type uploadResponse struct {
	ID             string                  `json:"id"`
	Classification protocol.Classification `json:"classification"`
	Description    string                  `json:"description"`
	KEKID          string                  `json:"kek_id"`
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
	classification := protocol.Classification(r.URL.Query().Get("classification"))
	if classification == "" {
		classification = protocol.ClassPII
	}

	ep, err := h.Encryptor.Encrypt(body)
	if err != nil {
		http.Error(w, "encrypt failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	d, err := h.Repo.Create(r.Context(), postgres.Document{
		UserID:         claims.Subject,
		Classification: classification,
		Description:    desc,
		Payload:        ep,
	})
	if err != nil {
		http.Error(w, "persist failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, uploadResponse{
		ID: d.ID, Classification: d.Classification, Description: d.Description, KEKID: ep.KEKID,
	})
}

// Get returns the document metadata (never the decrypted body — the Ask flow
// decrypts only for the agent pipeline).
func (h *Documents) Get(w http.ResponseWriter, r *http.Request) {
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
	respondJSON(w, http.StatusOK, struct {
		ID             string                  `json:"id"`
		Classification protocol.Classification `json:"classification"`
		Description    string                  `json:"description"`
		KEKID          string                  `json:"kek_id"`
	}{ID: d.ID, Classification: d.Classification, Description: d.Description, KEKID: d.Payload.KEKID})
}

// jsonDecoder is exported so handler tests can reuse it.
func jsonDecode(r *http.Request, out any) error {
	return json.NewDecoder(r.Body).Decode(out)
}
