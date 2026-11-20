# Invariant Gateway: A Transparent Debugging and Security Layer for AI Agents

**Source:** Invariant Labs Blog  
**Date:** March 6, 2025  
**Tags:** gateway, debugging, observability, security, OpenAI, Anthropic, open source

---

## The Debugging Problem in Agentic Systems

Debugging a traditional API service is straightforward: structured logs, request IDs, error codes. The call either succeeded or it didn't, and the payload is inspectable.

Debugging an AI agent is different in kind. An agent session might involve dozens of LLM calls, tool invocations, computer interactions, and branching decision paths. A failure at step 23 might be caused by something the agent did — or failed to do — at step 7. The information exists; it's scattered across raw JSON logs that are often thousands of lines long, with no tooling designed to navigate them.

Invariant Gateway is a proxy service that solves this by intercepting all agent traffic and capturing it into a structured, searchable, visualisable trace, without requiring any changes to the agent's code.

---

## How It Works

Gateway operates as a **pass-through proxy** between the agent and its LLM provider. The agent sends requests to the Gateway endpoint instead of directly to OpenAI or Anthropic; Gateway forwards the request, captures the full exchange, and returns the response with minimal added latency.

The only change required in existing agent code is the base URL:

```python
# Before
client = openai.OpenAI(api_key="...")

# After — route through Gateway
client = openai.OpenAI(
    api_key="...",
    base_url="https://gateway.invariantlabs.ai/api/v1/openai",
    default_headers={"Invariant-Authorization": f"Bearer {INVARIANT_API_KEY}"}
)
```

No other code changes. No SDK swap. No rewrite. The agent continues to work exactly as before; Gateway silently captures the traffic.

---

## What Gets Captured

Gateway captures the full interaction at every level:

**LLM-level** — Every prompt, completion, system message, and token count.

**Tool use** — Every tool call, its parameters, and the tool output returned.

**Computer interactions** — When the agent is using a browser or desktop automation, Gateway captures clicks, keystrokes, navigation events, and screenshots.

**Timing** — Precise timestamps at every step, making it possible to identify where latency is accumulating.

All captures flow into Invariant Explorer, where they're organised into trace views that a developer or security reviewer can navigate without reading raw JSON.

---

## Supported Providers and Frameworks

**LLM providers:** OpenAI, Anthropic, Google Gemini.

**Agent frameworks:** AutoGen, Swarm, Browser Use, and any framework built on top of the supported providers' SDKs.

Support for additional providers is roadmapped — because Gateway operates at the HTTP level, adding a new provider is primarily an authentication and schema normalisation task.

---

## Two Deployment Patterns

**Organisation-wide security monitoring.** A security or platform team deploys Gateway as shared infrastructure. All agents across the organisation route their traffic through it. The team gains a centralised view of what every agent is doing — essential for anomaly detection, audit compliance, and incident response.

**Individual developer debugging.** A developer routes their local agent sessions through Gateway during development. They can step through traces, annotate decision points, share traces with teammates via a link, and correlate failures with specific inputs.

Both patterns use the same Gateway infrastructure. Access control in Explorer determines what different users can see.

---

## Open Source and Local Deployment

Gateway is published as open-source software. Teams with data residency constraints or who aren't comfortable routing agent traffic through a third-party cloud service can deploy it locally or on-premise. The on-premise deployment loses the cloud Explorer UI but retains the trace capture capability and can be connected to a self-hosted Explorer instance.

---

## The Relationship to Guardrails

Gateway is the transport layer; Guardrails is the policy layer. Once traffic is flowing through Gateway, adding Guardrails is a configuration change that activates rule evaluation on every message.

| Component | Role |
|-----------|------|
| Gateway | Captures traffic, provides trace visibility |
| Guardrails | Evaluates security policies against traffic |
| Explorer | Visualises traces and policy decisions |

Running all three gives an agent system observability (what happened), security enforcement (what was blocked), and audit capability (why decisions were made) in a single integrated stack.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/announcing-invariant-gateway.html
- Invariant Guardrails (policy layer): https://invariantlabs.ai/blog/guardrails.html
- Invariant Explorer (trace visualisation): https://invariantlabs.ai/blog/explorer.html
- Gateway on GitHub: https://github.com/invariantlabs-ai/invariant
