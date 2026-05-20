// Package memory provides simple state storage abstractions for agents.
//
// Memory in multi-agent systems
//
// "Memory" can mean several different things in agentic architectures:
//
// - Short-term working memory: scratchpad state while solving a task
// - Conversation state: the evolving history of an interaction/session
// - Long-term memory: durable facts retrieved across sessions (often vector search)
// - Shared blackboard: state shared between agents to coordinate
//
// This repo intentionally starts with a tiny interface (KeyValueStore) and a
// trivial in-memory implementation. The goal is to show where memory fits in
// the architecture, not to prescribe a single storage strategy.
//
// How you'd use it
//
// In a more advanced version of this repo, Environment (pkg/agent) would expose
// a memory interface, and agents would read/write keys such as:
// - "conversation:<id>:summary"
// - "task:<id>:plan"
// - "agent:<id>:profile"
package memory

