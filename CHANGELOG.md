# Changelog

All notable changes to Genie are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **AFG framework-native edge** — a port of Genie's orchestration + provider
  layers onto Microsoft's Agent Framework for Go (`pkg/afg`, `cmd/af-serve`,
  `cmd/af-hello`). Serves `POST /v1/ask` + `GET /v1/ai-inventory` for the full
  58-agent governed catalog with no Postgres and no message bus, preserving the
  governance single-door invariant as agent-framework middleware.
- **Next.js browser console** — the embedded UI is now a Next.js app
  (`web-next/`), statically exported and committed into `pkg/web/handlers/ui/`
  (embedded via `//go:embed all:ui`). `go build` still needs no Node; regenerate
  with `make ui`. Clean, white, professional theme.
- **Concept glossaries** — `docs/glossary.md` (every load-bearing concept in the
  repo, grounded in real package paths) and `docs/langgraph-to-genie.md` (every
  LangGraph concept mapped to its Genie equivalent).

### Changed
- **`cmd/api` cut over to the agent framework — the message bus is removed from the
  production edge.** `POST /v1/ask` (and `/ask/stream`, `/chat/ws`) now run the governed
  `afg.QAService` pipeline inline instead of `comm.Bus` + `orchestration` +
  `busio.Correlator`. The pipeline drives the real `agents/*` logic, so its report is
  byte-identical to the old bus pipeline (oracle parity test in `pkg/afg/qa_service_test.go`).
  The in-process bus remains the `cmd/genie` CLI design and is preserved on the
  `legacy/bus-architecture` branch (`ec27f6b`). The `pkg/registry` in `cmd/api` now serves
  only as the AI-inventory / AIBOM / disclosures source.
- Wired the additional specialist agents into the inventory: the `[1.0.0]` release had
  **32** specialists wired; the running API now registers **58** specialists + 2 fallbacks
  (matching `README.md` / `CLAUDE.md`).
- The UI↔handler contract tests now assert against the compiled console bundle
  (API paths, auth fields, classification, SSE events, storage keys) instead of
  the retired hand-written `app.js`/`styles.css`.

### Deferred with the bus removal (tracked, documented in `cmd/api/main.go`)
- Incident-on-policy-deny / on-agent-error hooks and the auditor's eval-on-every-message
  subscription (were the orchestrator's / bus's job).
- MCP server bus-tools (`explain_finance` / `macro_context` / `rate_outlook`) and the
  `POST /mcp` route; `pkg/mcp` remains for a future agent-framework re-host.
- Per-hop SSE streaming (`/ask/stream`, `/chat/ws` now emit one progress event then the
  final report; a deterministic pipeline has no per-token stream to forward).
- BCP fallback routing for the `QAService` stages (`afg.Registry.RunWithFallback` exists
  but the pipeline does not yet route to it).

## [1.0.0] - 2026-07-08

First tagged release.

### Added
- `LICENSE` — the project is now released under the
  **PolyForm Noncommercial License 1.0.0** (see Changed/License below).
- `SECURITY.md` — private vulnerability disclosure policy.
- `CLAUDE.md` — repository guide, including a note that `geniepython/` and
  `microsoftagentframeworklearning/` are separate sibling projects.

### Changed
- **License:** relicensed from MIT to PolyForm Noncommercial 1.0.0 for v1.0.0
  onward. Noncommercial use only. Releases prior to v1.0.0 remain available under
  the MIT License; that grant continues to apply to those earlier versions.
- **README:** rewritten into a professional landing page (~1,700 → ~310 lines).
  Deep how-to content now links into `docs/`. Corrected the repository URL to this
  fork and stated the agent count accurately (32 specialist agents wired into
  the running API plus 2 fallbacks; 60+ implemented across the codebase).
- `.gitignore` now excludes the sibling projects and the `bin/` build output.

### Removed
- Six `docs/linkedin-*.md` marketing drafts (~1,056 lines) that carried stale
  "MIT licensed" claims, plus a stray root image left over from them.

### Fixed
- Corrected stale file paths in `docs/free-ai-mapping.md`
  (`pkg/incidents/annexure_vi.go` → `incidents.go`,
  `pkg/policy/loader.go` → `policy.go`).

[1.0.0]: https://github.com/PratikDhanave/genie/releases/tag/v1.0.0
