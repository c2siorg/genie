# Genie — AI Financial Assistant (Go)

> Open-source **AI financial assistant** in Go, built on Microsoft's
> [Multi-Agent Reference Architecture (MARA)](https://microsoft.github.io/multi-agent-reference-architecture/index.html).
> A thin HTTP edge gates requests with JWT + RBAC, persists encrypted
> documents in Postgres, and routes finance questions through a
> message-driven pipeline of specialist agents — fully traced via
> OpenTelemetry.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8)
![Architecture](https://img.shields.io/badge/Architecture-MARA-blue)
![OTel](https://img.shields.io/badge/observability-OpenTelemetry-success)
![License](https://img.shields.io/badge/license-MIT-lightgrey)

Repository: <https://github.com/c2siorg/genie>

---

## Table of contents

- [Why Genie](#why-genie)
- [System architecture](#system-architecture)
- [End-to-end finance flow](#end-to-end-finance-flow)
- [Repository layout](#repository-layout)
- [Quick start (CLI demo)](#quick-start-cli-demo)
- [Run the HTTP API with docker-compose](#run-the-http-api-with-docker-compose)
- [HTTP API: signup → upload → ask](#http-api-signup--upload--ask)
- [Authentication & Authorization](#authentication--authorization)
- [Document encryption](#document-encryption)
- [Governance & policies](#governance--policies)
- [Observability: traces, metrics, logs](#observability-traces-metrics-logs)
- [Scaffolding a new agent](#scaffolding-a-new-agent)
- [Testing](#testing)
- [Configuration reference](#configuration-reference)
- [Roadmap](#roadmap)

---

## Why Genie

Genie answers *"What should I do with my money?"* by combining deterministic
finance logic with specialist agents (ingestion, normalization, analysis,
forecasting, anomaly detection, recommendations). Every step is a message on
a bus, every message passes through governance, and every hop is traced.

The same shape — orchestrator + registry + bus + governance + memory +
observability + evaluation — is what MARA describes for production-grade
multi-agent systems.

---

## System architecture

```mermaid
flowchart LR
    subgraph Client
        UA[CLI / curl / SDK]
    end

    subgraph Edge["pkg/web (HTTP + Middleware)"]
        H[chi router]
        MW1[RequestID]
        MW2[Recovery]
        MW3[AccessLog]
        MW4[OTel Trace]
        MW5[JWT Auth]
        H --> MW1 --> MW2 --> MW3 --> MW4 --> MW5
    end

    subgraph Domain["Multi-Agent Platform (MARA)"]
        ORCH[pkg/orchestration]
        BUS[pkg/comm Bus]
        REG[pkg/registry]
        POL[pkg/governance Policies]
        AGENTS[(15 specialist agents)]
        ORCH --> BUS
        ORCH --> POL
        REG --> ORCH
        BUS --> AGENTS
    end

    subgraph Storage
        PG[(Postgres)]
        KV[(In-mem KV / Sessions)]
        DOCS[(Encrypted Documents)]
    end

    subgraph Observability
        OTLP[OTel Collector]
        TEMPO[Tempo]
        GRAF[Grafana]
        OTLP --> TEMPO --> GRAF
    end

    UA -->|REST + Bearer JWT| H
    MW5 --> |Publish msg| BUS
    AGENTS --> |Persist| PG
    PG --> DOCS
    AGENTS --> |Spans + Metrics| OTLP
    H --> |Spans| OTLP
    BUS --> |Spans| OTLP
```

### What lives where

| Layer | Package | Role |
| --- | --- | --- |
| Wire format | `pkg/protocol` | `Message`, `Classification`, metadata keys |
| Worker interface | `pkg/agent` | `Agent` + `Environment` |
| Discovery | `pkg/registry` | In-memory registry; capability lookup |
| Transport | `pkg/comm` | Pub/sub bus (in-mem; swap for Kafka/NATS) |
| Coordination | `pkg/orchestration` | Subscribes agents, enforces policy, traces |
| Safety | `pkg/governance` | Content length, required metadata, RBAC, classification, PII, prompt-injection |
| Memory | `pkg/memory` | Pluggable KV — local for sessions |
| Persistence | `pkg/storage/postgres` | pgx repos: users, accounts, encrypted documents, eval records |
| Crypto | `pkg/crypto` | Envelope AES-256-GCM + KEK resolvers |
| Auth | `pkg/auth` | JWT (HS256, stdlib), bcrypt, roles, claims |
| Observability | `pkg/observability` | slog + OTel traces/metrics; stdout or OTLP exporters |
| HTTP edge | `pkg/web` | chi router, middleware, handlers |
| Bus ↔ HTTP | `pkg/busio` | Correlator (await response by trace_id) |
| Agents | `agents/` | 15 specialists (see below) |

### The 15 specialists

```mermaid
flowchart TB
    SUP[financial_supervisor]
    ING[ingestor]
    NORM[normalizer]
    ENR[enricher]
    AN[analyzer]
    FC[forecaster]
    AD[anomaly_detector]
    REC[recommender]
    REP[reporter]

    CUR[currency_converter]
    EDU[financial_educator]
    MAC[macro_research]
    RAT[rate_watcher]
    LOA[loan_advisor]
    AUD[llm_auditor]

    SUP -->|kicks off| ING --> NORM --> ENR --> AN
    AN --> FC --> SUP
    AN --> AD --> SUP
    AN --> REC --> SUP
    AN --> SUP
    SUP --> REP

    AUD -. "broadcast subscriber, audits every msg" .-> SUP

    classDef adk fill:#fef6dd,stroke:#d99e2c
    class CUR,EDU,MAC,RAT,LOA,AUD adk
```

Yellow agents are the ADK-inspired adjacent specialists (currency, educator,
macro, rate-watcher, loan-advisor, auditor). They are first-class citizens in
the registry but the standard "ask" flow uses the main grey pipeline.

---

## End-to-end finance flow

What happens when a user uploads a CSV and asks *"Where am I overspending?"*:

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant API as cmd/api (HTTP)
    participant DB as Postgres
    participant ENC as pkg/crypto
    participant BUS as comm.Bus
    participant POL as Governance
    participant SUP as supervisor
    participant ING as ingestor
    participant N as normalizer
    participant EN as enricher
    participant AN as analyzer
    participant FC as forecaster
    participant AD as anomaly
    participant REC as recommender
    participant REP as reporter

    U->>API: POST /v1/users/login (email,password)
    API->>DB: SELECT user by email
    API->>U: 200 {token, user}

    U->>API: POST /v1/documents (CSV body, Bearer JWT)
    API->>ENC: Encrypt(csv) -> EncryptedPayload
    API->>DB: INSERT documents (payload JSONB)
    API->>U: 201 {id, classification, kek_id}

    U->>API: POST /v1/ask {question, document_id}
    API->>DB: SELECT documents WHERE id=?
    API->>ENC: Decrypt(payload)
    API->>BUS: Publish finance_question (with trace_id, roles, classification)
    BUS->>POL: Evaluate (length, metadata, RBAC, classification, injection)
    POL-->>BUS: allow
    BUS->>SUP: HandleMessage
    SUP->>BUS: To=ingestor (ingest_csv)
    BUS->>ING: HandleMessage
    ING->>BUS: raw_transactions
    BUS->>N: HandleMessage
    N->>BUS: normalized_transactions
    BUS->>EN: HandleMessage
    EN->>BUS: enriched_transactions
    BUS->>AN: HandleMessage
    AN->>BUS: 4× analysis_result fan-out
    BUS->>FC: forecast_result -> SUP
    BUS->>AD: anomalies -> SUP
    BUS->>REC: recommendations -> SUP
    BUS->>SUP: 4 fan-outs collected
    SUP->>BUS: final_report_request
    BUS->>REP: HandleMessage
    REP->>BUS: To=user (final_report)
    BUS-->>API: Correlator wakes the waiting handler
    API->>U: 200 {trace_id, report}
```

Trace context propagates across goroutines via `Message.Metadata` — the W3C
`traceparent` header is injected on publish and re-extracted by the
orchestrator before each agent runs, so the entire flow shows up as one
distributed trace in Tempo.

---

## Repository layout

```
genie/
├── cmd/
│   ├── api/           # HTTP service-edge binary (auth + RBAC + Postgres + OTLP)
│   ├── genie/         # CLI that runs the bus pipeline end-to-end in-process
│   ├── demo/          # original toy planner/executor/coordinator demo
│   └── scaffold/      # generates a new agent skeleton
├── agents/            # 15 specialist agents
├── pkg/
│   ├── protocol/      # Message + Classification
│   ├── agent/         # Agent + Environment
│   ├── registry/      # in-memory registry
│   ├── comm/          # in-memory pub/sub bus (with OTEL spans)
│   ├── orchestration/ # orchestrator (governance + tracing in the critical path)
│   ├── governance/    # policies: length, metadata, RBAC, classification, PII, injection
│   ├── memory/        # KV store interface + in-mem impl
│   ├── observability/ # slog + OTel (stdout or OTLP)
│   ├── eval/          # interaction records
│   ├── auth/          # JWT (stdlib), bcrypt, roles, claims
│   ├── crypto/        # envelope AES-GCM, KEK resolvers
│   ├── storage/postgres/ # pgxpool + migrations + repos
│   ├── busio/         # Correlator: await bus response by trace_id
│   └── web/           # chi router + middleware + HTTP handlers
├── data/sample.csv
├── deploy/local/      # tempo + otel-collector + grafana configs
├── docs/openapi.yaml  # full HTTP spec
├── tests/             # end-to-end integration test
├── Dockerfile
├── docker-compose.yaml
├── Makefile
└── .circleci/config.yml
```

**Module path:** `github.com/PratikDhanave/multi-agent-reference-architecture-go`

---

## Quick start (CLI demo)

No Postgres, no HTTP server. Runs the full bus pipeline in-process with
stdout OTel exporters.

```bash
go run ./cmd/genie
```

You should see structured logs and:

```text
=== FINAL REPORT ===
Genie Financial Report
Question: Where am I overspending vs last month?
Currency: INR
Income:  10000000 (minor units)
Expense: 3279800 (minor units)
Net:     6720200 (minor units)
Top categories: housing:rent, food:delivery, Utilities
Forecast: {...}
Recommendations: {...}
```

A `genie-traces.json` file is produced alongside the binary; it's a stream of
JSON-encoded OTel spans that mirror the sequence diagram above.

---

## Run the HTTP API with docker-compose

```bash
make compose-up
```

Brings up:

| Service | URL | Purpose |
| --- | --- | --- |
| `genie-api` | <http://localhost:8080> | the service |
| `postgres` | localhost:5432 | persistence (genie/genie/genie) |
| `otel-collector` | grpc :4317, http :4318 | receives OTLP |
| `tempo` | <http://localhost:3200> | trace backend |
| `grafana` | <http://localhost:3000> | UI (anonymous admin) |

In Grafana, open **Explore → Tempo** and run a trace search by service name
`genie-api`. Each Ask request appears as one distributed trace spanning the
HTTP server, the bus, governance, and every agent that handled a message.

Stop everything with `make compose-down`.

---

## HTTP API: signup → upload → ask

```bash
# 1) Sign up
TOKEN=$(curl -s -X POST localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","name":"Alice","password":"hunter2hunter2"}' \
  | jq -r .token)

# 2) Upload an encrypted CSV
DOC_ID=$(curl -s -X POST 'localhost:8080/v1/documents?description=Jan%20statement&classification=pii' \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @data/sample.csv \
  | jq -r .id)

# 3) Ask Genie
curl -s -X POST localhost:8080/v1/ask \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"question\":\"Where am I overspending?\",\"document_id\":\"$DOC_ID\"}" | jq .
```

Sample response:

```json
{
  "trace_id": "tr-1779514412090640000",
  "report": "Genie Financial Report\nQuestion: Where am I overspending?\nCurrency: INR\nIncome:  10000000 ...\n"
}
```

Full spec: [`docs/openapi.yaml`](docs/openapi.yaml). Render in any
OpenAPI viewer (Swagger UI, Redoc) for the interactive form.

---

## Authentication & Authorization

### Token lifecycle

```mermaid
sequenceDiagram
    actor U as User
    participant API
    participant Repo as UserRepo (Postgres)
    participant JWT as auth.Issuer (HS256)

    U->>API: POST /v1/users {email,name,password}
    API->>API: bcrypt(password)
    API->>Repo: INSERT users (...)
    API->>JWT: Issue(subject, email, [user])
    API->>U: {token, expires_at, user}

    Note over U: subsequent requests
    U->>API: GET /v1/users/me<br/>Authorization: Bearer <token>
    API->>JWT: Verify(token)
    JWT-->>API: claims{sub, roles, exp}
    API->>Repo: SELECT user
    API->>U: {id, email, roles, ...}
```

- **JWT**: HS256 signed by `GENIE_JWT_SECRET`. Issued for 60 minutes.
  Implemented in stdlib (`pkg/auth/jwt.go`) to keep the security surface
  small and auditable — no third-party JWT library.
- **Passwords**: bcrypt via `golang.org/x/crypto/bcrypt`, default cost (10).
- **Roles**: `user` (default), `advisor`, `admin`. Stored as a Postgres
  `TEXT[]` column on `users`.

### Authorization at two layers

Genie enforces authz in two places — **before** the HTTP handler and
**before** the agent sees the message:

```mermaid
flowchart LR
    REQ[Bearer JWT request] --> MW{mid.Auth}
    MW -- invalid --> R401[401]
    MW -- valid --> RR{mid.RequireRole?<br/>route gate}
    RR -- no match --> R403[403]
    RR -- ok --> HANDLE[Handler]
    HANDLE --> PUB[bus.Publish<br/>user_roles in metadata]
    PUB --> POL{governance.RBACPolicy}
    POL -- deny --> DROP[message dropped<br/>span marked Error]
    POL -- allow --> AGENT[Agent.HandleMessage]
```

- `pkg/web/mid.Auth` verifies the JWT and pins `auth.Claims` onto the
  request context.
- `pkg/web/mid.RequireRole(roles...)` is an optional route-level gate.
- `pkg/governance.RBACPolicy` runs on the bus. It reads
  `metadata["user_roles"]` (set by the HTTP layer) and denies messages
  whose required roles are not held.
- `AdminBypass: true` lets `admin` skip every RBAC denial.

This means a compromised handler can't sneak data to an agent it isn't
authorized to talk to — the bus policy is still the gate.

---

## Document encryption

CSV uploads are encrypted before they reach Postgres. Genie uses an
**envelope encryption** scheme: each document gets a fresh data encryption
key (DEK), the DEK is wrapped with the active key encryption key (KEK), and
the wrapped DEK + ciphertext are stored together.

```mermaid
flowchart LR
    PT[CSV plaintext]
    DEK[(DEK 32 bytes,<br/>fresh per doc)]
    KEK[(KEK<br/>env / KMS)]
    CT[ciphertext]
    WDEK[wrapped DEK]
    JSON[EncryptedPayload JSON<br/>{kek_id, wrapped_dek, nonce, ciphertext}]

    PT --AES-256-GCM--> CT
    DEK --AES-256-GCM--> WDEK
    KEK -.wraps.-> DEK
    CT --> JSON
    WDEK --> JSON
    JSON --> PG[(documents.payload JSONB)]
```

- **Algorithm**: AES-256-GCM for both DEK encryption of the document and
  KEK wrapping of the DEK.
- **Local**: `pkg/crypto.EnvKeyResolver` reads the KEK from
  `GENIE_KEK_BASE64` (32 bytes, base64 encoded — generate with
  `openssl rand -base64 32`).
- **Production**: `pkg/crypto.KMSKeyResolver` is the production shape. Plug
  in any KMS by implementing the `KMSClient` interface (AWS KMS, GCP KMS,
  HashiCorp Vault Transit). Genie never sees the raw KEK in the prod path.
- **Storage**: `EncryptedPayload` is stored in `documents.payload` as
  JSONB. Decryption happens *only* in the `/v1/ask` flow, in memory, and
  the plaintext exits the process boundary on the bus marked
  `classification=pii`.
- **Description**: an arbitrary user-supplied label and a
  `classification` query parameter (`public | internal | pii | secret`) are
  stored alongside each document, so audits and governance policies can
  reason about content sensitivity without ever decrypting.

Key rotation is a future feature — the schema already accommodates it
(`kek_id` per row); decryption first asks the resolver if it can serve that
KEK id, otherwise rejects.

---

## Governance & policies

Every message that crosses the bus is evaluated by a composite policy
**before** the destination agent's `HandleMessage` runs.

```mermaid
flowchart TB
    MSG[Message] --> C[CompositePolicy]
    C --> P1[MaxContentLengthPolicy]
    C --> P2[RequiredMetadataPolicy<br/>e.g. trace_id, user_id]
    C --> P3[RBACPolicy<br/>roles vs message type]
    C --> P4[ClassificationPolicy<br/>recipient ceiling vs msg class]
    C --> P5[PIIBlockPolicy<br/>regex on content]
    C --> P6[PromptInjectionPolicy<br/>known marker phrases]
    P1 & P2 & P3 & P4 & P5 & P6 --> D{any deny?}
    D -- yes --> SP[span.Error +<br/>denial counter +<br/>drop msg]
    D -- no --> A[agent.HandleMessage]
```

Policies are deliberately small and composable. The composite denies on the
first deny and reports the reason via OTel span attributes and the
`genie.governance.denials` counter.

To add a policy, implement:

```go
type Policy interface {
    Evaluate(ctx context.Context, msg protocol.Message) (PolicyResult, error)
}
```

and put it into the composite at startup.

---

## Observability: traces, metrics, logs

```mermaid
flowchart LR
    APP[genie-api / cmd/genie] --OTLP gRPC--> COL[otel-collector]
    COL --> TEMPO[(Tempo)]
    APP --slog JSON--> STDOUT[(stdout / Loki)]
    TEMPO --datasource--> GRAF[Grafana]
```

- **Traces**: spans around `http <method> <path>`, `governance.evaluate`,
  `bus.publish`, `agent.handle`. Trace context is propagated through
  `Message.Metadata` so async hops stay linked.
- **Metrics**:
  - `genie.bus.messages_published`
  - `genie.agent.messages_handled`
  - `genie.governance.denials`
  - `genie.agent.errors`
  - `genie.agent.handle_duration_ms` (histogram)
- **Logs**: structured slog (`pkg/observability.SlogLogger`). Use the
  `LogAttrs` method for hot paths.

Switch between exporters:

| Mode | When | Set |
| --- | --- | --- |
| stdout | CLI demo (`cmd/genie`) | nothing — default |
| OTLP gRPC | service (`cmd/api`) | `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317` (compose does this for you) |

---

## Scaffolding a new agent

Add a "tax estimator" agent without writing the boilerplate:

```bash
make scaffold name=tax_estimator cap=estimate_tax \
  in=analysis_result out=tax_estimate next=financial_supervisor
```

Genie generates:

```
agents/tax_estimator/
  tax_estimator.go      # full Agent implementation skeleton
  tax_estimator_test.go # passing table-driven test
```

It also prints the line you need to add to `cmd/api/main.go` and
`cmd/genie/main.go`:

```go
register(tax_estimator.New())
```

Fill in `HandleMessage`'s TODO with your domain logic.

---

## Testing

```bash
make test
```

This runs:

- Unit tests for every agent (table-driven, no I/O).
- `pkg/auth` JWT + bcrypt roundtrips.
- `pkg/crypto` envelope encryption roundtrips (with env KEK).
- `pkg/governance` policy decisions.
- End-to-end pipeline through the in-memory bus (`tests/integration_test.go`).

Postgres-backed integration tests are not wired in (no testcontainers
dependency yet). The repos are exposed via interfaces (`UserRepo`,
`AccountRepo`, `DocumentRepo`) so handler-level tests can substitute fakes.

---

## Configuration reference

| Variable | Required by | Description |
| --- | --- | --- |
| `GENIE_HTTP_ADDR` | `cmd/api` | listen address (default `:8080`) |
| `GENIE_JWT_SECRET` | `cmd/api` | HS256 secret bytes |
| `GENIE_KEK_BASE64` | `cmd/api` | 32-byte base64-encoded KEK |
| `GENIE_DB_DSN` | `cmd/api` | Postgres DSN |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `cmd/api` (optional) | enables OTLP exporter |
| `GENIE_OTEL_INSECURE` | `cmd/api` (optional) | `"true"` to skip TLS on OTLP |

Generate a key locally:

```bash
openssl rand -base64 32
```

---

## Roadmap

| Phase | Status | Notes |
| --- | --- | --- |
| Multi-agent platform | ✅ | MARA-aligned, message-driven |
| 9 core finance agents | ✅ | ingestor → reporter |
| 6 ADK-inspired agents | ✅ | currency, educator, macro, rate-watcher, loan, auditor |
| OTel traces + metrics | ✅ | stdout + OTLP exporters |
| HTTP API, JWT auth, RBAC | ✅ | chi + stdlib JWT |
| Postgres persistence | ✅ | pgx + embedded migrations |
| Envelope encryption (AES-GCM) | ✅ | env / KMS resolvers |
| Tempo + Grafana via compose | ✅ | local stack |
| CircleCI pipeline | ✅ | test + docker build |
| Scaffold generator | ✅ | `make scaffold name=...` |
| OpenAPI spec | ✅ | `docs/openapi.yaml` |
| Kubernetes manifests | 🚧 | kustomize overlays for local/prod |
| Postgres-backed eval store | 🚧 | currently in-memory |
| Key rotation | 🚧 | schema ready (`kek_id`), logic pending |
| Vector store / RAG knowledge layer | 🚧 | future tool integration |

---

## License

MIT. See [LICENSE](LICENSE) when added.

---

## References

- [Multi-Agent Reference Architecture](https://microsoft.github.io/multi-agent-reference-architecture/index.html)
- [Building blocks](https://microsoft.github.io/multi-agent-reference-architecture/docs/building-blocks/Building-Blocks.html)
- [Agents communication](https://microsoft.github.io/multi-agent-reference-architecture/docs/agents-communication/Agents-Communication.html)
- [Observability](https://microsoft.github.io/multi-agent-reference-architecture/docs/observability/Observability.html)
- [Security](https://microsoft.github.io/multi-agent-reference-architecture/docs/security/Security.html)
- [Google ADK samples — agent categories](https://github.com/google/adk-samples/tree/main/python/agents)
