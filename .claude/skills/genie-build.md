# /genie-build — Build Genie API Server

Build the Genie API server binary for the current platform.

## Usage

```
/genie-build [output-path]
```

## Examples

- `/genie-build` — Build to `bin/api`
- `/genie-build ./genie-api` — Build to custom path
- `/genie-build -race` — Build with race detector

## What it does

Executes: `go build -o bin/api ./cmd/api`

Creates an executable binary of the Genie API server with:
- e-Rupee commerce integration
- Payment orchestration
- Settlement processing
- Compliance engine
- Lineage tracking

## Build requirements

- Go 1.25.0+
- All dependencies in go.mod available
- No linting errors required to build

## Expected output

```
Building Genie API server...
✓ Build complete: bin/api
```

## After building

Run the server with: `./bin/api`

Or start the dev environment: `make up`

## Related

- `/genie-test` — Run tests
- `/genie-lint` — Run linting
- `/genie-vet` — Run static analysis
