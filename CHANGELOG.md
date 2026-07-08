# Changelog

All notable changes to Genie are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
