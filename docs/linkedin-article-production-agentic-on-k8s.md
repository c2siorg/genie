# Field Notes from Production Agentic Apps on Kubernetes

*Reading "Generative AI on Kubernetes" Chapter 9 with a multi-agent system already in production — what matched, what we did differently, what's a real gap.*

---

## Why this article exists

I just finished Chapter 9 of *Generative AI on Kubernetes* (O'Reilly, Holzner / Roland Huss et al., 2026). It's the most practical thing I've read this year on running agentic systems in production — taxonomies for security patterns, sober treatment of MCP and A2A, honest writing about state management.

I'm running a multi-agent financial assistant called Genie. It's open source, built on Microsoft's Multi-Agent Reference Architecture, aligned with the RBI FREE-AI report, and now ships 60+ specialist agents. Reading the chapter felt like having an external code reviewer go through our architecture and ask "did you think about this?"

The honest answer was mixed. Some of the chapter's patterns we'd already implemented. Some we'd implemented differently for reasons that hold up. Some are genuine gaps in our roadmap.

This is the field-notes version. If you're operating an agentic system today, these are the parts of the chapter that should keep you up at night.

---

## Security: four patterns, not one

The chapter lays out four MCP security patterns for the question "whose identity should the upstream API see when an agent calls a tool?":

1. Agent impersonation (token passthrough)
2. Service account delegation (Kubernetes-native)
3. Delegated identity via OAuth2 token exchange (RFC 8693)
4. Mutual TLS with SPIFFE/SPIRE

The framing alone is worth the price of the chapter. Most "agentic AI security" content collapses to "use an API key." The chapter's contribution is forcing you to make the trade-off explicit: simplicity vs attribution vs zero-trust.

Here's how Genie's posture maps:

### Pattern 1 — Agent impersonation

We do this. The user's JWT (HS256, 60-minute TTL) rides through `Authorization: Bearer …` on every `/v1/ask` call. The orchestrator extracts `user_id` and `user_roles` from the JWT into `msg.Metadata`, and `governance.RBACPolicy` evaluates against the roles on every bus hop. The audit log shows the actual user, not "the agent."

What the chapter warns about — token lifetime expiry on long-running tasks, scope explosion — we sidestep mostly by being short-running. Our pipelines complete in seconds, not hours. The day we ship a multi-hour research workflow, we'll need a refresh strategy.

### Pattern 2 — Service account delegation

We're K8s-deploy-agnostic today (compose works, Kubernetes manifests are roadmap), so this pattern isn't load-bearing for us yet. We get the *effect* — workload-to-workload trust without external token servers — through application-level RBAC enforced at two layers: HTTP middleware (`pkg/web/mid.RequireRole`) and bus governance (`pkg/governance.RBACPolicy`). Defence in depth without K8s primitives.

When we ship K8s manifests, this pattern becomes the natural choice for intra-cluster trust. The chapter's distinction between **server identity** and **agent identity** (which ServiceAccount token reaches the upstream API) is the call we'll need to make explicitly.

### Pattern 3 — OAuth2 token exchange (RFC 8693)

This is a gap. We don't implement it. The use case the chapter calls out — preserving both *who* (the user) and *what* (the agent) in a single signed token — is exactly what compliance teams want for AI-driven actions in regulated industries.

For a banking use case (which is what Genie targets), "Nurse Alice's medical-assistant agent accessed Patient 4711's records" is the audit shape regulators want. We get the *user* attribution today via the JWT passthrough; we don't yet get the *agent* attribution in the same token. We get it in the OTel trace, which is a different surface.

This is going on the roadmap. The chapter sold me.

### Pattern 4 — SPIFFE/SPIRE

Also a gap, also roadmap. We use envelope AES-256-GCM with a KMS-pluggable KEK for data-at-rest, and JWT for service-to-service. The chapter's argument — bearer tokens can be stolen, SVIDs cryptographically bound to the workload can't — is correct.

For a single-binary deployment of the Genie API behind an LB, SPIFFE is overkill. For a mature K8s deployment running 20+ MCP servers and 60+ agent pods, SPIFFE/SPIRE is the right answer. The chapter's operational caveat is the honest one: it's not bearer tokens, it's a CA you operate. Treat it as critical infrastructure or don't bother.

### MCP Gateways

The chapter introduces this as the "alternative" — instead of implementing security in every MCP server, centralise it at a gateway. We don't have one. We have one MCP server (Genie itself, exposing a curated set of read-only agents via `/mcp`). At one server, the gateway is overkill.

The day we run 10+ MCP servers across business units, the gateway pattern becomes load-bearing. The chapter names Microsoft's MCP Gateway, IBM's ContextForge, Envoy AI Gateway, Solo.io's agentgateway — useful market scan. Worth re-reading before committing.

---

## Inter-agent coordination: A2A

The chapter's treatment of A2A is the cleanest I've read. The framing — *MCP is for agents calling tools; A2A is for agents calling other agents* — is the right mental model.

Genie ships `pkg/a2a` (client + server). An A2A peer can call us; we can call other A2A peers. The chapter's three core concepts map directly:

- **Agent card** — yes, we expose `agent/getCard` listing skills, input modes, output modes, protocol versions.
- **Task lifecycle** — yes, `task/submit` returns a task ID; the requester can poll. We don't yet support the streaming subscription model the chapter describes; that's a feature gap.
- **Artifact streaming** — partial. We stream SSE inside `/v1/ask/stream` for our own bus events, but the A2A streaming spec for cross-agent artifact transfer isn't in our implementation yet.

The chapter notes that **ACP merged into A2A under the Linux Foundation in August 2025**. That's the kind of detail you can't get from blog posts; it lives in the chapter and the Agentic AI Foundation (AAIF) project pages. Worth noting because some teams are still building on pre-merge ACP libraries that are now unmaintained.

Where I'd nuance the chapter slightly: it says "treat agents as tools" via MCP loses A2A's richer semantics. True. But for early-stage multi-agent systems, modeling one agent as an MCP tool to another is a fine simplification while you figure out your capability surface. The migration path to A2A is mechanical once you have agent cards drafted.

---

## State management — where the chapter's experience shows

The chapter splits agent state into **short-term** (active conversation) and **long-term** (persisted across sessions). Then it walks through:

1. In-memory (works in dev, dies on pod restart)
2. KV store like Redis with TTL (production short-term)
3. Database for long-term (queryable, durable, audit-friendly)
4. Checkpointing for long-running workflows

This taxonomy maps to almost any agent system. Our shape:

### Short-term — `pkg/memory.EpisodicMemory`

Today: in-memory rolling buffer per session, with LLM-driven summarisation when the buffer exceeds threshold. Survives across in-process restarts via the orchestrator's bus subscription, but not across pod restarts.

The chapter's call to externalise from day one is correct. Our roadmap line item: implement `EpisodicMemory` over Redis or Postgres. The chapter's specific advice — TTL keyed on session inactivity, scale horizontally because all pods hit the same store — is exactly the migration plan.

### Long-term — `pkg/memory.LongTermMemory`

We shipped this recently as an *append-only* tier — durable consolidated facts about a user that survive across sessions ("primary bank: HDFC, confidence 0.85, source: 60d txn analysis"). Updates supersede rather than overwrite, so the history is preserved for audit.

The reference implementation is in-memory. The Postgres backing is the obvious next step. The chapter's distinction between "short-term needs low-latency access" (KV) and "long-term supports analytics, personalization, and audit requirements" (DB) is exactly the split. We just need to land both halves of it in code.

### Checkpointing — `pkg/workflow`

This is where we accidentally agree with the chapter for different reasons. We built `pkg/workflow` (DAG + Saga + HITL + event-sourced log) for the **SME loan workflow** — a multi-stage process with a human-approval gate that can take hours.

The chapter's checkpoint pattern (save state after each major step; resume from latest on restart) is what our event-sourced sink gives us, because every transition is recorded. A failure mid-DAG re-reads the sink's events and resumes from the last `completed` step. The motivation was different (regulator-friendly audit trail), but the operational benefit (pod evictions don't restart the workflow) falls out for free.

If you're starting an agentic system today and you know you'll have long-running workflows, an event-sourced step log is a better foundation than a "save checkpoint file every N steps" pattern. You get debugging, audit, and resume on the same data structure.

---

## What's universal across the chapter

Three principles I'd underline regardless of which protocol or framework you pick:

### 1. Make trust structural, not behavioural

Every security pattern in the chapter pushes you toward enforcement points that can't be skipped by a forgetful caller. SPIFFE binds identity to the workload; OAuth2 token exchange centralises the actor claim; the service account is mounted automatically, not passed.

The opposite — "trust this code path because we said to" — is the failure mode. We see it in our own code when an engineer adds a new LLM call and forgets to wrap it in the policy composite. The wrapper isn't optional; the orchestrator forces it. Same idea, different layer.

### 2. Long-running != hung; checkpoint it

An agent that takes 4 hours to complete a research task isn't broken. It's normal. The chapter's checkpoint advice — save intermediate state, resume on restart — is what makes long-running agents production-grade.

The harder lesson: design the agent's intermediate state to be *inspectable*. Our SME loan workflow's `workflow.State` is a key-value map that an on-call engineer can read mid-flight. The chapter's example (`step_001.json`, `step_002.json`) has the same property. When something goes wrong at step 17, you can read step 16's output and figure out why.

### 3. State externalisation is non-negotiable

The chapter is direct: "In-memory state will fail the moment you scale horizontally or survive a pod restart." We learned this the easier way (we read the chapter before scaling), but every team I've talked to that didn't externalise from day one ended up doing a painful retrofit.

The retrofit isn't just "move dict to Redis." It's "discover all the places state implicitly leaks (closures, goroutine-local data, lazy initialisers), and migrate each one carefully." The honest cost is weeks. The honest avoidance is "Redis from day one, even if the data lives there for 30 seconds."

---

## Things the chapter wisely doesn't cover

A few things I appreciated were left out, because they belong elsewhere:

- **Specific framework comparisons.** LangGraph, CrewAI, custom — the chapter names them only enough to make a point about coordination fragmentation. The operational patterns hold regardless.
- **Cost optimisation for LLM calls.** Different book chapter, probably a different book.
- **Agent evaluation and quality metrics.** Adjacent topic; out of scope.

The discipline of "operational patterns that endure across tools and standards" is what makes the chapter age well.

---

## What I'd add for the next edition

One area where I wish the chapter had gone deeper:

**The relationship between policy-as-code (governance) and AuthZ.** The chapter covers OAuth scopes, RBAC, SubjectAccessReview, SPIFFE allowlists. All of those are *transport-level* authz. There's a layer above that — *behavioural* policy that gates which tools an agent is allowed to call, or what classifications can leave the home region, or what content patterns trip an injection check. That layer is essential in regulated industries and the chapter alludes to it (OPA / Cedar in the gateway section) but doesn't go deep.

Genie's `pkg/governance` composite stacks ~10 policies that run on every bus hop. The chapter could have a sibling section on "policy at the message layer, not just at the transport layer." Maybe in the next edition.

---

## Concrete actions I'm taking after reading

1. **Add RFC 8693 token exchange to Genie's roadmap.** Specifically: agent runtime exchanges the user's JWT for a dual-identity token (user in `sub`, agent in `act`) before calling MCP servers. The chapter's worked example is the right shape.
2. **Move EpisodicMemory to Redis or Postgres.** Move LongTermMemory to Postgres. Today's in-memory reference implementations are sufficient for the sandbox; production deployments need the durability.
3. **Re-evaluate MCP Gateway when we cross 10 MCP servers.** Today we run one; the gateway is overkill. Bookmark the chapter's market scan.
4. **Plan SPIFFE/SPIRE for the K8s deployment.** Not for the compose stack. When we ship Helm charts, SVIDs replace JWT for service-to-service.
5. **A2A streaming subscription.** Catch up our A2A server to the streaming spec, not just polling.

That's a quarter's worth of work, all of it concretely scoped because the chapter named the shapes.

---

## The book and where to find it

*Generative AI on Kubernetes* by Hendrik Roland Huss et al., O'Reilly Media, 2026. Chapter 9 is the production patterns chapter. Other chapters cover architecture, RAG, observability — relevant but separately reviewed.

If you're operating an agentic system on Kubernetes (or planning to), buy the book. The chapter pays for itself the first time you correctly route a security trade-off because of the four-pattern framework.

The chapter cites Christian Posta's "[MCP Authorization Patterns for Upstream API Calls](https://oreil.ly/ufDox)" as foundational. Worth reading alongside.

---

## The repo

Genie is open source under MIT. Where the chapter's patterns landed in our code:

- Security — `pkg/auth/`, `pkg/governance/`, `pkg/crypto/`
- MCP — `pkg/mcp/` (client + server)
- A2A — `pkg/a2a/` (client + server)
- State — `pkg/memory/` (episodic + semantic + long-term tiers)
- Workflow checkpointing — `pkg/workflow/`
- Full docs — [`docs/`](docs/) — including 13 detailed per-agent pages and 7 per-package pages

```bash
git clone https://github.com/c2siorg/genie.git
go test ./...
```

---

If you've read the chapter and applied it to your own system, what's the one pattern that surprised you most? For me it was the OAuth2 token-exchange angle — I'd dismissed it as enterprise-y bureaucracy until the chapter framed it as the natural way to keep both user and agent identity in a single signed credential. Going on our roadmap as a result.

#GenerativeAI #Kubernetes #MCP #A2A #SPIFFE #ResponsibleAI #ProductionAI #FinTechIndia #BankingAI #OReilly
