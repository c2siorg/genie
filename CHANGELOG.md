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
- The UI↔handler contract tests now assert against the compiled console bundle
  (API paths, auth fields, classification, SSE events, storage keys) instead of
  the retired hand-written `app.js`/`styles.css`.

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
