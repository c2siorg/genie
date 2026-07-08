# Genie — AI Financial Assistant (Go)

> An **AI financial assistant** in Go, built on Microsoft's
> [Multi-Agent Reference Architecture (MARA)](https://microsoft.github.io/multi-agent-reference-architecture/index.html)
> and aligned with the **RBI FREE-AI** report (Aug 2025). Speaks **MCP** and **A2A**,
> runs **Ollama on-prem** by default, and bundles a full **GenAI engineering** layer
> (RAG, reasoning, memory, eval, safety, privacy).

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)
![Architecture](https://img.shields.io/badge/Architecture-MARA-blue)
![OTel](https://img.shields.io/badge/observability-OpenTelemetry-success)
![RBI FREE-AI](https://img.shields.io/badge/RBI-FREE--AI%20aligned-orange)
![License](https://img.shields.io/badge/license-PolyForm%20Noncommercial%201.0.0-lightgrey)

**60+ specialist finance agents** implemented across the codebase (61 agent packages
under `agents/`), **32 of them wired into the running API** today — plus 2 deterministic
fallback agents (34 registered in all) — covering retail finance, SME lending, KYC,
bancassurance, fraud, treasury, payments, and cyber.

---

## Contents

- [Why Genie](#why-genie) · [Architecture](#architecture) · [Quick start](#quick-start) ·
  [Using the API](#using-the-api) · [Capabilities](#capabilities) ·
  [Governance & FREE-AI](#governance--free-ai) · [Documentation](#documentation) ·
  [Development](#development) · [Roadmap](#roadmap) · [License](#license)

---

## Why Genie

Genie answers *"What should I do with my money?"* by combining deterministic finance
logic with specialist agents: ingestion → normalisation → analysis → forecasting →
anomaly detection → recommendations. **Every step is a message on a bus, every message
passes through governance, every hop is traced.**

That shape — orchestrator + registry + bus + governance + memory + observability +
evaluation — is what MARA prescribes for production multi-agent systems, and what the
RBI FREE-AI report maps to its 7 Sutras, 6 Pillars, and 26 Recommendations. Money is
never computed by a model: rupee figures come from deterministic, tested integer-paise
math. The LLM reasons; the ledger counts.

---

## Architecture

```mermaid
flowchart TB
    subgraph Clients
        UA[CLI / curl / SDK]
        BR[Browser / Passkey]
    end

    subgraph Edge["pkg/web — HTTP + WebSocket"]
        direction TB
        MW[chi router + middleware<br/>RequestID · Recovery · Log<br/>OTel · JWT Auth · RBAC · RateLimit]
        SSE[/v1/ask/stream SSE/]
        WS[/v1/chat/ws WebSocket/]
        OAUTH[/v1/oauth · /v1/webauthn/]
        MCPEP[/mcp JSON-RPC/]
    end

    subgraph Platform["MARA platform"]
        ORCH[pkg/orchestration]
        BUS[pkg/comm Bus]
        REG[pkg/registry]
        POL[pkg/governance Composite Policy]
        AGENTS[(specialist agents)]
        FB[Fallback agents]
        ORCH --> BUS
        ORCH --> POL
        REG --> ORCH
        BUS --> AGENTS
        AGENTS --> FB
    end

    subgraph AI["GenAI layer"]
        LLM[pkg/llm<br/>Mock · Ollama · Anthropic<br/>OpenAI · Gemini]
        RAG[pkg/rag<br/>hybrid · pgvector · rerank<br/>Self-RAG · CRAG]
        GRAG[pkg/graphrag<br/>entity graph]
        REAS[pkg/reasoning<br/>CoT · ReAct · Reflexion<br/>CoV · Step-Back]
        MEM[pkg/memory<br/>semantic + episodic]
        CONST[pkg/constitution<br/>7 Sutras]
        TOOL[pkg/toolkit · pkg/eval]
        SAFE[pkg/safety]
    end

    subgraph Data
        PG[(Postgres<br/>users · documents · incidents<br/>consents · mcp_tokens · pgvector)]
        VAULT[(Encrypted at rest<br/>pkg/crypto envelope)]
    end

    subgraph Observability
        OTLP[OTel Collector]
        TEMPO[(Tempo)]
        GRAF[Grafana]
        OTLP --> TEMPO --> GRAF
    end

    subgraph External
        KITE[Zerodha Kite MCP]
        AA[Sahamati AA]
        OLLAMA[Ollama runtime]
    end

    UA -->|REST + Bearer JWT| MW
    BR -->|Passkey| OAUTH
    MW --> SSE
    MW --> WS
    MW -->|Publish| BUS
    AGENTS --> LLM
    AGENTS --> RAG
    AGENTS --> GRAG
    AGENTS --> MEM
    LLM --> OLLAMA
    AGENTS --> KITE
    AGENTS --> AA
    AGENTS --> PG
    PG --> VAULT
    AGENTS --> OTLP
    MW --> OTLP
    LLM --> OTLP
    POL -.audit.-> AGENTS
    REAS -.via.-> LLM
    SAFE -.via.-> POL
    TOOL -.via.-> POL
    CONST -.via.-> POL
```

A request becomes a **message**, not a function call — agents never call each other
directly. Everything flows through the bus, which gives one seam each for governance,
tracing, fallbacks, and the live capability inventory. The load-bearing packages:

| Package | Role |
| --- | --- |
| `pkg/protocol` | Wire format: `Message`, `Classification`, metadata keys |
| `pkg/registry` | Capability discovery; drives `GET /v1/ai-inventory` |
| `pkg/comm` | Pub/sub bus (in-mem; swap for Kafka/NATS) |
| `pkg/orchestration` | Dispatch loop: policy → lookup → risk ceiling → invoke → fallback |
| `pkg/governance` | Composite policy: RBAC, classification, residency, consent, PII, injection, schema |
| `pkg/agent` | `Agent` + `Environment` + `RiskClass` |
| `agents/fallback` | Deterministic degraded answers when a primary fails |

Full layer-by-layer map in **[docs/architecture.md](docs/architecture.md)**.

---

## Quick start

**CLI demo — zero external dependencies.** Runs the full bus pipeline in-process with
stdout OTel exporters.

```bash
git clone https://github.com/PratikDhanave/genie.git
cd genie
go test ./...            # all packages green (uses the mock provider)
go run ./cmd/genie       # full pipeline → console
```

**Full stack via docker-compose** — API, Postgres, Ollama, and the observability stack:

```bash
make up      # build, boot, wait for /readyz, print URLs
make down    # tear down
```

| Service | URL | Purpose |
| --- | --- | --- |
| `genie-api` | <http://localhost:8080> | the service (UI at `/`) |
| `postgres` | localhost:5432 | persistence |
| `ollama` | localhost:11434 | on-prem LLM runtime |
| `grafana` | <http://localhost:3000> | traces via Explore → Tempo (`genie-api`) |

See **[docs/operations.md](docs/operations.md)** for env vars, deployment, and runbooks.

---

## Using the API

```bash
# 1) Sign up → JWT
TOKEN=$(curl -s -X POST localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","name":"Alice","password":"hunter2hunter2"}' \
  | jq -r .token)

# 2) Upload an encrypted CSV
DOC_ID=$(curl -s -X POST 'localhost:8080/v1/documents?description=Jan%20statement&classification=pii' \
  -H "Authorization: Bearer $TOKEN" --data-binary @data/sample.csv | jq -r .id)

# 3) Ask Genie
curl -s -X POST localhost:8080/v1/ask \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"question\":\"Where am I overspending?\",\"document_id\":\"$DOC_ID\"}" | jq .
```

`/v1/ask/stream` streams the same run as Server-Sent Events (AI disclosure → trace →
per-agent handoffs → final report); `/v1/chat/ws` is a bidirectional WebSocket. Every
`/v1/ask` appears in Grafana as one distributed trace across the HTTP server, the bus,
governance, and each agent. Full endpoint reference: **[docs/api.md](docs/api.md)** ·
[`docs/openapi.yaml`](docs/openapi.yaml).

---

## Capabilities

Each capability is a package with a dedicated design doc under
[`docs/packages/`](docs/packages/README.md):

- **Agents** — 32 live specialist agents behind the bus (ingestion, forecasting, anomaly,
  KYC, claims, SME lending, tax, portfolio, payments, cyber) plus 2 deterministic
  fallbacks. → [docs/agents](docs/agents/README.md)
- **LLM providers** — Mock · Ollama · Anthropic · OpenAI · Gemini, wrapped in
  Cache → Budget → Deadline → Circuit layers that bound autonomous reasoning.
- **Retrieval** — hybrid RAG + GraphRAG over pgvector, with rerank, Self-RAG, and CRAG.
- **Reasoning** — CoT, ReAct, Reflexion, Chain-of-Verification, Step-Back, semantic router.
- **Memory** — semantic + episodic + LLM summarisation + append-only long-term facts.
- **Safety & eval** — jailbreak/topic/toxicity/bias filters; RAGAS, drift, hallucination, Elo.
- **Auth** — JWT + bcrypt, OAuth 2.1, RFC 8628 device flow, WebAuthn passwordless.
- **Privacy & crypto** — AES-256-GCM envelope encryption, HMAC tokenisation, DP noise.
- **Protocols** — MCP (Zerodha Kite), A2A, CloudEvents, AsyncAPI, OpenInference. → [docs/protocols.md](docs/protocols.md)
- **Sovereignty & supply chain** — data-residency tags, on-prem inference, federated
  learning, AIBOM (CycloneDX 1.6 + Sigstore), DIDs + Verifiable Credentials.

---

## Governance & FREE-AI

Governance is **data, not code**. A board-approved policy YAML
([`config/ai-policy.example.yaml`](config/ai-policy.example.yaml)) is enforced by a
composite policy chain on *every* message — length, required metadata, RBAC,
classification ceiling, data residency, consent, explainability, PII regex,
prompt-injection, and JSON-schema checks — deny-on-first-failure. High-risk agents
(AML, VaR, KYC, payments) refuse to run without an `advisor`/`admin` role.

Every RBI FREE-AI recommendation maps to a concrete file path in
**[docs/free-ai-mapping.md](docs/free-ai-mapping.md)**. Two of the tooling seams:

```bash
make red-team    # adversarial probe corpus against the board policy (Rec 20)
make bcp-drill   # force a primary failure to verify the fallback fires (Rec 21)
```

---

## Documentation

Deep reference lives in **[`docs/`](docs/README.md)** — start there.

| Doc | Contents |
| --- | --- |
| [architecture.md](docs/architecture.md) | The core loop, packages, dispatch, fan-out/fan-in |
| [operations.md](docs/operations.md) | Env vars, deployment, docker stack, runbooks |
| [api.md](docs/api.md) · [openapi.yaml](docs/openapi.yaml) | HTTP/WebSocket endpoints |
| [protocols.md](docs/protocols.md) · [asyncapi.yaml](docs/asyncapi.yaml) | MCP, A2A, CloudEvents |
| [free-ai-mapping.md](docs/free-ai-mapping.md) | Every FREE-AI recommendation → file path |
| [agents/](docs/agents/README.md) · [packages/](docs/packages/README.md) | Per-agent and per-package pages |

---

## Development

```bash
make build          # compile every binary under cmd/ into bin/
make test           # go test -race ./...   (the CI gate; also runs go vet)
make run-cli        # CLI demo — no HTTP, Postgres, or LLM needed
make run-api-mock   # HTTP API with the mock LLM (needs Postgres + JWT/KEK env)
make scaffold name=<id> cap=<capability> in=<intype> out=<outtype> next=<agent>
```

CI runs `go vet` then `go test -race` on Go 1.25, then a docker build on `main`. Adding
an agent, running a single test, and the full env reference are covered in
[`CLAUDE.md`](CLAUDE.md) and [docs/operations.md](docs/operations.md).

---

## Roadmap

Kafka/NATS bus transport · a real reranker model behind `rag.Reranker` (BGE, ColBERT) ·
a KMS-backed `crypto.KeyResolver` (AWS / GCP / Vault) · wiring the remaining implemented
agents into the API · richer Grafana dashboards. Contributions welcome — see
[Contributing](#contributing).

## Contributing

```bash
make vet && make test   # run all checks before pushing
```

New cross-cutting rules go into the composite policy (config YAML), not into agents;
new agents are message-driven and ship with a paired `docs/agents/<id>.md`. Details in
[`CLAUDE.md`](CLAUDE.md).

## License

**[PolyForm Noncommercial License 1.0.0](LICENSE)** — free for any noncommercial purpose
(personal, research, education, non-profit and government use). Commercial use is not
permitted. See [`LICENSE`](LICENSE) for the full terms.

> Note: releases prior to v1.0.0 were made available under the MIT License; that grant
> continues to apply to those earlier versions. Genie v1.0.0 onward is licensed under
> PolyForm Noncommercial 1.0.0.

## References

- [Multi-Agent Reference Architecture](https://microsoft.github.io/multi-agent-reference-architecture/index.html)
- [RBI FREE-AI report (Aug 2025)](https://rbidocs.rbi.org.in/rdocs/PublicationReport/Pdfs/FREEAIR130820250A24FF2D4578453F824C72ED9F5D5851.PDF)
- [Model Context Protocol](https://modelcontextprotocol.io/) · [Agent2Agent Protocol](https://github.com/google/a2a) · [CloudEvents](https://cloudevents.io/)
- [CycloneDX ML-BOM](https://cyclonedx.org/capabilities/mlbom/) · [OpenInference](https://github.com/Arize-ai/openinference) · [W3C DID Core](https://www.w3.org/TR/did-core/)
