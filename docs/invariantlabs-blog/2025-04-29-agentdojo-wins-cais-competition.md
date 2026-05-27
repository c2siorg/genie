# AgentDojo Wins First Prize at the Center for AI Safety SafeBench Competition

**Source:** Invariant Labs Blog  
**Date:** April 29, 2025  
**Tags:** AgentDojo, AI safety, benchmarking, NeurIPS, Center for AI Safety

---

## The Competition

The Center for AI Safety's SafeBench competition challenges research teams to build benchmarks that evaluate AI systems across five dimensions: security, robustness, monitoring, alignment, and safety. First prize carries a $50,000 award and recognition from one of the most prominent safety-focused organisations in the field.

AgentDojo — developed jointly by Invariant Labs and ETH Zurich — took first place.

---

## What AgentDojo Measures

The benchmark addresses a gap in standard AI evaluation: most capability assessments measure only what an agent *can* do, not whether it can maintain that capability *under attack*.

AgentDojo evaluates two things simultaneously:

**Utility** — can the agent complete legitimate tasks assigned by users? The 97 tasks span realistic office-work scenarios: email management, Slack coordination, travel booking, banking operations.

**Security** — can an attacker cause the agent to fail, leak data, or perform unintended actions? The 629 security test cases pair with the utility tasks, measuring whether prompt injection attacks and adversarial inputs cause task failure, data exfiltration, or instruction override.

The dual measurement is important because utility and security trade off in real systems. A model that's highly restrictive might pass every security test but fail most utility tasks. A model optimised purely for task completion might pass utility tests but be trivially compromised on security. AgentDojo forces researchers to look at both axes at once.

---

## Key Findings from the Benchmark

When the framework was presented at NeurIPS 2024, the results surfaced a notable pattern:

- **GPT-4o** achieved the highest combined utility score across tasks.
- **Claude 3.5 Sonnet** showed the highest resilience to prompt injection attacks among tested models.

Neither model dominated both dimensions. The trade-off is real and measurable — which is exactly what makes the benchmark useful.

---

## Why "No Fixed Injections" Matters

Many security benchmarks use static attack strings: a fixed set of injection prompts evaluated against a fixed set of tasks. This measures performance against *known* attacks, which is useful for regression testing but doesn't evaluate generalisation.

AgentDojo uses dynamic evaluation: no attack prompt is fixed in the benchmark definition. Researchers can supply new attack strategies and new defence strategies, and the framework evaluates the resulting interaction. This means the benchmark remains relevant as the attack landscape evolves, rather than becoming a static target that defences can overfit to.

---

## Trace Inspection via Invariant Explorer

AgentDojo produces full execution traces for every evaluation run. These traces — including the agent's reasoning steps, tool calls, attack payloads, and defence activations — are viewable in Invariant Explorer.

The Explorer integration means a researcher can navigate directly from an aggregate score ("agent X achieved 74% security under GPT-4o") to the specific traces where the agent failed — examining exactly which prompt injection succeeded, what the model did in response, and where a different defence might have intervened.

This level of interpretability is uncommon in safety benchmarks, which typically report only summary metrics.

---

## The Research Team

AgentDojo was built by: Edoardo Debenedetti, Jie Zhang, Mislav Balunović, Luca Beurer-Kellner, Marc Fischer, and Florian Tramèr.

The work sits at the intersection of security and AI safety research — a combination that reflects Invariant's focus on translating academic rigour into tools that practitioners can deploy.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/agentdojo-wins-competition.html
- AgentDojo NeurIPS 2024 introduction: https://invariantlabs.ai/blog/agentdojo.html
- Invariant Explorer: https://invariantlabs.ai/blog/explorer.html
- Center for AI Safety SafeBench: https://safe.ai
