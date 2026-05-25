# Operational Patterns for Production Agentic AI

*What we converged on building Genie — a 60+ agent financial assistant in production — across security, coordination, and state. Honest about what works, what didn't, and where the gaps still are.*

---

## Why this matters

Most "AI in production" content stops at "we deployed a chatbot and it scaled." That's not the hard part. The hard part is what happens after:

- A user's identity needs to ride through three agent hops and an external API call. Whose token does the API see?
- Two agents need to coordinate on a 4-hour research task across pod restarts. How do they not lose context?
- Your guardrail policy fires on a Friday at 2 AM and the audit log has to make sense to a regulator on Monday.

These aren't "frameworks problems." They're operational problems that show up the same shape regardless of which framework you picked. The patterns below are what we settled on across 12 months of iteration on Genie — an open-source multi-agent financial assistant in Go, aligned with the RBI FREE-AI report.

If you're operating an agentic system today, or planning to, this is the playbook I wish someone had handed me on day one.

---

## Security: four patterns, not one

The first question that hits you in production is identity. An agent calls a tool that calls an upstream API. Whose identity should the API see?

There isn't one answer. There are four patterns, each making a different trade-off between simplicity, attribution, and zero-trust posture.

### Pattern 1 — Agent impersonation (token passthrough)

The user's access token rides all the way through. The agent calls the MCP server with `Authorization: Bearer <user-token>`. The MCP server passes that token to the upstream API. The upstream sees the user.

**What you get**: per-user audit trails for free. The upstream API's existing RBAC works unchanged. Regulators can see "Nurse Alice queried Patient 4711," not just "the agent did."

**What you pay**: token lifetime becomes a hot issue. If the user's token expires in 60 minutes and your task takes 4 hours, you ship a refresh strategy or you fail. Scope explosion is the other tax — the user's token needs scopes for every API the agent *might* call.

**When it fits**: short-running tasks, existing RBAC infrastructure that you don't want to rebuild, compliance shops that want individual-user attribution.

In Genie this is the default. The user's JWT (HS256, 60-min TTL) rides through every `/v1/ask`. The orchestrator extracts `user_id` + `user_roles` into `msg.Metadata`. Every bus hop re-evaluates `governance.RBACPolicy` against those roles.

### Pattern 2 — Service account delegation

The agent and the upstream API both live on Kubernetes. They authenticate to each other using the K8s primitives that are already there — ServiceAccount tokens mounted into every pod, RBAC rules in YAML.

**What you get**: no external token server, no separate identity provider. K8s already mints the tokens, rotates them, and validates them via the TokenReview API. Auth becomes a YAML edit.

**What you pay**: the upstream sees the *agent's* identity, not the user's. If you need per-user attribution, this isn't it. You can layer the user-id in a side-channel header, but you've split your auth story across two surfaces.

**When it fits**: intra-cluster service-to-service, agent-level rate limits, environments where compliance accepts agent-level attribution.

We don't lean on K8s ServiceAccounts in Genie today because we run as a single binary (compose stack, K8s manifests on the roadmap). We get a similar effect — workload trust without external token services — through application-level RBAC at two layers: HTTP middleware and bus governance. Different mechanism, same defence-in-depth.

### Pattern 3 — Delegated identity via OAuth2 token exchange (RFC 8693)

This is the sleeper. It's not as well-known as the others and it's the one I'd implement first if starting fresh today.

The idea: trade the user's token for a *dual-identity* token that carries both the user (`sub` claim) AND the agent (`act` claim). The upstream API can then enforce composite policies — *"allow if the user has permission AND the agent is authorized for this operation."*

**What you get**: cryptographically-signed proof of *who* (the user) AND *what* (the agent) for every action. Audit logs answer "which agent did which user trigger on which data" in a single token, not three logs joined together.

**What you pay**: you have to operate a token-exchange endpoint. Modern identity platforms (Keycloak, Auth0, Azure AD) support RFC 8693, but you have to wire it. The exchanged tokens should be cached on a `(user, agent, audience)` tuple keyed by the JWT's `exp` claim — never longer than the actual token lifetime.

**When it fits**: regulated industries where the compliance question is dual-identity ("which AI agent accessed this record on whose behalf?"), or any system where you want to grant the agent narrower scopes than the user has.

This is a real gap in Genie today. The compliance use case sells me — for a bank or a hospital, "Nurse Alice's medical-assistant agent accessed Patient 4711" is the right audit shape. It's going on the next-quarter roadmap.

### Pattern 4 — SPIFFE/SPIRE (zero-trust)

Bearer tokens — JWTs, ServiceAccount tokens, API keys — can be stolen. SPIFFE solves this by binding identity to the workload itself. Each pod gets a SPIFFE ID (e.g., `spiffe://example.com/ns/agents/sa/customer-support`) and an X.509 cert (SVID) that's automatically issued, rotated every ~30 minutes, and impossible to steal without compromising the pod itself.

**What you get**: zero-trust mTLS by default. Stolen credentials become a non-problem. Service mesh integration is natural.

**What you pay**: operational overhead. The SPIRE Server is critical infrastructure — treat it like your KMS. Registration entries to manage (or automate via the SPIRE Controller Manager). The learning curve is genuinely steeper than bearer tokens.

**When it fits**: production K8s deployments with 10+ services, regulated environments where credential theft is part of the threat model, any setup where you already run a service mesh.

This is roadmap for Genie. For our current shape (single API binary + Postgres + Ollama on compose), SPIFFE is overkill. When we ship Helm charts and start running 20+ agent pods, SVIDs replace JWTs for service-to-service. Today's JWT + envelope-encrypted secrets are the bridge.

### Pattern 5 — Layer them

In practice, the right answer is rarely one pattern in isolation. The strongest production posture I've seen layers:

- **SPIFFE/SPIRE for workload-to-workload mTLS** — cryptographic identity for the network hop
- **User identity in request metadata** — `X-User-ID` header or token-exchange dual-identity token
- **Policy engine (OPA, Cedar) for composite authorization** — "allow if workload is X AND user has access to record Y"

You get the security benefits of SPIFFE without losing per-user attribution. The audit log answers both questions. The compromise of a single workload doesn't grant access to arbitrary data.

### MCP Gateways — when to centralize

An alternative to implementing security in every MCP server: deploy a gateway that sits between agent runtimes and MCP servers. Centralize authN, authZ, rate limiting, audit. Several implementations shipped in 2025 — Microsoft's MCP Gateway, IBM's ContextForge, Envoy AI Gateway, Solo.io's agentgateway. The market is young; revisit when you need to choose.

**When it fits**: 10+ MCP servers, multi-tenant environments, complex authorization that's a pain to repeat in every server.

**When it doesn't**: one or two MCP servers, simple authZ. The gateway is one more thing to keep highly available.

We run one MCP server today (Genie itself, exposing curated read-only agents). The gateway is overkill at one server. The day we cross 10, the math flips.

---

## Inter-agent coordination: A2A vs MCP

Two protocols, two jobs, often confused:

- **MCP** connects agents to **tools** — synchronous request/response, "call this function, get this result." Ideal for integrating an agent with its operational environment.
- **A2A** connects agents to **other agents** — asynchronous task delegation with lifecycle tracking. Designed for when one agent needs another agent to *reason* about something, not just execute a function.

You *could* model another agent as an MCP tool. For simple delegation, it works. What you lose:

- **Capability discovery** — A2A's "agent card" lets one agent programmatically find another that exposes a specific skill. MCP doesn't have this concept.
- **Task lifecycle** — A2A submits a task, gets an ID, polls or subscribes to updates. MCP is request/response; long-running work has to be modeled awkwardly.
- **Artifact streaming** — A2A can stream partial results back as the agent works. MCP doesn't natively support this.

The rule we settled on: **A2A for cross-agent orchestration, MCP for tool integration within each agent**. Each agent uses MCP to connect to its own tools; agents use A2A to coordinate with each other.

Genie ships both: `pkg/mcp` (client + server) and `pkg/a2a` (client + server). The MCP server exposes curated read-only agents to Claude Desktop / Cursor / any MCP client. The A2A server lets one Genie instance call another as a first-class peer. We don't yet support A2A's streaming subscription model — polling only. That's a feature gap; on the list.

A useful update: ACP (IBM's Agent Communication Protocol) merged into A2A in August 2025 under the Linux Foundation. Both protocols are now under shared governance via the Agentic AI Foundation. If you're still building on pre-merge ACP libraries, plan a migration.

---

## State management: short, long, and checkpointed

The first thing that surprises teams shipping agentic systems: agents are *not* stateless REST APIs. A user asks one question, the agent retrieves three documents, infers a pattern, suggests an action. The next question depends on all of that context.

Where does the context live? How does it survive a pod restart? How do you scale horizontally when each agent instance needs the conversation history?

The pattern that holds up: split state into **short-term** and **long-term**, then add **checkpointing** for long-running workflows.

### Short-term — the active conversation

For development and prototyping: keep it in a Python dict / Go map in the pod's memory. Trivial, fast, no infrastructure.

For production: externalize from day one. A KV store like Redis is the canonical choice. Session ID as key, serialized conversation as value, TTL matching session expiration.

The argument for externalizing on day one isn't paranoia — it's that the retrofit is painful. In-memory state leaks into closures, goroutine-locals, lazy initializers. You don't fully understand all the places it lives until you try to move it. Weeks of work that could have been hours.

Deploy the KV store as a StatefulSet with a PersistentVolume. Pods restart, state survives. Horizontal scaling works because all pods hit the same store. TTL handles cleanup of abandoned sessions.

Genie's `pkg/memory.EpisodicMemory` is in-process today with LLM-driven summarisation when the buffer overflows. That's fine for the sandbox and CI; production needs the Redis or Postgres backing. On the migration list, sized as a week of work.

### Long-term — facts that survive sessions

Short-term memory is "what did we just discuss?" Long-term memory is "what do we *know* about this user?" — primary bank, risk appetite, dependents, monthly inflow average.

For long-term, KV stores aren't enough. You need SQL — both for cross-session queries ("show me all customers who mentioned pricing concerns last week") and for audit ("prove what the agent told this customer six months ago"). Compliance and analytics both require it.

Pattern that works: KV for short-term (every request reads/writes), DB for long-term (audit logs, user preferences, learnt patterns). Write to the DB asynchronously off the critical path so the user response stays fast. On session start, query the DB for relevant long-term facts and cache them in the short-term store.

Genie's `pkg/memory.LongTermMemory` is the reference implementation — append-only consolidated facts, supersede semantics ("primary bank: HDFC, superseded → primary bank: ICICI" both visible in history). The Postgres backing is the obvious migration. Same migration path: in-memory today, durable tomorrow.

### Checkpointing — for the long-runners

Some workflows take hours. An SME loan workflow that fetches GST data, runs cashflow analysis, checks CGTMSE eligibility, drafts an indicative offer, waits for a Relationship Manager's human approval, then drafts the sanction letter — that's not a single LLM call. That's a multi-stage pipeline that needs to survive pod evictions.

The pattern: save a checkpoint after each major step. If the pod is evicted, the agent resumes from the most recent checkpoint instead of starting over.

A robust shape: not "save a JSON file every N steps" but an **event-sourced step log**. Every transition (`started`, `completed`, `failed`, `awaiting_approval`, `approved`) is appended. Recovery is replay: read the log, find the last `completed` step, resume from the next one.

This gives you three things for free:

1. **Resumability** — the original goal.
2. **Auditability** — the regulator can see every state transition.
3. **Debuggability** — the on-call can read mid-flight state to understand what the agent was thinking at step 17.

Genie's `pkg/workflow` is exactly this — DAG + Saga compensation + HITL approval + event-sourced sink. We built it for the SME loan workflow's regulator-friendly audit trail. The pod-eviction survivability was a happy side effect.

---

## Three operational principles that hold across all of this

The patterns change as the protocols evolve. These three don't.

### 1. Make trust structural, not behavioural

Every security pattern above pushes you toward enforcement points the caller can't accidentally skip. SPIFFE binds identity to the workload. ServiceAccount tokens are mounted automatically. Governance composites evaluate every message before any agent runs.

The opposite — "remember to check authorization in this handler" — fails the first time a junior engineer ships a new handler without the check. The wrapper isn't a guideline; the orchestrator forces it. Same logic at every layer.

### 2. Long-running ≠ hung. Checkpoint it.

An agent that takes 4 hours to complete a research task isn't broken; it's normal. What makes it *production*-grade is that a pod eviction at hour 3 doesn't restart from hour 0.

The harder version of this principle: design checkpoint state to be *inspectable*. A key-value map (or an event log) that an on-call engineer can read mid-flight is worth ten "save the model weights" black boxes. When something goes wrong at step 17, you want to read step 16's output and figure out why.

### 3. Externalize state from day one

In-memory state will fail the moment you scale horizontally or survive a pod restart. The retrofit cost is weeks, not hours, because state leaks into more places than you think.

Even if the data only lives in Redis for 30 seconds, put it in Redis from day one. Future you will thank present you.

---

## What we're shipping next

This is what's on Genie's next-quarter roadmap, sized in weeks:

1. **OAuth2 token exchange (RFC 8693)** — agent runtime exchanges user JWT for a dual-identity token before MCP calls. Audit log answers "which agent on whose behalf" in one signed credential.
2. **EpisodicMemory → Redis / Postgres** — externalize short-term memory so pods can restart without dropping conversations.
3. **LongTermMemory → Postgres** — the Postgres backing for the consolidated-facts tier.
4. **A2A streaming subscription** — catch up the A2A server to the streaming spec, not just polling.
5. **Helm charts + SPIFFE/SPIRE** — when we ship K8s manifests, SVIDs replace JWT for service-to-service.

Each of those addresses a gap that production stress will hit before we're ready, unless we ship them first.

---

## The repo

Genie is open source under MIT. Everything above is in the codebase:

- Security — `pkg/auth/`, `pkg/governance/`, `pkg/crypto/`
- MCP — `pkg/mcp/` (client + server)
- A2A — `pkg/a2a/` (client + server)
- State — `pkg/memory/` (episodic + semantic + long-term tiers)
- Workflow checkpointing — `pkg/workflow/`
- Full docs — [`docs/`](docs/) — 13 detailed per-agent pages, 7 per-package pages, FREE-AI mapping, operations guide

```bash
git clone https://github.com/c2siorg/genie.git
go test ./...           # 100+ packages green
make compose-up         # full stack with Postgres + Tempo + Grafana + Ollama
```

---

If you've shipped a production agentic system, which of these patterns bit you hardest? For us it was the state-externalization retrofit — moving in-memory session state to Redis took longer than anything else in the year, and we mostly avoided the worst of it. Curious what others saw.

#GenerativeAI #Kubernetes #MCP #A2A #SPIFFE #ResponsibleAI #ProductionAI #FinTechIndia #BankingAI
