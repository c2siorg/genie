# HTTP API Reference

> Every endpoint Genie exposes, with sample curl, expected status codes,
> and gotchas. Companion to `docs/openapi.yaml` (the authoritative spec).

---

## Conventions

- Base URL: `http://localhost:8080/v1` for local dev.
- Auth: `Authorization: Bearer <jwt>` on all `/v1/*` endpoints except
  `/v1/users`, `/v1/users/login`, `/v1/disclosures`, the WebAuthn /
  OAuth endpoints, and `/v1/oauth/device/*`.
- Content type: `application/json` unless noted (CSV upload, audio
  upload).
- IDs: UUIDv4 strings.
- Timestamps: ISO 8601 UTC.
- Money: minor units (₹1 = 100) for ledger fields; rupees (₹1 = 1.0)
  for display fields. Each agent's doc says which.

---

## Health & disclosures

### `GET /healthz`

Liveness probe. Returns `200 {"status":"ok"}` if the process is up.

### `GET /readyz`

Readiness probe. Returns `200 {"status":"ok"}` if Postgres is reachable
and the LLM stack is configured. `500` otherwise.

### `GET /v1/disclosures`

Public, unauthenticated. Returns the active policy version, the FREE-AI
7 principles, agent counts by risk class, and the consumer AI-disclosure
banner. **FREE-AI Rec 25**.

```bash
curl -s http://localhost:8080/v1/disclosures | jq .
```

```json
{
  "agent_counts": { "high": 7, "low": 18, "medium": 9, "total": 34 },
  "home_region": "in",
  "incident_reporting_url": "/v1/incidents",
  "policy_approved_on": "2025-08-13",
  "policy_version": "0.1.0",
  "principles": ["Trust is the Foundation", "..."]
}
```

---

## Auth

### `POST /v1/users` — sign up

```bash
curl -s -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","name":"Alice","password":"hunter2hunter2"}'
```

Response `201`:

```json
{
  "token": "eyJ...",
  "expires_at": 1779612599,
  "user": {"id": "uuid", "email": "...", "name": "...", "roles": ["user"]}
}
```

Password rules: bcrypt-hashed; minimum 8 chars; never returned in any response.

### `POST /v1/users/login`

```bash
curl -s -X POST http://localhost:8080/v1/users/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"hunter2hunter2"}'
```

Same response shape as signup. Tokens are HS256-signed JWTs with 60-min
TTL. Roles are encoded at issue time — to pick up a role change you
must re-login.

### `GET /v1/users/me`

```bash
curl -s http://localhost:8080/v1/users/me -H "Authorization: Bearer $TOKEN"
```

### `POST /v1/oauth/device/*` (RFC 8628)

Replaces the manual paste-your-MCP-token-here flow. See [protocols.md](protocols.md).

### `POST /v1/webauthn/register/begin|finish` and `/v1/webauthn/login/begin|finish`

Passwordless passkey flow (Ed25519). See [protocols.md](protocols.md).

---

## Documents (encrypted at rest)

### `POST /v1/documents` — upload

```bash
curl -s -X POST 'http://localhost:8080/v1/documents?description=Jan%20statement&classification=pii' \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @data/sample.csv
```

The body is encrypted with a fresh DEK, the DEK wrapped by the active
KEK, and the envelope stored in `documents.payload` JSONB. Response:

```json
{"id":"uuid","classification":"pii","kek_id":"...","expires_at":"..."}
```

Classification options: `public`, `internal`, `pii`, `secret`.

### `GET /v1/documents` — list your docs

Returns metadata only; never plaintext. Pagination via `?limit=N&cursor=X`.

### `GET /v1/documents/:id` — fetch metadata

Same shape as the list item.

---

## Ask Genie

### `POST /v1/ask` — synchronous

```bash
curl -s -X POST http://localhost:8080/v1/ask \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"question":"Where am I overspending?","document_id":"<doc-id>"}' \
  | jq .
```

Response:

```json
{
  "trace_id": "tr-...",
  "report": "...final report text...",
  "ai_disclosure": "This response was produced by an AI pipeline..."
}
```

Behind the scenes: governance evaluates → bus publishes the question
to `financial_supervisor` → 8-stage pipeline runs → reporter emits the
final report → busio.Correlator wakes this HTTP handler.

### `POST /v1/ask/stream` — Server-Sent Events

```bash
curl -N -X POST http://localhost:8080/v1/ask/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"question":"...","document_id":"..."}'
```

Event stream:

```
event: ai_disclosure
data: This response was produced by an AI pipeline...

event: trace
data: tr-1779514412090640000

event: agent.handle
data: {"from":"ingestor","to":"normalizer","type":"raw_transactions",...}

...

event: report
data: Genie Financial Report ...
```

Useful for real-time UI updates as agents fire.

### `GET /v1/chat/ws` — WebSocket

Bi-directional chat. Same auth + RBAC. Each inbound JSON is a question;
each outbound is an agent event. Implementations in `pkg/web/handlers`.

---

## Governance & inventory (admin-only)

### `GET /v1/ai-inventory`

```bash
curl -s -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/v1/ai-inventory | jq .
```

Returns every registered agent with `id`, `name`, `capabilities`,
`risk_class`, `has_fallback`, `fallback_id`. Live — read from the
registry. **FREE-AI Rec 23**.

### `GET /v1/aibom`

Returns the AIBOM as CycloneDX 1.6 ML-BOM JSON, Ed25519-signed.
**FREE-AI Rec 23 + 24**.

### `GET /v1/incidents?limit=20`

Returns recent incidents (deny events, agent panics, budget breaches).
Annexure VI-shaped. **FREE-AI Rec 22**.

### `GET /v1/audit/log`

Returns the hash-chained audit log (newest first, paginated).

---

## MCP

### `POST /mcp` — Genie as MCP server

JSON-RPC streamable HTTP. Genie exposes a curated set of read-only
agents (`financial_educator`, `macro_research`, `rate_watcher`) as MCP
tools. Compatible with Claude Desktop, Cursor, and any MCP client.

### `POST /v1/mcp/tokens` — store an MCP token

Encrypts and stores third-party MCP credentials (e.g. Zerodha Kite). The
plaintext never leaves Postgres.

---

## Pprof (admin-only)

```bash
go tool pprof "http://localhost:8080/debug/pprof/heap?seconds=30" \
  -header "Authorization=Bearer $ADMIN_TOKEN"
```

Standard library `net/http/pprof` under `/debug/pprof/*`, behind JWT
auth + admin role.

---

## Error responses

All errors are JSON of shape:

```json
{"error": "<short_code>", "message": "<human-readable detail>"}
```

| Status | When |
|---|---|
| `400` | Invalid request shape |
| `401` | Missing/invalid JWT |
| `403` | Role mismatch (RBAC denial) |
| `404` | Document or resource not found |
| `409` | Conflict (duplicate signup, idempotency collision) |
| `413` | Payload too large (governance.MaxContentLengthPolicy) |
| `422` | Governance policy denial (PII, injection, residency, etc.) |
| `429` | Rate limit |
| `500` | Internal error — check `genie-api` logs |
| `503` | Stack not ready (Postgres unreachable, LLM down) |

---

## Rate limits

Per-principal token bucket via `pkg/web/mid.RateLimit`. Defaults:

- 60 requests / minute on `/v1/ask*`
- 600 / minute on `/healthz` and `/readyz`
- 10 / minute on `/v1/users` (sign-up flood protection)

Override via env if you fork.

---

## Spec

`docs/openapi.yaml` is the authoritative spec; validate via:

```bash
make openapi-validate
```

`docs/asyncapi.yaml` documents the bus event catalogue (the events that
appear on the SSE stream).
