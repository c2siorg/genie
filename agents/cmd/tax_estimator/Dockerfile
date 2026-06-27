# Multi-stage build for the tax_estimator agent (pkg/agent.Agent, served via /handle).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-tax_estimator ./agents/cmd/tax_estimator

# Distroless: no shell/package manager. K8s livenessProbe replaces HEALTHCHECK;
# graceful shutdown is the Go SIGTERM handler, not a preStop hook.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-tax_estimator /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-tax_estimator"]
