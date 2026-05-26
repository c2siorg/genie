# Agentic Security in Production: The Operational Playbook

*Twelve months of running a 60+ agent financial assistant. Dashboards, runbooks, drift detection, secret rotation, and the day-to-day rituals that decide whether your architecture survives contact with reality.*

---

## The shift from "is it secure?" to "is it still secure?"

Most security writing about agentic AI stops at deployment day. The architecture diagrams are drawn, the policy YAML is signed by the board, the red-team probe corpus is green in CI, the LLM provider has been benchmarked. Someone runs `make up`, the smoke test passes, and the article ends.

That is where the actual work starts.

I've spent the last 12 months running Genie — an open-source, RBI FREE-AI–aligned multi-agent financial assistant written in Go, with 60+ specialist agents — through enough production-like iteration to learn that *securing an agentic system on day one is roughly 30% of the job*. The other 70% is operating it: noticing when something has changed, knowing what to do at 02:17 IST when an alert fires, proving to a regulator six months later that you noticed and acted, and ensuring the safety net you built on day one is still there on day 300.

I wrote a companion piece — [The Complete Security Playbook for Production Agentic AI](linkedin-article-security-complete.md) — covering the architectural patterns: identity, residency, governance at the message bus, resilience wrappers, hash-chained audit logs, the nine attack surfaces. That article answers *"what do I build?"*. This one answers the question that comes after: *"now that I've built it, what do I actually do every day?"*

If the architecture piece was the blueprint, this is the building-maintenance manual. SLIs, dashboards, runbooks, drift detection, rotation playbooks, continuous adversarial testing, compliance posture as code. The unglamorous, indispensable work.

---

## The operational SLIs that matter

A microservice has SLIs you know cold: request rate, error rate, latency, saturation. An agentic system has those, plus a layer that traditional SRE training does not prepare you for. The metrics below are the ones I watch every single day. If any of them moves, I want to know within minutes.

### 1. Governance denial rate (per policy, per agent, per user)

Genie's `pkg/governance` composite stack runs about ten policies on every bus hop — `RBACPolicy`, `ClassificationPolicy`, `DataResidencyPolicy`, `ConsentPolicy`, `PIIBlockPolicy`, `PromptInjectionPolicy`, and the rest. Every denial is a counter increment on `genie.governance.denials` in `pkg/observability/metrics.go`, tagged with policy name, agent id, and user id (pseudonymised).

What I watch:

- **By policy**: each one has a normal hum. `PIIBlockPolicy` denies a small steady trickle. `PromptInjectionPolicy` should be near zero. A spike on any single policy is the first sign of a probe campaign, a buggy upstream, or a prompt regression.
- **By agent**: if `kyc_orchestrator` starts denying messages it never denied before, either the upstream changed shape or somebody shipped a prompt change.
- **By user**: a single user accounting for 80% of injection denials is a security researcher or an attacker. Either way, you want to know.

In the first six months I had the *aggregate* denial rate but no breakdowns. When denials spiked, I knew something had broken but spent an hour in traces finding what. The per-dimension breakdown was the highest-leverage observability investment we made.

### 2. Incident grade distribution

Every guardrail firing in Genie produces a structured incident payload — Annexure VI–shaped, written through `pkg/incidents` and routed through `pkg/incidents.Grade()` (a pure deterministic function in `pkg/incidents/grading.go`) into the hash-chained audit log at `pkg/compliance/audit.go`.

The daily SLI: % of incidents by severity (Informational, Low, Medium, High, Critical). Healthy distribution skews heavily toward Informational and Low. The shape of the curve is the SLI; absolute counts are noise. If the High percentage doubles week-over-week, that's the alert.

Watch the *first-offence flag*. `Grade()` returns `IsFirstOffense` based on a 30-day lookback. A High incident that is a first-offence for that failure mode is different from the fourth one this week.

### 3. Fallback firing rate (per agent — should be near zero)

Every primary agent that touches an LLM has a paired fallback in `agents/fallback/`. The fallback firing rate is a leading indicator of LLM degradation that bypasses your normal latency dashboards — by the time latency is bad, fallbacks are already firing.

Healthy baseline: ~0 fires per agent per day under steady-state traffic. A single fire is interesting. Five fires in an hour is an LLM outage in progress. The alert fires *before* customers feel degradation, because the fallback gives them a useful answer ("here's your last cached portfolio snapshot from 14:00 IST") instead of a 500.

Track per agent, not in aggregate. An aggregate fallback rate of 0.1% means nothing if `portfolio_advisor` is at 5% and the rest are at zero.

### 4. LLM circuit-breaker openings

`pkg/llm/circuit.go` opens after N consecutive errors (default 5). Every open is a metric. Every half-open recovery is a metric. The *open* event tells you when degradation started; the *recover* event tells you when it stopped. The difference is the impact window, which is what goes into the post-mortem and (if it crosses the threshold) the regulator notification.

A circuit that opens-recovers-opens-recovers in a 10-minute window is a flap. Worse than a sustained open — sustained opens get noticed; flaps slip past.

### 5. Token-exchange failure rate

Once OAuth2 token exchange (RFC 8693) ships — Genie's next-quarter item, but every team running 5+ MCP servers lands on it — the token-exchange endpoint is a critical dependency. Failures fall into three buckets: IdP unreachable, token invalid (underlying credential expired or revoked), policy denied (actor-subject combination not permitted). Each bucket has a different runbook. Don't conflate them.

### 6. Red-team probe pass rate (should always be 100%)

Genie ships `make red-team` (the `cmd/red-team` binary) which runs the adversarial probe corpus against the active composite policy. Every PR. If a single probe slips through, the build is red and the PR cannot merge.

The SLI for the running system: the *production policy YAML* should always pass 100% of the corpus. Not 99%. Not "all but two known issues." 100%, with the corpus being a strict superset of the historical attack patterns we've seen.

The corpus only ever grows. Anything else is regression.

### 7. Time-to-detect for new attack patterns

When a novel injection pattern shows up in the wild — not in your corpus, not in your test suite — how long does it take you to notice? The honest answer for most teams is "as long as it takes for a customer to complain." That's days. The target is hours. The way to hit hours: a daily sample of the top N denied messages (with PII redacted) lands in a queue that a human reviews. Patterns the existing policies aren't catching surface there. Add to the corpus. Re-deploy the policy. The loop closes in the same week.

### 8. Mean time to revoke (when a credential leaks)

From the moment you know a JWT, KEK, MCP token, or service-account credential is compromised, how long until it is unusable in production?

The architecture answer is "short TTLs, rotation is automatic." The operational answer is "do you know what the actual MTTR is, and have you measured it under load?" Genie's `pkg/auth/jwt.go` defaults to 60-minute TTLs. KEK rotation in `pkg/crypto` happens per-row via the `kek_id` field on the `EncryptedPayload`. Both architectures support fast rotation. But "supports rotation" and "has rotated, with a known MTTR, in the last 90 days" are very different claims.

Pick a target. Mine: under 15 minutes from compromise-known to credential-unusable across the fleet. Test it quarterly with a drill.

---

## The five dashboards every team needs

Pages of metrics with no named view is the same as no metrics. Below are the five named dashboards I keep open across two monitors. Each one answers one operational question, fast.

### Dashboard 1 — Live governance posture

*What policies are firing right now, on what traffic, for what reason?*

A single-page view fed by `genie.governance.denials` counters, tagged with policy, agent, and reason. Time-series for the last 60 minutes. Stacked-bar by policy. The reason field for the top 20 denials in the last hour, sorted by frequency. Pseudonymised user IDs in a separate panel for the top denied principals.

This is the dashboard you look at *first* when anything feels off. It tells you in 30 seconds whether the problem is in the policy layer or elsewhere.

### Dashboard 2 — Incident funnel

*Annexure VI–shaped, severity-graded, with disclosure status.*

Funnel from raw policy denial → graded incident → notified to customer → notified to regulator (where applicable). Drop-offs in the funnel are bugs in the disclosure pipeline. A medium-grade incident that never got a customer notification is a compliance gap, even if the technical resolution was clean.

Filterable by severity, by system (`kyc_orchestrator`, `payment_orchestrator`, etc.), by failure mode, by 7/30/90-day window. The same query the regulator will run is the query that builds this dashboard. Build it once; it earns its keep every quarter.

### Dashboard 3 — Agent health

*Per-agent error rate, fallback rate, p99 latency, panic count.*

One row per agent (we run 60+; the row height is small but every row matters). The columns are non-negotiable:

- Error rate (5-minute window, rolling)
- Fallback firing rate (15-minute window)
- p99 latency on `genie.agent.handle_duration_ms`
- Panic count (zero is the only acceptable value; any non-zero number is a P1)
- Messages handled (so you know if an agent has gone *quiet*, which is its own problem)

A quiet agent is suspicious. If `dividend_planner` normally handles 200 messages a day and handled 3 yesterday, either the routing changed or the agent silently crashed at startup. Both are bugs.

### Dashboard 4 — Identity flows

*Token issuance, exchange, revocation.*

JWT issuance rate, average TTL of issued tokens, count of refresh failures, OAuth device-flow polls (the Genie device flow lives in `pkg/auth/oauth_device`). Once token exchange ships: exchange request rate, exchange failure rate (by reason), exchanged-token cache hit rate.

Revocations are the most important panel. Every event has a reason: logout, admin revocation, compromise response, TTL expiry. If "compromise response" spikes, you have a security event in progress.

### Dashboard 5 — Adversarial verification

*Red-team CI history, BCP drill history, posture drift.*

A timeline. Every PR's `make red-team` result. Every nightly `make bcp-drill` result. Every change to the policy YAML hash (exposed on `/v1/disclosures`). The corpus size over time (it should monotonically grow). The most recent external penetration test's verdict.

The point of this dashboard isn't real-time response; it's *trend*. A policy hash that changes weekly is healthy iteration. One that hasn't changed in six months is policy rot.

---

## Incident response runbook for agentic systems

Microservice incident response is a solved problem — pull the on-call playbook, identify the failing service, scale or roll back, post-mortem. Agentic incidents are different in three specific ways: the failure can be in the policy layer (which microservice playbooks don't cover), the failure can be in the LLM (which is a third-party black box), and the failure can be *nondeterministic* (which makes reproduction hard).

Below is what we actually do, broken down by the first 5 minutes, 15 minutes, 1 hour, and 1 day.

### First 5 minutes — query the structured incident log

The first move is *not* to grep logs. The first move is to query `/v1/incidents` (or the underlying audit table, whichever your team has access to faster). The structured payload tells you:

- What fired (which policy, which agent, which failure mode)
- What user (pseudonymised)
- What classification (PII? Public? Internal?)
- What severity grade
- What `IsFirstOffense` says — is this a one-off or a pattern?

Five minutes is enough to know: is this a single user under attack, is this a campaign across users, is this a policy regression that shipped 90 minutes ago, or is this an upstream provider degrading?

The architecture article describes *why* incidents are structured at the source. The operational payoff is here: the first 5 minutes of triage are a query, not a grep.

### First 15 minutes — isolate

Isolation in an agentic system has more knobs than in microservices:

- **Revoke tokens** for the affected user (or principal class). The `pkg/auth/jwt.go` module supports revocation through the JTI denylist; the operational question is whether you've ever actually exercised the denylist under load. (See the secret rotation section.)
- **Drain the affected agent from the registry**. Genie's `pkg/registry/registry.go` is the source of truth for which agents are accepting messages. A drain is a registry update; the orchestrator will route around the drained agent. Far better than killing the pod (which leaves in-flight messages dangling).
- **Tighten the policy** — temporarily lower the relevant denial thresholds. Policy YAML reload is hot; the active hash will change on the dashboard, and the change will be in the audit log. (Do this from a board-approved emergency-procedures file, not by editing the live config. The point of policy-as-code is that emergencies don't bypass the review trail.)
- **Open the circuit** on the affected LLM provider. `pkg/llm/circuit.go` exposes a manual trip; the system will fall back to deterministic agents for the duration.

The 15-minute target: the active blast radius is bounded. Customer-facing degradation is the fallback agent's message, not a 500.

### First hour — trace replay via OTel

Now the forensics work begins. Genie emits OTel traces with the trace ID flowing through `msg.Metadata["trace_id"]` from the HTTP request through every bus hop. You can pull a single trace and see the full path: which agents handled the message, which policies evaluated, which LLM calls fired, with what budget and what circuit state.

The differentiator from microservices: *the LLM call is nondeterministic*. The same prompt at 10:00 and at 10:01 produces different completions. Reproducing the failure path requires capturing not just the input but the *exact* LLM response that triggered the misbehaviour.

The operational discipline: in incident-response mode, the LLM client logs the full request and response (with PII redacted via `pkg/governance/pii.go`) into a side channel. Expensive in storage, so it's off by default; the on-call's first move when escalating is to turn it on for the affected agent. The replay then has bounded LLM inputs — same prompt, same model, same temperature, recorded completion — and you can reproduce the path deterministically. A failing microservice is a function of its inputs; replay is trivial. A failing agent is a function of inputs *and* a stochastic completion; you have to capture both.

### First day — post-incident: close the loop

The post-mortem produces three artefacts, every time, without exception:

1. **A new probe in the red-team corpus** (`cmd/red-team/main.go`). Whatever the attack or failure mode was, the next PR cannot regress on it. Without this step, the same incident recurs in three months.
2. **A policy update**. Either a new rule in the DSL (`pkg/policy/dsl`), a tightening of an existing rule, or a new agent risk class. The change goes through the same board approval as any policy change — emergencies are not an excuse to bypass the review trail.
3. **A runbook diff**. Whatever was missing from the runbook that made the response slower than necessary gets added. Runbooks are living documents; the diff is the artefact.

The loop is closed when the next quarterly drill exercises the new runbook entry.

---

## Posture drift and how to detect it

Architecture is a snapshot. Posture is a function of time. Drift detection is the discipline that says: *what is true today that was not true on the day we last verified?*

Four drift signals I monitor, with the alert that should fire for each.

### Signal 1 — Policy YAML hash changed without a deployment

The active policy YAML hash is published on `GET /v1/disclosures`. The deployment pipeline knows when it shipped a new policy. The intersection is the alert: *hash changed AND no deployment in the last hour AND no change ticket in the queue*. That intersection is exactly zero in healthy operation. Anything else is an investigation.

The investigation: somebody edited the policy file on a running host (which they shouldn't be able to do), the file was corrupted, or the deployment pipeline is broken in a way that ships changes outside the formal release process. All three are bad. The alert is what surfaces them.

### Signal 2 — Agent risk class lowered without a governance review

Every agent in the `pkg/registry` declares a risk class. Lowering an agent's risk class — say, from `high` to `medium` — relaxes the policies that apply to it. That change should be infrequent and reviewed.

The alert: *risk class lowered on any agent AND no governance ticket linked AND no PR with the appropriate approvers*. The lowering itself is fine in principle; the lack of a paper trail is the bug.

### Signal 3 — A new dependency landed in go.mod

Supply chain is part of the security boundary. Every PR that modifies `go.mod` — added, removed, version bumped — triggers a supply-chain review. A CI job runs `go list -m -json all` against a known-good baseline and surfaces deltas to a human reviewer. Not glamorous. Prevents the next `event-stream` or `xz-utils` from landing because somebody bumped a transitive dependency without thinking about it.

### Signal 4 — The red-team probe corpus shrank

This one took me by surprise. Around month 8, a refactor moved test fixtures around, and a handful of probes accidentally moved out of the corpus path. The CI was still green, because the smaller corpus still passed. The bug was that the corpus was smaller than yesterday.

The alert: *corpus size today < corpus size yesterday*. Strict greater-than-or-equal-to is the invariant. Generalise the lesson: *anything that should only grow needs an alert when it shrinks.* Audit log row count. Agent count in the registry. Probe corpus. The list of policies in the composite. Each, alarmed on shrink.

---

## Continuous adversarial testing

Architectures decay because adversaries don't stop iterating. Adversarial testing is how you keep up.

### Red team in CI on every PR

Genie's `make red-team` runs `cmd/red-team/main.go` against the board-approved policy YAML on every PR. The corpus today covers prompt injection, PII patterns (account-shaped digit runs, emails, Aadhaar), residency violations (PII bound for a non-home region), classification escalations, and schema violations. Every probe asserts an expected denial reason; a probe that gets denied with the *wrong* reason still fails the build.

The corpus is the single most leveraged file in the repo. Probes added there protect every future PR.

### Quarterly external penetration test against the running policy

Internal red teaming finds the patterns you thought of. External penetration testing finds the patterns you didn't.

Every quarter, an external firm gets a sandbox deployment with the production policy loaded and a charter: "find a way to make Genie do something it shouldn't." Every working pattern becomes a probe in the corpus. Every failed pattern is also added (negative test — the policy should *continue* to deny these). Cost: real money. Value: the next attacker is going to try the same thing, and now you'll catch them.

### Bug bounty considerations for AI systems

AI systems have a fuzzier perimeter than HTTP APIs. Scope language matters more than usual:

- **In scope**: prompt injection that escalates privilege. Jailbreaking the model to bypass governance. Exfiltrating the audit log via a crafted query. Causing misclassification (PII as public, or vice versa). Triggering a fallback that leaks state. Defeating the residency policy. Forging an Annexure VI incident payload.
- **Out of scope**: making the model say a rude word. Hallucinations without a downstream effect. Social-engineering a human operator (separate program). DoS via cost-overrun (we have budget caps and accept the risk).

Researchers follow the scope you write. Vague scope produces vague reports.

---

## Secret rotation under production load

Architecture supports rotation. Operations *performs* rotation. The gap between the two is where most breaches actually happen.

### JWT secret rotation with a dual-secret window

The naive rotation: change `GENIE_JWT_SECRET`, restart. Every in-flight session is now invalid. Customer support gets a wave of "I got logged out" tickets.

The right pattern: dual-secret window. The verifier accepts both old and new for one full TTL (60 minutes default). The issuer signs only with the new one starting at T+0. At T+60, the old secret is removed. Zero customer-visible impact. The discipline: the rotation runbook says T+60 *exactly*. Not "next deploy." A stale dual-secret window extends the compromise window if the old secret was rotated because it leaked.

### KEK rotation via per-row kek_id

`pkg/crypto/envelope.go` stamps every `EncryptedPayload` with the `KEKID` that wrapped its DEK. New KEK rolls in via the `KeyResolver`'s `ActiveKEKID()`; the resolver knows how to `Unwrap` payloads stamped with any previously-active KEK ID. Old data is readable; new data uses the new key. A background job re-encrypts old data lazily.

The nuance: the resolver's list of historical KEKs is itself sensitive. It must live in the same KMS as the active KEK, with the same access controls. A common anti-pattern is keeping "old keys" in a less-protected store because "they're old." Old keys decrypt old data. They're often *more* sensitive, not less.

### Token-exchange caching invalidation

Once you cache exchanged tokens (keyed by `(user_subject, agent_identity, audience)`, TTL bounded by the JWT `exp` claim), the cache becomes a rotation surface. A revoked token must be evicted, not just denylisted at the IdP. Every revocation event publishes to a topic; every cache instance subscribes and evicts. The MTTR target ("under 15 minutes") is set by this loop's slowest segment.

### The "we can't rotate the secret because three services hardcoded it" anti-pattern

You will encounter this on the first attempted rotation. You change the secret, three things break, you change it back, you file a ticket to "fix it later," later doesn't happen.

The fix is not "fix the hardcoded references." The fix is to make the secret-loading code *fail loudly* on startup if the secret matches a known-bad value (the README example, common defaults, the empty string). Genie does this check in `cmd/api/main.go` — the process refuses to start with `GENIE_JWT_SECRET` set to anything that smells like a placeholder. Catches hardcoding at deploy time, when fixing it is cheap, instead of at rotation time, when fixing it is expensive.

---

## Compliance posture as code, not as document

The compliance officer's nightmare is a slide deck that claims "we encrypt at rest" while the actual system has three exceptions nobody documented. The fix is to make every compliance claim *queryable from the running system*.

### The active policy YAML hash is in /v1/disclosures

`GET /v1/disclosures` is public, unauthenticated. The response includes the policy version and the policy approval date. Tampering with the policy file in production changes the hash; the change is visible on a public URL. Anyone — the regulator, an auditor, a curious customer — can verify that the policy in production today is the policy that was approved.

Tampering is detectable from outside the system. That is a different posture than "tampering is logged internally and we'll notice."

### The agent inventory is built from the live registry

`GET /v1/ai-inventory` (admin-authenticated) is built by iterating `registry.List(ctx)` in `pkg/registry/registry.go`. Every agent that's actually running shows up. Every agent that's *not* running doesn't. The inventory cannot drift from the deployment, because it *is* the deployment.

A spreadsheet of agents is a document. The registry is the system. They cannot disagree.

### The audit log is hash-chained

`pkg/compliance/audit.go` implements an `AuditEntry` with `PrevHash` and `RowHash` (both hex SHA-256). The `Verify` method walks the chain and fails if any row's hash is inconsistent.

Tampering with a historical entry breaks the chain at every subsequent entry. The next verification pass detects it. Anchoring the chain periodically to an external timestamp (S3 + Object Lock, a notary service, etc.) extends the integrity guarantee to "an external party can verify."

### Quarterly attestation: export, verify, sign, send

The compliance ritual that closes the loop:

1. Export the audit log for the quarter as JSONL.
2. Run the `Verify` walk; the chain must verify clean.
3. Sign the export with a key the compliance officer holds.
4. Send the signed JSONL to the regulator (or hold it ready for when they ask).

The export is reproducible. The signature ties the export to a specific moment. The regulator can verify the chain independently. The whole loop is mechanical — no judgement calls, no spreadsheet reconciliation.

Compliance as code means the compliance work is the *export*, not the *narrative*. The narrative is whatever the regulator wants to read; the export is what's true.

---

## Five operational lessons learned

After twelve months, the lessons taped to the wall.

### 1. The dashboard you don't look at every day fails silently

I built a beautiful identity-flows dashboard in month 3. Looked at it twice a week, then stopped. In month 7 a data-source migration moved a metric and the dashboard had been showing flat zeros for ten days. I didn't notice because I wasn't looking.

The fix is not discipline. The fix is to wire an alert to *the dashboard query itself*. If a panel goes flat-zero or null for 15 minutes, an alert fires. The dashboard monitors itself.

### 2. The drill that doesn't run in CI rots in six months

`make bcp-drill` runs on every PR. A BCP drill that runs on every PR cannot rot — any refactor that breaks it makes the build red. A drill that runs "quarterly when somebody remembers" rots in a single quarter. Same for `make red-team`, the policy YAML hash check, the audit-chain `Verify` walk. Any safety mechanism not continuously exercised has a half-life of about six months.

### 3. The metric without an alert is a graveyard

Ten thousand metrics and zero alerts is not operational signal. Useful metrics have alerts. Alerts without useful metrics are noise. For every SLI above I have a corresponding alert with a defined threshold and a defined runbook. If I can't write the runbook, I shouldn't have the alert; if I shouldn't have the alert, I shouldn't be tracking the metric.

### 4. The runbook nobody has practiced is fiction

The runbook says "drain the affected agent from the registry." Has anyone on the team actually done that? Under load? At 02:17 IST with four other things on fire?

Quarterly fire-drills exercise every step. We pick a Tuesday afternoon, simulate a credential leak, run the response. The first three drills are awful and exposing. The fourth is smooth. The fifth finds something the first four didn't. The runbook is fiction until it's been practiced.

### 5. The handover that doesn't include the threat model leaks knowledge

Onboarding usually covers the codebase, the deployment pipeline, the on-call rotation. It usually does *not* cover the threat model, the historical incidents, the patterns we added to the corpus because of past attacks. That gap is how institutional security knowledge leaks: the new engineer ships a feature that re-introduces a pattern we explicitly blocked nine months ago, and the reviewer doesn't catch it because the reviewer wasn't on the team nine months ago either. Fix: a written threat model updated with every significant incident, in the onboarding pack, re-read by the whole team annually.

---

## What's next on the operational roadmap

Five concrete items on the next-quarter operational board:

1. **Self-monitoring dashboards** — every panel wired to an alert that fires if the panel goes flat-zero or null for 15 minutes. Eliminates the silent-dashboard failure mode.
2. **Automated quarterly attestation export** — `make attest` exports the quarter's audit log, runs `Verify`, signs the JSONL, and produces a regulator-ready bundle. Removes the "I forgot to run step 4" failure mode in the current manual ritual.
3. **Corpus expansion via traffic sampling** — daily sampler over denied messages, top-N patterns to a human reviewer, frictionless path from "novel pattern" to "probe in corpus." Closes the time-to-detect loop.
4. **Drill scheduler** — calendar integration that picks a random Tuesday each month and creates a drill ticket. We cannot accidentally skip a quarter.
5. **Per-agent SLO dashboards in Grafana** — every agent in the 60+ inventory gets a SLO panel (error rate, fallback rate, p99, quiet-agent alert). Full coverage so a quiet agent in the long tail can't hide.

Each is a gap operational reality surfaced. Each is being shipped because the cost of not shipping it is "you'll find out the bad way."

---

## The repo + a question

Everything above runs on the open-source Genie codebase: [github.com/c2siorg/genie](https://github.com/c2siorg/genie). The operational primitives:

- **Structured incidents** — `pkg/incidents/` with `Grade()` in `pkg/incidents/grading.go`
- **Hash-chained audit log** — `pkg/compliance/audit.go` with `Verify`
- **Composite governance policy** — `pkg/governance/` (~10 policies)
- **Policy DSL for board-authored rules** — `pkg/policy/dsl/`
- **Fallback agents** — `agents/fallback/`
- **Resilience wrappers** — `pkg/llm/` (deadline, circuit, budget, cache)
- **OTel-instrumented metrics** — `pkg/observability/metrics.go`
- **Envelope encryption with per-row KEK ID** — `pkg/crypto/envelope.go`
- **Adversarial probe corpus** — `cmd/red-team/main.go`, runnable via `make red-team`
- **BCP drill** — runnable via `make bcp-drill`

Public posture endpoints once a deployment is live:

```bash
curl https://your-deploy/v1/disclosures      # public, hash of active policy
curl -H "Authorization: Bearer $A" /v1/ai-inventory  # live registry-built inventory
curl -H "Authorization: Bearer $A" /v1/incidents?limit=20  # Annexure VI–shaped
```

The architectural companion piece is [The Complete Security Playbook for Production Agentic AI](linkedin-article-security-complete.md) — covering the patterns this article assumes have already shipped. Read it first if you're at day zero. Read this one if you're past day one and wondering what comes next.

---

A question for the practitioners reading: of the operational rituals above — the drills, the rotation runbooks, the drift alerts, the dashboard self-monitoring — which one took you longest to land in your team's muscle memory? For us it was the BCP drill in CI; the team initially resisted "another thing that can break the build," until the first time the drill caught a fallback regression that would have been an outage. Always curious how other teams converged on this.

#SecurityOperations #ResponsibleAI #SOC #IncidentResponse #RBI #FREEAI #BankingAI #FinTechIndia #ThreatHunting
