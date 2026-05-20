// Package eval provides evaluation hooks for multi-agent interactions.
//
// Why evaluation is a first-class building block
//
// Multi-agent systems are dynamic and can degrade subtly as you:
// - change prompts/tools/models
// - add agents or new routing logic
// - modify governance and constraints
//
// Evaluation gives you feedback loops:
// - Did the system solve the task?
// - How much did it cost? How long did it take?
// - Did it violate policies? Did it hallucinate?
// - Is performance improving or regressing over time?
//
// In this repo
//
// The evaluation layer is intentionally minimal: a Store interface and an in-memory
// implementation. A production system would add:
// - experiment runs with datasets
// - structured metrics and traces
// - CI gating (fail builds on regression)
// - human review workflows
package eval

