# Invariant Labs Blog — Rewritten Reference

This folder contains rewritten summaries and analyses of every post published
on the [Invariant Labs blog](https://invariantlabs.ai/blog). Invariant Labs
(acquired by Snyk in June 2025) was the primary research organisation mapping
the security landscape for MCP-based and agentic AI systems.

Each file is a fresh rewrite — not a copy of the original — that preserves the
technical substance, adds framing context, and notes connections to related
posts and tools.

---

## Why This Matters for Genie

Genie is an MCP-enabled multi-agent system. Every vulnerability class Invariant
documented is a potential attack surface for Genie deployments:

| Invariant finding | Genie mitigation |
|-------------------|-----------------|
| Tool Poisoning Attacks on MCP descriptions | `pkg/governance.TenantPolicy` + MCP-Scan at install |
| Cross-tenant data leaks | `pkg/storage/postgres` RLS + `WithTenant` GUC |
| Dual-identity audit gaps | `pkg/auth/tokenexchange` RFC 8693 `act` chain |
| Non-production agents serving traffic | `pkg/agent.Tier` dispatch gate |
| Prompt injection through tool outputs | `pkg/governance.CompositePolicy` bus inspection |

---

## Posts — Reverse Chronological

### 2025

| Date | Post |
|------|------|
| 2025-06-24 | [Snyk Acquires Invariant Labs](./2025-06-24-snyk-acquires-invariant-labs.md) |
| 2025-05-26 | [GitHub MCP Exploited: Private Repo Access via MCP](./2025-05-26-github-mcp-vulnerability.md) |
| 2025-04-29 | [AgentDojo Wins Center for AI Safety Competition](./2025-04-29-agentdojo-wins-cais-competition.md) |
| 2025-04-24 | [Partnership with Smithery: Registry-Level MCP Scanning](./2025-04-24-smithery-mcp-scan-partnership.md) |
| 2025-04-17 | [Introducing Guardrails: Contextual Security for Agents](./2025-04-17-introducing-guardrails.md) |
| 2025-04-11 | [Introducing MCP-Scan](./2025-04-11-introducing-mcp-scan.md) |
| 2025-04-07 | [WhatsApp MCP Exploited: Message History Exfiltration](./2025-04-07-whatsapp-mcp-exploited.md) |
| 2025-04-01 | [MCP Tool Poisoning Attacks — Security Notification](./2025-04-01-tool-poisoning-attacks.md) |
| 2025-03-06 | [Invariant Gateway: Debugging and Security Proxy](./2025-03-06-invariant-gateway.md) |
| 2025-01-24 | [Enhancing Browser Agent Safety with Guardrails](./2025-01-24-browser-agent-safety.md) |

### 2024

| Date | Post |
|------|------|
| 2024-12-23 | [Santa's Agent Challenge](./2024-12-23-santas-agent-challenge.md) |
| 2024-12-17 | [Releasing Explorer & Testing](./2024-12-17-explorer-and-testing.md) |
| 2024-12-11 | [AgentDojo at NeurIPS 2024](./2024-12-11-agentdojo.md) |
| 2024-10-08 | [CTF Insights: Cracking the Code](./2024-10-08-ctf-insights.md) |
| 2024-08-12 | [Invariant Labs — ETH Zurich Spin-Off](./2024-08-12-eth-spin-off.md) |
| 2024-08-05 | [Summer CTF Challenge 2024](./2024-08-05-summer-ctf-challenge.md) |
| 2024-07-25 | [Agents with Formal Security Guarantees (ICML 2024)](./2024-07-25-agents-formal-security.md) |
| 2024-07-10 | [What Hundreds of Web Agent Traces Taught Us](./2024-07-10-analyzing-web-agent-traces.md) |

---

## Theme Index

### MCP Security
- [Tool Poisoning Attacks](./2025-04-01-tool-poisoning-attacks.md)
- [MCP-Scan](./2025-04-11-introducing-mcp-scan.md)
- [WhatsApp MCP Exploited](./2025-04-07-whatsapp-mcp-exploited.md)
- [GitHub MCP Vulnerability](./2025-05-26-github-mcp-vulnerability.md)
- [Smithery Partnership](./2025-04-24-smithery-mcp-scan-partnership.md)

### Guardrails and Policy Enforcement
- [Introducing Guardrails](./2025-04-17-introducing-guardrails.md)
- [Browser Agent Safety](./2025-01-24-browser-agent-safety.md)
- [Agents with Formal Security Guarantees](./2024-07-25-agents-formal-security.md)

### Observability and Debugging
- [Invariant Gateway](./2025-03-06-invariant-gateway.md)
- [Explorer & Testing](./2024-12-17-explorer-and-testing.md)
- [Web Agent Trace Analysis](./2024-07-10-analyzing-web-agent-traces.md)

### Benchmarking and Research
- [AgentDojo at NeurIPS 2024](./2024-12-11-agentdojo.md)
- [AgentDojo Wins SafeBench](./2025-04-29-agentdojo-wins-cais-competition.md)
- [CTF Insights](./2024-10-08-ctf-insights.md)
- [Summer CTF 2024](./2024-08-05-summer-ctf-challenge.md)

### Company
- [ETH Spin-Off](./2024-08-12-eth-spin-off.md)
- [Snyk Acquisition](./2025-06-24-snyk-acquires-invariant-labs.md)

---

## Key Concepts Across the Corpus

**Toxic agent flow** — a sequence of agent actions where an early injected instruction causes harmful behaviour in a later step. The harm is in the sequence, not any single message.

**Tool Poisoning Attack (TPA)** — a malicious MCP tool description that contains hidden instructions visible to the model but not shown in the client UI.

**MCP Rug Pull** — a server operator modifying tool descriptions after user approval, inheriting the original trust grant for the new behaviour.

**Cross-origin escalation** — a malicious MCP server's tool description manipulating the model's behaviour toward a different, trusted MCP server.

**Link-preview exfiltration** — data smuggling via URLs that trigger automatic HTTP GET requests (Discord, Slack preview generation), encoding stolen data in URL parameters.

**Defence in depth for agents** — no single layer (model alignment, MCP-Scan, runtime guardrails, audit log) is sufficient alone; all four together provide meaningful security.
