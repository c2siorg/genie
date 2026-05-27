# Snyk Acquires Invariant Labs to Accelerate Agentic AI Security

**Source:** Invariant Labs Blog → Snyk newsroom  
**Date:** June 24, 2025  
**Tags:** acquisition, Snyk, agentic security, milestone

---

## The Announcement

Snyk, the developer security platform, announced the acquisition of Invariant Labs to deepen its agentic AI security capabilities. The acquisition brings Invariant's research bench — including MCP-Scan, Guardrails, AgentDojo, and the Gateway proxy — into Snyk's developer security platform.

The announcement phrase that captured the direction: *"Deepens Snyk's Research Bench."* This is an acqui-hire and technology integration, not just a product purchase.

---

## What Invariant Built

Over roughly two years of public work, Invariant Labs built a coherent stack for agentic AI security:

**Research layer:**
- AgentDojo — the first framework to jointly evaluate agent utility and security under adversarial conditions (NeurIPS 2024; won Center for AI Safety SafeBench competition, May 2025).
- Formal security guarantees work (ICML 2024) — the theoretical foundation for deterministic policy enforcement on agent traces.
- CTF competitions (Summer 2024, Autumn 2024) — empirical validation of attack surfaces with public datasets.

**Infrastructure layer:**
- Invariant Gateway — a transparent proxy that intercepts agent traffic for observability and policy enforcement.
- Invariant Guardrails — a contextual, deterministic policy engine for LLM and MCP traffic.
- MCP-Scan — the primary open-source security scanner for MCP server configurations.

**Ecosystem integration:**
- Partnership with Smithery to scan every server in the MCP registry.
- ETH Zurich spin-off with active academic research pipeline.

---

## Why the Timing Makes Sense

The acquisition came at the moment when MCP adoption was crossing from early-adopter to mainstream. The GitHub MCP server had 14,000 stars by May 2025. Zapier had deployed MCP integrations. Cursor and Claude Desktop had MCP as a core feature.

With that growth came the attack surface Invariant had been documenting since April 2025: Tool Poisoning Attacks, rug pulls, cross-origin escalations, WhatsApp exfiltration, GitHub private repository leaks. The threat landscape Invariant had been mapping was becoming real and high-visibility simultaneously.

Snyk's platform already secured developer code, open-source dependencies, containers, and infrastructure. Adding MCP and agentic security extends that coverage to the new surface where developers are building next.

---

## What It Means for Open Source

Snyk has a history of maintaining open-source projects it acquires or builds on. MCP-Scan, AgentDojo, and the Invariant SDK were all published under open-source licences. The announcement didn't specify changes to the open-source status of these tools.

---

## The Research Legacy

The most important output of Invariant's two years of work is arguably the conceptual framework as much as the tools: the idea that agent security is fundamentally a *dataflow* problem, not a content filtering problem; that defences must be contextual (sequence-aware) rather than per-message; that the MCP description field is an injection surface equivalent to SQL injection surfaces in the previous era; and that formal policy specification, not prompt-based guidance, is the right foundation for deterministic security guarantees.

Those ideas will continue to influence how the field builds security infrastructure for agentic systems, regardless of organisational structure.

---

## References

- Snyk acquisition announcement: https://snyk.io/news/snyk-acquires-invariant-labs-to-accelerate-agentic-ai-security-innovation/
- Invariant Labs blog: https://invariantlabs.ai/blog
- MCP-Scan: https://github.com/invariantlabs-ai/mcp-scan
- AgentDojo: https://github.com/ethz-spylab/agentdojo
