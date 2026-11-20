# Fool an Agent to Extract the Secret Password — Summer CTF 2024

**Source:** Invariant Labs Blog  
**Date:** August 5, 2024  
**Authors:** Dragoș Albăstroiu, Mislav Balunović  
**Tags:** CTF, challenge, adversarial, prompt injection, red team

---

## The Challenge

Invariant Labs launched the Invariant Summer '24 CTF — a public security competition where participants attempted to manipulate an AI agent into revealing a secret it was instructed to protect.

The scenario mirrors a class of real production risk: an agent that handles sensitive information in its context (a password, an API key, a user's private data) while also accepting inputs from external sources. The question the competition was designed to answer empirically: how hard is it, in practice, to get that information out?

---

## Why This Kind of Challenge Matters

The standard way to evaluate AI security is through academic benchmarks. These are rigorous but narrow — they test known attack types against predefined defence configurations, with researchers on both sides.

CTF competitions add a different dimension: a large number of participants with diverse backgrounds and motivations, trying unconstrained attack strategies against a real system. The tactics that succeed in a CTF are tactics that someone, eventually, will apply to production systems. Running the competition now — before those systems are widely deployed — generates ground truth about real attack surface.

The Summer '24 CTF preceded the more elaborate autumn challenge (October 2024 write-up) and served as the proof of concept that competitive adversarial evaluation of agents was both tractable and informative.

---

## What It Tests

The fundamental question: given an agent that has a secret and that processes external inputs, can an attacker craft input that causes the agent to output the secret?

Variants of this attack appear in every real agentic deployment:
- An agent reading customer-submitted text that contains injection payloads
- An agent processing emails where one email is from an attacker
- An agent using tools whose outputs are attacker-controlled
- An agent whose system prompt contains secrets the operator doesn't want disclosed

The CTF isolates the core dynamic and measures it directly.

---

## For Teams Building Agentic Systems

The competition's practical lesson: if your agent processes any external input — and every useful agent does — an attacker with enough submissions will eventually find an effective injection. The question is not "is prompt injection possible" (it is) but "what is the cost and how much blast radius does a successful attack have."

Defence strategies that emerged from the challenge and apply to production:

- **Separate the secret from the processing context.** If the agent doesn't need the secret to process user input, don't include it in the context during that phase.
- **Limit output channels.** An agent that can only respond to the immediate user, and can't send emails, post to Discord, or call external URLs, has a much smaller exfiltration surface.
- **Log and monitor for anomalous output patterns.** Successful injections often produce characteristic outputs — unusual formatting, unexpected URL inclusions, out-of-context responses. These are detectable if you're monitoring.

---

## Data Release

As with the Autumn 2024 CTF, Invariant committed to releasing anonymised submission data from this challenge. The full dataset, including successful and unsuccessful attack attempts, is available for researchers studying adversarial robustness.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/fool-an-agent-to-extract-the-secret-password.html
- Autumn 2024 CTF write-up (more detailed results): https://invariantlabs.ai/blog/ctf24-summary.html
- Formal security guarantees (July 2024): https://invariantlabs.ai/blog/icml2024-agents-formal-security.html
