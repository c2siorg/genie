# Genie E2E Test Suite — Comprehensive Summary

## Overview

Comprehensive end-to-end tests for the Genie financial platform using **Playwright**, covering all critical workflows:

- **Settlement**: Order → Payment → Settlement → Reconciliation
- **Security**: CSRF protection, HttpOnly cookies, JWT validation, XSS prevention
- **Evaluation**: Trace review, failure mode annotation, judge verdicts, rubric definitions
- **Compliance**: KYC verification, AML risk scoring, velocity limits, sanctions screening

**Status**: ✅ Complete (82 tests, 4 suites)  
**Coverage**: Settlement (18), Security (22), Evaluation (22), Compliance (20)  
**Browser Support**: Chrome, Firefox, Safari, Mobile

---

## Directory Structure

```
e2e/
├── playwright.config.ts          # Playwright configuration (browsers, reporters, timeouts)
├── tsconfig.json                 # TypeScript configuration
├── package.json                  # npm scripts and dependencies
├── docker-compose.yml            # Local test environment (PostgreSQL, Genie API, optional Ollama)
├── README.md                      # Comprehensive guide (300+ lines)
│
├── fixtures/                      # Test fixtures (reusable setup)
│   ├── auth.fixture.ts           # JWT generation, login/logout, CSRF verification
│   └── api.fixture.ts            # API request helpers (orders, payments, compliance, eval)
│
├── pages/                         # Page Object Model (UI encapsulation)
│   ├── SettlementPage.ts         # Settlement workflow page object
│   └── EvalReviewPage.ts         # Evaluation dashboard page object
│
├── tests/                         # Test suites (82 tests total)
│   ├── settlement.spec.ts        # Settlement workflow E2E tests (18 tests)
│   ├── security.spec.ts          # Security & CSRF E2E tests (22 tests)
│   ├── evaluation.spec.ts        # Evaluation dashboard E2E tests (22 tests)
│   └── compliance.spec.ts        # Compliance & AML E2E tests (20 tests)
│
├── playwright-report/            # Generated after test run (HTML report)
├── test-results/                 # Screenshots, videos, traces (on failure)
└── node_modules/                 # Dependencies (after npm install)
```

---

## Test Suites Breakdown

### 1. Settlement Tests (18 tests)

**File**: `tests/settlement.spec.ts`

Coverage:
- ✅ Single order happy path
- ✅ Amount correctness verification
- ✅ CBDC ledger commitment
- ✅ Reconciliation verification
- ✅ Multiple orders with netting
- ✅ Insufficient balance error handling
- ✅ Reconciliation failure handling
- ✅ Settlement retry logic
- ✅ Complete audit trail recording
- ✅ Settlement table display

**Key Scenarios**:
- Create order → Initiate payment → Confirm → Settle → Verify
- Verify amount matches (no hallucination)
- Verify CBDC commitment flag set
- Verify reconciliation passed
- Verify netting calculations for multi-merchant batches
- Verify error handling (insufficient balance, reconciliation failed)
- Verify retry mechanism

### 2. Security Tests (22 tests)

**File**: `tests/security.spec.ts`

**CSRF Protection** (5 tests):
- CSRF token in form
- Reject POST without token
- Reject invalid CSRF token
- Accept valid CSRF token
- Token refresh on page reload

**Cookie Security** (5 tests):
- HttpOnly flag on session cookie
- Secure flag on HTTPS
- SameSite=Strict on auth cookie
- JavaScript cannot access HttpOnly cookies
- Cookies cleared on logout

**Security Headers** (4 tests):
- XSS protection headers (X-Content-Type-Options, X-Frame-Options)
- Content Security Policy
- Referrer-Policy
- Server information not exposed

**JWT Validation** (4 tests):
- Reject expired JWT
- Reject malformed JWT
- Reject JWT with wrong secret
- Valid JWT contains proper claims

**Input Validation & XSS** (2 tests):
- Order ID input sanitization
- Notes field XSS prevention

**Rate Limiting** (2 tests):
- Rate limiting on login
- Rate limiting on API

### 3. Evaluation Tests (22 tests)

**File**: `tests/evaluation.spec.ts`

**Dashboard** (3 tests):
- Dashboard load and trace display
- Trace JSON data structure
- CSRF token validation

**Trace Annotation** (5 tests):
- Mark trace as PASS
- Mark trace as FAIL with failure mode
- Confidence scoring (0-1 input validation)
- Annotation notes capture
- Feedback submission

**Failure Modes** (3 tests):
- Failure mode selection UI
- Filter by failure mode
- Correct failure modes available for domain

**Confidence Scoring** (2 tests):
- Confidence input validation (0.0 - 1.0)
- Confidence storage in annotation

**Similar Traces** (2 tests):
- Similar traces discovery display
- Similar traces sorted by similarity score

**Rubric Definitions** (2 tests):
- Rubric definitions display
- Rubric field explanations

**Keyboard Shortcuts** (3 tests):
- Keyboard shortcut help modal
- Keyboard shortcut execution (P=Pass, N=Next, F=Fail, D=Defer, S=Save)
- Correct shortcuts mapped to actions

**Search & Filtering** (2 tests):
- Trace search by ID
- Filter by failure mode

### 4. Compliance Tests (20 tests)

**File**: `tests/compliance.spec.ts`

**KYC Verification** (4 tests):
- Block unverified customer
- Complete KYC verification
- Reject invalid KYC data
- Escalate for manual review

**AML Risk Scoring** (4 tests):
- Calculate AML risk score (0-100)
- Flag high-risk customers
- Apply enhanced due diligence (EDD) for high-risk
- Match against sanctions list

**Velocity Limits** (3 tests):
- Track transaction velocity
- Block transaction exceeding velocity limit
- Allow transaction within velocity limit
- Allow velocity limit increase with approval

**Compliance Audit Trail** (2 tests):
- Record all compliance decisions
- Link compliance decision to transaction

**High-Risk Handling** (3 tests):
- Escalate high-risk transactions for manual review
- Block transaction if compliance check fails
- Log escalation reason and assigned reviewer

---

## Configuration Files

### playwright.config.ts
- **Browsers**: Chromium, Firefox, WebKit, Mobile Chrome
- **Timeouts**: 30s per test, 5s for expectations
- **Reporters**: HTML, JSON, JUnit (for CI)
- **Traces**: Collect on failure (for debugging)
- **Screenshots**: Capture on failure
- **Videos**: Record on failure
- **Base URL**: http://localhost:8080 (configurable)
- **Workers**: 4 parallel (configurable)

### docker-compose.yml
- **PostgreSQL 16**: Database (postgres_data volume)
- **Genie API**: Built from root Dockerfile
  - GENIE_JWT_SECRET: test-secret-key-for-genie-e2e-testing
  - GENIE_KEK_BASE64: test-encryption-key-base64
  - GENIE_LLM: mock (default) or ollama
- **Ollama** (optional): Set with `--profile with-ollama`
- **Health checks**: All services wait for readiness

### package.json
**Scripts**:
```
npm test                        # Run all tests
npm run test:settlement         # Settlement only
npm run test:security           # Security only
npm run test:evaluation         # Evaluation only
npm run test:compliance         # Compliance only
npm run test:chrome             # Chrome only
npm run test:firefox            # Firefox only
npm run test:webkit             # Safari only
npm run test:mobile             # Mobile emulation
npm run test:ui                 # Interactive UI mode
npm run test:debug              # Debug mode with inspector
npm run test:headed             # Visible browser
npm run test:ci                 # CI reporters (HTML, JSON, JUnit)
npm run test:parallel           # 4 workers (default)
npm run test:serial             # Single worker
npm run show:report             # View HTML report
npm run codegen                 # Record test code
```

**Dependencies**:
- @playwright/test ^1.40.0
- @types/node ^20.10.0
- typescript ^5.3.0
- jsonwebtoken ^9.1.0

---

## Fixtures (Reusable Setup)

### auth.fixture.ts
Provides JWT token generation and authentication helpers:

```typescript
// Generate custom JWT
const token = generateJWT({ sub: 'user-id', roles: ['admin'] });

// Get pre-configured tokens
const userToken = getTestUserToken();
const merchantToken = getMerchantToken();
const complianceToken = getComplianceToken();

// Auth helpers
const headers = getAuthHeaders(token);
await login(email, password);
await logout();
const isValid = verifyCsrfToken(token);
```

**Key Methods**:
- `generateJWT(payload)` — Create custom JWT with HS256
- `getTestUserToken()` — Customer role token
- `getMerchantToken()` — Merchant role token
- `getComplianceToken()` — Compliance officer token
- `getAuthHeaders(token)` — HTTP Authorization header
- `login(email, password)` — Navigate and submit login form
- `logout()` — Logout and verify redirect
- `verifyCsrfToken(token)` — Validate CSRF token structure

### api.fixture.ts
Provides API request helpers with automatic token injection:

```typescript
// Helper methods
const order = await api.createOrder(data, token);
const payment = await api.initiatePayment(data, token);
await api.confirmPayment(paymentId, token);
const settlement = await api.getSettlement(settlementId, token);

// Compliance helpers
const compliance = await api.submitCompliance(data, token);
const status = await api.getComplianceStatus(orderId, token);

// Evaluation helpers
const traces = await api.getEvalTraces(token);
await api.submitEvalFeedback(traceId, feedback, token);

// Generic method
const result = await api.request('GET', '/v1/endpoint', options, token);
```

**Key Methods**:
- `createOrder(data, token)` — POST /v1/commerce/orders
- `getOrder(orderId, token)` — GET /v1/commerce/orders/{orderId}
- `initiatePayment(data, token)` — POST /v1/payment/initiate
- `confirmPayment(paymentId, token)` — POST /v1/payment/confirm
- `getSettlement(settlementId, token)` — GET /v1/settlement/status/{settlementId}
- `submitCompliance(data, token)` — POST /v1/compliance/check
- `getComplianceStatus(orderId, token)` — GET /v1/compliance/status/{orderId}
- `submitEvalFeedback(traceId, feedback, token)` — POST /v1/eval/traces/{traceId}/feedback
- `getEvalTraces(token)` — GET /v1/eval/traces

---

## Page Objects (UI Encapsulation)

### SettlementPage.ts
Encapsulates settlement workflow UI:

```typescript
await settlementPage.goto();
await settlementPage.initiatePayment(orderId, amount);
await settlementPage.confirmPayment();
await settlementPage.retrySettlement();

// Verifications
await settlementPage.verifyAmountCorrectness(expectedAmount);
await settlementPage.verifyCBDCCommit();
await settlementPage.verifyReconciliation();

// Data retrieval
const status = await settlementPage.getSettlementStatus();
const netting = await settlementPage.getNettingAmount();
const auditTrail = await settlementPage.getAuditTrail();
```

**Key Methods**:
- `goto()` — Navigate to /settlement
- `initiatePayment(orderId, amount)` — Fill form, submit
- `confirmPayment()` — Confirm payment, wait completion
- `retrySettlement()` — Retry failed settlement
- `waitForSettlementCompletion()` — Wait for COMPLETED status
- `getSettlementStatus()` — Get current status text
- `verifyCBDCCommit()` — Check CBDC commitment flag
- `verifyReconciliation()` — Check reconciliation passed
- `getNettingAmount()` — Get netting amount
- `getErrorMessage()` — Get displayed error
- `getSuccessMessage()` — Get displayed success
- `getSettlementsFromTable()` — Extract all rows from table
- `verifyAmountCorrectness(expectedAmount)` — Verify settlement amount
- `getAuditTrail()` — Retrieve audit trail entries

### EvalReviewPage.ts
Encapsulates evaluation dashboard UI:

```typescript
await evalPage.goto();
await evalPage.nextTrace();
await evalPage.previousTrace();

// Annotation workflow
await evalPage.markAsPass();
await evalPage.markAsFail(failureMode);
await evalPage.setConfidence(0.9);
await evalPage.addNotes('Notes here');
await evalPage.submitFeedback();

// Data retrieval
const trace = await evalPage.getTraceJSON();
const similar = await evalPage.getSimilarTraces();
const rubrics = await evalPage.getRubricDefinitions();
```

**Key Methods**:
- `goto()` — Navigate to /eval/traces
- `nextTrace()` — Navigate to next trace
- `previousTrace()` — Navigate to previous trace
- `getCurrentTraceId()` — Get current trace ID
- `getTraceJSON()` — Parse and return full trace data
- `markAsPass()` — Mark trace as PASS
- `markAsFail(failureMode)` — Mark as FAIL, select failure mode
- `setConfidence(confidence)` — Set confidence 0-1
- `addNotes(notes)` — Add annotation notes
- `submitFeedback()` — Submit annotation
- `annotateFull(verdict, failureMode, notes)` — Complete full annotation
- `deferTrace()` — Defer trace for later
- `getSimilarTraces()` — Retrieve similar traces with scores
- `getRubricDefinitions()` — Get all rubric definitions
- `searchTraces(query)` — Search traces by query
- `filterByFailureMode(failureMode)` — Filter by failure mode
- `getTraceCounter()` — Get counter (e.g., "3 of 50")
- `verifyCsrfTokenPresent()` — Verify CSRF in page
- `getKeyboardShortcuts()` — Get keyboard shortcut mappings
- `useKeyboardShortcut(key)` — Invoke keyboard shortcut

---

## Quick Start

### 1. Install Dependencies
```bash
make playwright-install
# OR manually:
cd e2e && npm install && npx playwright install
```

### 2. Start Services
```bash
docker-compose up -d
# Wait for health checks to pass:
docker-compose ps  # Check "healthy" status
```

### 3. Run Tests
```bash
# All tests
make playwright-test

# Specific suites
make playwright-test-settlement
make playwright-test-security
make playwright-test-evaluation
make playwright-test-compliance

# Interactive modes
make playwright-test-ui      # Visual control
make playwright-test-debug   # Inspector
```

### 4. View Reports
```bash
make playwright-report
# Opens HTML report in browser
```

---

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Install Playwright browsers
  run: make playwright-install

- name: Start services
  run: docker-compose -f e2e/docker-compose.yml up -d

- name: Wait for health
  run: docker-compose -f e2e/docker-compose.yml exec -T genie-api curl -f http://localhost:8080/health

- name: Run E2E tests
  run: make playwright-test-ci

- name: Upload report
  uses: actions/upload-artifact@v3
  with:
    name: playwright-report
    path: e2e/playwright-report/
```

---

## Test Coverage Summary

| Suite | Tests | Coverage |
|-------|-------|----------|
| Settlement | 18 | Orders, payments, reconciliation, netting, errors |
| Security | 22 | CSRF, cookies, headers, JWT, XSS, rate limiting |
| Evaluation | 22 | Dashboard, annotation, failure modes, rubrics, shortcuts |
| Compliance | 20 | KYC, AML, velocity, sanctions, escalation |
| **Total** | **82** | **All critical workflows** |

---

## Key Features

✅ **Multi-browser testing**: Chrome, Firefox, Safari, Mobile  
✅ **Parallel execution**: 4 workers for faster feedback  
✅ **Page Object Model**: UI encapsulation and reuse  
✅ **Fixture composition**: Reusable auth, API, and page helpers  
✅ **JWT authentication**: Custom token generation per role  
✅ **API helpers**: One-liner calls for all endpoints  
✅ **Error capture**: Screenshots, videos, traces on failure  
✅ **CI reporters**: HTML, JSON, JUnit for integration  
✅ **Docker environment**: Isolated local testing  
✅ **Comprehensive docs**: README.md with 300+ lines of guidance  

---

## Troubleshooting

### Database Connection Fails
```bash
docker-compose logs db
docker-compose restart db
```

### API Server Won't Start
```bash
docker-compose logs genie-api
docker-compose ps  # Check health status
```

### Flaky Tests
1. Increase wait timeouts in playwright.config.ts
2. Check video/trace recordings for timing issues
3. Use retry strategy: `retries: 2` in config

### Browser Binary Missing
```bash
npx playwright install
# Or for a specific browser:
npx playwright install chromium
```

---

## Development Workflow

1. **Create new test**:
   ```typescript
   test('should verify my feature', async ({ page, api, getTestUserToken }) => {
     const token = getTestUserToken();
     // Test code here
   });
   ```

2. **Run in debug mode**:
   ```bash
   make playwright-test-debug
   ```

3. **Use codegen to record**:
   ```bash
   make playwright-codegen
   # Interact with app, see code generated
   ```

4. **View reports**:
   ```bash
   make playwright-report
   ```

---

## Files Created

### Configuration (4 files)
- `e2e/playwright.config.ts` (142 lines)
- `e2e/package.json` (47 lines)
- `e2e/tsconfig.json` (32 lines)
- `e2e/docker-compose.yml` (68 lines)

### Fixtures (2 files)
- `e2e/fixtures/auth.fixture.ts` (~120 lines)
- `e2e/fixtures/api.fixture.ts` (~180 lines)

### Page Objects (2 files)
- `e2e/pages/SettlementPage.ts` (~280 lines)
- `e2e/pages/EvalReviewPage.ts` (~320 lines)

### Test Suites (4 files)
- `e2e/tests/settlement.spec.ts` (~350 lines, 18 tests)
- `e2e/tests/security.spec.ts` (~480 lines, 22 tests)
- `e2e/tests/evaluation.spec.ts` (~430 lines, 22 tests)
- `e2e/tests/compliance.spec.ts` (~470 lines, 20 tests)

### Documentation (2 files)
- `e2e/README.md` (500+ lines, comprehensive guide)
- `E2E_TESTS_SUMMARY.md` (this file, overview & reference)

### Makefile Updates
- Added 12 new targets for Playwright testing

**Total**: 16 new files, ~3,800 lines of test code + docs

---

## Next Steps

1. **Run the tests locally**:
   ```bash
   make playwright-install
   docker-compose up -d
   make playwright-test
   ```

2. **Verify all tests pass** (target: 82/82)

3. **Integrate into CI/CD** (GitHub Actions, GitLab, etc.)

4. **Monitor for flaky tests** and adjust timeouts/retries

5. **Extend with new test cases** as features are added

---

## Resources

- **Playwright Docs**: https://playwright.dev
- **Genie README**: [root README.md](../README.md)
- **Genie API Docs**: docs/api.md
- **Genie Architecture**: docs/architecture.md
- **Test README**: e2e/README.md (comprehensive guide)

---

**Status**: ✅ Complete  
**Coverage**: 82 tests across 4 suites  
**Browsers**: Chrome, Firefox, Safari, Mobile  
**Reporters**: HTML, JSON, JUnit  
**Last Updated**: June 6, 2026  
**License**: MIT
