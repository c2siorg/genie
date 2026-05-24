# Protocols Reference

> Detailed walkthrough of every protocol Genie speaks: MCP, A2A,
> CloudEvents, OAuth 2.1 + PKCE, OAuth Device Flow (RFC 8628),
> WebAuthn passkeys, OpenInference semantic conventions, CycloneDX 1.6
> ML-BOM.

---

## Model Context Protocol (MCP)

> Source: <https://modelcontextprotocol.io/>
> Spec: JSON-RPC 2.0 over streamable HTTP

### Genie as MCP **client**

`pkg/mcp/client` connects to any MCP server (Zerodha Kite hosted at
`https://mcp.kite.trade/mcp`, internal tool servers, etc.). The
`agents/portfolio_advisor` agent uses it to fetch holdings + positions
on demand.

```go
client := mcp.NewClient("https://mcp.kite.trade/mcp", bearerToken)
client.Initialize(ctx)
holdings, _ := client.CallTool(ctx, "get_holdings", nil)
```

### Genie as MCP **server**

`pkg/mcp/server` exposes Genie's read-only agents as MCP tools at
`/mcp`. Compatible with Claude Desktop, Cursor, any MCP-aware UI.

Currently exposed tools:

- `financial_education` — RAG-backed Q&A
- `macro_research` — one-shot macro brief
- `rate_watcher` — live rate snapshot

Writeable agents (recommender, payment_orchestrator) are intentionally
**not** exposed via MCP because the protocol doesn't carry per-tool
authz — a hostile client could misuse them.

### OAuth device flow for MCP onboarding

Replaces the manual paste-your-session-token anti-pattern with
RFC 8628:

```
POST /v1/oauth/device/authorize
→ {device_code, user_code "WDJF-K9X3", verification_uri, interval}

User opens verification_uri, types user_code, logs in via IdP.

POST /v1/oauth/device/token (polled every interval seconds)
→ authorization_pending
→ ...
→ {access_token, token_type, expires_in}
```

User codes are Crockford-style base32 (no 0/1/8/O) for typability.

---

## Agent-to-Agent (A2A)

> Source: <https://github.com/google/a2a>

Symmetric to MCP but at the **agent** granularity instead of per-tool.
Lets one Genie instance call another (or any A2A-compliant peer) as a
first-class peer agent.

### Genie as A2A client

```go
client := a2a.NewClient("https://peer-genie.example/a2a")
card, _ := client.GetAgentCard(ctx)        // discover skills
result, _ := client.SubmitTask(ctx, a2a.Task{
    SkillID: "explain_finance",
    Input:   map[string]any{"query": "what is a SIP?"},
})
```

### Genie as A2A server

`pkg/a2a/server` exposes agents via `agent/getCard` (capability
discovery) and `task/submit` (synchronous task execution). The shipped
default exposes the same read-only set as MCP.

### When to use A2A vs MCP

- **MCP**: cross-process tool calls; the calling LLM treats Genie's
  outputs as tool results in its reasoning loop.
- **A2A**: cross-process agent calls; Genie's calling agent treats the
  remote as another agent in the pipeline. Higher granularity, fewer
  hops.

---

## CloudEvents 1.0

> Source: <https://cloudevents.io/>

`pkg/cloudevents.Wrap(msg, source)` adapts any bus `Message` to a
CloudEvents 1.0 structured-mode JSON envelope:

```json
{
  "specversion": "1.0",
  "id": "uuid",
  "source": "genie://bus",
  "type": "com.c2siorg.genie.finance_question",
  "datacontenttype": "application/json",
  "genieclassification": "pii",
  "genietraceid": "tr-...",
  "data": { ...original message... }
}
```

Use case: forward Genie's bus to a Knative consumer, Kafka topic, or
AWS EventBridge bus. The `Unwrap` reverses the transform back into a
`protocol.Message`.

---

## AsyncAPI 3.0

`docs/asyncapi.yaml` is the bus event catalogue. Every message type
that flows on the bus is documented there — request/response shapes,
metadata requirements, producing agent. Useful when wiring an external
consumer.

---

## OAuth 2.1 + PKCE

> Spec: <https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/>

`pkg/auth/oauth2` implements the OAuth 2.1 authorisation-code flow with
mandatory PKCE (OAuth 2.1 forbids the older `plain` code-challenge
method — only `S256` is allowed).

```go
verifier, challenge, _ := oauth2.GenerateVerifier()
authResp, _ := server.Authorize(oauth2.AuthorizeRequest{
    ClientID:            "my-app",
    CodeChallenge:       challenge,
    CodeChallengeMethod: oauth2.MethodS256,
}, subject, email, roles)
tok, _ := server.Token(oauth2.TokenRequest{
    Code:         authResp.Code,
    CodeVerifier: verifier,
    ClientID:     "my-app",
})
```

Used internally for delegated clients (the upcoming mobile app, the
upcoming partner portal).

---

## OAuth Device Flow (RFC 8628)

`pkg/auth/oauth_device` implements RFC 8628. Used by:

- The Genie CLI to onboard MCP credentials without copy-paste tokens.
- Future TV / kiosk surfaces.

See the MCP section above for the user-facing flow.

---

## WebAuthn (passwordless passkeys)

`pkg/auth/webauthn` implements WebAuthn Level 3 with Ed25519 attestation.

### Registration flow

```
Browser                            API                    pkg/auth/webauthn
   │   POST /v1/webauthn/register/begin   │                       │
   │ ─────────────────────────────────────►│                       │
   │                                       │ BeginRegistration(uid)│
   │                                       │ ──────────────────────►
   │                                       │      ceremonyID,      │
   │                                       │      challenge        │
   │                                       │ ◄──────────────────────
   │       {challenge}                     │                       │
   │ ◄─────────────────────────────────────│                       │
   │                                       │                       │
   │ navigator.credentials.create()        │                       │
   │   (user touches Touch ID / Yubikey)   │                       │
   │                                       │                       │
   │ POST /v1/webauthn/register/finish     │                       │
   │ ─────────────────────────────────────►│ FinishRegistration(...)
   │   {credentialID, publicKey}           │ ──────────────────────►
   │                                       │ ◄──────────────────────
   │ 201                                   │                       │
   │ ◄─────────────────────────────────────│                       │
```

### Login flow

Same shape with `BeginAssertion` / `FinishAssertion`. Server verifies
the Ed25519 signature over the challenge, then issues a JWT.

Phishing-resistant by construction — the passkey is bound to the
origin.

---

## OpenInference semantic conventions

> Source: <https://github.com/Arize-ai/openinference>

LLM spans carry the OpenInference attributes:

- `llm.provider` (e.g. `"ollama"`, `"anthropic"`)
- `llm.model_name`
- `llm.token_count.prompt`
- `llm.token_count.completion`
- `llm.token_count.total`
- `llm.input_messages` / `llm.output_messages`
- `llm.cost` (where cost data is available)

Arize Phoenix, Langfuse, Helicone all consume these unchanged. No
extra wiring needed.

---

## CycloneDX 1.6 ML-BOM

> Source: <https://cyclonedx.org/capabilities/mlbom/>

`pkg/aibom.Manifest.ToCycloneDX()` produces a CycloneDX 1.6 ML-BOM
JSON document containing:

- Every registered agent with id, kind, capability, risk class
- Every LLM provider in use (model name, model family, hosted-or-on-prem)
- Every embedder
- Training-data classifications (where declared)
- Last-audited timestamps

Signed via `aibom.NewEd25519Signer()` → `aibom.Sign(doc, signer)`
produces a `SignedDocument` an external auditor can verify with the
issuer's public key.

Use case: regulator asks "what's running in your AI stack?" — you hand
them the signed ML-BOM. Live, generated on demand from the registry.

---

## W3C DIDs + Verifiable Credentials

`pkg/identity` ships minimum-viable **did:key** (Ed25519) +
**W3C Verifiable Credentials 1.1** with `Ed25519Signature2020` proofs:

```go
issuer, _ := identity.NewDIDKey()  // did:key:z...
vc := &identity.VerifiableCredential{
    Type: []string{"VerifiableCredential", "GenieAgentManifest"},
    CredentialSubject: map[string]any{
        "agentId":   "ingestor",
        "riskClass": "low",
        "auditedOn": "2026-05-23",
    },
}
identity.IssueVC(issuer, vc)
identity.VerifyVC(vc, issuer.Public)
```

Pairs with the AIBOM — each agent's manifest can be packaged as a VC
and shared with regulators in a portable, verifiable format.

---

## Summary table

| Protocol | Used by Genie as | Spec | Package |
|---|---|---|---|
| MCP | Client + Server | <https://modelcontextprotocol.io/> | `pkg/mcp` |
| A2A | Client + Server | <https://github.com/google/a2a> | `pkg/a2a` |
| CloudEvents 1.0 | Outbound envelope | <https://cloudevents.io/> | `pkg/cloudevents` |
| AsyncAPI 3.0 | Bus event spec | <https://asyncapi.com/> | `docs/asyncapi.yaml` |
| OpenAPI 3.x | HTTP spec | <https://swagger.io/specification/> | `docs/openapi.yaml` |
| OAuth 2.1 + PKCE | Authz server | draft-ietf-oauth-v2-1 | `pkg/auth/oauth2` |
| OAuth Device Flow | Authz client + server | RFC 8628 | `pkg/auth/oauth_device` |
| WebAuthn L3 | Authn (passkeys) | <https://www.w3.org/TR/webauthn-3/> | `pkg/auth/webauthn` |
| OpenInference semconv | OTel attribute set | <https://github.com/Arize-ai/openinference> | `pkg/observability` |
| CycloneDX 1.6 ML-BOM | AIBOM format | <https://cyclonedx.org/> | `pkg/aibom` |
| W3C DIDs | Identity | <https://www.w3.org/TR/did-core/> | `pkg/identity` |
| W3C VCs | Credentials | <https://www.w3.org/TR/vc-data-model/> | `pkg/identity` |
