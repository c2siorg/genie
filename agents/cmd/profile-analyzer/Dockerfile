# Multi-stage build for the profile-analyzer agent. The mod-download layer is shared across
# all agents (same go.mod/go.sum), so rebuilding one agent reuses the cache.
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-profile-analyzer ./agents/cmd/profile-analyzer

# Distroless: no shell, no package manager — minimal attack surface. Note this
# means no Dockerfile HEALTHCHECK (no shell); K8s uses the livenessProbe instead,
# and graceful shutdown is the Go binary's SIGTERM handler, not a preStop hook.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-profile-analyzer /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-profile-analyzer"]
