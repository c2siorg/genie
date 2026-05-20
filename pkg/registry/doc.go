// Package registry provides discovery and lookup of agents.
//
// What problem this solves
//
// In a multi-agent system, you typically do not want components to be wired
// together through direct imports and direct method calls:
// - It makes the system brittle (hard to add/remove agents)
// - It reduces observability (calls are hidden, messages are not traceable)
// - It complicates governance (no clear boundaries to enforce policy)
//
// Instead, agents are registered and discovered at runtime.
//
// How this is used in this repo
//
// - cmd/demo registers three agents into an in-memory registry.
// - pkg/orchestration reads registry.List() at startup and subscribes each agent
//   to the comm bus by its ID (i.e. its "address").
//
// Capability-based discovery
//
// The registry also supports FindByCapability. This is a common pattern in
// multi-agent platforms where a coordinator/dispatcher chooses an agent based
// on "skills" rather than hard-coded IDs.
//
// Production considerations (not implemented here)
//
// Real registries often include:
// - Versioning (agent v1/v2)
// - Health/status (ready, degraded, draining)
// - Dynamic membership (agents joining/leaving)
// - Load-aware routing metadata (queue depth, latency)
// - Trust boundaries (which agents can call which tools)
package registry

