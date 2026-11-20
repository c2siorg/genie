# Operations Guide

> Everything you need to run Genie locally and in production —
> dependencies, configuration, environment variables, observability, key
> rotation, troubleshooting.

---

## Three ways to run

| Mode | Command | What it needs | Use when |
|---|---|---|---|
| **CLI demo** (in-process pipeline) | `go run ./cmd/genie` | Go 1.25+ only | Sandbox, FREE-AI Rec 2 demo, CI smoke |
| **API only** (Postgres + Ollama + Genie) | `make run-api` | Go, Postgres, Ollama, KEK | Local dev with HTTP |
| **Full stack** (above + OTel + Tempo + Grafana) | `make up` | Docker + Compose | Local testing, demos to stakeholders |

`make up` is the recommended path for first-time users.

---

## Quick start (full stack)

```bash
git clone https://github.com/c2siorg/genie.git
cd genie

# Sanity check
go test ./...           # 100+ packages, all green
go vet ./...            # clean

# Boot the full stack
make up

# Wait for "ready" + URL banner:
#   Genie UI:    http://localhost:8080/
#   API health:  http://localhost:8080/healthz
#   Disclosures: http://localhost:8080/v1/disclosures
#   Grafana:     http://localhost:3000/  (admin/admin)

# Smoke test
make smoke

# Bring it down
make down
```

`make up` brings up: Postgres 16, Tempo, Grafana, OTel Collector,
Ollama (with `llama3.2:1b` + `nomic-embed-text` pulled by `ollama-pull`),
and `genie-api`.

---

## Environment variables (complete reference)

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `GENIE_HTTP_ADDR` | no | `:8080` | Listen address |
| `GENIE_JWT_SECRET` | **yes** | — | HS256 secret bytes for JWT signing |
| `GENIE_KEK_BASE64` | **yes** | — | 32-byte base64 key for envelope encryption (KEK) |
| `GENIE_DB_DSN` | **yes** | — | Postgres DSN (`postgres://user:pass@host:port/db?sslmode=disable`) |
| `GENIE_AI_POLICY` | no | `config/ai-policy.example.yaml` | Path to board-approved policy YAML |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | — | Enables OTLP exporter; leave unset for stdout |
| `GENIE_OTEL_INSECURE` | no | `false` | `true` to skip TLS on OTLP gRPC |
| `GENIE_LLM` | no | `mock` | `mock` (default) or `ollama` (real local LLM) |
| `GENIE_OLLAMA_URL` | no | `http://localhost:11434` | Ollama base URL |
| `GENIE_OLLAMA_CHAT` | no | `llama3.2:1b` | Chat model tag |
| `GENIE_OLLAMA_EMBED` | no | `nomic-embed-text` | Embedding model tag |
| `GENIE_LLM_BUDGET` | no | `1_000_000` | Daily token cap per principal |
| `GENIE_LLM_CACHE_TTL` | no | `600` | LLM response cache TTL (seconds) |
| `GENIE_LLM_TIMEOUT` | no | `30` | Per-LLM-call timeout (seconds) |
| `GENIE_LLM_CIRCUIT` | no | `5` | Circuit-breaker error threshold |

### Generating secrets

```bash
# JWT secret — 32 random bytes hex-encoded
openssl rand -hex 32

# KEK — 32 random bytes base64-encoded
openssl rand -base64 32
```

Set these in a `.env` file (never commit) or your secret manager.

---

## File system layout (what lives where in production)

```
/var/lib/genie/
├── (no document plaintext lives here — everything is in Postgres)
└── (no key material lives here — KEK is in KMS, JWT secret in env)

/etc/genie/
├── ai-policy.yaml        ← board-approved YAML, version-controlled
└── constitution.yaml     ← 7 Sutras

/var/log/genie/           ← optional file output for slog if you don't use stdout
```

In Docker / Kubernetes deployments these are mount-bound or
ConfigMap-bound; the container image itself is read-only.

---

## Bringing up Postgres alone (for `make run-api`)

If you don't want the full compose stack:

```bash
docker run -d --name genie-pg \
  -e POSTGRES_USER=genie -e POSTGRES_PASSWORD=genie -e POSTGRES_DB=genie \
  -p 5432:5432 \
  postgres:16
```

Then export:

```bash
export GENIE_DB_DSN="postgres://genie:genie@localhost:5432/genie?sslmode=disable"
export GENIE_JWT_SECRET="$(openssl rand -hex 32)"
export GENIE_KEK_BASE64="$(openssl rand -base64 32)"
export GENIE_LLM=mock      # or ollama if you have it running

make run-api
```

Migrations run on boot via `pkg/storage/postgres` embedded SQL files.

---

## Observability

### Local (default OTel stdout exporter)

`cmd/genie` writes traces to `genie-traces.json` in the working directory.
Open in a viewer of choice.

`cmd/api` defaults to OTLP if `OTEL_EXPORTER_OTLP_ENDPOINT` is set,
stdout otherwise.

### Compose stack — Tempo + Grafana

`make up` wires:

- **OTel Collector** at `:4317` (gRPC OTLP receiver)
- **Tempo** at `:3200` (trace backend)
- **Grafana** at `:3000` (UI; anonymous admin)

In Grafana → Explore → Tempo, search by service `genie-api`. Every
`/v1/ask` request appears as one distributed trace.

### Metrics

- `genie.bus.messages_published` — total messages
- `genie.agent.messages_handled` — per agent
- `genie.governance.denials` — per policy
- `genie.agent.errors` — per agent
- `genie.agent.handle_duration_ms` — histogram per agent
- `genie.llm.tokens` / `cost_micros` / `latency_ms`

OpenInference semantic conventions on LLM spans — picked up unchanged
by Arize Phoenix, Langfuse, and other LLM observability platforms.

### Long-horizon (BigQuery / Snowflake)

`pkg/observability/bq` provides a warehouse sink. See
[packages/observability-bq.md](packages/observability-bq.md) for
schema + wiring.

---

## Key rotation

Document encryption uses envelope encryption — fresh DEK per document,
wrapped by an active KEK. The schema stores `kek_id` per row so
rotation works without re-encrypting all documents at once:

1. **Generate a new KEK** in KMS, assign it a new `kek_id`.
2. **Update `GENIE_KEK_BASE64`** (or the KMS pointer) on the API.
3. **New uploads** automatically use the new KEK.
4. **Old documents** keep decrypting with their original KEK (the
   `kek_id` on each row tells the `KeyResolver` which KEK to use).
5. **Background rewrap job** (roadmap) re-encrypts older rows lazily.

Until the rewrap job ships, leave both old and new KEKs available in
your `KeyResolver`. Most KMS systems support multi-key rotation
natively (AWS KMS aliases, GCP KMS key versions).

---

## JWT secret rotation

Less complex than KEK rotation because tokens are short-lived (60 min):

1. Generate a new `GENIE_JWT_SECRET`.
2. Deploy with both old and new in a comma-separated env var (if your
   build supports it) — old tokens validate against the old secret, new
   tokens are issued against the new.
3. After 60 minutes, drop the old secret.

For zero-downtime, run two replicas: one with old secret as primary
verifier, one with new. Cycle.

---

## Q1 hardening — security primitives runbook

The four primitives shipped in the Q1 hardening pass each have a
specific operational concern. Read this section alongside
[packages/postgres-rls.md](packages/postgres-rls.md),
[packages/governance-tenant.md](packages/governance-tenant.md),
[packages/oauth-token-exchange.md](packages/oauth-token-exchange.md),
and [packages/agent-tier.md](packages/agent-tier.md).

### Running the RLS migration (`0005_rls.sql`)

The migration auto-runs at boot via `pkg/storage/postgres`'s embedded
SQL files. To run it manually against a managed Postgres:

```bash
docker compose exec postgres psql -U genie -d genie \
  -f /docker-entrypoint-initdb.d/0005_rls.sql
```

Verify RLS is forced on every table that should have it:

```sql
SELECT relname, relrowsecurity, relforcerowsecurity
  FROM pg_class
 WHERE relname IN ('documents','accounts','mcp_tokens','incidents','users');
-- expect relrowsecurity = t AND relforcerowsecurity = t for all five
```

If `relforcerowsecurity = f`, the policy is honoured for non-owner
roles but bypassed by the table owner. That's a configuration drift —
re-run the migration.

### Wiring tenant context into a new handler

```go
// HTTP handler that needs tenant-scoped reads
func (h *Handler) GetDocs(w http.ResponseWriter, r *http.Request) {
    claims, _ := mid.ClaimsFrom(r.Context())
    err := h.DB.WithTenant(r.Context(), claims.Subject, func(ctx context.Context, tx pgx.Tx) error {
        rows, err := tx.Query(ctx, "SELECT id, description FROM documents")
        // RLS applies — no WHERE needed; the GUC scopes the read
        return scan(rows)
    })
    ...
}
```

For admin-only routes (audit reader, inventory):

```go
err := h.DB.WithAdminContext(r.Context(), func(ctx context.Context, tx pgx.Tx) error {
    // sentinel '__admin__' is set; cross-tenant reads allowed
    ...
})
```

`WithAdminContext` must be gated by `mid.RequireRole(auth.RoleAdmin)`
at the router. The sentinel is the only legitimate cross-tenant key.

### Wiring TenantPolicy into the bus

```go
policy := governance.NewComposite(
    governance.RBACPolicy{
        RequiredRolesByType: map[string][]string{
            "finance_question": {"user", "admin"},
            "kyc_submit":       {"user", "admin"},
            "payment_request":  {"user", "admin"},
        },
        AdminBypass: true,
    },
    governance.TenantPolicy{
        AppliesTo: []string{"finance_question", "kyc_submit", "payment_request"},
        // AdminBypass left false — admin role does NOT cross tenants
        // on customer-facing routes. Use a separate policy stack for
        // admin-only routes if cross-tenant is needed there.
    },
    governance.MaxContentLengthPolicy{Max: 16 * 1024},
)
orch := orchestration.NewOrchestrator(reg, bus, policy, env)
```

The HTTP intake layer must populate `metadata.tenant_id` and
`metadata.expected_tenant` on every message. The orchestrator's
`OnPolicyDeny` hook should emit a metric so cross-tenant attempts
trigger an alert.

### Wiring the token-exchange service

```go
issuer := auth.NewIssuer(jwtSecret, "genie-api", []string{"genie-api"}, ttl)
exch   := tokenexchange.New(issuer, "genie-api")

// In the agent runtime, before calling an MCP server:
mcpToken, _, err := exch.Exchange(ctx, tokenexchange.Request{
    SubjectToken: userJWT,
    ActorID:      "kyc_orchestrator",
    Audience:     "mcp://kyc-server",
})
// pass mcpToken to the MCP client; downstream sees Subject=user, Actor=agent

// On logout / password change:
exch.Invalidate(userSubject)
```

The exchange Service is goroutine-safe. Per host, one Service is
sufficient. Cache TTL defaults to `min(token_exp, subject_exp) − 60s`;
override `SafetyMargin` for clock-skew-prone environments.

### Tier promotion checklist

Before promoting an agent from `TierBeta` to `TierProduction`:

- [ ] Agent declares `Tier() Tier { return TierProduction }` in code.
- [ ] Agent has a `RiskLevel()` declaration (or accept default RiskLow).
- [ ] Agent has unit tests covering each branch of `HandleMessage`.
- [ ] Agent has at least one integration test in `tests/` that hits
      it through the bus.
- [ ] Adversarial corpus (`pkg/safety` plugin run) passes — at minimum
      the prompt-injection and jailbreak suites.
- [ ] Fallback wired via `orchestrator.SetFallback(<id>, <fallback>)`
      and a BCP drill (`make bcp-drill`) confirms the fallback fires.
- [ ] Audit hooks emit on every output — verify via
      `make smoke && curl /v1/audit | jq`.
- [ ] Entry appears in `/v1/ai-inventory` with `tier = "production"`.
- [ ] Risk team has signed off in the deployment record.

Promote in a single commit so the tier change is auditable; the
deployment record carries the link.

### Verifying the security envelope holds

The end-to-end test that exercises tier + tenant + token exchange +
fallback together:

```bash
go test ./tests/... -run TestSecurityEnvelope -v
```

Every named test in that suite must pass for the defence-in-depth
contract to hold. If a test starts failing after a change, treat it
as a security regression and revert before debugging.

### Privileged Access Manager — time-bound elevation runbook

Production-grade configuration (4-eyes, 1h cap) in cmd/api wire-up:

```go
elevationSvc := elevation.New(auditLog)
elevationSvc.RequireApprovers = 2          // 4-eyes
elevationSvc.MaxDuration       = 1 * time.Hour
```

**Engineer flow** — needs admin for a real incident:

```bash
# 1. File the request (any authenticated user)
TOKEN=...   # engineer's normal token
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role":"admin","reason":"Investigate INC-2026-0042","ttl_seconds":1800}' \
  http://localhost:8080/v1/elevation/requests | jq .

# Returns: { "id": "uuid...", "status": "pending", ... }
```

**Approver flow** — admin reviews and approves:

```bash
# 2. List pending requests
ADMIN=...   # admin's token
curl -s -H "Authorization: Bearer $ADMIN" \
  "http://localhost:8080/v1/elevation/requests?limit=20" | jq '.[] | select(.status=="pending")'

# 3. Approve
curl -s -X POST -H "Authorization: Bearer $ADMIN" \
  http://localhost:8080/v1/elevation/requests/$GRANT_ID/approve | jq .

# Under 4-eyes, a SECOND admin must also approve before the grant
# becomes active. Each admin can approve at most once per grant.
```

**Revoke flow** — incident is closed early:

```bash
curl -s -X POST -H "Authorization: Bearer $ADMIN" \
  -H "Content-Type: application/json" \
  -d '{"reason":"INC-2026-0042 resolved, dropping elevation"}' \
  http://localhost:8080/v1/elevation/requests/$GRANT_ID/revoke | jq .
```

**Audit trail** — every transition is in the hash-chained log under
the `elevation.*` action prefix, linked by `audit_root = original
grant audit seq`. To pull a grant's full history:

```bash
curl -s -H "Authorization: Bearer $ADMIN" /v1/audit/log | \
  jq '.[] | select(.details.audit_root == '"$GRANT_AUDIT_SEQ"')'
```

---

## Adding a new LLM provider

1. Implement `pkg/llm.Provider` (and optionally `Embedder`, `VisionProvider`).
2. Return `"on-prem"` from `Region()` if it runs on your infra,
   otherwise `"us"` / `"eu"` / etc. — `DataResidencyPolicy` reads this.
3. Wire in `cmd/api/llmstack.go`, behind a `GENIE_LLM=<your-provider>` env switch.
4. Add a CI smoke that hits the provider with a mock prompt.

---

## Adding a new agent

```bash
make scaffold name=my_agent cap=my_capability in=my_request out=my_response next=financial_supervisor
```

This generates `agents/my_agent/my_agent.go` + a passing test. Replace
the `TODO` in `HandleMessage` with real logic, declare a `RiskLevel()`,
emit a `Disclaimer` on every output, and (for high-risk rejects) an
`IncidentPayload`. Register in `cmd/api/main.go` and `cmd/genie/main.go`.
Add a doc at `docs/agents/my_agent.md`.

`tests/agents_registry/` enforces ID uniqueness and `_test.go`
presence. CI will fail if you forget either.

---

## Common troubleshooting

### `/readyz` returns 500

- Postgres unreachable → check `GENIE_DB_DSN` and that the DB accepts
  connections.
- Migrations failed → check `genie-api` logs (`docker compose logs
  genie-api`). Most failures are typoed DSN or insufficient grants.

### `/v1/ai-inventory` returns 403

Your JWT lacks `admin` role. Promote your test user:

```bash
docker compose exec postgres psql -U genie -d genie -c \
  "UPDATE users SET roles = ARRAY['user','admin'] WHERE email = '<your-email>';"
```

Then **log out and back in** so a fresh token with the new roles is
issued — old tokens encode the roles at issue time.

### LLM calls time out

- `GENIE_LLM_TIMEOUT` defaults to 30s — increase for slow local Ollama.
- If using Ollama with a model that isn't pulled yet, the first call
  triggers a pull and takes minutes. Pre-pull via `ollama pull <model>`.

### Compose stack fails to come up

- Check Docker daemon is running: `docker info`.
- Free ports: 8080 (api), 5432 (pg), 3000 (grafana), 3200 (tempo), 4317
  (otel), 11434 (ollama).
- `make down` to clean, then `make up` again — sometimes the
  ollama-pull container hangs on first run; second run is fine.

### "agent not found in registry"

The orchestrator looked up an agent by ID that wasn't registered in
`cmd/api/main.go`. Either your agent isn't wired (add a `register(...)`
call) or your message has a typo in `msg.To`.

### A high-risk agent doesn't fire

Check `metadata.user_roles` on the message — `RiskHigh` requires
`advisor` or `admin`. The orchestrator drops messages that don't qualify
and records an incident.

---

## Production checklist

- [ ] `GENIE_JWT_SECRET` and `GENIE_KEK_BASE64` are in your secret manager, not in env files
- [ ] Postgres is on a private subnet, with TLS
- [ ] OTel collector ships traces to your APM (Tempo / Honeycomb / Datadog)
- [ ] Grafana is access-controlled (not anonymous-admin like the dev compose default)
- [ ] Ollama runs on a GPU host inside your VPC; not exposed publicly
- [ ] `config/ai-policy.example.yaml` is replaced with your board-approved version, with `board_approved_on` and `owner` filled
- [ ] CI runs `make red-team` and `make bcp-drill` on every PR
- [ ] PII regex (`pkg/governance/pii.go`) tuned for your jurisdiction
- [ ] `pkg/sovereignty.ProviderRegistry` is the allowlist of external LLM providers your bank actually permits
- [ ] `make compose-up` is replaced by Kubernetes manifests (roadmap; for now compose works in a single-node dev box)
