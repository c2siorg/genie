# Contributing to Genie

Thanks for your interest. Genie is an AI financial assistant in Go, built on
Microsoft's Multi-Agent Reference Architecture (MARA) and aligned with the RBI
FREE-AI framework. This guide covers how to build, test, and land a change.

## Ground rules

- Be respectful — see the [Code of Conduct](CODE_OF_CONDUCT.md).
- Genie is licensed under **PolyForm Noncommercial 1.0.0** ([LICENSE](LICENSE));
  by contributing you agree your contribution is licensed under the same terms.
- **Never** commit secrets, real credentials, or customer data. `.env` is gitignored.
- Report security issues privately — see [SECURITY.md](SECURITY.md), not a public issue.

## Getting set up

Requires **Go 1.25+**. Postgres + JWT/KEK env are only needed for the HTTP API.

```bash
make build          # compile every binary under cmd/ into bin/
make test           # go test -race ./...   (the CI gate)
make vet            # go vet ./...
make run-cli        # CLI demo — no HTTP, no Postgres, no LLM needed
make run-api-mock   # HTTP API with the mock LLM (no Ollama)
```

Run a single package or test:

```bash
go test -race ./pkg/governance/
go test -race -run TestName ./agents/supervisor/
```

## Before you open a PR

1. `gofmt -w` your changes (CI-clean formatting is required).
2. `make vet && make test` must pass — this is exactly what CI runs
   (`go vet ./...` then `go test -race ./...` on Go 1.25).
3. Update docs when you change behaviour they describe — docs code blocks are
   grep-able against real files and are treated as part of the contract.
4. Add a `CHANGELOG.md` entry under `## [Unreleased]`.

## Architecture conventions (the load-bearing ones)

- **Message-driven, always.** Agents never call each other directly — emit a
  `protocol.Message` with the right `Type` and let the bus route it. That
  preserves the single audit / trace / fallback / inventory seams.
- **Governance is data, not code.** New cross-cutting rules go into the composite
  policy (`config/ai-policy.example.yaml` + `pkg/governance`), applied to every
  message — not scattered into agents.
- **Adding an agent:** `make scaffold name=<id> cap=<capability> in=<intype>
  out=<outtype> next=<agent>` generates the package + a passing test. Then
  implement `HandleMessage`, declare `RiskLevel()`, give every output a
  disclaimer, register it in `cmd/api/main.go`, and add a paired
  `docs/agents/<id>.md` (the doc is part of the contract).
- Finance domain uses Indian English + regulator terms (CRR, IDV, NPA, GSTIN,
  PAN, IFSC). Money is handled in integer paise (`*_cents` fields).

See [`CLAUDE.md`](CLAUDE.md) and [`docs/architecture.md`](docs/architecture.md)
for the deep dive, and [`docs/glossary.md`](docs/glossary.md) for any term.

## Commit & PR style

- Conventional-commit style is preferred: `feat(afg): …`, `fix(web): …`,
  `docs: …`, `chore: …`, `test: …`.
- Keep PRs focused. Fill in the pull-request template.
- Branch off `main`; CI must be green before merge.
