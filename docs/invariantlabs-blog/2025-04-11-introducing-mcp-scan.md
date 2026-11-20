# Introducing MCP-Scan: A Security Scanner for Agentic Applications

**Source:** Invariant Labs Blog  
**Date:** April 11, 2025  
**Tags:** MCP, security scanner, tool poisoning, prompt injection, open source

---

## The Problem MCP-Scan Solves

Most developers plugging MCP servers into their agent stack have no way to inspect what those servers are actually sending to the model. The tool description field — the natural-language text that tells the model what a tool does — is the primary attack surface for Tool Poisoning Attacks, rug pulls, cross-origin escalations, and embedded prompt injections.

Until now, the only option was reading raw JSON configuration files and hoping nothing had changed since you last looked.

MCP-Scan brings systematic, automated inspection to that surface.

---

## What It Scans For

MCP-Scan targets four categories of threat:

**Tool Poisoning Attacks** — Tool descriptions containing hidden instructions that manipulate the model's behaviour. These are often obfuscated or formatted to be invisible in UI summary views while remaining fully legible in the model's context.

**MCP Rug Pulls** — Tool descriptions that have changed since the user's last approval. An attacker (or a compromised server operator) can swap a benign description for a malicious one after users have already trusted the tool.

**Cross-Origin Escalations** — Attacks where one MCP server's tool descriptions contain references to another server's tools, allowing the malicious server to override or manipulate trusted integrations without appearing to do so directly.

**Prompt Injection Attacks** — Payloads embedded inside tool descriptions designed to break context isolation and inject new instructions into the model's active session.

---

## How to Use It

Installation requires no configuration. Run a scan with:

```bash
uvx mcp-scan@latest
```

The scanner reads your MCP configuration files, connects to each configured server, retrieves the full tool descriptions, and performs both local pattern analysis and a cloud-based check via the Invariant Guardrails API.

To inspect an individual server's tool descriptions in detail:

```bash
uvx mcp-scan@latest inspect
```

This outputs the full description text for every tool on that server — exactly what the model will see — making it straightforward to manually review or pipe into additional analysis.

---

## Tool Pinning: Defeating Rug Pulls

MCP-Scan tracks tool descriptions over time by hashing each one at first scan and storing the hash. On subsequent scans, it recomputes the hash and alerts if the description has changed.

This is the structural defence against rug pulls: you approve a tool once, and MCP-Scan detects any modification to what you approved. The first scan establishes the baseline; every scan after that is a diff.

---

## Cross-Origin Detection

The cross-origin check scans all configured servers simultaneously and looks for tool descriptions that reference other servers' tool names or instruction namespaces. When Server A's description contains text that manipulates how the model calls Server B's tools, that's a cross-origin escalation — and MCP-Scan flags it.

---

## Data Privacy

MCP-Scan sends tool names and descriptions to Invariant's cloud API for the threat detection pass. It does not log what you *do* with your tools — no invocation payloads, no user messages, no tool outputs. Only the static descriptions that come from the server configuration.

Teams uncomfortable sharing tool descriptions externally can run MCP-Scan in local-only mode or enquire about private deployment options.

---

## Architecture: Where It Fits

MCP-Scan operates at installation and validation time — it's the equivalent of running a static analyser or a dependency audit before you deploy. For runtime interception (catching malicious content as it flows through the agent during a live session), use MCP-Scan in **proxy mode** or deploy Invariant Gateway.

The two modes are complementary:

| Mode | When | What it catches |
|------|------|-----------------|
| Scan (static) | At setup / on a schedule | Poisoned descriptions, rug pulls |
| Proxy (runtime) | During live agent sessions | Toxic agent flows, runtime injections |

---

## Open Source

The tool is published under an open-source licence on GitHub. Community contributions are welcome, particularly around new attack signature patterns as the MCP ecosystem continues to expand.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/introducing-mcp-scan.html
- MCP-Scan on GitHub: https://github.com/invariantlabs-ai/mcp-scan
- Tool Poisoning Attacks (background): https://invariantlabs.ai/blog/mcp-security-notification-tool-poisoning-attacks.html
- Smithery partnership (MCP-Scan protecting registry): https://invariantlabs.ai/blog/smithery-mcp-scan.html
