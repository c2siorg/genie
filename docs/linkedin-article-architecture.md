# What Multi-Agent Architecture Actually Looks Like in Production

*Why single-LLM "AI assistants" don't survive contact with real systems — and the seven load-bearing pieces of an architecture that does.*

---

## The pattern that keeps failing

Every week, another "AI assistant" demo built on the same shape: one HTTP endpoint, one mega-prompt, one LLM call, a thin retry loop. It demos beautifully. It dies in production.

Six months in, the team is rewriting from scratch — and rediscovering, the hard way, a pattern the distributed systems community has known for two decades. The pattern has a name. **Multi-agent architecture.** Microsoft formalised it as the **Multi-Agent Reference Architecture (MARA)**, and it's what production-grade LLM systems converge on regardless of language or framework.

This article is a tour of the seven pieces that actually do load-bearing work, with examples from an open-source Go implementation called **Genie**. The implementation language doesn't matter. The pattern does.

---

## Why the monolith breaks

A single-LLM-call assistant has five structural problems that no amount of prompt engineering fixes:

1. **It cannot be audited.** A 4,000-token system prompt that branches on input is not a specification. The compliance team's question — "show me what this does with a PAN number" — has no answer except "depends." Regulators won't accept that.
2. **It cannot be partially upgraded.** Every edit touches every capability. Every regression touches every capability.
3. **It cannot be differentially controlled.** Different operations need different caching, rate limits, and budgets. One LLM call gives you one set of controls.
4. **Failure is all-or-nothing.** Provider returns a 500, the whole assistant is down. There's no degraded path because there's nothing else in the system.
5. **Latency is the slowest path.** You cannot parallelise what you cannot decompose.

These are properties of the architecture. Polishing won't fix them.

---

## The seven load-bearing pieces

Skip any one of these and you re-create the failure modes in a new disguise.

### 1. The protocol

A typed message envelope. Not function calls — *messages*.

```go
type Message struct {
    From           string
    To             string
    Type           string
    Payload        []byte
    Classification Classification // public | internal | pii | secret
    Metadata       map[string]string
}
```

`Type` makes the system inspectable. `Classification` makes governance possible. `Metadata` carries W3C `traceparent` so async hops still show up as one distributed trace.

**Anti-pattern:** direct method calls between agents (`recommender.Recommend(analysis)`). You've just rebuilt the monolith with extra files.

### 2. The registry

Agents declare capabilities at registration:

```go
type Agent interface {
    Name() string
    Capabilities() []string
    RiskLevel() RiskClass
    HandleMessage(ctx context.Context, msg Message) ([]Message, error)
}
```

The orchestrator queries the registry to discover who handles what. Adding an agent is a one-line registration, not an orchestrator refactor. This is also what makes a live AI inventory endpoint real — built from the registry, it cannot drift from what's actually running.

### 3. The bus

Pub/sub transport so agents are decoupled in time and space. Start in-memory (a channel-backed pub/sub is ~200 lines). Swap for Kafka or NATS when you outgrow one process. The agents don't change.

**The decoupling matters more than the technology.** When the analyzer fans out to forecaster, anomaly_detector, and recommender in parallel — three messages, three goroutines, results aggregated by correlation ID — latency becomes max(stages) instead of sum(stages). 3× speedup with zero algorithmic work.

### 4. The orchestrator

The orchestrator subscribes agents to message types, **enforces policy before dispatch**, and traces every hop.

The critical word is *before*. Policy inside a handler can be skipped by a buggy or compromised handler. Policy in the orchestrator cannot be bypassed.

```go
func (o *Orchestrator) dispatch(ctx context.Context, msg Message) error {
    if err := o.policy.Evaluate(ctx, msg); err != nil {
        o.incidents.Record(msg, err)
        return err
    }
    return o.bus.Publish(ctx, msg)
}
```

### 5. Governance as middleware

Every message passes through a **composite policy** before any agent runs: length, required metadata, RBAC, classification ceiling, residency, consent, explainability, PII regex, prompt injection, JSON schema. Each is small, independently testable, composable. Loaded from a board-approved YAML.

**Why middleware, not handler code:** middleware is a single audit surface. The compliance team reads one composite, not 40 handlers. The red-team corpus runs against one composite. The denial counter is one metric. Put policy inside handlers and you re-verify it every time you add an agent.

### 6. Per-agent risk class

```go
func (a *AMLMonitor) RiskLevel() RiskClass { return RiskHigh }
```

The orchestrator enforces ceilings — a `RiskHigh` agent cannot execute on a message lacking an `advisor` or `admin` role. Autonomous loops (ReAct, Reflexion) are bounded by deadline, circuit, and budget wrappers on the LLM provider. An agent cannot accidentally run away or DoS a downstream.

### 7. Fallback agents

The piece that turns a research project into a production system.

```go
orchestrator.SetFallback("portfolio_advisor", deterministic.PortfolioFallback{})
```

When the primary times out or panics, the orchestrator dispatches a deterministic fallback that needs neither LLM nor network. The user gets a degraded but truthful answer — "live analytics unavailable, here's the cached snapshot from 14:00 IST." Audit log records the fallback. On-call gets paged. System stays up.

CI proves it works: a drill target forces a failure and asserts the fallback fires.

---

## What this buys you, concretely

- **Parallelism for free** — fan-out to multiple agents, aggregate by correlation ID. Latency = max(stages), not sum.
- **Surgical caching** — educator caches 6h, rate_watcher caches 5m, recommender doesn't cache. Each agent owns its policy.
- **Differential observability** — per-agent SLOs in Tempo. "p99 latency of anomaly_detector for `enriched_transactions` over 7 days" is a query, not a project.
- **Composable safety** — drop a new policy into the composite, it applies everywhere.
- **Surgical upgrades** — swap the model behind one agent; the other 39 are unaffected.
- **Honest failure stories** — uptime isn't hostage to the LLM provider's status page.

---

## What Genie ships

Genie is the open-source Go implementation. The architecture is the deliverable; the financial-assistant domain is the demonstration.

- **40+ specialist agents** — canonical pipeline (ingestor → normalizer → enricher → analyzer → forecaster → anomaly → recommender → reporter) plus 26 domain specialists (fraud, AML, LCR, VaR, ALM, tax, lending, complaint triage…)
- **5 LLM providers** behind one interface, with cost/cache/router/shadow/circuit/deadline/budget wrappers
- **Hybrid RAG** (vector + BM25 + RRF + cross-encoder rerank + HyDE + Self-RAG + Corrective RAG) plus GraphRAG entity walks
- **Reasoning patterns** — CoT, ReAct, Reflexion, Chain-of-Verification, Step-Back, Semantic Router
- **Workflow runtime** — DAG + Saga compensation + HITL approval + event-sourced log
- **MCP + A2A** for interop and federation
- **OpenTelemetry end-to-end** with OpenInference semantics

57 test packages, all green. The sandbox is the production code:

```bash
git clone https://github.com/c2siorg/genie.git
cd genie
go test ./...           # 57 packages green
go run ./cmd/genie      # full pipeline, in-process, no Postgres, no network
make compose-up         # full stack with Postgres + Tempo + Grafana + Ollama
```

---

## The takeaway

**The protocol and the bus are the architecture.** Everything else — the agents, the LLM, the RAG strategy, the prompts — is replaceable. The shape of how messages flow determines whether you can audit, upgrade, observe, and keep the system running.

Most "multi-agent" projects skip this part. They have multiple agents, but the agents call each other directly. They have governance, but it's inside the agents. They have observability, but it's per-agent and doesn't compose. They have fallbacks, but only for the LLM call itself.

That's not multi-agent architecture. That's a monolith with extra files.

The seven pieces are not optional.

---

**References**

- [Microsoft Multi-Agent Reference Architecture](https://microsoft.github.io/multi-agent-reference-architecture/)
- [Anthropic Model Context Protocol](https://modelcontextprotocol.io/)
- [Google Agent2Agent Protocol](https://github.com/google/a2a)
- [Genie — Go reference implementation (MIT)](https://github.com/c2siorg/genie)

If you're building multi-agent systems, what's the piece you skipped first — and what did it cost you?

#MultiAgent #AIArchitecture #SystemDesign #LLM #DistributedSystems #MARA #Golang #SoftwareArchitecture
