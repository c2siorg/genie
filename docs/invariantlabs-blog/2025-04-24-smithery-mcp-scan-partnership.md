# Invariant Partners with Smithery to Scan Every MCP Server on the Registry

**Source:** Invariant Labs Blog  
**Date:** April 24, 2025  
**Authors:** Marc Fischer, Luca Beurer-Kellner  
**Tags:** MCP, partnership, Smithery, registry, security scanning

---

## What the Partnership Does

Smithery is the primary public registry for MCP servers — the place developers go to find pre-built capabilities and where server authors publish their tools for others to discover. The catalogue covers thousands of integrations: databases, APIs, communication tools, code execution environments.

From this partnership forward, every server listed on Smithery is automatically scanned by MCP-Scan before it appears in search results. The scan findings are displayed directly on each server's registry page, giving users a security summary before they decide whether to connect.

This brings the "supply chain scanning" model that package registries like npm and PyPI have adopted for code vulnerabilities — running automated checks on every publish — to the MCP ecosystem.

---

## Why a Registry-Level Integration Matters

Previously, a user discovering an MCP server on Smithery had to make a trust decision based on the server's description, star count, and author reputation. There was no standardised information about whether the server's tool descriptions contained injection payloads, rug-pull risks, or cross-origin escalation patterns.

The gap was systematic: individual users didn't have the tooling to inspect tool descriptions before connecting, and server authors had no incentive to self-report vulnerabilities.

Registry-level scanning changes the incentive structure. A server that fails MCP-Scan gets a visible warning label before a single user has connected. That visibility creates pressure on server authors to ship clean descriptions from day one, rather than patching only after a reported incident.

---

## What MCP-Scan Checks at the Registry Level

The automated scans cover the same threat categories as local MCP-Scan runs:

- **Tool Poisoning** — embedded instructions in tool descriptions that manipulate model behaviour
- **Rug Pull Susceptibility** — whether the server's description-versioning pattern has changed in ways that would evade previous approvals
- **Cross-Origin Escalation Patterns** — tool descriptions that reference other servers' instruction namespaces
- **Prompt Injection Payloads** — known injection syntax embedded in descriptions

Registry-level scanning runs on publish and on a continuous schedule for existing servers, so a server that passes on day one can still be flagged if it's later modified.

---

## About Smithery

Smithery's model is analogous to what npm is for Node.js packages: a discovery and deployment platform that abstracts away the work of finding, evaluating, and wiring up capabilities. Users describe what they want their agent to do; Smithery surfaces the MCP servers that match; the integration is a configuration change, not a custom build.

The volume makes security review at the individual-server level impractical for users, which is exactly why registry-level automation matters.

---

## Getting Involved

Teams building MCP servers who want to integrate MCP-Scan into their CI pipeline before publishing to Smithery can use the same `uvx mcp-scan@latest` toolchain locally. The registry-level scan uses the same engine, so a server that passes locally should pass on Smithery.

Organisations building larger agentic systems who want runtime protection (not just static scanning at install time) can explore Invariant Guardrails, which adds the dataflow control layer on top of the static checks.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/smithery-mcp-scan.html
- Smithery MCP registry: https://smithery.ai
- MCP-Scan on GitHub: https://github.com/invariantlabs-ai/mcp-scan
- Introducing MCP-Scan (full technical detail): https://invariantlabs.ai/blog/introducing-mcp-scan.html
