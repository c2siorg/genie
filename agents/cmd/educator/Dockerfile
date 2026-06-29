# Multi-stage build for the educator agent (pkg/agent.Agent, served via /handle).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-educator ./agents/cmd/educator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-educator /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-educator"]
