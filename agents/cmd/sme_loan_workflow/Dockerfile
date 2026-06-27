# Multi-stage build for the sme_loan_workflow agent (pkg/agent.Agent, served via /handle).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-sme_loan_workflow ./agents/cmd/sme_loan_workflow

# Distroless: no shell/package manager. K8s livenessProbe replaces HEALTHCHECK;
# graceful shutdown is the Go SIGTERM handler, not a preStop hook.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-sme_loan_workflow /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-sme_loan_workflow"]
