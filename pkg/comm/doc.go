// Package comm implements the communication substrate for agents.
//
// Why "communication" is its own layer
//
// A common failure mode in early multi-agent prototypes is to let agents call
// each other directly (e.g. agentA.DoThing() calls agentB.DoThing()).
// That approach quickly becomes:
// - Tightly coupled (hard to add/remove/replace agents)
// - Hard to observe (no unified trace of interactions)
// - Hard to govern (policy checks are scattered)
//
// A message bus addresses those issues by making agent interaction explicit:
// - Agents emit protocol.Messages
// - The bus routes those messages to subscribers
// - Orchestration wires subscriptions (pkg/orchestration)
//
// In this repo
//
// The InMemoryBus is intentionally minimal and "good enough" for a demo:
// - It supports targeted delivery via Message.To (agent ID)
// - It supports broadcast subscribers (agentID == "")
// - It runs handlers asynchronously (goroutines)
//
// Production considerations (not implemented here)
//
// Real communication layers often add:
// - Delivery guarantees (at-least-once, exactly-once)
// - Backpressure and flow control
// - Ordering and partitioning
// - Dead-letter queues and retries
// - Persistent logs (event sourcing)
// - Distributed transport (NATS, Kafka, RabbitMQ, etc.)
package comm

