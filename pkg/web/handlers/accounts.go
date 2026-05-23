package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Accounts CRUD (limited subset: create + list).
type Accounts struct {
	Repo postgres.AccountRepo
}

type createAccountRequest struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

// Create makes a new account for the authenticated user.
func (h *Accounts) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	a, err := h.Repo.Create(r.Context(), claims.Subject, req.Name, req.Currency)
	if err != nil {
		http.Error(w, "create failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, a)
}

// List returns all accounts for the authenticated user.
func (h *Accounts) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	out, err := h.Repo.ListByUser(r.Context(), claims.Subject)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, out)
}
