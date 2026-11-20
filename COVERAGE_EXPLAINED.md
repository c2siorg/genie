# Test Coverage Explained

**What is it?** Measurement of how much of your code is tested  
**Why it matters?** Identifies untested code paths → potential bugs  
**Goal**: 80-90% coverage for production code

---

## 📊 Coverage Types

### 1. **Line Coverage** (Most Common)
```
Definition: % of code lines executed by tests

Example:
    function settleOrder(order) {
      if (order.amount > 0) {           // Line 2 - COVERED
        return processSettlement(order); // Line 3 - COVERED
      }
      return null;                       // Line 5 - NOT COVERED
    }

    Coverage = 2/3 lines = 66.7%
```

**How it works**:
- Run tests with coverage tracking
- Mark each line as executed or not
- Count: (lines executed) / (total lines)

### 2. **Branch Coverage** (More Accurate)
```
Definition: % of code decision branches taken by tests

Example:
    function calculateFee(amount, isMerchant) {
      if (isMerchant) {                    // Branch 1
        return amount * 0.01;              // Path A - COVERED
      } else {                             // Branch 2
        return amount * 0.05;              // Path B - NOT COVERED
      }
    }

    Coverage:
    - Branch 1 (isMerchant=true): COVERED
    - Branch 2 (isMerchant=false): NOT COVERED
    - Total: 50% branch coverage
```

**Why it matters**: You can execute a line without testing all its conditions

### 3. **Function Coverage**
```
Definition: % of functions that are called by tests

Example:
    function loginUser() { ... }       // TESTED
    function settleOrder() { ... }     // TESTED
    function archiveData() { ... }     // NOT TESTED
    
    Coverage: 2/3 = 66.7%
```

### 4. **Path Coverage** (Most Thorough)
```
Definition: % of all possible execution paths covered

Example:
    function processPayment(amount, method, retry) {
      if (amount > 0) {              // Path 1: True
        if (method === 'card') {     // Path 2: True
          if (retry) {               // Path 3: True
            return attemptRetry();    // Path A: T-T-T
          } else {
            return processCard();     // Path B: T-T-F
          }
        } else {                      // Path 2: False
          return processBank();       // Path C: T-F-X
        }
      } else {                        // Path 1: False
        return reject();              // Path D: F-X-X
      }
    }
    
    Total paths: 4 (A, B, C, D)
    If only A & B tested: 50% coverage
```

---

## 🎯 Coverage Metrics

### **Common Coverage Thresholds**

```
Coverage %   Status              Action
─────────────────────────────────────────
0-20%        🔴 Critical         MUST FIX
20-40%       🟠 Very Poor        Add tests urgently
40-60%       🟡 Poor             Improve significantly
60-75%       🟢 Good             Acceptable for MVP
75-85%       🟢 Very Good        Production-ready
85-95%       🟢 Excellent        High confidence
95%+         🟣 Comprehensive    All paths covered
```

### **Industry Standards**

```
Context                 Target Coverage
─────────────────────────────────────────
Hobby project           40%+
Startup MVP             60%+
Production SaaS         75-85%
Financial/Healthcare    90%+
Critical systems        95%+
Mission-critical code   99%+
```

---

## 📈 Genie's Coverage Status

### **Current State** (As of June 2026)

```
Layer                  Current    Target    Gap
─────────────────────────────────────────────
Overall Code           61.3%      90%+      28.7%
├─ Backend Unit        61.3%      90%       28.7%
├─ Frontend E2E        ~30%       95%       65%
├─ Frontend Unit       0%         70%       70%
├─ Accessibility       ~5%        80%       75%
└─ Integration         ~10%       85%       75%

Critical Gaps:
├─ Settlement (5.6%)   ← CRITICAL
├─ Database (0.9%)     ← CRITICAL
├─ Merchant (5.6%)     ← CRITICAL
└─ Frontend JS (0%)    ← NEEDS WORK
```

### **Coverage Breakdown by Package**

```go
pkg/
├─ eval/singleturn/        85%+ ✅ (well tested)
├─ governance/rbac/        82%+ ✅ (well tested)
├─ safety/policy/          80%+ ✅ (well tested)
├─ compliance/aml/         65%  🟡 (medium)
├─ compliance/kyc/         68%  🟡 (medium)
├─ eval/multiturn/         71%  🟡 (medium)
├─ cbdc/ledger/            45%  🟠 (low)
├─ lineage/audit/          35%  🟠 (very low)
├─ erupeepayment/          15%  🟠 (very low)
├─ web/handlers/           12%  🔴 (critical)
├─ merchant/onboarding/    5.6% 🔴 (critical)
├─ commerce/settlement/    5.6% 🔴 (critical)
└─ db/postgres/            0.9% 🔴 (critical)
```

---

## 🔧 How Coverage is Measured

### **Go (Backend)**

#### Running Coverage
```bash
# Generate coverage report
go test -cover ./...
# Output:
# ok  	genie/pkg/eval/singleturn	85% coverage
# ok  	genie/pkg/commerce	5.6% coverage

# More detailed
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
# Opens HTML report in browser
```

#### Coverage Report Shows
```
GREEN   = Covered (tested)
RED     = Not covered (not tested)
GRAY    = Excluded from coverage

File view:
  func CreateOrder() {
    ✅ if condition true     // GREEN - tested
    ❌ if condition false    // RED - not tested
    ✅ return statement      // GREEN - tested
  }
```

### **JavaScript (Frontend)**

#### Running Coverage
```bash
# Vitest (what we'll use)
npm run test:unit:coverage

# Output:
# ✓ app.js               65%
# ✓ eval_review.js       45%
# ✓ Overall              58%
```

#### Coverage Metrics
```javascript
Statements:   X% of all JS statements executed
Branches:     X% of all if/else branches taken
Functions:    X% of all functions called
Lines:        X% of all lines executed
```

### **E2E Tests (Playwright)**

E2E tests don't generate code coverage directly. Instead, they verify:
- ✅ User workflows work end-to-end
- ✅ Features integrate correctly
- ✅ Business logic flows properly

**E2E coverage is about workflow coverage, not code coverage**

```
Example:
✓ Settlement workflow tested  = User can settle
✓ Security workflow tested    = CSRF protection works
✓ Evaluation workflow tested  = Trace annotation works
```

---

## 💡 Types of Coverage Examples

### **Example 1: Settlement Amount Calculation**

```go
func CalculateSettlementAmount(order Order, fee float64) float64 {
  if order.Amount <= 0 {                    // Branch A
    return 0                                // Statement 1
  }
  if fee < 0 || fee > 0.1 {                 // Branch B
    return 0                                // Statement 2
  }
  return order.Amount - (order.Amount * fee) // Statement 3
}

Test 1: Normal amount
✓ Statement 1: Not executed
✓ Statement 2: Not executed
✓ Statement 3: EXECUTED
✗ Branch A: Not tested
✗ Branch B: Not tested
Coverage: 33% (1/3 statements)

Test 2: Zero amount
✓ Statement 1: EXECUTED
✓ Statement 3: Not executed
✓ Branch A: TESTED
Coverage: 50% (2/3 statements)

Test 3: Invalid fee
✓ Statement 2: EXECUTED
✓ Branch B: TESTED
Coverage: 100% (all branches)
```

### **Example 2: KYC Decision Logic**

```go
func ApproveKYC(customer Customer) (bool, string) {
  if customer.Age < 18 {                           // Branch 1
    return false, "Minor"                          // Path A
  }
  if !validateIdentity(customer.Document) {        // Branch 2
    return false, "Invalid ID"                     // Path B
  }
  if customer.Country == "SANCTIONED" {            // Branch 3
    return false, "Sanctioned country"             // Path C
  }
  return true, "Approved"                          // Path D
}

Coverage Analysis:
Paths possible: 4 (A, B, C, D)

Without tests: 0% coverage

With 1 test (happy path D): 25% coverage
With 2 tests (D + A): 50% coverage
With 3 tests (D + A + B): 75% coverage
With 4 tests (D + A + B + C): 100% coverage

Real-world: Most projects aim for 3+ tests = 75%+
```

---

## 🎯 What Should Be Tested?

### **HIGH Priority (Must Test)**
```
✅ Settlement workflow        - Financial correctness critical
✅ Payment processing         - Money movement critical
✅ KYC/AML logic             - Compliance critical
✅ Database operations        - Data integrity critical
✅ API endpoints             - External interface critical
✅ Security checks           - CSRF, XSS, auth critical
✅ Error handling            - Exceptions matter
```

### **MEDIUM Priority (Should Test)**
```
✅ Business logic            - Order creation, status updates
✅ Calculations             - Fees, netting, amounts
✅ State transitions        - Order states, settlement flow
✅ Validation rules         - Input validation, constraints
✅ Logging & auditing       - Compliance requirements
```

### **LOW Priority (Nice to Test)**
```
⚠️  Formatting logic         - JSON output formatting
⚠️  Display/UI layout        - HTML structure (E2E better)
⚠️  Trivial helpers          - Simple utility functions
⚠️  Error messages           - Text content (brittle tests)
```

---

## 📊 Coverage vs. Test Quality

### **Important**: High coverage ≠ Good tests

```
Bad Test (High Coverage, Poor Quality):
✗ Tests only happy path
✗ Doesn't verify results
✗ Brittle (breaks on refactors)
✗ No edge cases

    func TestSettlement(t *testing.T) {
      settleOrder(testOrder)  // Coverage +1, but what's verified?
      // No assertions!
    }
    Coverage: 100% but quality: 0%

Good Test (Lower Coverage, Better Quality):
✓ Tests multiple scenarios
✓ Verifies results with assertions
✓ Checks edge cases
✓ Resilient to refactors

    func TestSettlement_HappyPath(t *testing.T) {
      result := settleOrder(testOrder)
      assert.Equal(t, 1000, result.Amount)
      assert.Equal(t, "SETTLED", result.Status)
    }

    func TestSettlement_ZeroAmount(t *testing.T) {
      result := settleOrder(zeroOrder)
      assert.Equal(t, 0, result.Amount)
      assert.Error(t, result.Err)
    }

    Coverage: 80% but quality: 100% ✅
```

---

## 📋 Coverage Goals by Layer

### **Backend (Go)**

```
Current: 61.3%
Target:  90%

Package           Current  Target  Why
────────────────────────────────
settlement        5.6%     75%    CRITICAL - financial
database          0.9%     70%    CRITICAL - data layer
merchant          5.6%     75%    CRITICAL - business logic
compliance/aml    65%      85%    Important - regulatory
compliance/kyc    68%      85%    Important - regulatory
payment           15%      80%    Important - money flow
```

### **Frontend E2E (Playwright)**

```
Current: 30% (workflows)
Target:  95% (all workflows)

Workflow           Current  Target  Tests
────────────────────────────────────
settlement         50%      95%     18→35
security           70%      95%     22→40
evaluation         60%      95%     22→40
compliance         50%      95%     20→35

These are workflow coverage, not code coverage
```

### **Frontend Unit (JavaScript)**

```
Current: 0%
Target:  70%

File              Current  Target
────────────────────────
app.js            0%       70%
eval_review.js    0%       70%

Tests needed: 70 unit tests
```

### **Accessibility**

```
Current: ~5%
Target:  80%

Aspect            Current  Target
────────────────────────────
WCAG 2.1 AA       30%      95%
Keyboard nav      20%      90%
Screen reader     10%      85%
Mobile a11y       5%       80%
```

---

## 🚀 Coverage Improvement Strategy

### **Step 1: Find Gaps**
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Find red lines (not covered)
# Prioritize by:
# 1. Critical packages (settlement, database)
# 2. High-impact code paths
# 3. Error handling
```

### **Step 2: Write Tests**
```go
// For each uncovered line, write a test

func TestSettlement_AmountZero(t *testing.T) {
  result := CalculateSettlementAmount(Order{Amount: 0}, 0.01)
  assert.Equal(t, 0, result)  // Covers the uncovered line
}
```

### **Step 3: Re-measure**
```bash
go test -cover ./...
# Watch coverage % increase
```

---

## 📈 Coverage Trends

### **How Coverage Grows**

```
Week 1: Write tests for critical paths
   Coverage: 40% → 50%

Week 2: Add edge case tests
   Coverage: 50% → 60%

Week 3: Add error handling tests
   Coverage: 60% → 70%

Week 4-8: Fill gaps systematically
   Coverage: 70% → 85%

Week 9-10: Polish, refactor, final tests
   Coverage: 85% → 90%+
```

### **Coverage Distribution**

```
Typical project:
20% of code        = 80% of business value
80% of code        = 20% of business value

Focus testing on the 20% first!

Priority order:
1. Settlement logic      (financial correctness)
2. Compliance logic      (regulatory correctness)
3. Payment logic         (money movement)
4. Database operations   (data integrity)
5. API endpoints         (external interface)
6. Error handling        (robustness)
7. Everything else       (completeness)
```

---

## ⚖️ Coverage vs. Other Metrics

### **Code Coverage** (What we measure)
- "What % of code is executed?"
- Measured in line/branch %
- Range: 0-100%

### **Test Coverage** (What we create)
- "Are all features tested?"
- Measured in number of tests
- Examples: 82 E2E tests, 150 unit tests

### **Feature Coverage** (What matters)
- "Do all features work?"
- Measured in workflows passing
- Examples: Settlement works, Security works

### **Risk Coverage** (What we want)
- "Are risky areas thoroughly tested?"
- Focus on high-impact code
- Examples: Payment logic, compliance logic

---

## 🎓 Coverage Best Practices

### **DO ✅**
```
✅ Test the happy path first
✅ Test error scenarios
✅ Test edge cases (0, null, empty)
✅ Test boundary conditions
✅ Test integration points
✅ Verify actual results with assertions
✅ Write meaningful test names
✅ Keep tests independent
✅ Use mocks for external dependencies
✅ Refactor tests like production code
```

### **DON'T ❌**
```
❌ Test implementation details
❌ Test trivial getters/setters
❌ Write brittle tests
❌ Test just for coverage numbers
❌ Ignore failing tests
❌ Write tests without assertions
❌ Test private methods directly
❌ Make tests interdependent
❌ Write slow tests
❌ Comment out tests
```

---

## 📊 Coverage Report Interpretation

### **Example HTML Report**

```
Package: genie/pkg/commerce
Status: MEDIUM COVERAGE (62%)

File                 Statements  Branches  Functions  Lines
──────────────────────────────────────────────────────────
settlement.go        70%        45%       80%        70%
reconciliation.go    40%        20%       50%        40%
handler.go          50%        30%       60%        50%

RED (not covered)    20 statements
YELLOW (partial)     15 branches
GREEN (covered)      65 statements

Top uncovered lines:
- Line 145: reconciliation.go - error handling
- Line 203: settlement.go - edge case
- Line 89: handler.go - validation
```

### **How to Read**
1. **Statements**: Individual code statements
2. **Branches**: if/else, switch cases
3. **Functions**: Function calls
4. **Lines**: Physical lines of code

Focus on RED and YELLOW areas first.

---

## 🎯 Genie's Coverage Target

### **Our Goal: 88%+ Overall Coverage**

```
Frontend E2E        150 tests     95% workflow coverage
Frontend Unit       110 tests     70% code coverage
Frontend A11y       41 tests      85% WCAG coverage
Backend Unit        170 tests     90% code coverage
Integration         30 tests      85% flow coverage
Judge/Eval          100 tests     100% failure mode coverage
─────────────────────────────────────────────────────
TOTAL               1,331 tests   88%+ coverage

By package:
├─ Settlement       75%+ (from 5.6%)
├─ Database         70%+ (from 0.9%)
├─ Merchant         75%+ (from 5.6%)
├─ Compliance       85%+ (from 65%)
└─ Frontend         70%+ (from 0%)
```

---

## 📚 Key Takeaways

| Concept | Meaning |
|---------|---------|
| **Line Coverage** | % of code lines executed |
| **Branch Coverage** | % of if/else decisions tested |
| **Function Coverage** | % of functions called |
| **Path Coverage** | % of execution paths exercised |
| **Good Coverage** | 75-85% for production |
| **Excellent Coverage** | 85-95% for critical systems |
| **Test Quality** | Matters more than coverage % |
| **Coverage Gap** | Untested code = potential bugs |

---

## 🚀 Next Steps

1. **Understand Coverage**: Read this document ✓
2. **Check Current**: `make cover` (generates HTML report)
3. **View Report**: Open `.coverage/coverage.html` in browser
4. **Identify Gaps**: Look for RED (uncovered) areas
5. **Prioritize**: Focus on critical packages (settlement, database)
6. **Write Tests**: Use master plan to add 1,331 tests
7. **Measure**: Re-run `make cover` weekly to track progress

---

**TL;DR**: Coverage measures what % of your code is tested. Good coverage = confidence bugs won't slip through. We're going from 61% → 88%+ by adding 1,331 tests over 10 weeks. 🎯

