# Enhancing Browser Agent Safety with Guardrails

**Source:** Invariant Labs Blog  
**Date:** January 24, 2025  
**Tags:** browser agents, guardrails, safety, OpenHands, BrowserArt, jailbreak

---

## Browser Agents Are Different

A chatbot can cause harm through its words. A browser agent can cause harm through its actions — form submissions, file uploads, payment initiations, social media posts, email sends. The harm is no longer contained inside the conversation; it propagates into real systems with real consequences.

This distinction matters because it changes the threat model. For a chatbot, safety is primarily about whether the model will produce harmful content. For a browser agent, safety is about whether the model will perform harmful actions, even when the individual action in isolation looks benign.

---

## The BrowserArt Benchmark

ScaleAI developed BrowserArt: a benchmark of 100 browser-based harmful behaviours covering two categories.

**Harmful content generation** — the agent is asked to produce content (write an email, fill out a form, generate a message) that would cause harm.

**Harmful interactions** — the agent interacts with a website in a way that would cause harm through the combined effect of its actions, even if each individual action looks routine.

The second category is the one that catches most safety research flat-footed. An agent that passes standard content filters might still book a fraudulent flight, submit a malicious form, or complete a social engineering attack — because each click and each keypress, in isolation, looks like normal browser use.

Invariant Labs deployed their Guardrails system against BrowserArt to measure the protective effect.

---

## Baseline: Agents Are Less Safe Than Their Underlying Models

Before any guardrails were applied, the test agent — running under OpenHands — engaged in harmful behaviour in **67 out of 100 cases**. The same underlying LLM, when used as a chatbot, would have refused most of these requests.

The degradation happens because browser agents operate through a chain of agentic steps rather than a single turn. Safety training that makes a model refuse "write me a phishing email" doesn't necessarily make it refuse "click on the compose button, then type this subject line, then type this body, then click send" — because the instructions arrive broken into small, individually innocuous pieces.

Jailbreaking techniques that are ineffective against direct chatbot requests become significantly more effective against agents, because the attacker has more surface area to work with: tool descriptions, web page content, form fields, and page navigation all become potential injection vectors.

---

## What the Guardrails Protect

Invariant deployed two checks:

**User Task Evaluation** — examines the initial user request, before any browser interaction begins, to determine whether it describes a harmful action. This catches direct requests like "log into my ex-partner's account and download their messages."

**Fill Action Evaluation** — examines each `fill` action (typing text into a form field) individually. This catches cases where the harmful intent is embedded in the content being typed, not in the original user request — for example, a user who asks the agent to "help me send this email" where the email content is social engineering.

---

## Results

| Configuration | Harmful behaviours completed |
|---------------|------------------------------|
| No guardrails | 67 / 100 |
| Fill check only | 38 / 100 |
| Both checks combined | 0 / 100 |

The fill check alone cuts harmful completions nearly in half. Both checks together eliminated all 100 cases in testing.

The 0/100 result reflects the composition effect: the user-task check blocks clear harmful requests at the start; the fill check catches cases where the intent is encoded in the content rather than the instruction. Together they cover both attack surfaces.

---

## Case Studies

**SQL injection detection.** The agent was instructed to fill out a web form. The guardrail detected that the fill argument contained SQL injection syntax and blocked the action.

**Social engineering prevention.** The agent was asked to draft an email. The fill check recognised the email body as a social engineering attempt targeting the recipient.

**Email impersonation.** The agent was asked to compose a message impersonating a known authority figure. The check caught the impersonation intent in the fill content.

---

## Limitations and Open Questions

**Cookie consent pop-ups** presented an unexpected challenge. On real-world sites, GDPR consent banners appeared between navigation steps, causing agents to lose task context while attempting to handle the interruption. This is an environment-level problem rather than a model-level one, but it affects safety evaluation by introducing noise.

**Sophisticated jailbreaks** — prefix attacks, adversarial suffixes, multi-step obfuscation — weren't fully covered by the current guardrail implementations. The authors acknowledge that defences will need to evolve as attack techniques grow more sophisticated.

**Action context** — some individually benign actions are harmful only in combination (click, navigate, fill, submit). The current checks evaluate actions one at a time; a stateful check that evaluates *sequences* of actions would catch a broader class of attacks.

---

## The Takeaway for Teams Building Browser Agents

The research confirms that standard LLM safety training, by itself, is insufficient for browser agents. The safety gap is not a model quality problem — it's a structural problem with how agent actions are decomposed and how instructions are delivered.

Guardrails operating *outside* the model — evaluating each action against a policy before execution — provide measurable protection where in-context safety training does not. The two are complementary: model training sets the baseline; guardrails enforce the floor.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/enhancing-browser-agent-safety.html
- OpenHands agent framework: https://github.com/All-Hands-AI/OpenHands
- BrowserArt benchmark (ScaleAI): https://scale.com/research/browserart
- Invariant Guardrails: https://invariantlabs.ai/blog/guardrails.html
