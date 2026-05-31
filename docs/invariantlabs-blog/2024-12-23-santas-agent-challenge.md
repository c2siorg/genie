# Santa's Agent Challenge: Fixing a Broken Delivery Agent

**Source:** Invariant Labs Blog  
**Date:** December 23, 2024  
**Authors:** Luca Beurer-Kellner; concept by Kristian Bonde Nielsen, Mislav Balunović  
**Tags:** challenge, testing, agent debugging, system prompt, open source

---

## The Challenge

Invariant Labs ran a festive winter competition built around a real-world agent failure pattern: an AI agent that has all the tools it needs but consistently fails to complete its tasks.

The scenario: Santa's delivery management agent knows which presents to deliver and which addresses they go to. It has access to the right tools. It should work. It doesn't — some gifts reliably end up undelivered despite the agent appearing to try.

Participants were given the agent's code, a system prompt, and a set of unit tests written using Invariant's Testing library. The goal was to identify what was wrong with the system prompt and fix it so all deliveries succeeded reliably.

---

## Why This Pattern Matters

The challenge isn't whimsical for its own sake. It's a clean reproduction of a class of agent failure that shows up constantly in production systems:

- The agent has the right capabilities.
- The agent appears to be running normally.
- A subset of tasks consistently fail, or fail intermittently.
- The failure isn't in the code — it's in the agent's *instructions*.

System prompt debugging is one of the hardest problems in agentic development precisely because failures are behavioural rather than mechanical. A code bug throws an exception; a prompt bug produces subtly wrong behaviour that passes casual inspection and only surfaces under specific conditions.

---

## The Testing Library as Infrastructure

The challenge was structured around Invariant's Testing library, which enables developers to write precise unit tests for agent behaviour using trace-level assertions. A test might say: "for every present in the list, the agent must eventually call `deliver_present(address)` with the correct address."

The library makes the failure mode explicit: participants could run the test suite, see which deliveries failed, inspect the trace for those failures in Explorer, and iterate on the system prompt until all tests passed.

This is the workflow Invariant's tools are designed to enable. The challenge demonstrated it in a context where the feedback loop was fast and the goal was unambiguous.

---

## Debugging Approach

Effective participants followed a pattern that generalises well:

1. Run the test suite to identify which scenarios fail consistently.
2. Open the failing traces in Explorer and step through the agent's reasoning for those cases.
3. Identify the specific decision point where the agent diverges from the intended behaviour — usually a misinterpretation of the system prompt under specific input conditions.
4. Modify the system prompt to close the ambiguity, rerun the tests.

The key insight from the challenge: agent failures under fixed conditions are almost always traceable to a specific ambiguity or omission in the instructions. Trace inspection is the fastest path to finding it.

---

## Resources Used in the Challenge

- **Invariant Testing library** — for writing and running the unit tests.
- **Challenge Playground** — a hosted environment where participants could run test scenarios and observe agent behaviour without local setup.
- **Invariant Explorer** — for inspecting agent reasoning traces at the step level.
- **Discord** — for support and discussion among participants.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/santas-agent-challenge.html
- Invariant Testing library: https://github.com/invariantlabs-ai/invariant
- Invariant Explorer: https://invariantlabs.ai/blog/explorer.html
