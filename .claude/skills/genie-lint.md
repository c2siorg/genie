# /genie-lint — Run Code Linting

Run comprehensive linting across the Genie codebase.

## Usage

```
/genie-lint [flags]
```

## Examples

- `/genie-lint` — Run full linting (golangci-lint)
- `/genie-lint ./pkg/commerce` — Lint specific package
- `/genie-lint --fix` — Auto-fix issues where possible
- `/genie-lint -t 5m` — Set timeout

## What it does

Executes: `golangci-lint run ./...`

Checks for:
- ✅ Code style violations
- ✅ Potential bugs
- ✅ Inefficient code
- ✅ Misspelled words
- ✅ Unused code
- ✅ Complexity issues

## Expected output

```
pkg/commerce/workflow.go:42:2: var `result` is unused
   result, err := someFunction()

ok: 15 issues found (run 'golangci-lint run --fix' to fix)
```

## Available linters

Run `golangci-lint linters` to see all active linters

Common ones:
- `gofmt` — Code formatting
- `vet` — Built-in static analysis
- `gosimple` — Code simplification
- `staticcheck` — Static analysis
- `ineffassign` — Unused assignments
- `misspell` — Misspelled words

## Related

- `/genie-build` — Build the project
- `/genie-test` — Run tests
- `/genie-vet` — Run go vet
