package agentgov

import (
	"fmt"
	"time"

	"github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"
)

// ring assignment maps
var (
	ring0Agents = map[string]bool{
		"supervisor": true,
		"auditor":    true,
	}
	ring1Agents = map[string]bool{
		"analyzer":          true,
		"recommender":       true,
		"reporter":          true,
		"forecaster":        true,
		"educator":          true,
		"portfolio_advisor": true,
		"deep_research":     true,
	}
	ring2Agents = map[string]bool{
		"ingestor":                    true,
		"normalizer":                  true,
		"enricher":                    true,
		"currency":                    true,
		"macro":                       true,
		"rates":                       true,
		"loan":                        true,
		"tax_estimator":               true,
		"kyc_orchestrator":            true,
		"claim_adjudicator":           true,
		"sme_loan_workflow":           true,
		"invoice_processor":           true,
		"bulk_statement_analyzer":     true,
		"portfolio_advisor_fallback":  true,
		"recommender_fallback":        true,
		"anomaly":                     true,
		"cyber_guardian":              true,
		"payment_orchestrator":        true,
	}
	ring3Agents = map[string]bool{
		"aa_fetcher":          true,
		"voice":               true,
		"mpc_research":        true,
		"google_trends":       true,
		"auto_insurance":      true,
		"health_preauth":      true,
		"supply_chain_finance": true,
	}
)

// NewBundle constructs all AGT governance components and returns a ready Bundle.
func NewBundle(agentIDs []string) (*Bundle, error) {
	policy := agentmesh.NewPolicyEngine(defaultPolicyRules())

	audit := agentmesh.NewAuditLogger()

	trust := agentmesh.NewTrustManager(agentmesh.DefaultTrustConfig())

	ks := agentmesh.NewKillSwitchRegistry()

	slo, err := agentmesh.NewSLOEngine([]agentmesh.SLOObjective{
		{
			Name:      "agent.availability",
			Indicator: agentmesh.SLOAvailability,
			Target:    0.995,
			Window:    30 * 24 * time.Hour,
		},
		{
			Name:             "agent.latency",
			Indicator:        agentmesh.SLOLatency,
			Target:           0.95,
			Window:           30 * 24 * time.Hour,
			LatencyThreshold: 10 * time.Second,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("slo engine: %w", err)
	}

	rings := agentmesh.NewRingEnforcer()
	rings.SetRingPermissions(agentmesh.RingAdmin, []string{"*"})
	rings.SetRingPermissions(agentmesh.RingStandard, []string{
		"message.handle", "tool.call", "llm.complete", "document.read", "bus.publish",
	})
	rings.SetRingPermissions(agentmesh.RingRestricted, []string{
		"message.handle", "tool.call", "bus.publish",
	})
	rings.SetRingPermissions(agentmesh.RingSandboxed, []string{
		"message.handle",
	})

	identities := make(map[string]*agentmesh.AgentIdentity, len(agentIDs))
	for _, id := range agentIDs {
		identity, err := agentmesh.GenerateIdentity(id, capabilitiesFor(id))
		if err != nil {
			return nil, fmt.Errorf("generate identity for %s: %w", id, err)
		}
		identities[id] = identity

		switch {
		case ring0Agents[id]:
			rings.Assign(id, agentmesh.RingAdmin)
		case ring1Agents[id]:
			rings.Assign(id, agentmesh.RingStandard)
		case ring2Agents[id]:
			rings.Assign(id, agentmesh.RingRestricted)
		default:
			rings.Assign(id, agentmesh.RingSandboxed)
		}
	}

	return &Bundle{
		Policy:       policy,
		Audit:        audit,
		Trust:        trust,
		KillSwitches: ks,
		SLO:          slo,
		Rings:        rings,
		Identities:   identities,
	}, nil
}

// defaultPolicyRules returns sensible default rules: allow most operations,
// deny destructive ones, and rate-limit external-data calls.
func defaultPolicyRules() []agentmesh.PolicyRule {
	return []agentmesh.PolicyRule{
		{
			Action:   "data.delete",
			Effect:   agentmesh.Deny,
			Priority: 100,
			Scope:    agentmesh.Global,
		},
		{
			Action:   "system.shutdown",
			Effect:   agentmesh.Deny,
			Priority: 100,
			Scope:    agentmesh.Global,
		},
		{
			Action:    "external.fetch",
			Effect:    agentmesh.RateLimit,
			Priority:  50,
			Scope:     agentmesh.Global,
			MaxCalls:  100,
			Window:    "1m",
		},
		{
			Action:   "*",
			Effect:   agentmesh.Allow,
			Priority: 1,
			Scope:    agentmesh.Global,
		},
	}
}

// capabilitiesFor returns a basic capability list for the given agent.
func capabilitiesFor(agentID string) []string {
	switch {
	case ring0Agents[agentID]:
		return []string{"*"}
	case ring1Agents[agentID]:
		return []string{"message.handle", "tool.call", "llm.complete", "document.read", "bus.publish"}
	case ring2Agents[agentID]:
		return []string{"message.handle", "tool.call", "bus.publish"}
	default:
		return []string{"message.handle"}
	}
}
