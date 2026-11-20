<!-- Thanks for contributing to Genie! Keep PRs focused. -->

## What & why

<!-- What does this change and why? Link any issue: Closes #123 -->

## Type

- [ ] feat  · [ ] fix  · [ ] docs  · [ ] refactor  · [ ] test  · [ ] chore

## Checklist

- [ ] `make vet && make test` pass locally (the CI gate: `go vet` + `go test -race` on Go 1.25)
- [ ] `gofmt -w` run on changed files
- [ ] Docs updated where behaviour changed (docs are grep-able against the code)
- [ ] `CHANGELOG.md` updated under `## [Unreleased]`
- [ ] New agent? registered in `cmd/api/main.go` + paired `docs/agents/<id>.md` added
- [ ] No secrets, real credentials, or customer data in the diff

## Notes for the reviewer

<!-- Anything non-obvious: trade-offs, follow-ups, screenshots for UI changes. -->
