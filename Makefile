SHELL := /bin/bash
.DEFAULT_GOAL := help

GO ?= go
PKG := ./...
BIN_DIR := bin

# LLM stack defaults — Ollama is the default for both `make up` (compose) and
# `make run-api` (local). Override any of these on the command line, e.g.
#   make run-api GENIE_LLM=mock
#   make run-api GENIE_OLLAMA_CHAT=llama3.1:8b
GENIE_LLM          ?= ollama
GENIE_OLLAMA_URL   ?= http://localhost:11434
GENIE_OLLAMA_CHAT  ?= llama3.2:1b
GENIE_OLLAMA_EMBED ?= nomic-embed-text

export GENIE_LLM GENIE_OLLAMA_URL GENIE_OLLAMA_CHAT GENIE_OLLAMA_EMBED

.PHONY: help
help: ## Show available targets
	@awk 'BEGIN{FS=":.*?## "}/^[a-zA-Z_-]+:.*?## /{printf "  \033[36m%-18s\033[0m %s\n",$$1,$$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Compile every binary under cmd/ into bin/
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/ ./cmd/...

.PHONY: test
test: ## Run all unit tests
	$(GO) test -race $(PKG)

.PHONY: e2e
e2e: ## Run the user-simulation test against a running stack (signup → ask → disclosures)
	$(GO) test -tags=e2e -v ./tests/sim/...

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
run-api: ## Run the HTTP API locally with Ollama (requires GENIE_DB_DSN, GENIE_JWT_SECRET, GENIE_KEK_BASE64)
	$(GO) run ./cmd/api

.PHONY: run-api-mock
run-api-mock: ## Run the HTTP API locally with the mock LLM (no Ollama dependency)
	GENIE_LLM=mock $(GO) run ./cmd/api

.PHONY: up
up: ## One-command boot — build, start, wait for readiness, print URLs
	@docker compose up --build -d
	@printf "waiting for genie-api"
	@for i in $$(seq 1 90); do \
		if curl -fsS http://localhost:8080/readyz >/dev/null 2>&1; then \
			printf " ready\n\n"; \
			echo "  Genie UI:    http://localhost:8080/"; \
			echo "  API health:  http://localhost:8080/healthz"; \
			echo "  Disclosures: http://localhost:8080/v1/disclosures"; \
			echo "  Grafana:     http://localhost:3000/  (admin/admin)"; \
			echo ""; \
			exit 0; \
		fi; \
		printf "."; \
		sleep 1; \
	done; \
	printf " timeout\n"; \
	docker compose logs --tail=40 genie-api; \
	exit 1

.PHONY: down
down: ## Stop and remove the local stack
	docker compose down -v

.PHONY: smoke
smoke: ## End-to-end smoke test against a running stack (healthz, signup, /ask, disclosures)
	@set -euo pipefail; \
	BASE=$${GENIE_BASE_URL:-http://localhost:8080}; \
	EMAIL="smoke-$$(date +%s)@genie.local"; \
	assert_field() { echo "$$1" | grep -q "\"$$2\"" || { echo "  FAIL: missing field \"$$2\" in response: $$1"; exit 1; }; }; \
	echo "→ $$BASE/healthz"; \
	curl -fsS "$$BASE/healthz" >/dev/null && echo "  ok"; \
	echo "→ $$BASE/readyz"; \
	curl -fsS "$$BASE/readyz" >/dev/null && echo "  ok"; \
	echo "→ POST /v1/users  ($$EMAIL)"; \
	SIGNUP=$$(curl -fsS -X POST "$$BASE/v1/users" \
	  -H 'Content-Type: application/json' \
	  -d "{\"email\":\"$$EMAIL\",\"name\":\"Smoke\",\"password\":\"hunter2hunter2\"}"); \
	TOKEN=$$(printf '%s' "$$SIGNUP" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'); \
	[ -n "$$TOKEN" ] || { echo "  FAIL: no token in $$SIGNUP"; exit 1; }; \
	echo "  ok (token len=$${#TOKEN})"; \
	echo "→ GET /v1/users/me"; \
	ME=$$(curl -fsS "$$BASE/v1/users/me" -H "Authorization: Bearer $$TOKEN"); \
	assert_field "$$ME" email; echo "  ok"; \
	echo "→ POST /v1/accounts"; \
	ACC=$$(curl -fsS -X POST "$$BASE/v1/accounts" \
	  -H "Authorization: Bearer $$TOKEN" -H 'Content-Type: application/json' \
	  -d '{"name":"Salary","currency":"INR"}'); \
	assert_field "$$ACC" id; echo "  ok"; \
	echo "→ POST /v1/documents (upload tiny CSV)"; \
	CSV=$$'date,description,category,amount,type\n2026-01-01,Salary,Income,50000,credit\n2026-01-05,Swiggy,Food,350,debit\n2026-02-01,Rent,Housing,15000,debit'; \
	DOC=$$(printf '%s' "$$CSV" | curl -fsS -X POST "$$BASE/v1/documents?description=smoke&classification=internal" \
	  -H "Authorization: Bearer $$TOKEN" -H 'Content-Type: text/csv' --data-binary @-); \
	DOC_ID=$$(printf '%s' "$$DOC" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p'); \
	[ -n "$$DOC_ID" ] || { echo "  FAIL: no doc id in $$DOC"; exit 1; }; \
	echo "  ok (doc=$$DOC_ID)"; \
	echo "→ warming Ollama (loading $${GENIE_OLLAMA_CHAT} into memory — once per host)"; \
	curl -fsS -o /dev/null -m 240 -X POST "http://localhost:11434/api/generate" \
	  -H 'Content-Type: application/json' \
	  -d "{\"model\":\"$${GENIE_OLLAMA_CHAT}\",\"prompt\":\"hi\",\"stream\":false,\"keep_alive\":\"30m\"}" \
	  && echo "  ok" || echo "  skipped (ollama not reachable on :11434)"; \
	echo "→ POST /v1/ask"; \
	ASK=$$(curl -fsS --max-time 120 -X POST "$$BASE/v1/ask" \
	  -H "Authorization: Bearer $$TOKEN" -H 'Content-Type: application/json' \
	  -d "{\"question\":\"Summarise this month's spending in one sentence.\",\"document_id\":\"$$DOC_ID\"}"); \
	assert_field "$$ASK" report; echo "  ok"; \
	echo "→ GET /v1/disclosures"; \
	DISC=$$(curl -fsS "$$BASE/v1/disclosures"); \
	assert_field "$$DISC" policy_version; echo "  ok"; \
	echo ""; \
	echo "smoke: PASS"

.PHONY: compose-up
compose-up: up ## Alias for `up` (kept for backward compatibility)

.PHONY: compose-down
compose-down: down ## Alias for `down` (kept for backward compatibility)

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
