# Multi-stage build for the portfolio_advisor agent (pkg/agent.Agent, served via /handle).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /genie-portfolio_advisor ./agents/cmd/portfolio_advisor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /genie-portfolio_advisor /
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-portfolio_advisor"]
