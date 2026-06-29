SHELL      := /bin/bash
.DEFAULT_GOAL := help

# ─────────────────────────────────────────────────────────────────────────────
# Toolchain
# ─────────────────────────────────────────────────────────────────────────────
GO         ?= go
GOFLAGS    ?=
PKG        := ./...
BIN_DIR    := bin
COVER_DIR  := .coverage

GOLANGCI   ?= golangci-lint
STATICCHK  ?= staticcheck

# ─────────────────────────────────────────────────────────────────────────────
# LLM stack — Ollama is the default everywhere.
# Override on the command line:
#   make run-api  GENIE_LLM=mock
#   make eval-live EVAL_PROVIDER=openai EVAL_MODEL=gpt-4.1
# ─────────────────────────────────────────────────────────────────────────────
GENIE_LLM          ?= ollama
GENIE_OLLAMA_URL   ?= http://localhost:11434
GENIE_OLLAMA_CHAT  ?= llama3.2:1b
GENIE_OLLAMA_EMBED ?= nomic-embed-text

export GENIE_LLM GENIE_OLLAMA_URL GENIE_OLLAMA_CHAT GENIE_OLLAMA_EMBED

# Multi-turn eval flags
EVAL_DATASET   ?= pkg/eval/multiturn/data/agent_multiturn.json
EVAL_PROVIDER  ?= ollama
EVAL_MODEL     ?= $(GENIE_OLLAMA_CHAT)
EVAL_BASE_URL  ?= $(GENIE_OLLAMA_URL)
EVAL_WORKERS   ?= 4
EVAL_COVER_PKG ?= ./pkg/eval/multiturn/...

# Coverage thresholds
# Honest ratchet floor: total statement coverage is 60.0% as of the Phase 0
# baseline (see COVERAGE.md). The gate fails if coverage drops BELOW this floor.
# Raise this number as vertical slices (plan.md Phases 2-6) land — never lower it.
COVER_MIN  ?= 60   # minimum % line coverage required by `make ci` / `make ci-gate`

# ─────────────────────────────────────────────────────────────────────────────
# help — auto-generated from ## comments
# ─────────────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@printf "\n\033[1mGenie — multi-agent financial platform\033[0m\n\n"
	@printf "\033[33mQuick start:\033[0m\n"
	@printf "  make check              # vet + build + fast tests (no race, no judge)\n"
	@printf "  make ci                 # full CI pipeline (lint + race + cover + eval)\n"
	@printf "  make up                 # start full Docker stack\n"
	@printf "  make eval               # run multi-turn evals offline (no LLM judge)\n"
	@printf "  make playwright-install # install Playwright browsers for E2E testing\n"
	@printf "  make playwright-test    # run 82 comprehensive E2E tests\n\n"
	@printf "\033[33mTargets:\033[0m\n"
	@awk 'BEGIN{FS=":.*?## "} \
	     /^##/{printf "\n  \033[1;37m%s\033[0m\n", substr($$0,4)} \
	     /^[a-zA-Z_-]+:.*?## /{printf "  \033[36m%-22s\033[0m %s\n",$$1,$$2}' \
	     $(MAKEFILE_LIST)
	@printf "\n"

# ─────────────────────────────────────────────────────────────────────────────
## Build
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Compile every binary under cmd/ into bin/
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/ ./cmd/...

.PHONY: build-eval
build-eval: ## Build only the eval-multiturn binary
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/eval-multiturn ./cmd/eval-multiturn

.PHONY: docker-build
docker-build: ## Build the production Docker image (genie-api:dev)
	docker build -t genie-api:dev .

# ─────────────────────────────────────────────────────────────────────────────
## Code quality
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: fmt
fmt: ## Run gofmt -w on all Go files
	@$(GO) fmt $(PKG)
	@echo "fmt: done"

.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKG)

.PHONY: lint
lint: ## Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	$(GOLANGCI) run $(PKG)

.PHONY: staticcheck
staticcheck: ## Run staticcheck
	$(STATICCHK) $(PKG)

.PHONY: tidy
tidy: ## go mod tidy — remove unused deps
	$(GO) mod tidy

.PHONY: tidy-check
tidy-check: ## Fail if go.mod/go.sum are not tidy (CI gate)
	@cp go.mod go.mod.bak && cp go.sum go.sum.bak
	@$(GO) mod tidy
	@diff go.mod go.mod.bak || { echo "go.mod is not tidy — run: make tidy"; rm go.mod.bak go.sum.bak; exit 1; }
	@diff go.sum go.sum.bak || { echo "go.sum is not tidy — run: make tidy"; rm go.mod.bak go.sum.bak; exit 1; }
	@rm go.mod.bak go.sum.bak
	@echo "tidy-check: ok"

# ─────────────────────────────────────────────────────────────────────────────
## Tests
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all unit tests with race detector
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-fast
test-fast: ## Run unit tests without race detector (faster local loop)
	$(GO) test -count=1 $(PKG)

.PHONY: test-pkg
test-pkg: ## Test a single package. Usage: make test-pkg PKG=./pkg/eval/multiturn/...
	$(GO) test -race -v -count=1 $(PKG)

.PHONY: cover
cover: ## Generate HTML coverage report (opens in browser)
	@mkdir -p $(COVER_DIR)
	$(GO) test -race -coverprofile=$(COVER_DIR)/coverage.out -covermode=atomic $(PKG)
	$(GO) tool cover -html=$(COVER_DIR)/coverage.out -o $(COVER_DIR)/coverage.html
	@echo "coverage report: $(COVER_DIR)/coverage.html"
	@open $(COVER_DIR)/coverage.html 2>/dev/null || xdg-open $(COVER_DIR)/coverage.html 2>/dev/null || true

.PHONY: cover-check
cover-check: ## Fail if total line coverage is below COVER_MIN (default 60%)
	@mkdir -p $(COVER_DIR)
	@$(GO) test -coverprofile=$(COVER_DIR)/coverage.out -covermode=atomic $(PKG) >/dev/null
	@# Exclude thin entry-point wrapper packages (agents/cmd/*, cmd/eval-golden) —
	@# no unit-testable logic; covered by live smoke tests + the golden gate.
	@grep -vE '/agents/cmd/|/cmd/eval-golden/' $(COVER_DIR)/coverage.out > $(COVER_DIR)/coverage.floor.out
	@pct=$$($(GO) tool cover -func=$(COVER_DIR)/coverage.floor.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	 echo "coverage (excl. entry-point wrappers): $${pct}% (min: $(COVER_MIN)%)"; \
	 awk -v got="$$pct" -v min="$(COVER_MIN)" 'BEGIN{if(got+0 < min+0){print "FAIL: coverage below threshold"; exit 1}}'

.PHONY: ci-gate
ci-gate: ## Minimal reliable CI gate: build must compile, tests must pass, coverage must hold (used by .github/workflows/ci.yml)
	@echo "── ci-gate: build ─────────────────────────────"
	$(GO) build ./...
	@echo "── ci-gate: test ──────────────────────────────"
	$(GO) test -count=1 $(PKG)
	@echo "── ci-gate: coverage floor ($(COVER_MIN)%) ────"
	@$(MAKE) cover-check
	@echo "✅ ci-gate passed (build + tests + coverage ≥ $(COVER_MIN)%)"

.PHONY: e2e
e2e: ## Run user-simulation e2e tests against a running stack
	$(GO) test -tags=e2e -v -timeout=5m ./tests/sim/...

# ─────────────────────────────────────────────────────────────────────────────
## Frontend (React UI)
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: ui-install
ui-install: ## Install frontend (React) dependencies
	cd frontend && npm install --no-audit --no-fund

.PHONY: ui-test
ui-test: ## Run frontend unit/component tests (vitest)
	cd frontend && npm test

.PHONY: ui-build
ui-build: ## Build the React app into the Go embed tree (pkg/web/handlers/ui/app)
	cd frontend && npm run build

.PHONY: ui-dev
ui-dev: ## Run the Vite dev server (hot reload) for the React UI
	cd frontend && npm run dev

.PHONY: playwright-install
playwright-install: ## Install Playwright dependencies and browsers
	cd e2e && npm install && npx playwright install

.PHONY: playwright-test
playwright-test: ## Run all Playwright E2E tests (requires running docker-compose stack)
	cd e2e && npm test

.PHONY: playwright-test-ui
playwright-test-ui: ## Run Playwright tests in UI mode (interactive)
	cd e2e && npm run test:ui

.PHONY: playwright-test-debug
playwright-test-debug: ## Run Playwright tests in debug mode with inspector
	cd e2e && npm run test:debug

.PHONY: playwright-test-settlement
playwright-test-settlement: ## Run settlement workflow E2E tests only
	cd e2e && npm run test:settlement

.PHONY: playwright-test-security
playwright-test-security: ## Run security & CSRF E2E tests only
	cd e2e && npm run test:security

.PHONY: playwright-test-evaluation
playwright-test-evaluation: ## Run evaluation dashboard E2E tests only
	cd e2e && npm run test:evaluation

.PHONY: playwright-test-compliance
playwright-test-compliance: ## Run compliance & AML E2E tests only
	cd e2e && npm run test:compliance

.PHONY: playwright-test-ci
playwright-test-ci: ## Run Playwright tests with CI reporters (HTML, JSON, JUnit)
	cd e2e && npm run test:ci

.PHONY: playwright-report
playwright-report: ## View Playwright HTML test report
	cd e2e && npm run show:report

.PHONY: playwright-codegen
playwright-codegen: ## Record test code by interacting with app (requires running API)
	cd e2e && npm run codegen

# ─────────────────────────────────────────────────────────────────────────────
## Multi-turn Evals
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: eval
eval: build-eval ## Run multi-turn evals offline — no LLM judge (fast, CI-safe)
	@echo "running multi-turn evals [skip-judge=true, workers=$(EVAL_WORKERS)]"
	$(BIN_DIR)/eval-multiturn \
	  -dataset   $(EVAL_DATASET) \
	  -workers   $(EVAL_WORKERS) \
	  -skip-judge

.PHONY: eval-live
eval-live: build-eval ## Run multi-turn evals WITH LLM judge (requires Ollama or OPENAI_API_KEY)
	@echo "running multi-turn evals [provider=$(EVAL_PROVIDER), model=$(EVAL_MODEL), judge=true]"
	$(BIN_DIR)/eval-multiturn \
	  -dataset   $(EVAL_DATASET) \
	  -provider  $(EVAL_PROVIDER) \
	  -model     $(EVAL_MODEL) \
	  -base-url  $(EVAL_BASE_URL) \
	  -workers   $(EVAL_WORKERS)

.PHONY: eval-json
eval-json: build-eval ## Emit eval results as JSON (pipe to jq, save to file, etc.)
	$(BIN_DIR)/eval-multiturn \
	  -dataset   $(EVAL_DATASET) \
	  -provider  $(EVAL_PROVIDER) \
	  -model     $(EVAL_MODEL) \
	  -base-url  $(EVAL_BASE_URL) \
	  -workers   $(EVAL_WORKERS) \
	  -skip-judge \
	  -json

.PHONY: eval-cover
eval-cover: ## Run eval package tests with coverage
	@mkdir -p $(COVER_DIR)
	$(GO) test -race -coverprofile=$(COVER_DIR)/eval.out \
	  -covermode=atomic $(EVAL_COVER_PKG)
	$(GO) tool cover -func=$(COVER_DIR)/eval.out | tail -5

.PHONY: ci-eval
ci-eval: ci-eval-golden ci-judge-validation ## Master target: run all regression + judge accuracy tests

.PHONY: eval-golden
eval-golden: ## Run the deterministic golden gate (Track A, offline) with a report
	$(GO) run ./cmd/eval-golden

.PHONY: ci-eval-golden
ci-eval-golden: ## Golden dataset gate — real deterministic scoring, offline, CI-safe (Track A)
	@echo "running golden gate (deterministic judges, offline)..."
	$(GO) test -race -count=1 -run "TestGoldenGate|TestEvaluateCase|TestLoadGoldenDir|TestExpectedVerdict" ./pkg/eval/golden
	$(GO) run ./cmd/eval-golden

.PHONY: ci-judge-validation
ci-judge-validation: ## Validate the LLM judges (needs Ollama/OPENAI_API_KEY — nightly, not PR)
	@echo "validating LLM judges (requires an LLM backend)..."
	$(GO) test -race -count=1 ./pkg/eval/judges/...

# ─────────────────────────────────────────────────────────────────────────────
## Governance & safety
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: red-team
red-team: ## Adversarial probe corpus against the AI policy (RBI FREE-AI Rec 20)
	$(GO) run ./cmd/red-team -policy config/ai-policy.example.yaml

.PHONY: bcp-drill
bcp-drill: ## Failover drill — force portfolio_advisor failure → verify fallback (Rec 21)
	GENIE_BCP_DRILL=1 $(GO) run ./cmd/genie

.PHONY: governance-test
governance-test: ## Run all governance package tests (policy, RBAC, sovereignty, compliance)
	$(GO) test -race -v -count=1 \
	  ./pkg/governance/... \
	  ./pkg/agentgov/... \
	  ./pkg/safety/... \
	  ./pkg/compliance/...

.PHONY: agenttools-test
agenttools-test: ## Run agent tool tests (file, shell, code execution, web search)
	$(GO) test -race -v -count=1 ./pkg/agenttools/...

.PHONY: singleturn-test
singleturn-test: ## Run single-turn tool-selection eval tests (lesson 03)
	$(GO) test -race -v -count=1 ./pkg/eval/singleturn/...

.PHONY: singleturn-eval
singleturn-eval: ## Run single-turn evals against Ollama (needs GENIE_OLLAMA_CHAT)
	$(GO) run ./cmd/eval-singleturn \
	  -dataset pkg/eval/singleturn/data/file-tools.json \
	  -workers $(EVAL_WORKERS)

.PHONY: singleturn-eval-laminar
singleturn-eval-laminar: ## Run single-turn evals → Laminar traces (auto-loads .env; needs LMNR_PROJECT_API_KEY)
	@set -a; [ -f .env ] && . ./.env; set +a; \
	$(GO) run ./cmd/eval-singleturn \
	  -dataset pkg/eval/singleturn/data/file-tools.json \
	  -workers $(EVAL_WORKERS) \
	  -laminar

.PHONY: eval-laminar
eval-laminar: ## Run multi-turn evals → Laminar traces (auto-loads .env; needs LMNR_PROJECT_API_KEY)
	@set -a; [ -f .env ] && . ./.env; set +a; \
	$(GO) run ./cmd/eval-multiturn \
	  -dataset $(EVAL_DATASET) \
	  -skip-judge \
	  -workers $(EVAL_WORKERS) \
	  -laminar

.PHONY: opa-test
opa-test: ## Run OPA engine Go tests (inline Rego + integration)
	$(GO) test -race -v -count=1 ./pkg/opa/...

.PHONY: opa-policy-test
opa-policy-test: ## Run Rego unit tests with the opa CLI (requires opa in PATH)
	@which opa >/dev/null 2>&1 || (echo "opa CLI not found — install from https://www.openpolicyagent.org/docs/latest/#running-opa"; exit 1)
	opa test policies/ -v

.PHONY: opa-fmt
opa-fmt: ## Format all .rego policy files (requires opa in PATH)
	@which opa >/dev/null 2>&1 || (echo "opa CLI not found"; exit 1)
	opa fmt -w policies/

# ── Lesson 10: Memory ────────────────────────────────────────────────────────

# ── Lesson 13: RAG tools ──────────────────────────────────────────────────

.PHONY: rag-test
rag-test: ## Lesson 13 — Run RAG knowledge-base tool tests
	$(GO) test -race -v -count=1 -run "TestRAG\|TestSearch\|TestIngest" ./pkg/agenttools/...
	$(GO) test -race -v -count=1 ./pkg/rag/...

# ── Lesson 14: Reflexion ──────────────────────────────────────────────────

.PHONY: reflexion-test
reflexion-test: ## Lesson 14 — Run Reflexion self-critique tests
	$(GO) test -race -v -count=1 -run TestRunner_Reflexion ./pkg/agentic/...
	$(GO) test -race -v -count=1 ./pkg/reasoning/...

# ── Runner unit tests ─────────────────────────────────────────────────────

.PHONY: runner-test
runner-test: ## Run Runner.Run() unit tests (fake LLM server, no Ollama needed)
	$(GO) test -race -v -count=1 -run TestRunner ./pkg/agentic/...

.PHONY: memory-test
memory-test: ## Lesson 10 — Run persistent memory tool tests
	$(GO) test -race -v -count=1 -run TestMemory ./pkg/agenttools/...
	$(GO) test -race -v -count=1 ./pkg/memory/...

# ── Lesson 11: Supervisor ─────────────────────────────────────────────────────

.PHONY: supervisor-test
supervisor-test: ## Lesson 11 — Run multi-agent supervisor tests
	$(GO) test -race -v -count=1 -run TestSupervisor ./pkg/agentic/...

# ── Lesson 12: MCP Tools bridge ───────────────────────────────────────────────

.PHONY: mcp-tools-test
mcp-tools-test: ## Lesson 12 — Run MCP→agenttools bridge tests
	$(GO) test -race -v -count=1 -run TestMCP ./pkg/agenttools/...

.PHONY: hitl-test
hitl-test: ## Run Human-in-the-Loop approval package tests
	$(GO) test -race -v -count=1 ./pkg/hitl/...

.PHONY: hitl-demo
hitl-demo: ## Run multi-turn eval with CLI HITL prompts (requires Ollama)
	$(GO) run ./cmd/eval-multiturn \
	  -dataset pkg/eval/multiturn/data/agent_multiturn.json \
	  -hitl cli \
	  -skip-judge

# ─────────────────────────────────────────────────────────────────────────────
## Run locally
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: run-cli
run-cli: ## CLI demo — no HTTP, no Postgres
	$(GO) run ./cmd/genie

.PHONY: run-api
run-api: ## HTTP API — auto-detects Ollama, falls back to mock (needs GENIE_DB_DSN, JWT, KEK)
	$(GO) run ./cmd/api

.PHONY: run-api-ollama
run-api-ollama: ## HTTP API forced to Ollama (set GENIE_OLLAMA_CHAT to choose model)
	GENIE_LLM=ollama $(GO) run ./cmd/api

.PHONY: run-api-mock
run-api-mock: ## HTTP API with mock LLM — no Ollama dependency
	GENIE_LLM=mock $(GO) run ./cmd/api

.PHONY: run-demo
run-demo: ## Run the multi-agent demo scenario
	$(GO) run ./cmd/demo

# ─────────────────────────────────────────────────────────────────────────────
## Ollama model management
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: ollama-check
ollama-check: ## Verify Ollama is running and the chat model is available
	@curl -sf $(GENIE_OLLAMA_URL)/api/tags >/dev/null 2>&1 \
	  && echo "✓ Ollama running at $(GENIE_OLLAMA_URL)" \
	  || (echo "✗ Ollama not reachable at $(GENIE_OLLAMA_URL)  — run: ollama serve" && exit 1)
	@curl -sf $(GENIE_OLLAMA_URL)/api/tags | python3 -c \
	  "import json,sys; models=[m['name'] for m in json.load(sys.stdin)['models']]; \
	   print('  models:', ', '.join(models)); \
	   ok='$(GENIE_OLLAMA_CHAT)' in models or any(m.startswith('$(GENIE_OLLAMA_CHAT)'.split(':')[0]) for m in models); \
	   print('✓ $(GENIE_OLLAMA_CHAT) available' if ok else '✗ $(GENIE_OLLAMA_CHAT) not pulled — run: make ollama-pull')" \
	  2>/dev/null || true

.PHONY: ollama-pull
ollama-pull: ## Pull the default Ollama chat + embed models (qwen3.5 + nomic-embed-text)
	ollama pull $(GENIE_OLLAMA_CHAT)
	ollama pull $(GENIE_OLLAMA_EMBED)

.PHONY: ollama-warm
ollama-warm: ## Load the chat model into memory (avoids cold-start on first request)
	@curl -fsS -m 120 -X POST "$(GENIE_OLLAMA_URL)/api/generate" \
	  -H 'Content-Type: application/json' \
	  -d '{"model":"$(GENIE_OLLAMA_CHAT)","prompt":"hi","stream":false,"keep_alive":"30m"}' \
	  >/dev/null && echo "ollama-warm: $(GENIE_OLLAMA_CHAT) loaded" \
	  || echo "ollama-warm: skipped (ollama not reachable at $(GENIE_OLLAMA_URL))"

.PHONY: ollama-list
ollama-list: ## List models currently loaded in Ollama
	@curl -fsS "$(GENIE_OLLAMA_URL)/api/tags" | \
	  $(GO) run -e - <<'EOF' 2>/dev/null || \
	  curl -fsS "$(GENIE_OLLAMA_URL)/api/tags"
	EOF

# ─────────────────────────────────────────────────────────────────────────────
## Docker stack (full observability: Postgres + Ollama + OTel + Prometheus + Grafana)
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: up
up: ## Build + start the full stack; wait for readiness; print URLs
	@docker compose up --build -d
	@printf "waiting for genie-api"
	@for i in $$(seq 1 90); do \
		if curl -fsS http://localhost:8080/readyz >/dev/null 2>&1; then \
			printf " ready\n\n"; \
			echo "  Genie API:   http://localhost:8080/"; \
			echo "  Health:      http://localhost:8080/healthz"; \
			echo "  Metrics:     http://localhost:9464/metrics"; \
			echo "  Prometheus:  http://localhost:9090/"; \
			echo "  Grafana:     http://localhost:3000/  (admin / admin)"; \
			echo "  Jaeger/Tempo: http://localhost:16686/  (if enabled)"; \
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
down: ## Stop and remove the local stack (volumes included)
	docker compose down -v

.PHONY: restart
restart: down up ## Full teardown + restart

.PHONY: logs
logs: ## Tail logs for all services (Ctrl-C to stop)
	docker compose logs -f

.PHONY: logs-api
logs-api: ## Tail genie-api logs only
	docker compose logs -f genie-api

.PHONY: ps
ps: ## Show running container status
	docker compose ps

.PHONY: smoke
smoke: ## End-to-end smoke test against a running stack (healthz → signup → ask → disclosures)
	@set -euo pipefail; \
	BASE=$${GENIE_BASE_URL:-http://localhost:8080}; \
	EMAIL="smoke-$$(date +%s)@genie.local"; \
	assert_field() { echo "$$1" | grep -q "\"$$2\"" || { echo "  FAIL: missing field \"$$2\" in: $$1"; exit 1; }; }; \
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
	echo "→ POST /v1/documents (tiny CSV)"; \
	CSV=$$'date,description,category,amount,type\n2026-01-01,Salary,Income,50000,credit\n2026-01-05,Swiggy,Food,350,debit\n2026-02-01,Rent,Housing,15000,debit'; \
	DOC=$$(printf '%s' "$$CSV" | curl -fsS -X POST \
	  "$$BASE/v1/documents?description=smoke&classification=internal" \
	  -H "Authorization: Bearer $$TOKEN" -H 'Content-Type: text/csv' --data-binary @-); \
	DOC_ID=$$(printf '%s' "$$DOC" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p'); \
	[ -n "$$DOC_ID" ] || { echo "  FAIL: no doc id in $$DOC"; exit 1; }; \
	echo "  ok (doc=$$DOC_ID)"; \
	$(MAKE) --no-print-directory ollama-warm; \
	echo "→ POST /v1/ask"; \
	ASK=$$(curl -fsS --max-time 120 -X POST "$$BASE/v1/ask" \
	  -H "Authorization: Bearer $$TOKEN" -H 'Content-Type: application/json' \
	  -d "{\"question\":\"Summarise this month spending in one sentence.\",\"document_id\":\"$$DOC_ID\"}"); \
	assert_field "$$ASK" report; echo "  ok"; \
	echo "→ GET /v1/disclosures"; \
	DISC=$$(curl -fsS "$$BASE/v1/disclosures"); \
	assert_field "$$DISC" policy_version; echo "  ok"; \
	echo ""; echo "smoke: PASS"

# ─────────────────────────────────────────────────────────────────────────────
## Observability
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: metrics
metrics: ## Print current Prometheus metrics from genie-api (:9464)
	@curl -fsS http://localhost:9464/metrics | grep -E '^genie_' | head -40 \
	  || echo "metrics endpoint not reachable (is the stack running?)"

.PHONY: traces
traces: ## Open Grafana Tempo / Jaeger UI in browser
	@open http://localhost:16686 2>/dev/null || xdg-open http://localhost:16686 2>/dev/null || \
	  echo "open http://localhost:16686 in your browser"

.PHONY: grafana
grafana: ## Open Grafana in browser
	@open http://localhost:3000 2>/dev/null || xdg-open http://localhost:3000 2>/dev/null || \
	  echo "open http://localhost:3000 in your browser (admin/admin)"

.PHONY: prometheus
prometheus: ## Open Prometheus UI in browser
	@open http://localhost:9090 2>/dev/null || xdg-open http://localhost:9090 2>/dev/null || \
	  echo "open http://localhost:9090 in your browser"

# ─────────────────────────────────────────────────────────────────────────────
## Spec Validation (Spec-Kit Integration)
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: spec-validate-all
spec-validate-all: ## Validate all specs against implementation
	@echo "Validating all phase specifications..."
	@./scripts/validate-specs.sh all
	@echo "spec-validate-all: PASS"

.PHONY: spec-validate-phase2
spec-validate-phase2: ## Validate Phase 2 (Commerce) spec
	@./scripts/validate-specs.sh 2

.PHONY: spec-validate-phase3
spec-validate-phase3: ## Validate Phase 3 (Compliance) spec
	@./scripts/validate-specs.sh 3

.PHONY: spec-validate-phase4
spec-validate-phase4: ## Validate Phase 4 (Governance) spec
	@./scripts/validate-specs.sh 4

.PHONY: spec-validate-phase5
spec-validate-phase5: ## Validate Phase 5 (Evaluation) spec
	@./scripts/validate-specs.sh 5

.PHONY: spec-validate-phase6
spec-validate-phase6: ## Validate Phase 6 (Assistant) spec
	@./scripts/validate-specs.sh 6

.PHONY: spec-validate-phase7
spec-validate-phase7: ## Validate Phase 7 (Advisor) spec
	@./scripts/validate-specs.sh 7

.PHONY: spec-strict
spec-strict: ## Validate all specs in strict mode (fail on warnings)
	@STRICT=true ./scripts/validate-specs.sh all

# ─────────────────────────────────────────────────────────────────────────────
## Tooling
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: scaffold
scaffold: ## Generate a new agent skeleton. Usage: make scaffold name=<id> cap=<cap> in=<type> out=<type> next=<agent>
	$(GO) run ./cmd/scaffold -name=$(name) -capability=$(cap) -intype=$(in) -outtype=$(out) -next=$(next)

.PHONY: openapi-validate
openapi-validate: ## Validate docs/openapi.yaml (requires npx)
	npx --yes @apidevtools/swagger-cli validate docs/openapi.yaml

.PHONY: tools
tools: ## Install development tooling (golangci-lint, staticcheck)
	@echo "installing golangci-lint..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "installing staticcheck..."
	$(GO) install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "tools: done"

# Backward-compat aliases
.PHONY: compose-up compose-down
compose-up:  up   ## Alias for `up`
compose-down: down ## Alias for `down`

# ─────────────────────────────────────────────────────────────────────────────
## Pipeline shortcuts
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: check
check: vet build test-fast ## Quick local sanity check: vet + build + tests (no race, no judge)
	@echo "check: PASS"

.PHONY: ci
ci: tidy-check vet lint build test cover-check agenttools-test singleturn-test opa-test hitl-test memory-test supervisor-test mcp-tools-test rag-test reflexion-test runner-test eval ci-eval spec-strict ## Full CI pipeline (run before push)
	@echo ""
	@echo "╔══════════════════════════════════╗"
	@echo "║         CI: ALL PASSED           ║"
	@echo "╚══════════════════════════════════╝"

.PHONY: all
all: ci eval-live smoke ## Everything: CI + live evals (needs Ollama) + smoke tests (needs stack)
	@echo "all: PASS"

# Convenience: clean generated artifacts
.PHONY: clean
clean: ## Remove bin/ and .coverage/
	@rm -rf $(BIN_DIR) $(COVER_DIR)
	@echo "clean: done"
