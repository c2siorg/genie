# Genie E2E Test Suite

Comprehensive end-to-end tests for the Genie financial platform using Playwright, covering all critical workflows:

- **Settlement**: Order → Payment → Settlement → Reconciliation
- **Security**: CSRF protection, HttpOnly cookies, JWT validation, XSS prevention
- **Evaluation**: Trace review, failure mode annotation, judge verdicts, rubric definitions
- **Compliance**: KYC verification, AML risk scoring, velocity limits, sanctions screening

## Quick Start

### Prerequisites

- Node.js 18+
- Docker & Docker Compose
- Go 1.25+
- npm or yarn

### Setup

1. **Install dependencies:**

```bash
cd e2e
npm install
npx playwright install  # Install browser binaries
```

2. **Start local environment:**

```bash
# With mock LLM (fastest)
docker-compose up -d

# Or with Ollama for realistic LLM testing
docker-compose --profile with-ollama up -d
```

3. **Wait for services to be healthy:**

```bash
docker-compose ps  # Check "healthy" status
```

## Running Tests

### All Tests

```bash
npm test
```

### Specific Test Suites

```bash
npm run test:settlement    # Settlement workflow tests
npm run test:security      # Security & CSRF tests
npm run test:evaluation    # Evaluation dashboard tests
npm run test:compliance    # Compliance & AML tests
```

### Browser-Specific Tests

```bash
npm run test:chrome        # Chromium only
npm run test:firefox       # Firefox only
npm run test:webkit        # WebKit (Safari) only
npm run test:mobile        # Mobile Chrome emulation
```

### Interactive Modes

```bash
npm run test:ui            # UI mode with visual control
npm run test:debug         # Debug mode (opens inspector)
npm run test:headed        # Headed mode (visible browser)
```

### Parallel & Serial Execution

```bash
npm run test:parallel      # 4 workers (default)
npm run test:serial        # Single worker (no parallelism)
```

### CI Environment

```bash
npm run test:ci            # Outputs JSON, JUnit, HTML reports
```

## Test Organization

```
e2e/
├── playwright.config.ts        # Playwright configuration
├── fixtures/
│   ├── auth.fixture.ts         # JWT, login, CSRF helpers
│   └── api.fixture.ts          # API request helpers
├── pages/
│   ├── SettlementPage.ts       # Settlement UI page object
│   └── EvalReviewPage.ts       # Evaluation dashboard page object
├── tests/
│   ├── settlement.spec.ts      # Settlement workflow E2E tests
│   ├── security.spec.ts        # CSRF, cookies, headers, rate-limit
│   ├── evaluation.spec.ts      # Trace annotation, judges, rubrics
│   └── compliance.spec.ts      # KYC, AML, velocity, sanctions
├── docker-compose.yml          # Local test environment
└── package.json
```

## Test Coverage

### Settlement Tests (18 tests)

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

### Security Tests (22 tests)

**CSRF Protection:**
- ✅ CSRF token in form
- ✅ Reject POST without token
- ✅ Reject invalid CSRF token
- ✅ Accept valid CSRF token
- ✅ Token refresh on page reload

**Cookie Security:**
- ✅ HttpOnly flag on session cookie
- ✅ Secure flag on HTTPS
- ✅ SameSite=Strict on auth cookie
- ✅ JavaScript cannot access HttpOnly cookies
- ✅ Cookies cleared on logout

**Security Headers:**
- ✅ XSS protection headers
- ✅ Content Security Policy
- ✅ Referrer-Policy
- ✅ Server information not exposed

**JWT Validation:**
- ✅ Reject expired JWT
- ✅ Reject malformed JWT
- ✅ Reject JWT with wrong secret
- ✅ Valid JWT contains proper claims

**Input Validation:**
- ✅ Order ID input sanitization
- ✅ Notes field XSS prevention
- ✅ SQL injection in search
- ✅ Rate limiting on login
- ✅ Rate limiting on API

### Evaluation Tests (22 tests)

- ✅ Dashboard load and trace display
- ✅ Trace JSON data structure
- ✅ CSRF token validation
- ✅ Mark trace as PASS
- ✅ Mark trace as FAIL with failure mode
- ✅ Confidence scoring (0-1)
- ✅ Annotation notes
- ✅ Feedback submission
- ✅ Trace navigation (next/previous)
- ✅ Similar traces discovery
- ✅ Rubric definitions display
- ✅ Trace search by ID
- ✅ Filter by failure mode
- ✅ Keyboard shortcuts (P, N, D, F, S)
- ✅ Trace counter display

### Compliance Tests (20 tests)

**KYC Workflow:**
- ✅ Block unverified customer
- ✅ Complete KYC verification
- ✅ Reject invalid KYC data
- ✅ Escalate for manual review

**AML Risk Scoring:**
- ✅ Calculate risk score (0-100)
- ✅ Flag high-risk customers
- ✅ Require enhanced due diligence
- ✅ Match against sanctions list

**Velocity Limits:**
- ✅ Track transaction velocity
- ✅ Block transaction exceeding limit
- ✅ Allow transaction within limit
- ✅ Allow limit increase with approval

**Compliance Audit Trail:**
- ✅ Record all decisions
- ✅ Link decision to transaction

**High-Risk Handling:**
- ✅ Escalate high-risk transactions
- ✅ Block failed compliance checks

## Configuration

### Environment Variables

```bash
# API endpoint
BASE_URL=http://localhost:8080

# LLM backend (for realistic testing)
GENIE_LLM=mock              # mock | ollama | openai
GENIE_OLLAMA_URL=http://ollama:11434
GENIE_OLLAMA_CHAT=llama3.2:1b

# Database
GENIE_DB_DSN=postgres://genie:genie_dev_password@db:5432/genie_test?sslmode=disable

# JWT
GENIE_JWT_SECRET=test-secret-key-for-genie-e2e-testing

# Encryption
GENIE_KEK_BASE64=dGVzdC1lbmNyeXB0aW9uLWtleS0zMi1ieXRlcy1iYXNlNjQ=

# Test behavior
SKIP_WEB_SERVER=0           # 1 to skip launching API
CI=1                        # Set in CI environments
SECURE_COOKIES_ENFORCED=0   # 1 to enforce HTTPS cookies
```

### Playwright Configuration

See `playwright.config.ts` for:
- Browser configurations (Chromium, Firefox, WebKit, Mobile)
- Screenshot and video capture settings
- Trace collection for debugging
- Timeout settings
- Reporter configuration

## Page Objects

### SettlementPage

```typescript
// UI interactions
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

### EvalReviewPage

```typescript
// Navigation & tracing
await evalPage.goto();
await evalPage.nextTrace();
await evalPage.previousTrace();
const traceId = await evalPage.getCurrentTraceId();

// Annotation workflow
await evalPage.markAsPass();
await evalPage.markAsFail(failureMode);
await evalPage.setConfidence(0.9);
await evalPage.addNotes('Notes here');
await evalPage.submitFeedback();
await evalPage.deferTrace();

// Data retrieval
const trace = await evalPage.getTraceJSON();
const similar = await evalPage.getSimilarTraces();
const rubrics = await evalPage.getRubricDefinitions();
const shortcuts = await evalPage.getKeyboardShortcuts();
```

## Fixtures

### authFixture

Provides JWT token generation and auth helpers:

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

### apiFixture

Provides API request helpers:

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

## Reporting

Test results are generated automatically:

```bash
# View HTML report
npm run show:report

# JSON report (for CI parsing)
playwright-report/index.json

# JUnit XML (for Jenkins/GitHub Actions)
test-results.xml

# Video & Screenshots
test-results/
├── screenshots/
├── videos/
└── traces/
```

## Debugging

### Debug Mode (with Inspector)

```bash
npm run test:debug
```

Opens Playwright Inspector with:
- Step-through execution
- Element locator helper
- Console access

### Trace Files

Traces are captured for failed tests:

```bash
npx playwright show-trace playwright-report/trace.zip
```

Explore:
- Network requests
- DOM snapshots
- JavaScript console logs
- Screenshots at each step

### Video Playback

Failed tests record videos:

```bash
test-results/my-test-fails/video.webm
```

### Codegen (Record Tests)

Generate test code by interacting with the app:

```bash
npm run codegen
# Opens browser, shows interactions as code
# Useful for creating new tests
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Install Playwright browsers
  run: npm ci --cwd e2e && npx playwright install

- name: Start services
  run: docker-compose -f e2e/docker-compose.yml up -d

- name: Wait for health
  run: docker-compose -f e2e/docker-compose.yml exec -T genie-api curl -f http://localhost:8080/health

- name: Run E2E tests
  run: npm ci --cwd e2e && npm run test:ci --cwd e2e

- name: Upload report
  uses: actions/upload-artifact@v3
  with:
    name: playwright-report
    path: e2e/playwright-report/
```

### Local CI

```bash
npm run test:ci
```

Generates reports suitable for CI environments.

## Troubleshooting

### Tests timeout

Increase timeout in `playwright.config.ts`:

```typescript
timeout: 60_000,  // 60 seconds per test
```

### Database connection fails

Ensure PostgreSQL is healthy:

```bash
docker-compose ps
docker-compose logs db
```

### API server won't start

Check environment variables and logs:

```bash
docker-compose logs genie-api
docker-compose ps
```

### Flaky tests

1. **Increase waits**: Use `waitForLoadState('networkidle')`
2. **Retry strategy**: Set `retries` in config
3. **Video debug**: Check recording for timing issues

```typescript
timeout: 30_000,
expect: { timeout: 5_000 },
retries: process.env.CI ? 2 : 0,
```

## Best Practices

1. **Page Objects**: Always use page objects for UI interactions
2. **Fixture Composition**: Extend fixtures for better code reuse
3. **Parallel Tests**: Design tests to run independently
4. **Assertions**: Use `expect()` for all validations
5. **Cleanup**: Use `beforeEach`/`afterEach` for setup/teardown
6. **Test Isolation**: Each test should be self-contained
7. **Comments**: Document complex test scenarios
8. **Error Messages**: Use `.catch()` to provide helpful context

## Resources

- [Playwright Documentation](https://playwright.dev)
- [Genie API Documentation](../docs/api.md)
- [Test Architecture](./ARCHITECTURE.md)
- [Contributing Tests](./CONTRIBUTING.md)

---

**Last Updated**: June 6, 2026  
**Status**: Complete ✅  
**Coverage**: Settlement, Security, Evaluation, Compliance  
**Browsers**: Chrome, Firefox, Safari, Mobile
