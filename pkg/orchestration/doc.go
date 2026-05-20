// Package orchestration implements the "control plane" that wires agents together.
//
// What orchestration means in a multi-agent architecture
//
// Orchestration is responsible for coordinating agents and enforcing system-level
// invariants. Typical responsibilities include:
// - Routing: deciding which agent(s) receive which messages
// - Governance: ensuring messages/actions comply with policies
// - Lifecycle: starting/stopping agents, handling dynamic membership
// - Observability: producing a coherent trace of cross-agent interactions
//
// In this repository
//
// This package provides a small Orchestrator that demonstrates the core idea:
//
//  1) Read the list of agents from the registry
//  2) Subscribe each agent to a communication bus using the agent's ID
//  3) For each incoming message:
//      - run governance policy checks
//      - invoke the target agent's HandleMessage
//      - publish any outbound messages back onto the bus
//
// This is the central "message pump" for the demo system.
//
// Why orchestration is separate from pkg/agent
//
// Keeping orchestration separate avoids dependency cycles and clarifies
// responsibilities:
// - pkg/agent defines what an agent is
// - pkg/orchestration defines how agents are coordinated
package orchestration

