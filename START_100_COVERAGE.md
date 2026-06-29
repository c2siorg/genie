# 100% Test Coverage — Getting Started

**Goal**: Comprehensive testing across E2E, Backend, UI, Evaluation  
**Timeline**: 10 weeks to reach 88%+ coverage with 1,331 tests  
**Status**: Ready to start

---

## 📖 Quick Navigation

- **Master Plan**: `TEST_COVERAGE_MASTER_PLAN.md` (complete 10-week roadmap)
- **Week 1-2 Guide**: Phase 1 — Frontend E2E Extension
- **Week 3-4 Guide**: Phase 2 — Frontend Unit & Components  
- **Week 5-6 Guide**: Phase 3 — Accessibility & Visual
- **Week 7-8 Guide**: Phase 4 — Backend Unit Tests
- **Week 9-10 Guide**: Phase 5 — Integration & Judge

---

## 🚀 Week 1-2: Frontend E2E Extension

### Current State
```
E2E Tests: 82 tests
├─ Settlement:  18 tests ✓
├─ Security:    22 tests ✓
├─ Evaluation:  22 tests ✓
└─ Compliance:  20 tests ✓
```

### Target State
```
E2E Tests: 150 tests
├─ Settlement:  35 tests (+17)
├─ Security:    40 tests (+18)
├─ Evaluation:  40 tests (+18)
└─ Compliance:  35 tests (+15)
```

### Day-by-Day Breakdown

#### Days 1-2: Settlement Extensions (17 new tests)

**Create**: `e2e/tests/settlement-extended.spec.ts`

```typescript
import { test, expect } from '@playwright/test';
import { SettlementPage } from '../pages/SettlementPage';

// New test suite
test.describe('Settlement Extended Features', () => {
  // 1. Multiple merchants settlement
  test('should settle multiple merchants with netting', async ({ page, api }) => {
    // Create 3 orders from different merchants
    // Settle all together
    // Verify netting applied correctly
  });

  // 2. Partial refund workflow
  test('should handle partial refund settlement', async ({ page, api }) => {
    // Create order, settle, then refund 50%
    // Verify refund amount and status
  });

  // 3. Chargeback handling
  test('should handle chargeback lifecycle', async ({ page, api }) => {
    // Settle, then submit chargeback
    // Verify reversal
  });

  // ... 14 more tests (see master plan for details)
});
```

**Effort**: 3-4 hours per developer-day  
**Verify**: `npm run test:settlement` shows 35 tests passing

#### Days 3-4: Security Extensions (18 new tests)

**Create**: `e2e/tests/security-extended.spec.ts`

Focus areas:
- CORS validation (4 tests)
- SQL injection prevention (5 tests)
- API authentication (4 tests)
- Advanced rate limiting (5 tests)

**Effort**: 3-4 hours per developer-day  
**Verify**: `npm run test:security` shows 40 tests passing

#### Days 5-6: Evaluation Extensions (18 new tests)

**Create**: `e2e/tests/evaluation-extended.spec.ts`

Focus areas:
- Multi-select failure modes (6 tests)
- Trace clustering (4 tests)
- Advanced search/filter (4 tests)
- Rubric management (2 tests)
- Export/reporting (2 tests)

**Effort**: 3-4 hours per developer-day  
**Verify**: `npm run test:evaluation` shows 40 tests passing

#### Days 7-8: Compliance Extensions (15 new tests)

**Create**: `e2e/tests/compliance-extended.spec.ts`

Focus areas:
- Advanced KYC (4 tests)
- Advanced AML (4 tests)
- Velocity features (4 tests)
- Reporting (2 tests)
- Full flow integration (2 tests)

**Effort**: 3-4 hours per developer-day  
**Verify**: `npm run test:compliance` shows 35 tests passing

### Checkpoint: Week 1-2

```bash
# Verify all E2E tests pass
cd e2e
npm test

# Expected output:
# ✓ settlement.spec.ts (35 tests)
# ✓ security.spec.ts (40 tests)
# ✓ evaluation.spec.ts (40 tests)
# ✓ compliance.spec.ts (35 tests)
# ━━━━━━━━━━━━━━━━━━━━━━━━━
# ✓ 150 tests passed

# Generate report
npm run show:report
```

---

## 🎯 Week 3-4: Frontend Unit & Component Tests

### Current State
```
Unit/Component Tests: 0 tests
Frontend code coverage: ~35-45%
```

### Target State
```
Unit/Component Tests: 110 tests
Frontend code coverage: 60-70%
```

### Day-by-Day Breakdown

#### Days 1-3: Setup & app.js Unit Tests (35 tests)

**Step 1: Install testing framework**
```bash
cd e2e
npm install --save-dev vitest happy-dom @vitest/ui
npm install --save-dev @testing-library/dom @testing-library/user-event
```

**Step 2: Create test file**
```bash
mkdir -p tests/unit
touch tests/unit/app.test.js
```

**Step 3: Write CSRF tests (8 tests)**
```javascript
import { describe, it, expect, beforeEach } from 'vitest';
import { extractCSRFTokenFromResponse, refreshCSRFToken } from '../../fixtures/auth.fixture';

describe('CSRF Token Management', () => {
  it('should extract CSRF token from response header', () => {
    // Test implementation
  });

  it('should handle missing CSRF token gracefully', () => {
    // Test implementation
  });

  // ... 6 more tests
});
```

**Step 4: Write API client tests (12 tests)**
```javascript
describe('API Client', () => {
  it('should make GET request', async () => {
    // Mock fetch, verify request handling
  });

  it('should add CSRF token to POST requests', async () => {
    // Verify header injection
  });

  // ... 10 more tests
});
```

**Step 5: Write session management tests (8 tests)**
```javascript
describe('Session Management', () => {
  it('should clear session on logout', () => {
    // Verify state cleanup
  });

  // ... 7 more tests
});
```

**Effort**: 4-5 hours per developer-day  
**Verify**: `npm run test:unit tests/unit/app.test.js` shows 35 tests passing

#### Days 4-6: eval_review.js Unit Tests (35 tests)

**Create**: `tests/unit/eval_review.test.js`

Focus areas:
- Trace loading (5 tests)
- Annotation workflow (8 tests)
- Submission (5 tests)
- Navigation (6 tests)
- Similarity/Clustering (5 tests)
- Keyboard shortcuts (4 tests)
- Rubrics (2 tests)

**Effort**: 4-5 hours per developer-day  
**Verify**: `npm run test:unit tests/unit/eval_review.test.js` shows 35 tests passing

#### Days 7-8: Component Tests (40 tests)

**Create component test files**:
```bash
mkdir -p tests/components
touch tests/components/LoginForm.test.ts
touch tests/components/SettlementForm.test.ts
touch tests/components/EvalTraceViewer.test.ts
touch tests/components/TraceAnnotationForm.test.ts
```

**LoginForm.test.ts** (8 tests):
- Render form fields
- Form submission
- Validation
- Error display
- Loading states
- Disabled states
- Form reset
- Success message

**SettlementForm.test.ts** (10 tests):
- Render fields
- Amount validation
- Merchant selection
- Form submission
- Success/error messages
- Auto-fill
- Currency conversion
- Netting preview
- Confirmation
- Performance

**EvalTraceViewer.test.ts** (8 tests):
- Render trace JSON
- Syntax highlighting
- Collapsible sections
- Copy to clipboard
- Full-screen mode
- Search within trace
- Metadata display
- Navigation

**TraceAnnotationForm.test.ts** (10 tests):
- Render form
- Button interactions
- Failure mode selection
- Confidence slider
- Notes input
- Form submission
- Validation
- Error display
- Success confirmation
- State management

**Effort**: 4 hours per developer-day  
**Verify**: `npm run test:components` shows 40 tests passing

### Checkpoint: Week 3-4

```bash
# Run all unit tests
npm run test:unit

# Expected:
# app.test.js: 35 tests ✓
# eval_review.test.js: 35 tests ✓
# Component tests: 40 tests ✓
# ━━━━━━━━━━━━━━━━━━━━
# ✓ 110 tests passed
# Coverage: 60-70%

# Generate coverage report
npm run test:unit:coverage
```

---

## 🎨 Week 5-6: Accessibility & Visual Tests

### Current State
```
A11y/Visual Tests: 0 tests
WCAG 2.1 compliance: Not validated
Visual regression: Not tested
```

### Target State
```
A11y/Visual Tests: 41 tests
WCAG 2.1 Level AA: Verified
Visual regression: Baseline captured
```

### Day-by-Day Breakdown

#### Days 1-2: WCAG Compliance Tests (8 tests)

**Create**: `tests/a11y/wcag-compliance.spec.ts`

```typescript
import { test, expect } from '@playwright/test';
import { injectAxe, checkA11y } from 'axe-playwright';

test.describe('WCAG 2.1 Level AA Compliance', () => {
  test('settlement page should have no a11y violations', async ({ page }) => {
    await page.goto('http://localhost:8080/settlement');
    await injectAxe(page);
    await checkA11y(page);
  });

  test('evaluation page should meet color contrast', async ({ page }) => {
    // Use axe to check contrast ratios
  });

  // ... 6 more tests
});
```

**Install axe-core**:
```bash
npm install --save-dev @axe-core/playwright axe-playwright
```

**Effort**: 2-3 hours  
**Verify**: `npm run test:a11y` shows 8 tests passing

#### Days 3-4: Keyboard Navigation Tests (6 tests)

**Create**: `tests/a11y/keyboard-navigation.spec.ts`

```typescript
test.describe('Keyboard Navigation', () => {
  test('tab order should be logical in settlement form', async ({ page }) => {
    // Tab through form, verify order
    // Check focus visible at each step
  });

  test('modal should be escapable with Escape key', async ({ page }) => {
    // Open modal, press Escape, verify closed
  });

  // ... 4 more tests
});
```

**Effort**: 2-3 hours  
**Verify**: `npm run test:a11y` shows 6 additional tests

#### Days 5-6: Screen Reader & Mobile a11y (5 tests)

**Create**: `tests/a11y/screen-reader.spec.ts`

```typescript
test.describe('Screen Reader Support', () => {
  test('form labels should be announced', async ({ page }) => {
    // Use getByRole, verify labels
  });

  test('error messages should be announced as alerts', async ({ page }) => {
    // Trigger error, verify live region
  });

  // ... 3 more tests
});
```

**Create**: `tests/a11y/mobile-a11y.spec.ts`

```typescript
test.describe('Mobile Accessibility', () => {
  test('touch targets should be 48px minimum', async ({ page }) => {
    // Check button/link sizes on mobile viewport
  });

  test('page should support pinch zoom to 200%', async ({ page }) => {
    // Test zoom functionality
  });
});
```

**Effort**: 2-3 hours  
**Verify**: `npm run test:a11y` shows 11 tests total

#### Days 7-8: Visual Regression Tests (18 tests)

**Create visual test files**:
```bash
mkdir -p tests/visual
touch tests/visual/settlement-page.spec.ts
touch tests/visual/evaluation-dashboard.spec.ts
touch tests/visual/responsive-design.spec.ts
touch tests/visual/dark-mode.spec.ts
touch tests/visual/animations.spec.ts
```

**settlement-page.spec.ts** (5 tests):
```typescript
test('settlement page initial state', async ({ page }) => {
  await page.goto('http://localhost:8080/settlement');
  await expect(page).toHaveScreenshot('settlement-initial.png');
});

test('settlement form filled state', async ({ page }) => {
  // Fill form and capture
  await expect(page).toHaveScreenshot('settlement-filled.png');
});

// ... 3 more states (success, error, loading)
```

**Effort**: 3-4 hours  
**Verify**: `npm run test:visual` shows 18 tests passing with baseline screenshots

### Checkpoint: Week 5-6

```bash
# Run all a11y/visual tests
npm run test:a11y
npm run test:visual

# Expected:
# WCAG compliance: 8 tests ✓
# Keyboard nav: 6 tests ✓
# Screen reader: 5 tests ✓
# Visual regression: 18 tests ✓
# ━━━━━━━━━━━━━━━━━━━━
# ✓ 41 tests passed
# ✓ WCAG 2.1 Level AA verified
```

---

## 🔧 Week 7-8: Backend Unit Tests

### Current State
```
Backend Coverage: 61.3%
├─ Settlement:  5.6% ← CRITICAL
├─ Database:    0.9% ← CRITICAL
├─ Merchant:    5.6% ← CRITICAL
└─ Compliance:  65%
```

### Target State
```
Backend Coverage: 85%+
├─ Settlement:  75%+ (+70)
├─ Database:    70%+ (+60)
├─ Merchant:    75%+ (+40)
└─ Compliance:  85%+ (+20)
```

### Days 1-2: Settlement Package (50 tests)

**Create**: `pkg/commerce/settlement_flow_test.go`

```go
package commerce

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCreateSettlement_HappyPath(t *testing.T) {
	// Setup database
	db := setupTestDB(t)
	defer db.Close()

	// Create order
	order := createTestOrder(db, t)

	// Create settlement
	settlement, err := CreateSettlement(db, order.ID)
	assert.NoError(t, err)
	assert.NotNil(t, settlement)
	assert.Equal(t, "PENDING", settlement.Status)
}

func TestCreateSettlement_Validation(t *testing.T) {
	// Test invalid input validation
}

// ... 18 more tests for:
// - UpdateSettlementStatus
// - GetSettlement
// - ListSettlements
// - Amount calculation
// - Netting application
// - CBDC commitment
// - State transitions
// - Error handling
// - Concurrency
```

**Run**:
```bash
cd pkg/commerce
go test -v -cover settlement_flow_test.go
# Target: 20 tests passing
```

**Create**: `pkg/commerce/reconciliation_test.go` (15 tests)

**Create**: `pkg/commerce/handler_test.go` (15 tests)

**Effort**: 5-6 hours per developer-day  
**Verify**: `go test -cover ./pkg/commerce/...` shows 75%+ coverage

### Days 3-4: Database Package (60 tests)

**Create**: `pkg/db/postgres/migrations_test.go` (10 tests)
```go
func TestMigrationsRun(t *testing.T) {
	// Verify all migrations apply
	// Check idempotency
	// Validate schema
}
```

**Create**: `pkg/db/postgres/orders_test.go` (15 tests)
**Create**: `pkg/db/postgres/settlements_test.go` (15 tests)
**Create**: `pkg/db/postgres/transactions_test.go` (12 tests)
**Create**: `pkg/db/postgres/queries_test.go` (8 tests)

**Effort**: 5-6 hours per developer-day  
**Verify**: `go test -cover ./pkg/db/postgres/...` shows 70%+ coverage

### Days 5-6: Merchant Package (40 tests)

**Create**: `pkg/merchant/handler_test.go` (15 tests)
**Create**: `pkg/merchant/validation_test.go` (12 tests)
**Create**: `pkg/merchant/kyc_check_test.go` (10 tests)
**Create**: `pkg/merchant/settlement_prep_test.go` (3 tests)

**Effort**: 4-5 hours per developer-day  
**Verify**: `go test -cover ./pkg/merchant/...` shows 75%+ coverage

### Days 7-8: Compliance Package (20 tests)

**Create**: `pkg/compliance/velocity_test.go` (10 tests)
**Create**: `pkg/compliance/aml_advanced_test.go` (10 tests)

**Effort**: 3-4 hours per developer-day  
**Verify**: `go test -cover ./pkg/compliance/...` shows 85%+ coverage

### Checkpoint: Week 7-8

```bash
# Run backend unit tests
go test -v -cover ./pkg/...

# Expected:
# Settlement: 50 tests ✓ (75%+)
# Database:   60 tests ✓ (70%+)
# Merchant:   40 tests ✓ (75%+)
# Compliance: 20 tests ✓ (85%+)
# ━━━━━━━━━━━━━━━━━━━━
# ✓ 170 tests passed
# Overall coverage: 85%+

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 🔗 Week 9-10: Integration & Judge Tests

### Days 1-4: Integration Tests (30 tests)

**Create**: `tests/integration/settlement_workflow_test.go` (10 tests)

```go
func TestEndToEnd_OrderToSettlement(t *testing.T) {
	// 1. Create order
	// 2. Initiate payment
	// 3. Confirm payment
	// 4. Settle
	// 5. Verify reconciliation
	// Verify all steps succeed
}
```

**Create**: `tests/integration/compliance_workflow_test.go` (10 tests)
**Create**: `tests/integration/cross_agent_test.go` (10 tests)

**Effort**: 4-5 hours per developer-day  
**Verify**: `go test -v ./tests/integration/...` shows 30 tests passing

### Days 5-8: Judge Validation (100 tests)

**Enhance**: `pkg/eval/judge_calibration_test.go` (25 tests)

```go
func TestSettlementJudge_TPRTNRCalculation(t *testing.T) {
	// Load test set (100 traces)
	// Run settlement judge on each
	// Calculate TPR, TNR, confidence intervals
	// Assert TPR >= 0.90, TNR >= 0.90
}
```

**Enhance**: `pkg/eval/failure_mode_coverage_test.go` (20 tests)

```go
func TestFailureMode_HasMinimumCoverage(t *testing.T) {
	// For each of 40 failure modes:
	// Assert >= 5 golden test cases
	// Assert >= 1 judge validation
	// Assert >= 1 E2E test
	// Assert documentation exists
}
```

**Effort**: 3-4 hours per developer-day  
**Verify**: `go test -v ./pkg/eval/...` shows 100+ tests passing

### Final Checkpoint: Week 9-10

```bash
# Run complete test suite
make test-coverage-100

# Expected output:
# ✅ Frontend E2E: 150 tests
# ✅ Frontend Unit: 70 tests
# ✅ Frontend Components: 40 tests
# ✅ Frontend A11y: 23 tests
# ✅ Frontend Visual: 18 tests
# ✅ Backend Unit: 170 tests
# ✅ Integration: 30 tests
# ✅ Judge/Eval: 100 tests
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ✅ TOTAL: 1,331 tests
# ✅ Coverage: 88%+
# ✅ Status: MISSION ACCOMPLISHED 🎉
```

---

## 📊 Progress Tracking

### Week-by-Week Checklist

**Week 1-2: E2E Frontend**
- [ ] Settlement extended (17 tests)
- [ ] Security extended (18 tests)
- [ ] Evaluation extended (18 tests)
- [ ] Compliance extended (15 tests)
- [ ] All 150 E2E tests passing
- [ ] E2E report generated

**Week 3-4: Unit/Components**
- [ ] Vitest setup
- [ ] app.js tests (35)
- [ ] eval_review.js tests (35)
- [ ] Component tests (40)
- [ ] 110 tests passing
- [ ] 60-70% coverage

**Week 5-6: A11y/Visual**
- [ ] WCAG tests (8)
- [ ] Keyboard nav (6)
- [ ] Screen reader (5)
- [ ] Mobile a11y (4)
- [ ] Visual tests (18)
- [ ] 41 tests passing
- [ ] WCAG 2.1 verified

**Week 7-8: Backend Unit**
- [ ] Settlement (50)
- [ ] Database (60)
- [ ] Merchant (40)
- [ ] Compliance (20)
- [ ] 170 tests passing
- [ ] 85%+ coverage

**Week 9-10: Integration/Judge**
- [ ] Integration (30)
- [ ] Judge (100)
- [ ] 130 tests passing
- [ ] Judge TPR/TNR ≥ 0.90
- [ ] Full suite: 1,331 tests
- [ ] 88%+ coverage

---

## 🎯 Success Metrics

| Metric | Week 1-2 | Week 5-6 | Week 10 |
|---|---|---|---|
| **Total Tests** | 150 | 250 | 1,331 |
| **Frontend E2E** | 150 | 150 | 150 |
| **Frontend Unit** | — | 110 | 110 |
| **Frontend A11y** | — | 41 | 41 |
| **Backend** | — | — | 170 |
| **Integration** | — | — | 30 |
| **Judge** | — | — | 100 |
| **Coverage** | 30% | 50% | 88% |

---

## 🚀 Getting Started Right Now

### **Immediate (Next 2 Hours)**

```bash
# 1. Clone the plan
cat TEST_COVERAGE_MASTER_PLAN.md

# 2. Set up workspace
cd /Users/genesis/Developer/goworkspace/src/github.com/c2siorg/genie

# 3. Create Week 1 feature branch
git checkout -b phase-100-coverage-week1

# 4. Start settlement extensions
cd e2e
touch tests/settlement-extended.spec.ts

# 5. Copy template from master plan
# Implement first 5 tests
npm test tests/settlement-extended.spec.ts
```

### **This Week's Goals**

```bash
# Day 1-2
npm test tests/settlement-extended.spec.ts
# Target: 17 tests passing

# Day 3-4
npm test tests/security-extended.spec.ts
# Target: 18 tests passing

# Day 5-6
npm test tests/evaluation-extended.spec.ts
# Target: 18 tests passing

# Day 7-8
npm test tests/compliance-extended.spec.ts
# Target: 15 tests passing

# End of week
npm test
# Target: 150 E2E tests passing
```

---

## 📚 Resources

- **Full Plan**: `TEST_COVERAGE_MASTER_PLAN.md` (200 lines)
- **E2E Docs**: `e2e/README.md`
- **E2E Summary**: `E2E_TESTS_SUMMARY.md`
- **Playwright Quickstart**: `PLAYWRIGHT_QUICKSTART.md`
- **Test Architecture**: `e2e/playwright.config.ts`

---

## ❓ FAQ

**Q: Can I parallelize this work?**  
A: Yes! Weeks 1-2 can be split: one dev on settlement, another on security. Week 7-8 can parallelize across 4 packages.

**Q: What if we skip some areas?**  
A: Focus on Week 1-2 (E2E is high ROI) and Week 7-8 (backend coverage is critical). Skip Week 5-6 if time is short.

**Q: How do I measure progress?**  
A: Use the test count as primary metric. Coverage reports secondary. Aim for ~150 tests/week.

**Q: What's the minimum viable subset?**  
A: Week 1-2 (E2E) + Week 7-8 (Backend) = 320 tests, 75% coverage. Skip Unit/A11y if needed.

---

**Status**: Ready to start  
**Estimated Completion**: 10 weeks  
**Target Coverage**: 88%+  
**Final Test Count**: 1,331 tests

### 🎯 **START HERE**: Week 1-2 Frontend E2E Extension

See `TEST_COVERAGE_MASTER_PLAN.md` for detailed implementation guides for each phase.

**Good luck! 🚀**
