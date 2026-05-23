package handlers

import (
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/aibom"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

// AIBOM serves the AI Bill of Materials.
type AIBOM struct {
	Reg     registry.Registry
	Builder *aibom.Builder
}

// Get handles GET /v1/aibom (admin gated at the router level).
func (h *AIBOM) Get(w http.ResponseWriter, r *http.Request) {
	doc := h.Builder.Render(h.Reg.List(r.Context()))
	respondJSON(w, http.StatusOK, doc)
}
