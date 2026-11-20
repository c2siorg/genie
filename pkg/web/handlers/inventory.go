// inventory.go — GET /v1/ai-inventory handler.
//
// ─── What this endpoint serves ─────────────────────────────────────────────
//
// The live AI inventory as required by FREE-AI Rec 23 ("AI Inventory").
// Reads every registered agent and returns id, name, capabilities,
// risk_class, tier, and fallback wiring.
//
// Admin-only — the route is gated at the router level with
// RequireRole(auth.RoleAdmin). Two reasons it's admin-only:
//
//   - The capability list and the fallback wiring map together describe
//     attack surface (which agent reaches which downstream service,
//     which agent has no fallback). Defence in depth says we don't
//     hand that map to anonymous users.
//   - The risk class + tier columns are the inputs to the risk team's
//     promotion-gate decision. Customer-facing routes have no business
//     reading them.
//
// ─── The tier field (Q1 hardening) ─────────────────────────────────────────
//
// The Tier column was added in the Q1 hardening pass — FREE-AI Rec 17
// (Product Approval) wants the inventory to reflect where each agent
// sits on the Sketch → Prototype → Beta → Production pipeline. The risk
// team reads this column to spot non-production agents that are still
// receiving customer-facing traffic.
//
// The UI contract test (pkg/web/handlers/ui_security_test.go::
// TestInventory_ListIncludesTier and TestInventory_TierFieldStableJSONName)
// pins the field name "tier" against accidental drift — a rename to
// "promotion_stage" or "level" would silently break the dashboard column
// without those tests.
package handlers

import (
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

// Inventory implements GET /v1/ai-inventory per RBI FREE-AI Rec 23.
//
// Lists every registered agent with its risk class, capabilities, and
// whether a fallback is configured. Admin-gated at the router level.
//
// Construction: the host wires this in cmd/api/main.go, passing the
// shared registry and the orchestrator's fallback map. The handler is
// stateless beyond those two pointers — safe to share across goroutines.
type Inventory struct {
	// Reg is the agent registry. We read List(ctx) on every request
	// rather than caching, because:
	//   - The list is short (typically <50 agents).
	//   - Registry mutations are extremely rare (only on startup).
	//   - A cache would risk staleness if someone hot-reloaded an
	//     agent registration in the future.
	Reg registry.Registry

	// Fallbacks maps primary agent id → fallback agent id (or empty if
	// no fallback). Sourced from the orchestrator's SetFallback wiring.
	// Used to populate HasFallback + FallbackID on each item.
	Fallbacks map[string]string // primary -> fallback id (or nil)
}

// InventoryItem is one row of the response.
//
// Tier was added in the Q1 hardening pass — FREE-AI Rec 17 (Product
// Approval) wants the inventory to reflect where each agent sits on the
// Sketch → Prototype → Beta → Production pipeline. The risk team reads
// this column to spot non-production agents that are still receiving
// customer-facing traffic.
//
// Field order in this struct doesn't affect JSON output (encoding/json
// emits in declaration order, but consumers shouldn't depend on it).
// What matters is the JSON tag names — see TestInventory_TierFieldStableJSONName
// for the contract test that pins "tier".
type InventoryItem struct {
	// ID is the agent's stable identifier. Same string the bus uses
	// for routing, same string the audit log records.
	ID string `json:"id"`

	// Name is the human-readable label, used in the UI table header
	// and in log messages.
	Name string `json:"name"`

	// Capabilities lists the agent's advertised skills (the strings
	// returned from Agent.Capabilities()). Used by the supervisor for
	// routing and by the risk team to spot capability overlap between
	// agents.
	Capabilities []string `json:"capabilities"`

	// RiskClass is one of "low", "medium", "high" — the agent's
	// declared blast-radius level (pkg/agent.RiskClass). RiskHigh
	// agents require HITL on every output.
	RiskClass agent.RiskClass `json:"risk_class"`

	// Tier is one of "sketch", "prototype", "beta", "production" —
	// the agent's promotion stage (pkg/agent.Tier). Undeclared
	// defaults to "prototype" via agent.TierOf. Customer-facing
	// dispatch refuses any agent below "production".
	Tier agent.Tier `json:"tier"`

	// HasFallback is true if the orchestrator has a SetFallback wiring
	// for this agent's id. A "true" with no FallbackID would be a bug;
	// the two are populated together.
	HasFallback bool `json:"has_fallback"`

	// FallbackID is the id of the fallback agent (empty if HasFallback
	// is false). omitempty keeps the JSON output clean for the no-
	// fallback case.
	FallbackID string `json:"fallback_id,omitempty"`
}

// List handles GET /v1/ai-inventory.
//
// Reads the registry, projects each agent into an InventoryItem, and
// writes the response as JSON. Status 200 on success. There's no
// pagination — the list is short by construction.
//
// Error handling: this method does not return errors. The registry's
// List doesn't fail, and the JSON marshalling can only fail on
// programming bugs (unexported channel field on InventoryItem etc.) —
// none of which are possible in this struct. If respondJSON itself
// fails it logs and returns 500.
func (h *Inventory) List(w http.ResponseWriter, r *http.Request) {
	// Snapshot the agent list. Registry.List returns the current
	// agents — typically called once per request because the list is
	// short and registry mutations are rare.
	all := h.Reg.List(r.Context())
	// Pre-size the output slice to avoid append-grow churn.
	out := make([]InventoryItem, 0, len(all))
	for _, a := range all {
		// Look up the fallback id for this agent. Map lookup is O(1);
		// the two-value form distinguishes "no fallback configured"
		// (ok=false) from "fallback id is empty" (which shouldn't
		// happen in practice but defence-in-depth says check).
		fb, hasFB := h.Fallbacks[a.ID()]
		out = append(out, InventoryItem{
			ID:           a.ID(),
			Name:         a.Name(),
			Capabilities: a.Capabilities(),
			// RiskOf returns the declared risk class or RiskLow as a
			// safe default if the agent doesn't implement RiskAware.
			RiskClass: agent.RiskOf(a),
			// TierOf returns the declared tier or TierPrototype as a
			// safe default — the "default-to-Prototype" rule that
			// makes undeclared agents non-production.
			Tier:        agent.TierOf(a),
			HasFallback: hasFB,
			FallbackID:  fb,
		})
	}
	// respondJSON writes Content-Type, the status code, and the JSON
	// body. Implemented in pkg/web/handlers/util.go (or similar).
	respondJSON(w, http.StatusOK, out)
}
