# LinkedIn knowledge posts — Genie

Two posts. Each teaches a concept first, then uses Genie as the worked example. Designed to start conversations, not pitch a project.

---

## Post 1 — Architecture focus (~2,100 chars)

**Why single-LLM "AI assistants" don't survive production — and what multi-agent architecture actually looks like.**

Most AI assistants today are one prompt-stuffed LLM call wrapped in an API. It demos beautifully. It dies in production. Here's the pattern I keep seeing fail, and the pattern that holds up.

**Why the monolith breaks:**

→ You cannot audit a 4,000-token mega-prompt. Compliance teams ask "what does it do?" and the answer is "depends on the input"
→ You cannot upgrade one capability without re-testing all of them
→ You cannot rate-limit, cache, or budget different operations differently
→ Failure modes are all-or-nothing — the LLM either works or the whole system is down
→ Latency is the slowest path through a single giant prompt

**What multi-agent architecture actually means (the load-bearing parts):**

1. **Protocol** — a typed message envelope (`from`, `to`, `type`, `payload`, `classification`, `metadata`). Not "function calls" — *messages*. This is what makes the system inspectable
2. **Registry** — agents declare capabilities; the orchestrator discovers them. No hard-coded wiring
3. **Bus** — pub/sub transport so agents are decoupled. Start in-memory, swap for Kafka/NATS when you scale
4. **Orchestrator** — subscribes agents to message types, enforces policy *before* dispatch, traces every hop
5. **Governance as middleware** — every message passes through a composite policy *before* any agent runs. Not after. Not "we'll add it later"
6. **Per-agent risk class** — `RiskLow` / `RiskMedium` / `RiskHigh` declared on the agent itself. The orchestrator enforces ceilings
7. **Fallback agents** — every primary has a deterministic degraded-mode handler. The LLM going down is not an outage

This is Microsoft's **Multi-Agent Reference Architecture (MARA)** in one paragraph. It's what production-grade multi-agent systems converge on.

I built **Genie** as a Go reference implementation — 40+ specialist agents (ingestor → normalizer → enricher → analyzer → forecaster → anomaly → recommender → reporter, plus fraud, AML, VaR, tax, lending specialists). Every message carries W3C `traceparent` so async hops show up as one OpenTelemetry trace in Tempo. Every LLM call is wrapped in cost/cache/budget/circuit/deadline.

The pattern matters more than the implementation. If you're building anything serious with LLMs, start with the protocol and the bus — not the prompt.

Reference: https://microsoft.github.io/multi-agent-reference-architecture/
Worked example: https://github.com/c2siorg/genie

What's the biggest production-readiness gap you've hit with single-LLM systems? Curious what others are seeing.

#MultiAgent #AIArchitecture #LLM #SystemDesign #Golang #MARA #OpenTelemetry #DistributedSystems

---

## Post 2 — Compliance focus (~2,200 chars)

**The RBI FREE-AI report has 26 recommendations. Here's how 7 of them translate from policy text to actual code.**

In August 2025, the RBI published the Framework for Responsible and Ethical Enablement of AI — the most concrete AI guidance any major central bank has produced. 9 months in, the gap between "we read the report" and "we implement the report" is wide. Here's what the implementation actually looks like for the recommendations that matter most.

**Rec 4 — Indigenous AI Models.** Every LLM provider declares a `Region()`. A `DataResidencyPolicy` denies any PII-classified message from leaving the home region. On-prem Ollama for hot path (anything touching PAN/account/transaction), hosted frontier model for cold path (macro research, generic education). The router is a config choice, not a code change.

**Rec 14 — Board-Approved Policy.** YAML in version control with a `board_approved_on` field. Loaded at boot. The board edits a file; the running system obeys it. Every load is hash-logged so an auditor can prove which version was active on which date. No more "the policy is a Word doc on SharePoint."

**Rec 15 — Data Lifecycle Governance.** Envelope encryption: fresh AES-256-GCM DEK per document, wrapped by a KEK held in KMS. Raw KEK never touches application memory in prod. `expires_at` columns + a retention job that purges every 6h. The lifecycle is a column, a job, and a key resolver — not a slide.

**Rec 20 — Red Teaming.** An adversarial probe corpus runs against the *active* composite policy on every commit. Not a mocked policy. The exact bytes production is running. If a guardrail breaks, CI fails.

**Rec 21 — BCP for AI.** Every agent has a deterministic fallback. When the LLM circuits-open or the network partitions, the user gets a degraded but truthful answer — not a 500. A CI drill forces a failure and verifies the fallback fires.

**Rec 22 — AI Incident Reporting.** The Annexure VI form auto-generates on policy denial, agent panic, budget breach, or safety scorer trip. Structured, not free text. Hash-chained into a tamper-evident audit log. The reporting timeline becomes a query.

**Rec 25 — Disclosures.** A public unauthenticated endpoint returns the active policy version, the LLM providers in use, the residency posture, and the data classifications handled. This is what should live on every RE's website.

**The pattern across all 26:** responsible AI is a property of the running system, not a paragraph in a policy document. Every claim needs a `grep`-able file path, a test that fails when it breaks, and a trace that proves it ran.

I open-sourced a Go reference implementation called **Genie** that maps each recommendation to a specific package, so teams building under FREE-AI have something concrete to start from or argue with.

→ Report: https://rbidocs.rbi.org.in/rdocs/PublicationReport/Pdfs/FREEAIR130820250A24FF2D4578453F824C72ED9F5D5851.PDF
→ Code: https://github.com/c2siorg/genie

If you're building AI in Indian banking — at an RE, fintech, SRO, or regulator — which recommendations are hardest to implement at your shop? Let's compare notes.

#RBI #FREEAI #ResponsibleAI #SovereignAI #FinTechIndia #BankingCompliance #AIGovernance #OpenSource
