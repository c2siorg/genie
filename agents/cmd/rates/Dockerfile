# Multi-stage build for the rates agent (pkg/agent.Agent, served via /handle).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-rates ./agents/cmd/rates

# Distroless: no shell/package manager. K8s livenessProbe replaces HEALTHCHECK;
# graceful shutdown is the Go SIGTERM handler, not a preStop hook.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-rates /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-rates"]
