// Package protocol defines the shared "wire format" used to move information
// between components in this repository.
//
// Why this package exists
//
// In multi-agent systems, many packages need to agree on a message shape:
// agents, orchestration, governance, communication, observability, evaluation,
// and sometimes tool adapters. If each package defines its own Message type,
// you either end up duplicating conversions everywhere or you introduce import
// cycles (e.g. comm importing agent, agent importing comm, etc.).
//
// To keep dependencies acyclic and the architecture composable, the core types
// that "everyone needs" live here:
//
// - Message: the unit of communication
// - MessageRole: semantic role (user/system/agent/tool/etc.)
//
// Architectural note
//
// This package intentionally does NOT define:
// - Agent interfaces (those belong to pkg/agent)
// - Routing/bus semantics (pkg/comm)
// - Governance/policy evaluation (pkg/governance)
// - Orchestration and subscription wiring (pkg/orchestration)
//
// Think of protocol as: "If we serialized our system interactions, what is the
// minimal common schema we would write to the log or send over the network?"
//
// Message identity and correlation
//
// Message.ID is a unique identifier for a single message instance. Real systems
// typically add additional correlation fields (conversation id, trace id, span id,
// causation id, parent message id, etc.). In this reference implementation we
// keep things simple, but the Metadata map provides a safe extension point.
package protocol

