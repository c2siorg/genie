// Package agent defines the core "worker" abstraction: an Agent.
//
// In the Microsoft Multi-Agent Reference Architecture, an "agent" is best
// understood as a specialized service that:
// - Receives tasks/requests/events (messages)
// - Applies domain-specific reasoning or logic
// - Produces outputs (messages) that may be routed to other agents
//
// This repository keeps the Agent interface intentionally small:
//
// - Orchestration is handled elsewhere (pkg/orchestration)
// - Communication is handled elsewhere (pkg/comm)
// - Governance/policy is handled elsewhere (pkg/governance)
// - Persistence/memory is handled elsewhere (pkg/memory)
//
// Why a small interface matters
//
// Multi-agent systems need to evolve. A narrow interface makes it easy to:
// - Add new agent implementations without refactoring the platform
// - Substitute an agent backed by a model, a rules engine, or a human-in-the-loop queue
// - Unit test agents in isolation (you can mock Environment)
//
// Messages and the protocol package
//
// The Agent API uses the shared protocol Message type (re-exported here for
// convenience). That prevents import cycles between foundational packages.
package agent

