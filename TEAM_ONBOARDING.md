# Genie Platform — Team Onboarding Guide

Welcome to the Genie team! This guide covers the 6-phase platform, spec-driven development, and how to implement Phase 7+.

---

## 15-Minute Quick Start

### 1. Clone & Build
```bash
git clone https://github.com/c2siorg/genie.git
cd genie

# Backend build
go mod download
go build ./cmd/api
go test ./... # Should see 87 tests pass

# Frontend build
cd frontend && npm install && npm run build
# Should output: 81.54KB gzipped
```

### 2. Understand the 6 Phases
Each phase is a complete workspace with real API integration:

| Phase | Purpose | Real APIs |
|-------|---------|-----------|
| 2 | Orders, payments, settlement | 8 |
| 3 | KYC, AML, velocity, HITL approval | 6 |
| 4 | Agent fleet, incidents, kill-switch | 9 |
| 5 | Judge calibration, golden datasets | 7 |
| 6 | Conversational AI with LLM | 3 |

All documented in `/specs/phase[2-6]_*.md`

### 3. Review Specs
```bash
# Read Phase 2 (Commerce) spec
cat specs/phase2_commerce.md

# Understand the structure:
# - API Endpoints (request/response types)
# - Type Definitions (validation rules)
# - Test Requirements (what must pass)
# - Success Criteria (definition of done)
```

### 4. Run Validation
```bash
# Validate all phases
make spec-validate-all

# Validate specific phase
make spec-validate-phase2

# Strict mode (used in CI)
make spec-strict
```

---

## Developer Roles

### **Backend Engineer** (Go)

**Day 1-2: Understand architecture**
- Read `CLAUDE.md` (project guidelines)
- Read `pkg/commerce/` (Phase 2 example)
- Read `specs/phase2_commerce.md` (what Phase 2 implements)

**Day 3-4: Explore multi-agent system**
- Understand message bus (`pkg/busio/`)
- Understand agent interface (`pkg/agent/`)
- See how Commerce agents orchestrate (`agents/*/`)

**Week 1: Start Phase 7 (Advisor)**
1. Create backend types: `pkg/advisor/types.go`
2. Implement ProfileAnalyzer agent: `pkg/advisor/profile_analyzer.go`
3. Implement FinancialAnalyst agent: `pkg/advisor/analyst.go`
4. Write unit tests for each

**Check-in**: `go test ./pkg/advisor... -v` all passing

**Week 2: Complete Phase 7**
1. Implement RecommendationGenerator
2. Create API endpoints: `pkg/web/handlers/advisor.go`
3. Wire in compliance checker (from Phase 3)
4. Run full test suite: `go test ./pkg/advisor... -v` (16 tests)

**Validation**: `make spec-validate-phase7` passes

### **Frontend Engineer** (TypeScript/React)

**Day 1-2: Understand UI structure**
- Review `CLAUDE.md` (React patterns)
- Review `frontend/src/workspaces/pages.tsx` (workspace examples)
- Review `frontend/src/components/Money.tsx` (custom components)

**Day 3-4: Understand hooks & types**
- Review `frontend/src/hooks/useCommerce.ts` (real API integration)
- Review `frontend/src/types/commerce.ts` (type definitions)
- Understand apiFetch (CSRF + cookies)

**Week 1: Create Phase 7 types & hooks**
1. Create `frontend/src/types/advisor.ts` (Action, Risk, Recommendation, etc.)
2. Create `frontend/src/hooks/useAdvisor.ts` (API hooks)
3. Verify types compile: `npm run build`

**Check-in**: No TypeScript errors, types match spec

**Week 2: Implement Advisor UI**
1. Create `Advisor` component in `workspaces/pages.tsx`
2. Integrate hooks (useAdvisor)
3. Add form for recommendation request
4. Display recommendations with actions + risks
5. Show recommendation history

**Validation**: `npm run build` succeeds, 81KB gzipped target maintained

### **QA / Test Engineer**

**Day 1-2: Understand test structure**
- Review spec format: `specs/phase2_commerce.md`
- Review test requirements: each spec has 10-20 test cases listed
- Review existing tests: `go test -list ./pkg/commerce`

**Day 3: Understand CI/CD**
- Review `.github/workflows/` (if exists)
- Review new Makefile targets: `make spec-validate-*`
- Understand CI flow: vet → lint → build → test → coverage → spec-validate

**Week 1: Create Phase 7 test plan**
1. Extract test requirements from `specs/phase7_advisor.md`
2. Map to code coverage (ProfileAnalyzer, FinancialAnalyst, etc.)
3. Create test matrix (happy path + edge cases)

**Week 2: Validate Phase 7**
1. Run all tests: `go test ./pkg/advisor... -v` (16 tests)
2. Check coverage: `go test -cover ./pkg/advisor`
3. Run spec validation: `make spec-validate-phase7`
4. Update CI to run Phase 7 tests

**Validation**: All tests passing, spec validation clean

### **Product/Planning**

**Week 1: Phase 7 Planning**
1. Review `specs/phase7_advisor.md` (complete specification)
2. Understand dependencies: Phase 2, 3, 7 must be complete first
3. Timeline: 4-5 hours (spec-first) vs 8-10 hours (code-first)

**Week 2: Roadmap & Phase 8**
1. Review `specs/phase8_analytics.md` (Analytics workspace)
2. Plan Phase 8 kickoff (depends on Phase 7)
3. Plan Phase 9+ (automation, extensions)

---

## Development Workflow

### 1. Picking a Task

**Find the spec first**:
```bash
# Want to implement Phase 7?
cat specs/phase7_advisor.md

# Find API endpoints you need to build
# Find type definitions to create
# Find test requirements to cover
```

**The spec is your contract**:
- What endpoints to implement
- What request/response types
- What validation rules
- What tests must pass
- What success means

### 2. Implementation Flow

**Backend**:
```bash
# 1. Create types
vim pkg/advisor/types.go

# 2. Implement logic
vim pkg/advisor/profile_analyzer.go

# 3. Write tests
vim pkg/advisor/profile_analyzer_test.go

# 4. Run tests
go test ./pkg/advisor... -v

# 5. Validate against spec
make spec-validate-phase7
```

**Frontend**:
```bash
# 1. Create types (matching backend)
vim frontend/src/types/advisor.ts

# 2. Create hooks (API integration)
vim frontend/src/hooks/useAdvisor.ts

# 3. Create component
vim frontend/src/workspaces/pages.tsx # Update Advisor function

# 4. Build & verify types
npm run build

# 5. Validate types match backend spec
# (Spec defines types for both TS & Go)
```

### 3. Testing Locally

```bash
# Backend
go test ./pkg/advisor... -v
go test -cover ./pkg/advisor

# Frontend
npm run build
npm test

# Full CI locally
make ci  # This runs everything including spec-validate-all
```

### 4. Before Push

```bash
# 1. Validate all specs pass
make spec-validate-all

# 2. Run full CI
make ci

# 3. Check git status
git status

# 4. Create PR (not push directly)
gh pr create --title "Phase 7: Advisor workspace" --body "..."
```

---

## Key Concepts

### **Spec-Driven Development**

The specs (in `/specs/`) are your requirements. They define:
- Exactly which APIs to build
- Exactly what types to create
- Exactly which tests must pass
- Exactly what success means

Instead of asking "what should we build?", you read the spec.

### **Type Safety Across Stack**

All types defined in spec match:
- **TypeScript** (`frontend/src/types/`)
- **Go** (`pkg/*/`)
- **OpenAPI** (`docs/openapi.yaml`)

Mismatch = build error. No silent failures.

### **Spec Validation in CI**

Every PR must pass:
```bash
make spec-validate-all
```

This checks:
- Endpoints exist
- Types match spec
- Tests passing
- Success criteria met

### **Multi-Agent Architecture**

Each phase has agents that work together via message bus:
- Commerce: OrderCreator → PaymentInitiator → SettlementExecutor
- Compliance: VelocityChecker → AMLScreener → FraudDetector
- Advisor: ProfileAnalyzer → FinancialAnalyst → RecommendationGenerator

Each agent gets a message, processes it, sends result to next agent.

---

## Common Tasks

### Task 1: Implement a New Endpoint

1. **Find it in spec**: `specs/phase7_advisor.md` → "POST /v1/advisor/recommendation"
2. **Understand request/response**: Read spec endpoint definition
3. **Create handler**:
   ```go
   // pkg/web/handlers/advisor.go
   func (h *Handler) GenerateRecommendation(w http.ResponseWriter, r *http.Request) {
       // Read spec for exact behavior
   }
   ```
4. **Register in router**: `pkg/web/web.go` or `cmd/api/main.go`
5. **Test**:
   ```bash
   go test ./pkg/web/handlers -run TestGenerateRecommendation -v
   ```
6. **Validate**: `make spec-validate-phase7`

### Task 2: Add a Type Definition

1. **Find it in spec**: `specs/phase7_advisor.md` → "Recommendation type"
2. **Create in both places**:
   - Go: `pkg/advisor/types.go`
   - TypeScript: `frontend/src/types/advisor.ts`
3. **Ensure they match** (names, fields, validation)
4. **Test both builds**:
   ```bash
   go build ./cmd/api
   npm run build  # From frontend/
   ```

### Task 3: Write a Test

1. **Find test requirement in spec**: `specs/phase7_advisor.md` → "Unit Tests" section
2. **Create test file**: `pkg/advisor/recommendation_generator_test.go`
3. **Write test**:
   ```go
   func TestRecommendationGenerator_GeneratesCorrectly(t *testing.T) {
       // Test follows spec requirement
   }
   ```
4. **Run test**: `go test ./pkg/advisor -run TestRecommendationGenerator -v`

---

## Useful Commands

```bash
# Build & Test
make check              # Quick sanity check (no race, no judge)
make ci                 # Full CI (everything)
make spec-validate-all  # Spec validation only

# Per-Phase
make spec-validate-phase7  # Validate Phase 7 only
make test-phase7           # Test Phase 7 only (if exists)

# Frontend
cd frontend && npm run build
cd frontend && npm test

# Git
git status
git diff
git log --oneline -10
```

---

## Common Pitfalls

❌ **Don't**: Implement without reading spec first  
✅ **Do**: Read spec, understand requirements, then implement

❌ **Don't**: Have types in TypeScript but not Go  
✅ **Do**: Create types in BOTH places, ensure they match

❌ **Don't**: Skip tests because "it works locally"  
✅ **Do**: Write tests that match spec requirements

❌ **Don't**: Push without running `make ci`  
✅ **Do**: Run full CI locally first, then push

---

## Getting Help

- **Spec questions**: Read `specs/DEVELOPMENT_GUIDE.md`
- **Type definitions**: Check existing phase types in `/frontend/src/types/`
- **API integration**: Review `frontend/src/hooks/useCommerce.ts` (real example)
- **Architecture**: Read `CLAUDE.md`
- **Slack**: #genie-team channel

---

## Your First Week Path

**Day 1**: Clone, build, read Phase 2 spec  
**Day 2**: Read Phase 6 (Assistant) implementation, understand hooks  
**Day 3**: Read Phase 7 (Advisor) spec completely  
**Day 4**: Set up your dev environment, run `make ci`  
**Day 5**: Start Phase 7 implementation (types + first agent)

By end of Week 1, you'll have created working Advisor types and ProfileAnalyzer agent.

---

## Success = Spec Passes

When your code is done:
- `go test ./pkg/advisor... -v` → All 16 tests pass ✅
- `npm run build` → No TypeScript errors ✅
- `make spec-validate-phase7` → All validations pass ✅

That's it. You're done. Spec = definition of done.

---

**Welcome to Genie!** 🚀

We build financial systems the spec-first way: clear contracts, fast implementation, zero ambiguity.

Questions? Read the spec, check the code, ask the team.
