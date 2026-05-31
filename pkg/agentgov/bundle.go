// Package agentgov bridges the Microsoft Agent Governance Toolkit with
// Genie's orchestration layer. It owns all AGT components and wires them
// into orchestrator hooks, HTTP handlers, and bus-level enforcement.
package agentgov

import "github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"

// Bundle groups all AGT governance components used at runtime.
type Bundle struct {
	Policy       *agentmesh.PolicyEngine
	Audit        *agentmesh.AuditLogger
	Trust        *agentmesh.TrustManager
	KillSwitches *agentmesh.KillSwitchRegistry
	SLO          *agentmesh.SLOEngine
	Rings        *agentmesh.RingEnforcer
	Identities   map[string]*agentmesh.AgentIdentity
}
