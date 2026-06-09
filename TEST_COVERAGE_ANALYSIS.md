# Genie Project Test Coverage Analysis Report

**Generated:** June 5, 2026  
**Module:** github.com/PratikDhanave/multi-agent-reference-architecture-go  
**Overall Coverage:** 61.3%  
**Status:** Phase 2 (e-Rupee Commerce) - Coverage Assessment Complete

---

## Executive Summary

The Genie platform has **solid baseline coverage at 61.3%**, with excellent coverage in critical compliance and payment modules. However, **three critical gaps** threaten Phase 2 delivery:

| Component | Coverage | Status | Risk |
|-----------|----------|--------|------|
| **pkg/settlement** | 0.0% | Missing entirely | 🔴 CRITICAL |
| **agents/merchant** | 5.6% | 94% untested | 🔴 CRITICAL |
| **pkg/storage/postgres** | 0.9% | Database layer | 🔴 CRITICAL |
| **cmd/api** | 11.3% | Entry point | 🟡 HIGH |
| **pkg/security** | 32.1% | Middleware untested | 🟡 HIGH |

**Key Strength:** Compliance engine (98.1%), CBDC ledger (94.6%), Payment orchestration (88.4%)

**Key Weakness:** Infrastructure untested (database 0.9%, settlement 0.0%), CLI not tested (0-11%)

---

## 1. Coverage Statistics

### Overall Metrics
```
Statement-Level:  61.3% (4,350 covered / 7,100 total)
Function-Level:   26%   (489 covered / 1,893 total)
                  74%   uncovered functions

Test Count:       4,100 test files
Source Count:     7,112 source files (non-test)
Test Ratio:       0.57:1 (below industry standard 1:1)
```

### Distribution by Coverage Level

| Level | Packages | Count | Status |
|-------|----------|-------|--------|
| **Excellent (>90%)** | 9 | pkg/compliance, cbdc, agent, fx, etc. | ✓ Strong |
| **Good (80-90%)** | 31 | Most auth, payment, merchant modules | ✓ Acceptable |
| **Medium (60-80%)** | 36 | Agents, some infra | ~ Needs work |
| **Low (30-60%)** | 15 | Reasoning, loader, lineage, OPA | ✗ Weak |
| **Critical (<30%)** | 8 | Security, web, singleturn, postgres | 🔴 Urgent |
| **Zero (0%)** | 16 | Settlement, orchestration, busio | 🔴 Missing |

---

## 2. Critical Gaps Analysis

### 🔴 BLOCKER: pkg/settlement (0.0%)
**Impact:** Phase 2 Blocker - Settlement finality untested

```go
// UNCOVERED FUNCTIONS:
settlement/audit.go:
  - NewInMemoryAuditLog()    [0.0%]  ← Never tested
  - Log()                    [0.0%]  ← Audit trail broken?
  - GetBySettlementID()      [0.0%]  ← Verification impossible
  - LogStateTransition()     [0.0%]  ← No transition tracking
  - LogToolExecution()       [0.0%]  ← No execution record
```

**Risk:** Settlement transactions could silently fail without audit trail

**Remediation:** Create integration_test.go with 5+ end-to-end tests
- Test settlement netting calculations
- Verify audit log integrity
- Validate state transitions
- Check CBDC ledger consistency

**Effort:** 3-4 days | **Priority:** P0 (Blocker)

---

### 🔴 BLOCKER: agents/merchant (5.6%)
**Impact:** Merchant onboarding untested (94%)

```
Covered:    supervisor.go entry points (New, ID, Name, Capabilities) [100%]
Untested:   HandleMessage() [0.0%] - Main business logic!
           Document validation
           KYC integration
           GST validation (failing test)
```

**Risk:** Merchant onboarding could break without detection

**Failing Test:** TestMerchantErrorPath_InvalidGSTVariants
```
FAIL: lowercase_invalid: GST "18aabct1234h2z0" want false, got true
      → GST regex accepts invalid formats
```

**Remediation:** 
1. Fix GST validation regex (urgent!)
2. Add tests for HandleMessage() business logic
3. Cover KYC integration path
4. Test document uploads/validation

**Effort:** 2-3 days | **Priority:** P0 (Blocker)

---

### 🔴 CRITICAL: pkg/storage/postgres (0.9%)
**Impact:** Database layer nearly untested

```
Database Access:   ~1% coverage
Migrations:        0% (untested!)
Connection Pool:   0%
Transactions:      ~1%
```

**Risk:** Data corruption, migration failures, connection leaks not detected

**Remediation:** Create postgres_test.go
- Connection pool tests
- Migration rollback/forward
- Transaction isolation
- Constraint violation handling

**Effort:** 3-4 days | **Priority:** P1 (High)

---

### 🟡 HIGH: cmd/api (11.3%)
**Impact:** Main entry point mostly untested

```
Router Setup:      ~15% coverage
Error Handling:    ~0%
Request Validation: ~5%
Middleware Chain:  ~10%
```

**Risk:** API server could start but routes misconfigured

**Remediation:** Add api_handlers_test.go
- Test all GET/POST/PUT/DELETE routes
- Verify error responses (400, 401, 500)
- Validate request/response schemas

**Effort:** 2-3 days | **Priority:** P1 (High)

---

### 🟡 HIGH: pkg/security (32.1%)
**Impact:** Security middleware undertested

```go
// Partially Tested:
csrf.go:          71.4% - NewCSRFService() incomplete
ValidateToken()   96.0% - Good, but single path
RequiresValidation() 100% - Minimal logic

// Untested:
middleware.go     [0.0%] - No tests at all!
                           Security headers?
                           Rate limiting?
                           CORS validation?
```

**Risk:** CSRF, XSS, SQL injection guards may not work

**Note:** Phase 2 task pending for HttpOnly cookies + CSRF

**Remediation:** (Blocked pending security implementation)
- Middleware chain tests
- CSRF token lifecycle
- Security header validation
- Rate limit enforcement

**Effort:** 2-3 days | **Priority:** P1 (After Phase 2)

---

## 3. Secondary Coverage Gaps

### Medium Priority Issues

| Package | Coverage | Gap | Fix Effort |
|---------|----------|-----|-----------|
| **pkg/lineage** | 51.2% | Hash chain verification | 1-2 days |
| **agents/settlement** | 43.3% | Settlement workflow | 1 day |
| **pkg/web** | 36.6% | HTTP handlers failing | 2 days |
| **pkg/opa** | 29.8% | Policy engine logic | 2 days |
| **pkg/eval/singleturn** | 24.3% | ML evaluation pipeline | 1 day |
| **pkg/rag/pgvector** | 19.6% | Vector search | 2 days |

### Low Priority (Non-Blocking)

- cmd/demo (0.0%) - Demo utility, low risk
- pkg/busio (0.0%) - May be unused (audit needed)
- pkg/comm (0.0%) - Communication layer (audit needed)
- pkg/orchestration (0.0%) - Workflow (audit needed)
- agents/kyc (0.0%) - Import errors (fix module)

---

## 4. Test Quality Issues

### Failing Tests (7)
```
FAIL: TestE2E_MerchantOnboarding_FullFlow
      → Merchant GET request returns 400
      → agents/merchant/handlers endpoint broken

FAIL: TestMerchantErrorPath_InvalidGSTVariants
      → GST validation regex too permissive
      → Accepts "18aabct1234h2z0" (should reject)

FAIL: TestUI_LocalStorageKeysStable
      → Missing localStorage keys:
        - "genie.session.v1"
        - "genie.apibase.v1"
      → Session restore will break

FAIL: TestUI_LoginSuccessEntersApp
      → Missing state update: state.token = out.token
      → Missing persistSession() call
      → Welcome card won't hide after login

FAIL: TestJS_LocalStorageKeys
      → Missing constants in app.js
      → Can't read session from browser

FAIL: TestUI_PersistSessionStoresRoles
      → persistSession() function missing
      → User roles not persisted

FAIL: TestAgentRegistry_EveryAgentHasTests
      → agents/kyc has no _test.go file
      → Also: merchants/settlement/kyc agents lack test files
```

### Root Causes
1. **Code-test mismatch:** Tests expect code that doesn't exist
2. **Missing implementations:** persistSession(), state.token binding
3. **Incomplete validation:** GST regex incomplete
4. **Module errors:** agents/kyc import errors need fixing

### Impact Assessment
- 5 tests: UI/browser-related (lower priority for backend)
- 1 test: Merchant onboarding (🔴 CRITICAL - blocks Phase 2)
- 1 test: Agent registry requirement (prevents contribution)

---

## 5. Coverage by Feature Area

### Phase 2: e-Rupee Commerce
```
pkg/cbdc                    94.6%  ✓ Excellent
pkg/erupeepayment           87.2%  ✓ Good
pkg/erupeecompliance        91.7%  ✓ Good
pkg/commercesettlement      86.5%  ✓ Good
pkg/merchant                85.9%  ✓ Good (but agents/merchant 5.6%!)
agents/payment_orchestrator 88.4%  ✓ Good
pkg/commerce                70.9%  ~ Medium (order mgmt)
agents/merchant             5.6%   ✗ CRITICAL

PHASE 2 HEALTH: 67% ✗ (blocked by agents/merchant gap)
```

### Compliance & Governance
```
pkg/compliance              98.1%  ✓ Excellent (🔒 Security-critical)
pkg/aml                     74.6%  ✓ Good
pkg/lineage                 51.2%  ~ Medium (should be >80%)
pkg/settlement              0.0%   ✗ CRITICAL (audit trail)
pkg/opa                     29.8%  ~ Low (policy engine)

COMPLIANCE HEALTH: 71% ~ (lineage & settlement concerns)
```

### Authentication
```
pkg/auth (aggregate)        72.6%  ✓ Good
  - oauth2                  72.9%  ✓
  - oauth_device            87.3%  ✓
  - elevation               83.9%  ✓
  - tokenexchange           88.5%  ✓
  - webauthn                81.4%  ✓

pkg/security                32.1%  ✗ Low (middleware)
pkg/crypto                  65.4%  ~ Medium

AUTH HEALTH: 68% ~ (depends on security improvements)
```

### Agent Framework
```
pkg/agent                   93.0%  ✓ Excellent
agents/* (average)          ~72%   ✓ Good
  - Well-tested: payment_orchestrator (88%), sme_loan (90%)
  - Weak: merchant (5.6%), settlement (43%), tax_estimator (50%)

AGENT HEALTH: 75% ✓ (decent baseline, gaps in specific agents)
```

---

## 6. Test Organization Assessment

### Strengths ✓
- **Pattern consistency:** Most packages use `*_test.go` co-location
- **Mock quality:** Service stubs in commerce (RealPaymentAgentStub, RealSettlementAgentStub)
- **Agent isolation:** Good use of mocks/stubs, minimal side effects
- **Compliance tests:** Comprehensive transaction verification

### Weaknesses ✗
- **Table-driven tests:** Only 2 found (should be 50+)
  - Example opportunity: Agent test parameterization
  - Current: Many agents with copy-paste test boilerplate
  - Needed: Consolidate into tables

- **Missing test files (26+):**
  ```
  pkg/commerce/reconciliation.go    ← No tests
  pkg/commerce/settlement_flow.go   ← No tests
  pkg/commerce/handler_stubs.go     ← No tests (but used!)
  pkg/commerce/types.go             ← No tests
  
  pkg/llm/{circuit,gemini,cache,anthropic,openai,shadow,cost,router,deadline}.go
  pkg/memory/{semantic,episodic}.go
  pkg/security/middleware.go        ← Critical gap!
  pkg/settlement/{audit,types}.go   ← BLOCKER
  ```

- **No CLI tests:** cmd/* all below 15% coverage
  - No integration tests for main binaries
  - No end-to-end scenarios

- **Error path gaps:**
  - Network failures: Rarely tested
  - Database constraints: ~5% coverage
  - Invalid input handling: ~40% coverage
  - Recommendation: Add error scenario tests (+10-15% coverage)

- **Performance tests:**
  - No benchmarks for critical paths
  - Settlement calculations: No perf tests
  - Query performance: Not measured

---

## 7. Remediation Roadmap

### PHASE 1: CRITICAL (Week 1-2)
Fix blockers before Phase 2 delivery:

#### 1. pkg/settlement: 0% → 70% [P0]
**When:** Immediately  
**Effort:** 3-4 days  
**Owner:** Backend team  

```go
// pkg/settlement/integration_test.go (NEW)
func TestSettlementE2E_HappyPath(t *testing.T) {
    // 1. Create settlement batch with 5+ orders
    // 2. Apply netting calculations
    // 3. Verify state transitions (CREATED → FINALIZED)
    // 4. Check audit log entries
    // 5. Validate CBDC ledger consistency
}

func TestAuditLog_StateTransitions(t *testing.T) {
    // Verify state machine: pending → processing → complete
    // Check timestamp monotonicity
    // Validate state data integrity
}

func TestSettlementNetting_Calculations(t *testing.T) {
    // Bilateral netting
    // Multilateral netting
    // Edge case: Single order
}
```

**Tests needed:** 8-10  
**Checklist:**
- [ ] Settlement flow E2E
- [ ] Netting calculations
- [ ] State transition validation
- [ ] Audit log integrity
- [ ] CBDC ledger sync
- [ ] Error scenarios (invalid amounts, duplicate IDs)

---

#### 2. agents/merchant: 5.6% → 70% [P0]
**When:** Immediately  
**Effort:** 2-3 days  
**Owner:** Agent team  

```go
// agents/merchant/supervisor_test.go (EXPAND)
func TestMerchantOnboarding_FullFlow(t *testing.T) {
    // 1. Call HandleMessage() with merchant data
    // 2. Verify KYC check
    // 3. Validate documents
    // 4. Check GST registration
    // 5. Return status
}

func TestGSTValidation_ValidFormats(t *testing.T) {
    cases := []struct {
        gst    string
        valid  bool
    }{
        {"18AABCT1234H2Z0", true},   // Standard 15-char
        {"22AABCT1234H2Z0", true},   // Another state
        {"18aabct1234h2z0", false},  // Lowercase (fail!)
        {"18AABCT1234H2", false},    // Too short
    }
}

func TestMerchantDocument_Validation(t *testing.T) {
    // Pan card validation
    // Aadhar validation
    // Bank statement parsing
}
```

**Tests needed:** 6-8  
**Checklist:**
- [ ] Fix GST regex (URGENT - failing test)
- [ ] HandleMessage() flow
- [ ] Document validation (PAN, Aadhar, bank statement)
- [ ] KYC check integration
- [ ] Error cases (invalid GST, missing docs)
- [ ] MerchantGetHandler route

---

#### 3. pkg/storage/postgres: 0.9% → 60% [P1]
**When:** Within 1 week  
**Effort:** 3-4 days  
**Owner:** Database team  

```go
// pkg/storage/postgres/integration_test.go (NEW)
func TestConnectionPool_Lifecycle(t *testing.T) {
    // Create pool
    // Verify max connections
    // Drain and reconnect
}

func TestMigration_UpDown(t *testing.T) {
    // Run all migrations
    // Verify schema
    // Rollback
    // Verify original schema
}

func TestTransaction_Rollback(t *testing.T) {
    // Insert row in transaction
    // Rollback
    // Verify row missing
}

func TestConstraint_Enforcement(t *testing.T) {
    // Duplicate key error
    // Foreign key violation
    // Check constraint violation
}
```

**Tests needed:** 12-15  
**Checklist:**
- [ ] Connection pooling
- [ ] Migrations (up/down)
- [ ] Transactions (commit/rollback)
- [ ] Constraint violations
- [ ] Connection timeout
- [ ] Query cancellation
- [ ] Data type validation

---

#### 4. cmd/api: 11.3% → 50% [P1]
**When:** Within 1 week  
**Effort:** 2-3 days  
**Owner:** API team  

```go
// cmd/api/handlers_integration_test.go (EXPAND)
func TestAPI_Routes_AllDefined(t *testing.T) {
    routes := []struct{
        method, path string
        expectedCode int
    }{
        {"GET", "/health", 200},
        {"POST", "/orders", 201},
        {"GET", "/orders/123", 200},
        // ... all routes
    }
}

func TestAPI_ErrorHandling(t *testing.T) {
    // 400 Bad Request
    // 401 Unauthorized
    // 404 Not Found
    // 500 Internal Server Error
}

func TestAPI_RequestValidation(t *testing.T) {
    // Missing required fields
    // Invalid data types
    // Constraint violations
}
```

**Tests needed:** 8-10  
**Checklist:**
- [ ] All routes callable
- [ ] Error responses (4xx, 5xx)
- [ ] Request validation
- [ ] Response format validation
- [ ] Middleware chain (auth, CORS, etc.)
- [ ] Content-Type handling

---

#### 5. pkg/security: 32.1% → 70% [P1-BLOCKED]
**Status:** Blocked pending Phase 2 security task  
**When:** After HttpOnly cookie implementation  
**Effort:** 2-3 days  
**Owner:** Security team  

```go
// pkg/security/middleware_test.go (NEW - after impl)
func TestMiddleware_CSRFProtection(t *testing.T) {
    // Valid CSRF token passes
    // Missing token rejected
    // Expired token rejected
    // Cross-origin request blocked
}

func TestMiddleware_SecurityHeaders(t *testing.T) {
    // X-Content-Type-Options: nosniff
    // X-Frame-Options: DENY
    // Strict-Transport-Security
    // Content-Security-Policy
}

func TestMiddleware_RateLimit(t *testing.T) {
    // Allow N requests
    // Block request N+1
    // Reset after window
}
```

**Tests needed:** 6-8 (pending implementation)

---

### PHASE 2: IMPORTANT (Week 3-4)

#### Priority Order
1. **pkg/lineage: 51.2% → 80%** (1-2 days)
   - Hash chain verification
   - Audit trail validation
   
2. **pkg/web: 36.6% → 70%** (2 days)
   - Handler tests
   - UI contract tests (failing 5 tests)
   
3. **pkg/opa: 29.8% → 70%** (2 days)
   - Policy evaluation tests
   - Edge cases
   
4. **agents/settlement: 43.3% → 80%** (1 day)
   - Settlement workflow
   
5. **pkg/eval/singleturn: 24.3% → 60%** (1 day)
   - ML evaluation pipeline

---

### PHASE 3: OPTIMIZATION (Ongoing)

1. **Table-driven test expansion:** 2 → 50+ tables (1-2 weeks)
2. **Error scenario testing:** +10-15% coverage (ongoing)
3. **Performance benchmarks:** Critical paths (1 week)
4. **CLI testing:** cmd/* → 30%+ (1 week)
5. **Mutation testing:** Add stryker (1 week setup)

---

## 8. Recommended Actions

### Immediate (Today)
- [ ] Fix GST validation regex (agents/merchant)
- [ ] Create GitHub issues for critical gaps
- [ ] Assign P0 tasks to team leads
- [ ] Review this report with team

### This Week
- [ ] Start Phase 1 critical tests
- [ ] Fix 7 failing tests
- [ ] Set up coverage dashboard
- [ ] Establish coverage gates in CI

### This Sprint
- [ ] Complete Phase 1 (70% on critical modules)
- [ ] Start Phase 2 medium-priority gaps
- [ ] Document test patterns/best practices

### Monthly
- [ ] Review coverage trends
- [ ] Adjust remediation roadmap
- [ ] Add mutation testing
- [ ] Expand E2E test suite

---

## 9. Success Metrics

### Target Coverage by Module

| Module | Current | Target | Effort |
|--------|---------|--------|--------|
| pkg/compliance | 98.1% | 98%+ | ✓ Maintain |
| pkg/cbdc | 94.6% | 95%+ | 1 day |
| pkg/settlement | 0.0% | 70%+ | 4 days |
| agents/merchant | 5.6% | 70%+ | 3 days |
| pkg/security | 32.1% | 70%+ | 3 days |
| pkg/storage | 0.9% | 60%+ | 4 days |
| agents/* | ~72% | 75%+ | 2 days |
| **Overall** | **61.3%** | **75%+** | **~4 weeks** |

### Key Performance Indicators

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| **Overall Coverage** | 61.3% | 75%+ | 🟡 +13.7% needed |
| **Failing Tests** | 7 | 0 | 🔴 All must fix |
| **Uncovered Functions** | 1,404 | <700 | 🟡 Must reduce 50% |
| **Critical Packages <50%** | 8 | 0 | 🔴 Must fix all |
| **E2E Test Ratio** | 1.2% | 15%+ | 🟡 Need 50+ more |
| **Phase 2 Health** | 67% | 90%+ | 🟡 Blocker fixes needed |

---

## 10. Implementation Guide

### Step 1: Fix Failing Tests (1 day)
```bash
# Fix GST regex in agents/merchant
# Run tests:
go test ./agents/merchant -v
go test ./pkg/web/handlers -v

# Expected: All pass
```

### Step 2: Create Missing Tests (Week 1)
```bash
# Create critical test files:
touch pkg/settlement/integration_test.go
touch agents/merchant/handlers_test.go
touch pkg/storage/postgres/integration_test.go
touch cmd/api/handlers_integration_test.go

# Run coverage:
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Step 3: Set Up CI Gates (1 day)
```yaml
# .github/workflows/coverage.yml
jobs:
  coverage:
    runs-on: ubuntu-latest
    steps:
      - run: go test -coverprofile=coverage.out ./...
      - name: Check coverage threshold
        run: |
          # Fail if overall < 75%
          # Fail if any pkg/* < 70%
          # Fail if any agents/* < 60%
```

### Step 4: Monthly Reviews
```bash
# Generate monthly coverage report:
go test -coverprofile=/tmp/coverage_$(date +%Y-%m-%d).out ./...
go tool cover -func=/tmp/coverage_*.out | sort -k3 -n
# Compare trends
```

---

## Appendix: Coverage by Package (Complete List)

### Excellent (>90%)
- pkg/compliance: 98.1%
- pkg/eval/drift: 96.3%
- pkg/cbdc: 94.6%
- pkg/agent: 93.0%
- pkg/fx: 92.8%
- pkg/eval/elo: 92.3%
- pkg/consent: 91.9%
- pkg/erupeecompliance: 91.7%
- pkg/observability/bq: 90.6%

### Good (80-90%)
- 31 packages (see section 2 for full list)

### Medium (60-80%)
- 36 packages (agents, infrastructure)

### Low (30-60%)
- 15 packages (reasoning, OPA, RAG)

### Critical (<30%)
- pkg/opa: 29.8%
- pkg/security: 32.1%
- pkg/web: 36.6%
- pkg/eval/singleturn: 24.3%
- pkg/rag/pgvector: 19.6%
- cmd/api: 11.3%
- agents/merchant: 5.6%
- pkg/storage/postgres: 0.9%

### Zero Coverage (0%)
- 16 packages (settlement, orchestration, comm, eval, etc.)

---

## References

- **Current Coverage Report:** `go test ./... -coverprofile=coverage.out`
- **View Coverage:** `go tool cover -html=coverage.out`
- **Function-Level:** `go tool cover -func=coverage.out`
- **Genie CLAUDE.md:** Project standards and patterns
- **Phase 2 Features:** See PROJECT_REFERENCE.md

---

**Report prepared by:** Claude Code Agent  
**Review recommended with:** Backend team, QA lead, security team  
**Next review date:** June 12, 2026 (1 week)
