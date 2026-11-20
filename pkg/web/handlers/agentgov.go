package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agentgov"
	"github.com/go-chi/chi/v5"
	"github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"
)

// AgentGov exposes the AGT governance bundle as HTTP endpoints.
// All routes require RoleAdmin (enforced at the router level in router.go).
type AgentGov struct {
	Bundle *agentgov.Bundle
}

// GetTrust handles GET /v1/governance/trust/{agentID}.
// Returns the current TrustScore for the named agent.
func (h *AgentGov) GetTrust(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	score := h.Bundle.Trust.GetTrustScore(agentID)
	respondJSON(w, http.StatusOK, map[string]any{
		"agent_id": agentID,
		"score":    score,
	})
}

// agentSummary is returned by ListAgents.
type agentSummary struct {
	AgentID    string               `json:"agent_id"`
	TrustScore agentmesh.TrustScore `json:"trust_score"`
	Ring       int                  `json:"ring"`
	RingName   string               `json:"ring_name"`
}

// ListAgents handles GET /v1/governance/agents.
// Returns all registered agents with their trust score and ring assignment.
func (h *AgentGov) ListAgents(w http.ResponseWriter, r *http.Request) {
	var agents []agentSummary
	for id := range h.Bundle.Identities {
		score := h.Bundle.Trust.GetTrustScore(id)
		ring, _ := h.Bundle.Rings.GetRing(id)
		agents = append(agents, agentSummary{
			AgentID:    id,
			TrustScore: score,
			Ring:       int(ring),
			RingName:   ringName(ring),
		})
	}
	respondJSON(w, http.StatusOK, agents)
}

// GetAudit handles GET /v1/governance/audit.
// Accepts optional query params: agent_id, action.
func (h *AgentGov) GetAudit(w http.ResponseWriter, r *http.Request) {
	filter := agentmesh.AuditFilter{
		AgentID: r.URL.Query().Get("agent_id"),
		Action:  r.URL.Query().Get("action"),
	}
	entries := h.Bundle.Audit.GetEntries(filter)
	if entries == nil {
		entries = []*agentmesh.AuditEntry{}
	}
	respondJSON(w, http.StatusOK, entries)
}

// killSwitchBody is the request body for POST/DELETE /v1/governance/killswitch.
type killSwitchBody struct {
	Scope   string `json:"scope"` // "global", "agent:<id>", or "capability:<cap>"
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func (b *killSwitchBody) toScope() agentmesh.KillSwitchScope {
	switch {
	case b.Scope == "global":
		return agentmesh.GlobalKillSwitchScope()
	case strings.HasPrefix(b.Scope, "agent:"):
		return agentmesh.AgentKillSwitchScope(strings.TrimPrefix(b.Scope, "agent:"))
	case strings.HasPrefix(b.Scope, "capability:"):
		return agentmesh.CapabilityKillSwitchScope(strings.TrimPrefix(b.Scope, "capability:"))
	default:
		return agentmesh.GlobalKillSwitchScope()
	}
}

// ActivateKillSwitch handles POST /v1/governance/killswitch.
func (h *AgentGov) ActivateKillSwitch(w http.ResponseWriter, r *http.Request) {
	var body killSwitchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	event, err := h.Bundle.KillSwitches.Activate(
		body.toScope(),
		agentmesh.KillSwitchReason(body.Reason),
		body.Message,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, event)
}

// ClearKillSwitch handles DELETE /v1/governance/killswitch.
func (h *AgentGov) ClearKillSwitch(w http.ResponseWriter, r *http.Request) {
	var body killSwitchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	event, err := h.Bundle.KillSwitches.Clear(
		body.toScope(),
		agentmesh.KillSwitchReason(body.Reason),
		body.Message,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, event)
}

// ListKillSwitches handles GET /v1/governance/killswitch.
// Returns the full kill-switch event history.
func (h *AgentGov) ListKillSwitches(w http.ResponseWriter, r *http.Request) {
	history := h.Bundle.KillSwitches.History()
	if history == nil {
		history = []agentmesh.KillSwitchHistoryEntry{}
	}
	respondJSON(w, http.StatusOK, history)
}

// GetSLO handles GET /v1/governance/slo.
// Returns SLO reports for both objectives.
func (h *AgentGov) GetSLO(w http.ResponseWriter, r *http.Request) {
	avail, errA := h.Bundle.SLO.Evaluate("agent.availability")
	latency, errL := h.Bundle.SLO.Evaluate("agent.latency")

	type sloResult struct {
		Report agentmesh.SLOReport `json:"report"`
		Error  string              `json:"error,omitempty"`
	}
	out := map[string]sloResult{}
	if errA != nil {
		out["agent.availability"] = sloResult{Error: errA.Error()}
	} else {
		out["agent.availability"] = sloResult{Report: avail}
	}
	if errL != nil {
		out["agent.latency"] = sloResult{Error: errL.Error()}
	} else {
		out["agent.latency"] = sloResult{Report: latency}
	}
	respondJSON(w, http.StatusOK, out)
}

// GetRing handles GET /v1/governance/rings/{agentID}.
// Returns the ring assignment for a specific agent.
func (h *AgentGov) GetRing(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	ring, assigned := h.Bundle.Rings.GetRing(agentID)
	if !assigned {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"agent_id": agentID,
		"ring":     int(ring),
		"name":     ringName(ring),
	})
}

func ringName(r agentmesh.Ring) string {
	switch r {
	case agentmesh.RingAdmin:
		return "admin"
	case agentmesh.RingStandard:
		return "standard"
	case agentmesh.RingRestricted:
		return "restricted"
	default:
		return "sandboxed"
	}
}
