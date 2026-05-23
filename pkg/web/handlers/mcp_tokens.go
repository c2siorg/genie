package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// MCPTokens lets users store the per-provider session tokens that Genie
// needs to talk to remote MCP servers (e.g. Zerodha Kite).
type MCPTokens struct {
	Repo      postgres.MCPTokenRepo
	Encryptor *crypto.Encryptor
}

type storeTokenRequest struct {
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
}

type storeTokenResponse struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	KEKID    string `json:"kek_id"`
}

// Store encrypts and persists the user's MCP session token. The plaintext
// only lives in process memory long enough to be sealed.
func (h *MCPTokens) Store(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	var req storeTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Provider == "" || req.Endpoint == "" || req.Token == "" {
		http.Error(w, "provider, endpoint, token required", http.StatusBadRequest)
		return
	}
	ep, err := h.Encryptor.Encrypt([]byte(req.Token))
	if err != nil {
		http.Error(w, "encrypt failed", http.StatusInternalServerError)
		return
	}
	t, err := h.Repo.Upsert(r.Context(), postgres.MCPToken{
		UserID:   claims.Subject,
		Provider: req.Provider,
		Endpoint: req.Endpoint,
		Payload:  ep,
	})
	if err != nil {
		http.Error(w, "persist failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, storeTokenResponse{
		ID: t.ID, Provider: t.Provider, Endpoint: t.Endpoint, KEKID: t.Payload.KEKID,
	})
}
