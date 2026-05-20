// Package governance implements "guardrails" for multi-agent interactions.
//
// What "governance" means here
//
// In the reference architecture, governance is the set of mechanisms that
// constrain and monitor agent behavior so the system is:
// - Safe (prevents disallowed actions)
// - Predictable (enforces invariants)
// - Auditable (explains why something was allowed/denied)
// - Evolvable (policies can change without rewriting agents)
//
// Where governance sits in the flow
//
// In this repository, governance is evaluated by the orchestrator:
//
//   protocol.Message -> Orchestrator -> Policy.Evaluate -> (allow/deny) -> Agent.HandleMessage
//
// That placement is intentional:
// - It centralizes enforcement at a clear boundary.
// - It avoids duplicating policy checks inside each agent.
//
// Extending this package
//
// You can add policies such as:
// - Allowlist/denylist by sender/recipient/capability
// - Content classification and redaction
// - Tool-use authorization checks ("agent X may call tool Y")
// - Rate limiting / quotas / cost controls
// - Data residency and privacy constraints
package governance

