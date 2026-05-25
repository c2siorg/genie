# BCP for AI: Forced-Failure Drills and Deterministic Fallbacks

*Why your uptime shouldn't be hostage to OpenAI's status page — and how to prove it isn't.*

---

## The 3-AM page nobody wants

It's 3 AM. Your phone buzzes. The on-call dashboard is red. Your AI-driven portfolio advisor is returning 500s. Customers are complaining.

You check the dependency. Your LLM provider is having an incident — three regions affected, "we are investigating." Last week's outage took 90 minutes; the week before it was 4 hours. You're powerless until they're back.

This is the failure mode every team that ships AI features eventually meets. Your product's uptime is now the union of your uptime and your AI vendor's uptime. The vendor's promise is "four nines"; the math says your effective uptime is closer to three.

The RBI FREE-AI report calls this out in **Recommendation 21 (Business Continuity Planning for AI)**: *Regulated Entities shall ensure continuity of customer-facing services when AI systems fail.*

The recommendation doesn't tell you how. This article does.

---

## Two patterns, neither of which work

### Pattern 1: "Use a second LLM provider"

Sounds reasonable. In practice:

- Both providers fail at the same time more often than you'd think (correlated cloud outages, shared infrastructure, rate limits during simultaneous traffic spikes).
- The second provider has a different prompt format and different response shape; you ship a second integration that bit-rots between incidents.
- Cost doubles.
- You still don't have a story for "what if all LLM providers are down" (Cloudflare, network, your egress firewall, etc.).

### Pattern 2: "Cache the last good response"

Also reasonable-sounding. In practice:

- The user's question is different from the cached one.
- The cached response is stale — last month's portfolio snapshot is misleading.
- Customers expect a fresh answer; "here's what we told someone else last week" isn't it.

Neither pattern solves the problem because both assume the answer has to come from an LLM. **The actual answer is to compose with a non-LLM fallback that's deterministic, fast, and truthful.**

---

## The fallback agent pattern

For every primary agent that depends on an LLM, register a paired **fallback agent** that doesn't.

```go
orchestrator.SetFallback("portfolio_advisor", deterministic.PortfolioFallback{})
orchestrator.SetFallback("recommender", deterministic.RecommenderFallback{})
orchestrator.SetFallback("educator", deterministic.EducatorFallback{})
```

When the primary times out, circuit-breaks, or panics, the orchestrator dispatches the fallback. The fallback:

- **Doesn't call the LLM.** No external dependency.
- **Doesn't make network calls.** Whatever data it needs is already in the local stores (Postgres, in-memory cache).
- **Is deterministic.** Pure function over the cached state. Same input, same output.
- **Is truthful.** It doesn't fabricate. It says what it knows.

A real example. Genie's `portfolio_advisor` calls Zerodha Kite via MCP for live holdings. When MCP times out, the fallback returns:

> *"Your live portfolio analytics are temporarily unavailable. Here's your last successfully fetched snapshot, from 14:00 IST today, with positions as of yesterday's close. The detailed analytics view will resume when our market data feed is restored. Estimated time: ~30 minutes."*

That's a useful answer. It tells the user:

1. What they asked for is unavailable.
2. What we can show them instead (and how stale it is).
3. When we expect it to recover.

The customer doesn't get the live answer, but they don't get a 500 either. The system stays up. The on-call still gets paged — fallback firing is itself an alertable event — but customer-facing uptime is preserved.

---

## What "deterministic" means for a fallback

Three properties:

1. **No external I/O.** If the fallback also calls a flaky service, it's not a fallback — it's another point of failure.

2. **Computation bounded.** The fallback returns in milliseconds. No iterative reasoning, no retries, no clock-skewed waits.

3. **Output is truthful with caveats.** If you don't know the answer, say so. *"We can't compute this right now; here's a known-good approximation"* is acceptable. *"The answer is 42"* fabricated from thin air is not.

Most fallbacks are 20-50 lines of Go reading from a local cache. They're boring. They're load-bearing.

---

## Forced-failure drills — the part most teams skip

Building the fallback is half the work. Verifying it actually fires when the primary fails is the other half. **CI should force the primary to fail and assert that the fallback runs.**

In Genie:

```bash
make bcp-drill
```

Internally this:

1. Spins up the in-process pipeline.
2. Sends a `portfolio_request` to the supervisor.
3. **Forces the `portfolio_advisor` to panic** (via a hook the test framework wires).
4. Asserts the fallback fired within the deadline.
5. Asserts the response carried the expected disclaimer about degraded mode.
6. Asserts the incident log shows the primary failed and the fallback recovered.

If any of those assertions fail, the build fails. **The drill runs on every PR.**

Why this matters: fallback code is the kind of code that rots. It's never exercised in production until the day you really need it. By that point, six months of "minor refactors" have broken the fallback in ways nobody noticed. The drill is what keeps it alive.

---

## The three layers of resilience

A complete BCP-for-AI story has three layers, each independent:

### Layer 1: LLM call wrappers

`pkg/llm` wraps every provider in:

- `DeadlineProvider` — per-call timeout (default 30s)
- `CircuitProvider` — breaker after N consecutive errors (default 5)
- `BudgetedProvider` — daily per-principal token cap (default 1M)
- `CachedProvider` — exact-match cache by hash of (model, messages, temp)
- `ChainProvider` — try primary, fall back to secondary on error

These keep a single bad call from cascading. A timed-out call doesn't hang the request; a circuit-opened provider doesn't get re-tried for the next minute; a budget breach denies cleanly.

### Layer 2: Fallback agents

The pattern above. When the primary fails despite the LLM wrappers, the orchestrator routes to the deterministic fallback.

### Layer 3: Forced-failure drills

The CI gate. Every PR runs a `make bcp-drill` that forces the primary down and asserts the fallback fires.

Each layer covers a different failure mode:

- LLM down for one call → layer 1 catches it
- LLM consistently down for the agent → layer 2 catches it
- Layer 2 silently rotted six months ago → layer 3 catches it

Defence in depth.

---

## What the fallback can NOT do

The fallback isn't the primary's clever cousin. It's the safety net. Specifically:

- **No personalised reasoning.** "Based on your last 90 days of spending, you should..." needs the LLM. The fallback can return the most recent cached recommendation with a "stale, last computed at..." stamp.
- **No new analyses.** The fallback shows what's already in the cache; it doesn't compute new derivative views.
- **No fund movements.** The fallback for `payment_orchestrator` should refuse to process new payments. Money movement on degraded mode is a regulatory hazard. Better to tell the customer "payment service temporarily limited; try again in 5 minutes."

Be honest about what the fallback degrades. Customers handle "your X is temporarily unavailable, here's Y" much better than they handle 500s or hallucinated answers.

---

## What this earns under FREE-AI

- **Rec 21 (BCP for AI)** — directly. The pattern *is* the recommendation.
- **Rec 16 (System Governance)** — bounded autonomous loops via the LLM wrappers.
- **Rec 18 (Disclosure)** — degraded-mode responses include a disclaimer.
- **Rec 22 (Annexure VI)** — every fallback firing produces a structured incident in the log.

Four recommendations addressed by three concrete patterns + one CI gate.

---

## How to retrofit this on an existing system

If your codebase doesn't have fallback agents today, the migration is:

1. **List the failure modes.** Which LLM calls, if down, take a user-facing feature down? Prioritise these.

2. **Build the cache.** What's the last-known-good answer for this feature, and when was it computed? Cache it (Postgres, Redis, in-memory) with a staleness timestamp.

3. **Write the fallback.** A small function that reads from the cache and returns the answer with a "this is stale, refresh expected at..." disclaimer.

4. **Wire the orchestrator.** Add `SetFallback(primaryID, fallbackAgent)` at startup.

5. **Add the drill.** A test that forces the primary down (mock, kill switch, hook) and asserts the fallback fires. Run it in CI.

6. **Page on fallback firing.** Fallback fires = something's wrong. The on-call should know.

Steps 1-3 are domain work. Steps 4-5 are infrastructure. Step 6 is operational. All of them are within reach of a small team in a sprint or two.

---

## What this is worth

Three numbers, real ones, from teams who've shipped this:

- **Customer-perceived uptime** during an LLM provider's bad day: ~95 % → ~99.5 %. The 4.5 % gap is the fallback's degraded-but-truthful response.
- **3-AM pages**: from "every time a vendor incident" to "only when the vendor incident is sustained enough that the fallback also gets exercised heavily." Roughly 5x fewer.
- **Audit conversation length**: shortened by an order of magnitude. *"Yes, we have BCP for our AI systems; here's the CI gate that proves it."*

The economics work.

---

## The thesis

If your AI feature can't degrade gracefully, it's not production-ready. If your degradation path doesn't run in CI, it's not maintained. If your fallback isn't honest about what it can't do, it's worse than no fallback.

**Build the fallback. Force the failure. Page when it fires.**

That's the whole recipe. Three sentences. The work is in writing them out and putting them in version control.

---

## The repo

Genie is open source under MIT.

- `agents/fallback/` — the deterministic fallback agents
- `pkg/orchestration/orchestrator.go` — the `SetFallback` wiring
- `pkg/llm/*.go` — the wrapper chain (deadline, circuit, budget, cache, chain)
- `Makefile` — `make bcp-drill` target

```bash
git clone https://github.com/c2siorg/genie.git
make bcp-drill
# Forces portfolio_advisor failure; asserts fallback fires.
```

---

If you've shipped BCP for AI in a different shape — graceful UI degradation, queued retry, on-prem mirror — I'd genuinely like to compare. The fallback pattern works for us; there are other patterns I haven't tried.

#BusinessContinuity #ResponsibleAI #RBI #FREEAI #BankingAI #FinTechIndia #SoftwareReliability #Resilience
