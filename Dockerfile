FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/genie-api ./cmd/api

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=build /out/genie-api /genie-api
COPY config /config
ENV GENIE_AI_POLICY=/config/ai-policy.example.yaml
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/genie-api"]
