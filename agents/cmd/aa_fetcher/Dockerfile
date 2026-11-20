# Multi-stage build for the aa_fetcher agent (pkg/agent.Agent, served via /handle).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-aa_fetcher ./agents/cmd/aa_fetcher

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-aa_fetcher /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-aa_fetcher"]
