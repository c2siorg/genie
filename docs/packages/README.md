# Packages — Detailed Reference

This subtree documents the **seven platform packages** that landed with
the ADK-inspired extension wave. Each doc explains the package's
contract, its design rationale, integration points, and the FREE-AI /
Indian-banking concern it addresses.

The pre-existing platform packages (`pkg/agent`, `pkg/protocol`,
`pkg/registry`, `pkg/comm`, `pkg/orchestration`, `pkg/governance`, the
LLM provider stack, RAG, reasoning, etc.) are documented at the
package level via `doc.go` headers and the root README's "What lives
where" table.

---

## Index

| Doc | Where in code | Purpose |
|---|---|---|
| [policy-dsl.md](policy-dsl.md) | `pkg/policy/dsl/` | Tiny CEL-style expression DSL so the risk team can author governance rules without code changes (FREE-AI Rec 6) |
| [memory-longterm.md](memory-longterm.md) | `pkg/memory/longterm.go` | Third memory tier — append-only consolidated facts that survive across sessions |
| [loader-xlsx-and-ocr.md](loader-xlsx-and-ocr.md) | `pkg/loader/xlsx.go`, `pkg/loader/scanned_pdf.go` | XLSX cell extractor (stdlib-only) + Tesseract OCR fallback for scanned PDFs |
| [safety-plugins.md](safety-plugins.md) | `pkg/safety/plugin.go` | Pluggable safety guardrail plugins — uniform interface for jailbreak / Model Armor / Bedrock Guardrails / Lakera adapters |
| [agent-skill-registry.md](agent-skill-registry.md) | `pkg/agent/skill.go` | Progressive-disclosure skill manifests for supervisor agents to manage context bloat |
| [observability-bq.md](observability-bq.md) | `pkg/observability/bq/` | Warehouse JSONL sink — dual-write traces + metrics to BigQuery / Snowflake / Kafka for long-horizon agent analytics |
| [voice-streaming.md](voice-streaming.md) | `agents/voice/streaming.go` | Chunked streaming ASR/TTS for real conversational UX |

---

## Common design choices across all seven

- **Stdlib-only or near-stdlib-only.** Where possible (DSL, XLSX, memory, observability sink) we did not pull in third-party libraries. The dependency graph in `go.mod` stays small, the binary stays small, the audit surface stays small.
- **Pluggable interfaces over concrete implementations.** Every package exposes an interface a host application can implement (`Resolver`, `TrendFetcher`, `Plugin`, `Sink`, `Consolidator`, `StreamingVoiceProvider`). The shipped types are reference implementations.
- **Pure functions where possible.** Decision logic in the DSL, the OCR pipeline, the memory tier — all expressible as pure functions for unit-test reproducibility.
- **Honest about partial coverage.** Each doc names what the package does *not* do, so readers don't get surprised in production.

---

## Choosing the right package for the job

| You want to… | Use |
|---|---|
| Add a governance rule without shipping code | `pkg/policy/dsl` (then load the YAML in cmd/api) |
| Remember a fact about a user across sessions | `pkg/memory` LongTermMemory |
| Parse an Excel bank-statement export | `pkg/loader.XLSXLoader` |
| OCR a scanned KYC PDF | `pkg/loader.ScannedPDFLoader` or `AutoOCR` |
| Plug Model Armor as a guardrail | `pkg/safety.HTTPShield` + register in `safety.Registry` |
| Reduce context size on a supervisor with many sub-agents | `pkg/agent.SkillRegistry` |
| Dual-write agent traces to BigQuery | `pkg/observability/bq.JSONLSink` + a BQ load job |
| Stream ASR partials to a Bhashini WebSocket | `agents/voice.StreamingAgent` with a `StreamingVoiceProvider` impl |

---

## Conventions

- All packages compile clean under `go vet ./...`.
- All packages have a `_test.go` with hermetic tests (no live network).
- Every interface has at least one stub/reference implementation so
  callers can ship a working integration test before the real provider
  is wired.
