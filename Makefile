SHELL := /bin/bash
.DEFAULT_GOAL := help

GO ?= go
PKG := ./...
BIN_DIR := bin

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN{FS=":.*?## "}/^[a-zA-Z_-]+:.*?## /{printf "  \033[36m%-18s\033[0m %s\n",$$1,$$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Compile every binary under cmd/ into bin/
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/ ./cmd/...

.PHONY: test
test: ## Run all tests
	$(GO) test -race $(PKG)

.PHONY: vet
vet: ## go vet
	$(GO) vet $(PKG)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: run-cli
run-cli: ## Run the CLI demo (no HTTP, no Postgres)
	$(GO) run ./cmd/genie

.PHONY: run-api
run-api: ## Run the HTTP API locally (requires GENIE_DB_DSN, GENIE_JWT_SECRET, GENIE_KEK_BASE64)
	$(GO) run ./cmd/api

.PHONY: compose-up
compose-up: ## Start the local stack (Postgres, Tempo, Grafana, otel-collector, genie-api)
	docker compose up --build -d

.PHONY: compose-down
compose-down: ## Stop and remove the local stack
	docker compose down -v

.PHONY: scaffold
scaffold: ## Generate a new agent. Usage: make scaffold name=<id> cap=<capability> in=<intype> out=<outtype> next=<agent>
	$(GO) run ./cmd/scaffold -name=$(name) -capability=$(cap) -intype=$(in) -outtype=$(out) -next=$(next)

.PHONY: docker-build
docker-build: ## Build the production image
	docker build -t genie-api:dev .

.PHONY: red-team
red-team: ## Run the adversarial probe corpus against the board-approved policy (RBI FREE-AI Rec 20)
	$(GO) run ./cmd/red-team -policy config/ai-policy.example.yaml

.PHONY: bcp-drill
bcp-drill: ## Force a portfolio_advisor failure to verify the fallback agent activates (RBI FREE-AI Rec 21)
	GENIE_BCP_DRILL=1 $(GO) run ./cmd/genie

.PHONY: openapi-validate
openapi-validate: ## Validate docs/openapi.yaml against the OpenAPI schema (requires npx swagger-cli)
	npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml
