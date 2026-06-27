# Multi-stage build for the supply_chain_finance agent (pkg/agent.Agent, served via /handle).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-supply_chain_finance ./agents/cmd/supply_chain_finance

# Distroless: no shell/package manager. K8s livenessProbe replaces HEALTHCHECK;
# graceful shutdown is the Go SIGTERM handler, not a preStop hook.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-supply_chain_finance /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-supply_chain_finance"]
