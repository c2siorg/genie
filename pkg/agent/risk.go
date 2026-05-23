package agent

// RiskClass labels an agent's expected impact when it fails or is misused.
// Aligned with the RBI FREE-AI report (Recommendation 14): board-approved
// policies must classify use cases as Low / Medium / High, with progressively
// stricter governance applied.
type RiskClass string

const (
	// RiskLow — internal automation, document summarisation, glossary lookup.
	// Failure is contained; no customer-facing harm.
	RiskLow RiskClass = "low"
	// RiskMedium — customer-facing assistance, fraud signals, basic chatbots.
	// Errors may inconvenience customers; human override expected.
	RiskMedium RiskClass = "medium"
	// RiskHigh — credit decisioning, autonomous fund movement, autonomous KYC.
	// Errors have material customer or systemic consequences; human-in-the-loop
	// mandatory.
	RiskHigh RiskClass = "high"
)

// RiskAware is an optional capability an Agent implementation can advertise.
// The registry inspects this via type assertion; agents that don't implement
// it default to RiskLow. Keeping the surface optional avoids breaking
// existing agents.
type RiskAware interface {
	RiskLevel() RiskClass
}

// RiskOf returns the agent's declared risk class, or RiskLow if unspecified.
func RiskOf(a Agent) RiskClass {
	if r, ok := a.(RiskAware); ok {
		return r.RiskLevel()
	}
	return RiskLow
}
