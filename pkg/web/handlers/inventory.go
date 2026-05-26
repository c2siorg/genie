package handlers

import (
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

// Inventory implements GET /v1/ai-inventory per RBI FREE-AI Rec 23.
//
// Lists every registered agent with its risk class, capabilities, and whether
// a fallback is configured. Admin-gated at the router level.
type Inventory struct {
	Reg       registry.Registry
	Fallbacks map[string]string // primary -> fallback id (or nil)
}

// InventoryItem is one row of the response.
//
// Tier was added in the Q1 hardening pass — FREE-AI Rec 17 (Product
// Approval) wants the inventory to reflect where each agent sits on the
// Sketch → Prototype → Beta → Production pipeline. The risk team reads
// this column to spot non-production agents that are still receiving
// customer-facing traffic.
type InventoryItem struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Capabilities []string        `json:"capabilities"`
	RiskClass    agent.RiskClass `json:"risk_class"`
	Tier         agent.Tier      `json:"tier"`
	HasFallback  bool            `json:"has_fallback"`
	FallbackID   string          `json:"fallback_id,omitempty"`
}

// List handles GET /v1/ai-inventory.
func (h *Inventory) List(w http.ResponseWriter, r *http.Request) {
	all := h.Reg.List(r.Context())
	out := make([]InventoryItem, 0, len(all))
	for _, a := range all {
		fb, hasFB := h.Fallbacks[a.ID()]
		out = append(out, InventoryItem{
			ID:           a.ID(),
			Name:         a.Name(),
			Capabilities: a.Capabilities(),
			RiskClass:    agent.RiskOf(a),
			Tier:         agent.TierOf(a),
			HasFallback:  hasFB,
			FallbackID:   fb,
		})
	}
	respondJSON(w, http.StatusOK, out)
}
