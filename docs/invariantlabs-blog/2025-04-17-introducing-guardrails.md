# Introducing Guardrails: The Contextual Security Layer for the Agentic Era

**Source:** Invariant Labs Blog  
**Date:** April 17, 2025  
**Tags:** guardrails, agentic security, LLM, MCP, policy, open source

---

## Why Prompt-Based Security Isn't Enough

The standard approach to AI safety has been to put rules in the system prompt: "don't share PII," "don't execute dangerous code," "refuse requests for sensitive information." This works for simple chatbot cases, but it breaks under two conditions that are routine in agentic systems:

1. **Prompt injection** — an attacker can override system prompt instructions with crafted content in the user message or in tool outputs.
2. **Context collapse** — in a long multi-step agent session, earlier safety instructions are statistically less influential over later decisions than nearby context. The model may comply with a safety rule at step 1 and violate it at step 15 because something in between changed the operative context.

Guardrails solves both problems by operating *outside* the model rather than inside it. Security rules are evaluated by a deterministic enforcement layer, not by the model's probabilistic behaviour.

---

## What Guardrails Does

Guardrails is a contextual security layer that sits between the agent and its LLM provider (via Invariant Gateway) and between the agent and its MCP servers. It intercepts messages in both directions, evaluates them against a policy, and blocks or modifies content that violates the policy before it reaches the model.

The "contextual" part is the distinguishing feature. Traditional content filters look at individual messages in isolation. Guardrails evaluates the *flow* — a message is flagged not just for what it says but for what preceded it. "Send this URL to Slack" might be fine; "read a spreadsheet, then send a URL to Slack" triggers a dataflow rule that catches link-preview exfiltration.

---

## The Seven Guardrailing Capabilities

**API Secret Detection** — Scans outbound and inbound messages for credentials, tokens, and API keys. Blocks messages where the model is about to send a secret to an external system.

**PII Detection** — Identifies personally identifiable information — names, phone numbers, national ID numbers, financial account identifiers — and prevents it from flowing to tools or outputs where it doesn't belong.

**Dataflow Control** — The most powerful capability. Defines allowed and prohibited sequences of tool calls. Example rule: "after reading from a user's private spreadsheet, no outbound Slack message may contain a URL." This directly prevents the link-preview exfiltration class of attacks.

**Tool Call Guardrails** — Restricts which tools can be called, with what parameters, and from which context. A rule might say: "the `execute_code` tool may only be called if the preceding user message was a direct programming request" — blocking cases where injected instructions trigger code execution.

**Code Guardrails** — Prevents unsafe patterns in code the model generates or executes: `eval()`, `exec()`, unvalidated subprocess calls, imports of specific dangerous modules.

**Content Guardrails** — Blocks copyrighted content reproduction and toxic or harmful output from reaching the user.

**Loop Detection** — Identifies agents that have entered pathological retry loops — the same action repeated more than N times — and halts them before they rack up API costs or side effects.

---

## How Rules Are Expressed

Rules use a deterministic policy language built on expressive predicates, not on model-evaluated prompts. An example rule:

```
# Block any tool call to execute_code after the agent reads from an untrusted URL
deny {
  trace.contains(tool_call("fetch_url", url=UNTRUSTED))
  trace.contains(tool_call("execute_code"))
  trace.order("fetch_url") < trace.order("execute_code")
}
```

Because the evaluation is deterministic, the rule either fires or it doesn't. There's no probability distribution, no "the model usually refuses this." A policy that's configured fires 100% of the time, regardless of model, prompt, or context.

---

## Integration

Guardrails runs through Invariant Gateway, which acts as a transparent proxy. Existing agent code requires only a single-line change: point the base URL at the Gateway endpoint. The gateway intercepts all LLM and MCP traffic, applies the policy, and forwards or blocks as appropriate.

For teams that want to audit what the guardrails are doing, Invariant Explorer provides a full trace view: every message, every tool call, every policy decision, timestamped and searchable.

---

## Open Source

The full source is on GitHub. Invariant publishes under an open-source licence so teams can inspect, fork, and extend the policy engine. An interactive playground is available for testing rules without deploying infrastructure.

---

## Where This Fits in a Defence-in-Depth Stack

Guardrails is not a replacement for good agent design — it's an additional enforcement layer. The relationship to other defences:

| Layer | What it does |
|-------|-------------|
| Model alignment | Broad behavioural guidance; fails under injection |
| MCP-Scan (static) | Inspects tool descriptions before deployment |
| Guardrails (runtime) | Enforces dataflow rules during live sessions |
| Audit log | Records what happened for post-incident review |

Running all four layers provides defence in depth: a bypass at one layer is caught at the next.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/guardrails.html
- Invariant Gateway (proxy layer): https://invariantlabs.ai/blog/announcing-invariant-gateway.html
- MCP-Scan (static scanner): https://invariantlabs.ai/blog/introducing-mcp-scan.html
- GitHub: https://github.com/invariantlabs-ai/invariant
