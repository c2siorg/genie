# What Hundreds of AI Web Agent Traces Taught Us

**Source:** Invariant Labs Blog  
**Date:** July 10, 2024  
**Author:** Mislav Balunović  
**Tags:** web agents, trace analysis, debugging, WebArena, failure patterns, benchmarks

---

## The Premise

Web agents — LLM-based systems that navigate browsers to complete user tasks like shopping, research, and account management — are impressive in demos and fragile in production. The gap between "works on the example task" and "works reliably across a realistic distribution of tasks" is large and poorly understood.

Invariant Labs approached this empirically. Rather than proposing theoretical improvements and testing them on benchmarks, the team analysed hundreds of actual agent execution traces to find where agents fail, then built targeted fixes. The result: up to 16% performance improvement on WebArena's OpenStreetMap subset and 7% improvement on ShoppingAdmin — meaningful gains from understanding the failure modes rather than scaling the model.

---

## Why Trace Analysis Is the Right Starting Point

A web agent session is a sequence of decisions: what to click, what to type, what to search for, what to do when the page looks different from expected. Each decision is visible in the trace. When an agent fails, the cause is somewhere in that trace — usually not at the final step, and usually not obvious from the final output alone.

Without trace analysis, debugging is guesswork: tweak the system prompt, rerun the benchmark, see if the number goes up. With trace analysis, the failure mode is precise: "the agent failed on this class of task because it appended text to a field that already had content, rather than clearing it first."

The precision matters because it points directly at the fix.

---

## Five Failure Patterns Found in the Data

### 1. Looping

Agents trapped in action loops — repeating the same operation indefinitely — account for a significant share of hard failures. The most instructive case: an agent searching for a product by typing a query into a search field. On each iteration, the agent typed the query *in addition to* the existing text in the field, because it didn't clear the field first. The search string grew longer on each iteration, returning increasingly unhelpful results, until the agent exhausted its step budget.

**Fix:** Modify the `type` action implementation to clear the field's existing content before inserting new text. This is a tool-level fix, not a prompt fix — the agent's reasoning was correct; the tool's behaviour was wrong.

### 2. Hallucination

Language models produce text based on training data as well as context. When an agent is asked to retrieve information from a webpage and include it in an output, it sometimes substitutes memorised plausible-sounding values for the actual values on the page.

The traced example: an agent asked to compose a contact message for a specific customer invented a name and email address that matched the *type* of information requested but didn't match the actual page content.

**Fix:** Explicit prompting to prioritise retrieved environmental data over recalled information. "Use only information visible on the current page; do not infer or recall" is more reliable than a general instruction to be accurate.

### 3. Environment Errors

Some failures are not model failures — they're failures of the interface between the agent and its environment. Dropdown menus tested through accessibility tree interfaces failed because the accessibility tree didn't expose the options in a form the agent could use.

**Fix:** Introduce a dedicated `select_option` action that queries the DOM directly for HTML `<select>` and `<option>` elements, rather than relying on the accessibility tree representation. The fix is in the toolset, not the model.

### 4. Ignoring Instructions

Agents sometimes return partial results without completing the specified task, either because an early answer is available and seems sufficient, or because the instruction included a temporal or filtering condition that wasn't salient enough in the prompt.

The traced example: asked for January 2023 best-sellers, the agent returned current best-sellers — apparently because "best sellers" matched a page element before the date filter was applied.

**Fix:** Strengthened prompting that foregrounds the filtering condition and explicitly instructs the agent to verify that all constraints in the request have been satisfied before returning a result.

### 5. Benchmark Design Artifacts

Not all "failures" are agent failures. Some stem from the benchmark's evaluation methodology: overly strict string-matching requirements that fail a correct answer because of minor formatting differences, or environment sensitivity that makes tasks reproducible only under specific initial conditions.

These are worth cataloguing separately because they affect score reporting and can make agents look worse than they are on dimensions that don't reflect real-world capability.

---

## Quantified Results

| Task Set | Baseline | Improved |
|----------|----------|----------|
| WebArena — OpenStreetMap | 30% | 46% (+16pp) |
| WebArena — ShoppingAdmin | 24% | 31% (+7pp) |

These gains came from targeted fixes to three root causes: the looping fix, the environment error fix, and prompted improvements to instruction adherence. No model change. No architecture change.

---

## The Four Axes of Agent Improvement

Beyond the immediate findings, the analysis suggested a broader framework:

**Capability of base models.** Stronger models generalise better to novel tasks. The improvement from GPT-4 to Claude 3.5 Sonnet on WebArena is larger than most algorithmic improvements — the bitter lesson is real.

**Environment-agent interface.** Web interfaces are designed for humans. Accessibility trees, JavaScript-heavy rendering, and consent banners all create friction that well-designed agent tools can reduce.

**Algorithmic improvements.** ReAct, planning-before-execution, and reflection loops improve agent performance on complex multi-step tasks by providing structure for the model's reasoning.

**Error detection and recovery.** Agents that can detect they're in a failure state — looping, hallucinating, or receiving unexpected page content — and recover from it without human intervention are substantially more capable in practice than those that don't.

---

## The Broader Lesson

The most consistent finding from the trace analysis: agent failures are not random. They cluster into patterns, and those patterns point at specific, fixable causes. The hard part is building the tooling to see the patterns — which requires logging every trace, building a way to navigate those traces efficiently, and developing the intuition to recognise failure signatures.

Invariant's tools (Explorer, the Testing library) were designed specifically to make this analysis tractable at scale.

---

## References

- Invariant Labs original post: https://invariantlabs.ai/blog/what-we-learned-from-analyzing-web-agents.html
- WebArena benchmark: https://webarena.dev
- Invariant Explorer: https://invariantlabs.ai/blog/explorer.html
- StepAgent (SteP) agent framework reference in results
