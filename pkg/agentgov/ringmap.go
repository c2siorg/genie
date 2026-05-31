package agentgov

// RingMap returns a map from agentID → integer ring number (0–3) for all
// agents in agentIDs. This is used to populate the OPA engine's AgentRings
// data so the Rego policies can enforce ring-based capability rules.
//
//	0 = Admin   (supervisor, auditor)
//	1 = Standard (analyzers, advisors)
//	2 = Restricted (data processors)
//	3 = Sandboxed (external integrations)
func RingMap(agentIDs []string) map[string]int {
	m := make(map[string]int, len(agentIDs))
	for _, id := range agentIDs {
		switch {
		case ring0Agents[id]:
			m[id] = 0
		case ring1Agents[id]:
			m[id] = 1
		case ring2Agents[id]:
			m[id] = 2
		default:
			m[id] = 3
		}
	}
	return m
}
