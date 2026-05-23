package handlers

import (
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

// Disclosures implements GET /v1/disclosures per RBI FREE-AI Rec 25.
//
// Returns the AI governance summary intended for public annual-report
// disclosure: policy version, sutras, principles, and aggregate AI usage
// counts by risk class.
type Disclosures struct {
	Reg                  registry.Registry
	PolicyVersion        string
	PolicyApprovedOn     string
	Principles           []string
	HomeRegion           string
	IncidentReportingURL string
}

// Get handles GET /v1/disclosures. Public — no auth.
func (h *Disclosures) Get(w http.ResponseWriter, r *http.Request) {
	counts := map[agent.RiskClass]int{
		agent.RiskLow:    0,
		agent.RiskMedium: 0,
		agent.RiskHigh:   0,
	}
	all := h.Reg.List(r.Context())
	for _, a := range all {
		counts[agent.RiskOf(a)]++
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"policy_version":         h.PolicyVersion,
		"policy_approved_on":     h.PolicyApprovedOn,
		"principles":             h.Principles,
		"home_region":            h.HomeRegion,
		"incident_reporting_url": h.IncidentReportingURL,
		"agent_counts": map[string]any{
			"total":  len(all),
			"low":    counts[agent.RiskLow],
			"medium": counts[agent.RiskMedium],
			"high":   counts[agent.RiskHigh],
		},
	})
}
