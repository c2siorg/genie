# Releasing Explorer & Testing: Visualise and Understand AI Agents

**Source:** Invariant Labs Blog  
**Date:** December 17, 2024  
**Tags:** Explorer, testing, observability, debugging, open source, agent traces

---

## The Trace Problem

Every AI agent leaves a trace: a chronological record of every LLM call, every tool invocation, every response, every decision branch. In theory this record contains everything needed to understand why the agent behaved a particular way. In practice, raw traces are JSON files that might be thousands of lines long, with no navigation, no annotation, no search, and no way to share a specific moment with a teammate.

The gap between "the information exists" and "the information is usable" is exactly what Invariant Explorer was built to close.

---

## Explorer: Observability for Agent Behaviour

Explorer is an open-source observability platform that turns raw agent traces into a navigable, annotatable, shareable interface.

**Chronological visualisation.** Traces are displayed as a timeline of events — LLM messages, tool calls, tool outputs, model reasoning steps — in the order they occurred. A developer can immediately see the full sequence of an agent session rather than constructing it mentally from JSON fragments.

**Annotation for collaboration.** Team members can annotate specific moments in a trace: "this is where the agent hallucinated a product name," "this tool call is the one that caused the data leak." Annotations persist and are visible to anyone with access to the trace, making async debugging practical.

**Filter and search.** Traces can be searched by content, filtered by event type, and sliced to show only the segment of interest. Rather than reading a thousand-line trace to find one tool call, a developer queries for it directly.

**Sharing for issue tracking.** Any trace or trace segment can be linked directly — pasted into a GitHub issue, a Slack message, or a post-mortem doc. The linked view opens in Explorer with full context, rather than requiring the recipient to set up their own environment to inspect a JSON dump.

---

## The Public Benchmark Registry

A secondary benefit of Explorer is what it does for benchmark legibility. Popular agent benchmarks — SWE-Bench, Cybench, Webarena — are typically distributed as raw datasets: thousands of task definitions and thousands of corresponding agent-execution JSON files.

Researchers working with these benchmarks spend significant time on data wrangling before they can answer a question as simple as "which tasks did agent X fail, and why?" Explorer's public registry ingests these benchmarks and makes them navigable by anyone — no local setup required.

---

## Testing: Unit Tests for Agent Behaviour

Alongside Explorer, Invariant released a test library designed to bring the discipline of unit testing to agent development.

Traditional unit tests verify that a function returns the right output given a specific input. Agent testing is harder because:
- Agent sessions are stochastic — the same input can produce different traces.
- The unit of correctness is often a behavioural pattern across a session, not a single output value.
- A test that passes on one trace might fail on a re-run of the same session.

Invariant's testing library addresses this with **trace-level assertions**: instead of asserting on final outputs, tests assert on patterns in the trace itself. "The agent must not call `execute_code` after reading from an external URL" is expressible as a test. "The agent must eventually call `send_email` if the user asked it to send an email" is expressible as a test.

The library integrates with Explorer, so test failures surface as annotated traces — the developer can see exactly which step in the trace caused the assertion to fail.

---

## Test-Driven Agent Development

The combination of Explorer and Testing enables a workflow that mirrors test-driven development for traditional software:

1. Write a test that captures a desired behavioural invariant (or a known failure mode).
2. Run the agent; inspect the trace in Explorer.
3. Modify the agent (system prompt, tool selection, planning logic) to make the test pass.
4. Regression test by running the full test suite on every change.

This brings systematic quality assurance to a domain — agent development — where the current state of practice is largely "run the agent and see if it works."

---

## Open Source Availability

Both Explorer and the Testing library are published as open-source packages:

```bash
pip install invariant-sdk       # Explorer integration + Testing library
```

Documentation and examples are available on GitHub and PyPI.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/explorer.html
- AgentDojo (uses Explorer for trace inspection): https://invariantlabs.ai/blog/agentdojo.html
- GitHub: https://github.com/invariantlabs-ai/invariant
