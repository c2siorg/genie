# LinkedIn post — Genie / RBI FREE-AI

---

## Option A — the "show, don't tell" post (~1,900 chars)

The RBI FREE-AI report dropped 9 months ago.

7 Sutras. 6 Pillars. 26 Recommendations. The clearest mandate any major central bank has issued on responsible AI.

And yet most "AI in banking" demos today are still a chatbot wrapped around a US-hosted LLM with no audit trail, no incident form, and no clue where the customer's PAN just travelled.

So I built **Genie** — an open-source AI financial assistant in Go that treats every FREE-AI recommendation as a line of code, not a slide.

What's in the box:

→ **40+ specialist agents** (fraud, AML, LCR, VaR, tax harvester, cashflow underwriter, mule detector, complaint triage…) — each with a risk class, a schema, and its own test suite
→ **Board-approved policy as YAML** — the CRO edits a file, the running system obeys it (Rec 14)
→ **Ollama on-prem by default** — PII never leaves the home region; hosted LLMs only see public queries (Rec 4 + Sovereign AI)
→ **Envelope AES-256-GCM** encryption per document, KMS-pluggable (Rec 15)
→ **Annexure VI incidents** auto-generated on policy denial, panic, or budget breach (Rec 22)
→ **Red-team corpus** runs against the *active* policy on every commit (Rec 20)
→ **Tamper-evident hash-chained audit log** + consent ledger
→ **MCP + A2A + OAuth 2.1 + WebAuthn passkeys**
→ **OpenTelemetry traces** — every user question = one distributed trace across HTTP, governance, bus, every agent, every LLM call

Stack: Go 1.25 · chi · Postgres + pgvector · Ollama · Tempo · Grafana · MARA architecture.

57 test packages, all green. `go run ./cmd/genie` and the full pipeline runs in-process — no Postgres, no network. The sandbox **is** the production code (Rec 2).

The argument is simple: **responsible AI is a property of the system, not a property of the press release.** The only way to prove it is to publish the system.

MIT licensed. Clone it, break it, fork it.

→ https://github.com/c2siorg/genie

#ResponsibleAI #RBI #FREEAI #SovereignAI #FinTech #Golang #OpenSource #BankingAI #IndiaStack

---

## Option B — the "hook + story" post (~1,400 chars)

Most "responsible AI" claims in Indian banking fall apart the moment you ask one question:

*"Show me the file."*

Show me the file that says PII can't leave India. Show me the file that defines what counts as a high-risk agent. Show me the audit log that proves which policy was running on May 14. Show me the red-team corpus that fails CI when a guardrail breaks.

If those files don't exist, the policy is a poster.

I open-sourced **Genie** — an AI financial assistant in Go, built against the RBI FREE-AI report's 26 recommendations. Every claim has a `grep`-able file path:

📄 `config/ai-policy.example.yaml` → board-approved policy, loaded at boot (Rec 14)
📄 `pkg/governance/residency.go` → denies PII leaving home region (Rec 4)
📄 `pkg/incidents/annexure_vi.go` → auto-generates the regulator form (Rec 22)
📄 `cmd/red-team/` → adversarial probes vs the live policy (Rec 20)
📄 `pkg/compliance/audit.go` → hash-chained tamper-evident log
📄 `pkg/llm/ollama.go` → on-prem inference, region="on-prem"

40+ specialist agents. MCP + A2A. WebAuthn passkeys. OpenTelemetry end-to-end. Ollama by default, Anthropic/OpenAI/Gemini optional.

`go test ./...` → 57 packages green
`make red-team` → all probes denied as expected
`make compose-up` → full stack with Postgres + Tempo + Grafana

MIT. Built for the FREE-AI era.

→ https://github.com/c2siorg/genie

#RBI #FREEAI #ResponsibleAI #SovereignAI #FinTechIndia #Golang #OpenSource

---

## Option C — the punchy one-liner post (~900 chars)

The RBI FREE-AI report has 26 recommendations.

Most "AI in banking" pitches address none of them.

So I open-sourced **Genie** — an AI financial assistant in Go where every recommendation is a package:

✅ Board-approved policy as YAML
✅ Ollama on-prem for PII, hosted LLMs only for public queries
✅ Envelope AES-256-GCM encryption + KMS interface
✅ Annexure VI incidents auto-generated
✅ Hash-chained audit log + consent ledger
✅ Red-team corpus running in CI
✅ MCP + A2A + WebAuthn passkeys
✅ Full OpenTelemetry distributed traces
✅ 40+ specialist agents (fraud, AML, VaR, LCR, ALM, tax, lending…)

57 test packages. MIT licensed. Built on Microsoft's MARA architecture.

`go run ./cmd/genie` and the full pipeline runs on your laptop in 30 seconds.

Responsible AI is a property of the system, not the press release.

→ https://github.com/c2siorg/genie

#RBI #FREEAI #ResponsibleAI #SovereignAI #Golang
