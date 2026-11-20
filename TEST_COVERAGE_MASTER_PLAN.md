# 100% Test Coverage Master Plan — Genie Financial Platform

**Goal**: Comprehensive testing across all layers (Frontend E2E, Backend, UI, Evaluation)  
**Target**: 90%+ coverage for all critical paths  
**Timeline**: 8-10 weeks  
**Status**: Starting Phase 1

---

## 📊 Coverage Matrix: Current → Target

```
┌─────────────────────────────────────────────────────────┐
│                    TEST PYRAMID                         │
├─────────────────────────────────────────────────────────┤
│                      Evaluation                          │  (Judge accuracy, failure modes)
│                     ~300 tests ✅                        │
├─────────────────────────────────────────────────────────┤
│              E2E + UI Integration Tests                 │  (Playwright + Visual + a11y)
│           Current: 82 → Target: 180 tests               │
├─────────────────────────────────────────────────────────┤
│              Component + Unit Tests (JS/Go)             │  (UI components, JS logic, API)
│         Current: 0 → Target: 150 tests                  │
├─────────────────────────────────────────────────────────┤
│                   Backend Unit Tests                    │  (Settlement, merchant, DB)
│         Current: 61.3% → Target: 90%+                   │
└─────────────────────────────────────────────────────────┘

TOTALS:
├─ Frontend (E2E + UI):        82 → 250+ tests
├─ Backend (Unit + Integ):     ~500 → 700+ tests  
├─ Evaluation (Judge + evals): 300+ tests ✅
└─ TOTAL:                      ~1,250 tests
```

---

## Phase 1: Expand Frontend E2E Tests (Weeks 1-2)

### 1.1 Extend Settlement E2E (18 → 35 tests)

**Current**: 18 tests  
**Add**: 17 tests

```typescript
// tests/settlement-extended.spec.ts
├─ Happy Path Extensions (8 tests)
│  ├─ Multiple merchants settlement
│  ├─ Multi-currency settlement (if supported)
│  ├─ Partial refund workflow
│  ├─ Chargeback handling
│  ├─ Settlement reversal
│  ├─ Batch settlement processing
│  ├─ Settlement schedule/delayed
│  └─ Webhook notifications
│
├─ Edge Cases (9 tests)
│  ├─ Zero amount settlement
│  ├─ Very large amount (overflow)
│  ├─ Concurrent settlements (race condition)
│  ├─ Settlement timeout
│  ├─ Settlement with pending payment
│  ├─ Settlement rollback scenario
│  ├─ Invalid merchant ID
│  ├─ Settlement after order cancellation
│  └─ Settlement with insufficient inventory
│
└─ Performance (3 tests)
   ├─ Settlement response time < 2s
   ├─ Batch settlement (100 orders) < 10s
   └─ Settlement under load (10 concurrent)
```

**Effort**: 3-4 days

### 1.2 Extend Security E2E (22 → 40 tests)

**Current**: 22 tests  
**Add**: 18 tests

```typescript
// tests/security-extended.spec.ts
├─ CORS & Origin Validation (4 tests)
│  ├─ Cross-origin requests blocked
│  ├─ Same-origin requests allowed
│  ├─ CORS headers validation
│  └─ Preflight request handling
│
├─ SQL Injection Prevention (5 tests)
│  ├─ Order ID field
│  ├─ Merchant ID field
│  ├─ Customer name field
│  ├─ Search query field
│  └─ Filter parameters
│
├─ API Authentication (4 tests)
│  ├─ Missing Authorization header
│  ├─ Invalid token format
│  ├─ Expired token + refresh
│  └─ Token scope validation
│
├─ Rate Limiting (5 tests)
│  ├─ Login endpoint (5 attempts/min)
│  ├─ API endpoint (100 requests/min)
│  ├─ Settlement endpoint (10 requests/min)
│  └─ Rate limit headers
│  └─ Retry-After header
│
└─ Input Sanitization (3 tests)
   ├─ HTML/JS injection in notes
   ├─ Path traversal in file upload
   └─ Null byte injection
```

**Effort**: 3-4 days

### 1.3 Extend Evaluation E2E (22 → 40 tests)

**Current**: 22 tests  
**Add**: 18 tests

```typescript
// tests/evaluation-extended.spec.ts
├─ Advanced Annotation (6 tests)
│  ├─ Multi-select failure modes
│  ├─ Nested failure mode categories
│  ├─ Custom failure mode creation
│  ├─ Failure mode deprecation
│  ├─ Bulk annotation operation
│  └─ Undo/redo annotation
│
├─ Trace Clustering (4 tests)
│  ├─ Semantic similarity clustering
│  ├─ Failure mode clustering
│  ├─ Temporal clustering (by date)
│  └─ Custom cluster creation
│
├─ Advanced Search & Filter (4 tests)
│  ├─ Full-text search in trace content
│  ├─ Date range filtering
│  ├─ Agent version filtering
│  ├─ Multi-criteria search (AND/OR)
│  
├─ Rubric Management (2 tests)
│  ├─ Rubric score calculation
│  └─ Rubric weighting in final score
│
├─ Export & Reporting (2 tests)
│  ├─ Export annotations as CSV
│  └─ Generate annotation report
│
└─ Collaboration (2 tests)
   ├─ Multi-annotator view
   └─ Annotation conflict resolution
```

**Effort**: 3-4 days

### 1.4 Extend Compliance E2E (20 → 35 tests)

**Current**: 20 tests  
**Add**: 15 tests

```typescript
// tests/compliance-extended.spec.ts
├─ KYC Advanced (4 tests)
│  ├─ Multi-document KYC
│  ├─ KYC expiration & renewal
│  ├─ KYC document verification
│  └─ KYC appeal process
│
├─ AML Advanced (4 tests)
│  ├─ Dynamic risk recalculation
│  ├─ PEP list update handling
│  ├─ Sanctions list sync
│  └─ Cross-border transaction monitoring
│
├─ Velocity Advanced (4 tests)
│  ├─ Rolling window velocity
│  ├─ Velocity reset after period
│  ├─ Velocity limit per merchant
│  └─ Velocity waiver workflow
│
├─ Reporting & Compliance (2 tests)
│  ├─ Regulatory report generation
│  └─ Compliance metrics dashboard
│
└─ Integration Tests (2 tests)
   ├─ Full KYC → AML → Velocity flow
   └─ Compliance decision feed to settlement
```

**Effort**: 3-4 days

### **Phase 1 Subtotal: 82 → 150 Playwright E2E tests**

---

## Phase 2: Frontend Unit & Component Tests (Weeks 3-4)

### 2.1 JavaScript Unit Tests (70 tests)

**Setup**:
```bash
npm install --save-dev vitest happy-dom @vitest/ui
npm install --save-dev @testing-library/dom @testing-library/user-event
```

**Test Files**:

#### app.js (35 tests)
```javascript
tests/unit/app.test.js
├─ CSRF Token Management (8 tests)
│  ├─ extractCSRFTokenFromResponse
│  ├─ refreshCSRFToken success
│  ├─ refreshCSRFToken failure (401)
│  ├─ refreshCSRFToken race condition
│  ├─ clearSession
│  └─ CSRF validation
│
├─ API Client (12 tests)
│  ├─ GET request handling
│  ├─ POST with JSON body
│  ├─ PUT request
│  ├─ DELETE request
│  ├─ Request with custom headers
│  ├─ 401 Unauthorized handling
│  ├─ 403 Forbidden + retry
│  ├─ Network error handling
│  ├─ Response JSON parsing
│  ├─ Response error handling
│  └─ Request timeout
│  └─ Automatic CSRF token injection
│
├─ Session Management (8 tests)
│  ├─ Login form submission
│  ├─ Login validation
│  ├─ Login error handling
│  ├─ Logout
│  ├─ Session restore from cookie
│  ├─ Session expiration handling
│  └─ Multiple concurrent requests
│
├─ Form Validation (5 tests)
│  ├─ Email validation
│  ├─ Password validation
│  ├─ Form submission
│  ├─ Field error display
│  └─ Form reset
│
└─ State Management (2 tests)
   ├─ State initialization
   └─ State persistence
```

#### eval_review.js (35 tests)
```javascript
tests/unit/eval_review.test.js
├─ Trace Loading (5 tests)
│  ├─ Load traces from API
│  ├─ Parse trace JSON
│  ├─ Handle API error
│  ├─ Handle empty traces
│  └─ Update trace counter
│
├─ Annotation Workflow (8 tests)
│  ├─ setLabel (pass/fail/uncertain)
│  ├─ Set primary failure mode
│  ├─ Set secondary failures
│  ├─ Confidence value update
│  ├─ Notes input handling
│  ├─ Defer checkbox toggle
│  ├─ Defer reason input
│  └─ Clear annotation state
│
├─ Annotation Submission (5 tests)
│  ├─ submitAnnotation success
│  ├─ submitAnnotation API call
│  ├─ Submission validation
│  ├─ Submission error handling
│  └─ Duplicate submission prevention
│
├─ Trace Navigation (6 tests)
│  ├─ nextTrace advancement
│  ├─ previousTrace navigation
│  ├─ getCurrentTraceId
│  ├─ Trace counter update
│  ├─ Boundary handling (first/last)
│  └─ Navigation with pending annotation
│
├─ Similarity & Clustering (5 tests)
│  ├─ Load similar traces
│  ├─ Sort by similarity
│  ├─ Calculate similarity score
│  ├─ Filter by failure mode
│  └─ Cluster detection
│
├─ Keyboard Shortcuts (4 tests)
│  ├─ Shortcut N (next)
│  ├─ Shortcut P (previous)
│  ├─ Shortcut F (fail)
│  ├─ Shortcut S (submit)
│  └─ Shortcut D (defer)
│
├─ Rubric Management (2 tests)
│  ├─ Load rubrics
│  └─ Display rubric definitions
│
└─ State Management (2 tests)
   ├─ Annotator ID persistence
   └─ State reset on logout
```

**Effort**: 5-6 days

### 2.2 Component Tests (40 tests)

```typescript
tests/components/
├─ LoginForm.test.ts (8 tests)
│  ├─ Render form fields
│  ├─ Form submission
│  ├─ Email validation
│  ├─ Password validation
│  ├─ Error message display
│  ├─ Loading state
│  ├─ Disabled state after submit
│  └─ Form reset
│
├─ SettlementForm.test.ts (10 tests)
│  ├─ Render form fields
│  ├─ Amount input validation
│  ├─ Merchant selection
│  ├─ Form submission
│  ├─ Success message
│  ├─ Error handling
│  ├─ Auto-fill from order
│  ├─ Currency conversion
│  ├─ Netting preview
│  └─ Confirmation dialog
│
├─ EvalTraceViewer.test.ts (8 tests)
│  ├─ Render trace JSON
│  ├─ Syntax highlighting
│  ├─ Collapsible sections
│  ├─ Copy to clipboard
│  ├─ Full-screen view
│  ├─ Search within trace
│  ├─ Trace metadata display
│  └─ Trace navigation
│
├─ TraceAnnotationForm.test.ts (10 tests)
│  ├─ Render form
│  ├─ Pass button click
│  ├─ Fail button click
│  ├─ Failure mode selection
│  ├─ Confidence slider
│  ├─ Notes input
│  ├─ Submit button
│  ├─ Form validation
│  ├─ Error message display
│  └─ Success confirmation
│
└─ Shared Components (4 tests)
   ├─ Tabs component
   ├─ Modal dialog
   ├─ Toast notification
   └─ Loading spinner
```

**Effort**: 4-5 days

### **Phase 2 Subtotal: 70 + 40 = 110 unit/component tests**

---

## Phase 3: Accessibility & Visual Tests (Weeks 5-6)

### 3.1 Accessibility Tests (23 tests)

```typescript
tests/a11y/
├─ WCAG 2.1 Level AA Compliance (8 tests)
│  ├─ Color contrast ratios
│  ├─ Button size/touch targets (48px)
│  ├─ Form labels (all inputs)
│  ├─ Error messages linked to inputs
│  ├─ Focus indicators visible
│  ├─ Heading hierarchy valid
│  ├─ List structure valid
│  └─ Alt text for images
│
├─ Keyboard Navigation (6 tests)
│  ├─ Tab order logical
│  ├─ All buttons keyboard accessible
│  ├─ Form keyboard submittable
│  ├─ Modal keyboard escapable
│  ├─ Skip links functional
│  └─ Focus trap in modal
│
├─ Screen Reader Support (5 tests)
│  ├─ ARIA labels present
│  ├─ Form hints announced
│  ├─ Alerts announced (live regions)
│  ├─ Loading state announced
│  └─ Trace complexity explained
│
├─ Mobile Accessibility (3 tests)
│  ├─ Touch targets 48px minimum
│  ├─ Zoom to 200% functional
│  └─ Orientation change handling
│
└─ Semantic HTML (1 test)
   └─ Validate semantic HTML structure
```

**Effort**: 3-4 days

### 3.2 Visual/Screenshot Tests (18 tests)

```typescript
tests/visual/
├─ Settlement Page (5 tests)
│  ├─ Initial load state
│  ├─ Form filled state
│  ├─ Success state
│  ├─ Error state
│  └─ Loading state
│
├─ Evaluation Dashboard (5 tests)
│  ├─ Trace list view
│  ├─ Trace detail view
│  ├─ Annotation form
│  ├─ Success message
│  └─ Error state
│
├─ Responsive Design (5 tests)
│  ├─ Mobile layout (375px)
│  ├─ Tablet layout (768px)
│  ├─ Desktop layout (1440px)
│  ├─ Touch-friendly spacing
│  └─ Typography scaling
│
├─ Dark Mode (2 tests)
│  ├─ Dark mode colors
│  └─ Dark mode contrast
│
└─ Animation (1 test)
   └─ Smooth transitions
```

**Effort**: 3-4 days

### **Phase 3 Subtotal: 23 + 18 = 41 a11y/visual tests**

---

## Phase 4: Backend Unit Test Expansion (Weeks 7-8)

### 4.1 Settlement Package (0.9% → 75%)

**Current**: ~5 tests  
**Target**: 50 tests

```go
pkg/commerce/
├─ settlement_flow_test.go (20 tests)
│  ├─ CreateSettlement happy path
│  ├─ CreateSettlement validation
│  ├─ CreateSettlement duplicate detection
│  ├─ UpdateSettlementStatus state transitions
│  ├─ GetSettlement by ID
│  ├─ ListSettlements with filters
│  ├─ Settlement amount calculation
│  ├─ Netting application (bilateral)
│  ├─ Netting application (multilateral)
│  ├─ CBDC commitment logic
│  ├─ Settlement timeout handling
│  ├─ Settlement retry logic
│  ├─ Settlement rollback
│  ├─ Concurrent settlement handling
│  ├─ Settlement with invalid merchant
│  ├─ Settlement with insufficient balance
│  ├─ Settlement with pending payment
│  ├─ Settlement state machine validation
│  ├─ Settlement audit trail
│  └─ Settlement performance (< 100ms)
│
├─ reconciliation_test.go (15 tests)
│  ├─ VerifyOrderSettlement happy path
│  ├─ VerifyOrderSettlement mismatch detection
│  ├─ Verify CBDC ledger alignment
│  ├─ Verify lineage integrity
│  ├─ Reconciliation state check
│  ├─ IsReconciled validation
│  ├─ HandleReconciliationFailure
│  ├─ Reconciliation retry
│  ├─ Batch reconciliation
│  ├─ Reconciliation with missing lineage
│  ├─ Reconciliation timeout
│  ├─ Reconciliation with hash mismatch
│  ├─ Reconciliation report generation
│  ├─ Reconciliation metrics
│  └─ Reconciliation performance
│
└─ handler_test.go (15 tests)
   ├─ POST /v1/commerce/orders
   ├─ GET /v1/commerce/orders/{id}
   ├─ POST /v1/payment/initiate
   ├─ POST /v1/payment/confirm
   ├─ GET /v1/settlement/status/{id}
   ├─ POST /v1/settlement/batch
   ├─ POST /v1/settlement/reconcile
   ├─ Handler error responses
   ├─ Handler validation
   ├─ Handler authorization
   ├─ Handler rate limiting
   ├─ Handler logging
   ├─ Handler metrics
   ├─ Handler concurrent requests
   └─ Handler performance
```

**Effort**: 5-6 days

### 4.2 Database Layer (0.9% → 70%)

**Current**: ~5 tests  
**Target**: 60 tests

```go
pkg/db/postgres/
├─ migrations_test.go (10 tests)
│  ├─ Run all migrations
│  ├─ Idempotency
│  ├─ Schema validation
│  ├─ Foreign key constraints
│  ├─ Index creation
│  ├─ Rollback capability
│  ├─ Data type validation
│  ├─ Default values
│  ├─ Constraints
│  └─ Migration order
│
├─ orders_test.go (15 tests)
│  ├─ CreateOrder
│  ├─ GetOrder by ID
│  ├─ ListOrders with filters
│  ├─ UpdateOrder status
│  ├─ DeleteOrder (soft delete)
│  ├─ Order amount calculation
│  ├─ Order validation
│  ├─ Concurrent order creation
│  ├─ Order query performance
│  ├─ Order with items relationship
│  ├─ Order merchant relationship
│  ├─ Order customer relationship
│  ├─ Order lineage relationship
│  ├─ Order pagination
│  └─ Order sorting
│
├─ settlements_test.go (15 tests)
│  ├─ CreateSettlement
│  ├─ GetSettlement
│  ├─ ListSettlements
│  ├─ UpdateSettlement status
│  ├─ Settlement amount
│  ├─ Settlement with netting
│  ├─ Settlement CBDC commitment
│  ├─ Settlement reconciliation flag
│  ├─ Settlement batch operation
│  ├─ Settlement query performance
│  ├─ Settlement relationship integrity
│  ├─ Settlement date filtering
│  ├─ Settlement state validation
│  ├─ Settlement merchant aggregation
│  └─ Settlement concurrent updates
│
├─ transactions_test.go (12 tests)
│  ├─ BeginTx
│  ├─ CommitTx
│  ├─ RollbackTx
│  ├─ Nested transactions
│  ├─ Transaction isolation
│  ├─ Transaction timeout
│  ├─ Transaction deadlock handling
│  ├─ Transaction concurrent access
│  ├─ Transaction error handling
│  ├─ Transaction logging
│  ├─ Transaction metrics
│  └─ Transaction performance
│
└─ queries_test.go (8 tests)
   ├─ Query execution
   ├─ Query error handling
   ├─ Query timeout
   ├─ Query result mapping
   ├─ Query NULL handling
   ├─ Query date handling
   ├─ Query JSON columns
   └─ Query performance
```

**Effort**: 5-6 days

### 4.3 Merchant Package (5.6% → 75%)

**Current**: ~10 tests  
**Target**: 40 tests

```go
pkg/merchant/
├─ handler_test.go (15 tests)
│  ├─ POST /v1/merchant/onboard
│  ├─ GET /v1/merchant/{id}
│  ├─ PUT /v1/merchant/{id}
│  ├─ GET /v1/merchant/{id}/settlements
│  ├─ Handler validation
│  ├─ Handler authorization
│  ├─ Handler error responses
│  ├─ Handler rate limiting
│  ├─ Handler concurrent requests
│  ├─ Handler performance
│  ├─ Handler logging
│  ├─ Handler metrics
│  ├─ Handler caching
│  ├─ Handler pagination
│  └─ Handler filtering
│
├─ validation_test.go (12 tests)
│  ├─ MerchantID validation
│  ├─ MerchantName validation
│  ├─ MerchantEmail validation
│  ├─ BankAccount validation
│  ├─ SettlementAccount validation
│  ├─ IFSC code validation
│  ├─ GSTIN validation
│  ├─ PAN validation
│  ├─ Business type validation
│  ├─ URL validation
│  ├─ Phone number validation
│  └─ Document upload validation
│
├─ kyc_check_test.go (10 tests)
│  ├─ ValidateMerchantKYC happy path
│  ├─ KYC rejection (invalid doc)
│  ├─ KYC escalation (manual review)
│  ├─ KYC document verification
│  ├─ KYC expiration check
│  ├─ KYC renewal
│  ├─ KYC concurrent checks
│  ├─ KYC caching
│  ├─ KYC error handling
│  └─ KYC performance
│
└─ settlement_prep_test.go (3 tests)
   ├─ PrepareSettlement
   ├─ SettlementAccount validation
   └─ Settlement amount calculation
```

**Effort**: 4-5 days

### 4.4 Compliance Package (65% → 85%)

**Current**: ~40 tests  
**Target**: 60 tests (add 20)

```go
pkg/compliance/
├─ velocity_test.go (10 tests)
   ├─ Check velocity happy path
   ├─ Velocity exceeded
   ├─ Velocity reset after period
   ├─ Velocity per merchant
   ├─ Velocity concurrent updates
   ├─ Velocity cache
   ├─ Velocity persistence
   ├─ Velocity calculation
   ├─ Velocity audit log
   └─ Velocity performance

├─ aml_test.go (10 tests)
   ├─ Score calculation
   ├─ High-risk detection
   ├─ PEP list matching
   ├─ Sanctions list matching
   ├─ Risk flag determination
   ├─ EDD requirement
   ├─ AML concurrent checks
   ├─ AML caching
   ├─ AML error handling
   └─ AML performance
```

**Effort**: 3-4 days

### **Phase 4 Subtotal: 50 + 60 + 40 + 20 = 170 backend tests**

---

## Phase 5: Backend Integration Tests (Weeks 9-10)

### 5.1 End-to-End Workflow Tests (30 tests)

```go
tests/integration/
├─ settlement_workflow_test.go (10 tests)
│  ├─ Order → Payment → Settlement → Reconciliation
│  ├─ Multi-merchant settlement with netting
│  ├─ Settlement with compliance checks
│  ├─ Settlement with KYC validation
│  ├─ Settlement with AML checks
│  ├─ Settlement with velocity limits
│  ├─ Settlement rollback scenario
│  ├─ Settlement with database failures
│  ├─ Settlement with API failures
│  └─ Settlement performance under load
│
├─ compliance_workflow_test.go (10 tests)
│  ├─ KYC → AML → Settlement flow
│  ├─ High-risk customer escalation
│  ├─ Velocity limit enforcement
│  ├─ Sanctions list match
│  ├─ PEP detection
│  ├─ Compliance override workflow
│  ├─ Compliance audit trail
│  ├─ Compliance concurrent checks
│  ├─ Compliance state consistency
│  └─ Compliance decision persistence
│
└─ cross_agent_test.go (10 tests)
   ├─ Payment agent ↔ Settlement agent
   ├─ Settlement agent ↔ Compliance agent
   ├─ Settlement agent ↔ Lineage agent
   ├─ Compliance agent ↔ KYC agent
   ├─ Multi-agent state consistency
   ├─ Multi-agent error handling
   ├─ Multi-agent concurrent operations
   ├─ Multi-agent failure recovery
   ├─ Multi-agent event ordering
   └─ Multi-agent performance
```

**Effort**: 4-5 days

### 5.2 Judge Validation (100 tests)

**Enhancement to existing judge tests**:

```go
pkg/eval/
├─ judge_calibration_test.go (25 tests)
│  ├─ Settlement judge TPR/TNR
│  ├─ Compliance judge TPR/TNR
│  ├─ Orchestration judge TPR/TNR
│  ├─ Lineage judge TPR/TNR
│  ├─ Judge confidence intervals
│  ├─ Judge failure cases
│  ├─ Judge false positive rate
│  ├─ Judge false negative rate
│  ├─ Judge edge case handling
│  ├─ Judge concurrent evaluation
│  ├─ Judge caching
│  ├─ Judge performance
│  ├─ Judge timeout handling
│  ├─ Judge error recovery
│  ├─ Judge accuracy on new data
│  ├─ Judge drift detection
│  ├─ Judge improvement tracking
│  ├─ Judge comparison (judge A vs B)
│  ├─ Judge voting mechanism
│  ├─ Judge weighted scoring
│  ├─ Judge threshold tuning
│  ├─ Judge bootstrap intervals
│  ├─ Judge ablation testing
│  ├─ Judge hallucination detection
│  └─ Judge faithfulness checking
│
└─ failure_mode_coverage_test.go (20 tests)
   └─ Every failure mode has:
      ├─ ≥5 golden test cases
      ├─ ≥1 judge validation test
      ├─ ≥1 E2E coverage test
      ├─ ≥1 scenario test
      └─ Documentation & remediation path
```

**Effort**: 3-4 days

### **Phase 5 Subtotal: 30 + 100 = 130 integration/judge tests**

---

## 📊 Final Coverage Matrix

```
LAYER                   CURRENT      TARGET       EFFORT
────────────────────────────────────────────────────────
Frontend E2E            82 tests     150 tests    2 weeks
Frontend Components     0 tests      110 tests    2 weeks
Frontend A11y+Visual    0 tests      41 tests     2 weeks
────────────────────────────────────────────────────────
Backend Unit            ~500         700 tests    2 weeks
Backend Integration     0 tests      30 tests     1 week
────────────────────────────────────────────────────────
Judge/Evaluation        199 tests    300 tests    1 week
────────────────────────────────────────────────────────
TOTAL                   ~781 tests   ~1,331 tests 10 weeks

COVERAGE IMPROVEMENT:
┌──────────────┬─────────┬────────┐
│ Area         │ Current │ Target │
├──────────────┼─────────┼────────┤
│ Frontend E2E │   30%   │  95%   │
│ Frontend UI  │    0%   │  80%   │
│ Backend      │  61.3%  │  90%   │
│ Evaluation   │   65%   │ 100%   │
│ Overall      │  ~40%   │  88%   │
└──────────────┴─────────┴────────┘
```

---

## 🎯 Execution Strategy

### **Week 1-2: Frontend E2E Extension**
```bash
# Day 1-2: Settlement extensions (17 tests)
cd e2e && npm test tests/settlement-extended.spec.ts

# Day 3-4: Security extensions (18 tests)
npm test tests/security-extended.spec.ts

# Day 5-6: Evaluation extensions (18 tests)
npm test tests/evaluation-extended.spec.ts

# Day 7-8: Compliance extensions (15 tests)
npm test tests/compliance-extended.spec.ts

Checkpoint: 150 E2E tests passing
```

### **Week 3-4: Frontend Unit & Components**
```bash
# Day 1-3: Unit tests setup
npm install --save-dev vitest happy-dom
npm run test:unit

# Day 4-6: Component tests
npm run test:components

# Day 7-8: Run full suite
npm run test:unit
npm run test:components
npm run test:coverage

Checkpoint: 110 unit/component tests passing
```

### **Week 5-6: Accessibility & Visual**
```bash
# Day 1-3: A11y tests
npm test tests/a11y/

# Day 4-6: Visual tests
npm test tests/visual/

# Day 7-8: Accessibility audit
npm run a11y:audit

Checkpoint: 41 a11y/visual tests passing
```

### **Week 7-8: Backend Unit Tests**
```bash
# Day 1-2: Settlement expansion
cd .. && go test -v ./pkg/commerce/... -cover

# Day 3-4: Database tests
go test -v ./pkg/db/postgres/... -cover

# Day 5-6: Merchant tests
go test -v ./pkg/merchant/... -cover

# Day 7-8: Compliance tests
go test -v ./pkg/compliance/... -cover

Checkpoint: Backend coverage 61% → 85%
```

### **Week 9-10: Integration & Judge Validation**
```bash
# Day 1-4: Integration tests
go test -v ./tests/integration/... -cover

# Day 5-8: Judge validation
go test -v ./pkg/eval/... -run Judge

Checkpoint: All integration tests passing, judge TPR/TNR ≥ 0.90
```

---

## 🔄 CI/CD Integration

### **Makefile Targets (Add)**

```makefile
# Frontend testing
.PHONY: test-frontend
test-frontend:
	cd e2e && npm test

.PHONY: test-frontend-unit
test-frontend-unit:
	cd e2e && npm run test:unit

.PHONY: test-frontend-components
test-frontend-components:
	cd e2e && npm run test:components

.PHONY: test-frontend-a11y
test-frontend-a11y:
	cd e2e && npm test tests/a11y/

.PHONY: test-frontend-visual
test-frontend-visual:
	cd e2e && npm test tests/visual/

# Backend testing
.PHONY: test-backend-settlement
test-backend-settlement:
	$(GO) test -v -cover ./pkg/commerce/...

.PHONY: test-backend-integration
test-backend-integration:
	$(GO) test -v -cover ./tests/integration/...

# Full coverage
.PHONY: test-coverage-100
test-coverage-100: test-frontend test-backend-settlement test-backend-integration ci-eval
	@echo "✅ Coverage Report:"
	@echo "Frontend E2E: $(shell cd e2e && npm test 2>&1 | grep -o '[0-9]* passed')"
	@echo "Backend: $(shell $(GO) test -cover ./... 2>&1 | tail -1)"
	@echo "Judge validation: $(shell $(GO) test -run Judge ./pkg/eval -v 2>&1 | grep -o '[0-9]* passed')"
```

### **GitHub Actions**

```yaml
name: 100% Coverage Gate

on: [push, pull_request]

jobs:
  frontend-e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      - run: cd e2e && npm install && npm test
      - uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: playwright-report
          path: e2e/playwright-report/

  frontend-unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
      - run: cd e2e && npm install && npm run test:unit:coverage
      - uses: codecov/codecov-action@v3
        with:
          files: ./e2e/coverage/coverage-final.json
          flags: frontend

  backend-unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      - run: go test -cover -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
          flags: backend

  judge-validation:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test -v -run Judge ./pkg/eval/...

  coverage-report:
    runs-on: ubuntu-latest
    needs: [frontend-e2e, frontend-unit, backend-unit, judge-validation]
    steps:
      - name: Report Coverage
        run: |
          echo "✅ Frontend E2E: 150+ tests"
          echo "✅ Frontend Unit: 110+ tests"
          echo "✅ Backend Unit: 170+ tests"
          echo "✅ Integration: 30+ tests"
          echo "✅ Judge: 100+ tests"
          echo "━━━━━━━━━━━━━━━━━━━━"
          echo "✅ TOTAL: 1,331 tests"
          echo "✅ Overall Coverage: 88%+"
```

---

## 📋 Checklist

### **Phase 1: Frontend E2E** (Week 1-2)
- [ ] Settlement extended tests (17)
- [ ] Security extended tests (18)
- [ ] Evaluation extended tests (18)
- [ ] Compliance extended tests (15)
- [ ] All 150 E2E tests passing
- [ ] E2E report generated

### **Phase 2: Frontend Unit/Components** (Week 3-4)
- [ ] Vitest setup
- [ ] app.js unit tests (35)
- [ ] eval_review.js unit tests (35)
- [ ] Component tests (40)
- [ ] All 110 tests passing
- [ ] Coverage report 60%+

### **Phase 3: Accessibility & Visual** (Week 5-6)
- [ ] A11y tests (23)
- [ ] Visual tests (18)
- [ ] WCAG 2.1 compliance verified
- [ ] All 41 tests passing
- [ ] Accessibility audit report

### **Phase 4: Backend Unit** (Week 7-8)
- [ ] Settlement package tests (50)
- [ ] Database tests (60)
- [ ] Merchant tests (40)
- [ ] Compliance tests (20)
- [ ] Settlement coverage 5.6% → 75%
- [ ] Database coverage 0.9% → 70%

### **Phase 5: Integration & Judge** (Week 9-10)
- [ ] Integration tests (30)
- [ ] Judge calibration (25)
- [ ] Failure mode coverage (20)
- [ ] Judge TPR/TNR ≥ 0.90
- [ ] All 1,331 tests passing
- [ ] Final coverage report 88%+

---

## 🎁 Deliverables

### **Code**
- ✅ 68 new Playwright test files (150 tests)
- ✅ 2 Vitest unit test files (70 tests)
- ✅ 5 Component test files (40 tests)
- ✅ 4 A11y test files (23 tests)
- ✅ 4 Visual test files (18 tests)
- ✅ 15 Go test files (170 tests)
- ✅ 3 Integration test files (30 tests)
- ✅ Enhanced judge tests (100 tests)

### **Documentation**
- ✅ Testing guide (100+ pages)
- ✅ Test coverage matrix
- ✅ Accessibility audit report
- ✅ Performance benchmarks
- ✅ Judge calibration report
- ✅ CI/CD configuration examples

### **Metrics**
- ✅ 1,331 total tests
- ✅ 88%+ overall coverage
- ✅ 95% frontend E2E coverage
- ✅ 90% backend coverage
- ✅ 100% failure mode coverage
- ✅ Judge TPR/TNR ≥ 0.90

---

## 💰 Resource Estimate

```
Role                    Weeks   Effort
──────────────────────────────────────
Full-stack engineer     10      100%
QA/Test engineer        10      80%
DevOps (CI/CD)          2       50%
──────────────────────────────────────
Total effort: ~18 weeks equivalent
Calendar time: 10 weeks (2 people)
```

---

## 🚀 Success Criteria

✅ **Phase 1**: 150 E2E tests, all passing  
✅ **Phase 2**: 110 unit/component tests, 60%+ coverage  
✅ **Phase 3**: 41 a11y/visual tests, WCAG 2.1 compliant  
✅ **Phase 4**: 170 backend tests, 85%+ coverage  
✅ **Phase 5**: 30 integration + 100 judge tests, 88%+ overall  

**Final Goal**: 1,331 tests, 88%+ coverage, all critical paths covered

---

**Status**: Plan ready, awaiting approval  
**Start Date**: Next sprint  
**Target Completion**: 10 weeks  
**Coverage Target**: 88%+ (from current ~40%)
