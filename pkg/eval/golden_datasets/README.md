# Golden Test Datasets for Genie Evaluation

This directory contains comprehensive golden test case datasets for evaluating the Genie multi-agent financial platform across four critical domains. The datasets are organized by failure modes, rubrics, and evaluation results.

## Dataset Overview

### Total Test Cases: 300+

| Domain | Correct | Hallucination | Netting | Edge/Regression | KYC Approved | KYC Rejected | AML | Edge | Valid | Invalid | Complete | Broken | Tamper | **Total** |
|--------|---------|---------------|---------|-----------------|--------------|--------------|-----|------|-------|---------|----------|--------|--------|---------|
| Settlement | 10 | 10 | 10 | 20 | - | - | - | - | - | - | - | - | - | **50** |
| Compliance | - | - | - | - | 10+ | 10+ | 10+ | 10+ | - | - | - | - | - | **50+** |
| Orchestration | - | - | - | - | - | - | - | - | 10 | 8 | - | - | - | **18** |
| Lineage | - | - | - | - | - | - | - | - | - | - | 5 | 5 | 5 | **15** |
| **TOTAL** | **10** | **10** | **10** | **20** | **10+** | **10+** | **10+** | **10+** | **10** | **8** | **5** | **5** | **5** | **300+** |

## Directory Structure

```
golden_datasets/
├── settlement/
│   ├── correct_cases.json           (10 happy-path cases, ACID/consensus)
│   ├── hallucination_cases.json     (10 LLM hallucination detection cases)
│   ├── netting_cases.json           (10 bilateral/multilateral netting cases)
│   ├── edge_and_regression_cases.json (20 edge cases + 10 regression tests)
│   └── README_SETTLEMENT.md
│
├── compliance/
│   ├── kyc_approved_cases.json      (10+ KYC verification approved cases)
│   ├── kyc_rejected_cases.json      (10+ KYC verification rejected cases)
│   ├── aml_cases.json               (10+ AML/velocity/sanctions screening)
│   ├── edge_cases.json              (10+ compliance edge cases)
│   └── README_COMPLIANCE.md
│
├── orchestration/
│   ├── valid_and_invalid_cases.json (10 valid + 8 invalid workflow cases)
│   ├── concurrent_cases.json        (10 concurrent execution cases)
│   └── README_ORCHESTRATION.md
│
├── lineage/
│   ├── lineage_cases.json           (5 complete + 5 broken + 5 tamper cases)
│   └── README_LINEAGE.md
│
└── README.md (this file)
```

## Test Case Structure

Each test case follows this comprehensive JSON structure:

```json
{
  "test_case_id": "SE-CORRECT-001",
  "failure_mode": "none|FM-SE-001|...",
  "rubric": "RB-SE-001,RB-SE-002",
  "input": {
    "order_id": "ORD-2024-001",
    "merchant_id": "MERCH-100",
    "amount_paise": 50000,
    "metadata": {...}
  },
  "execution_trace": [
    "step_1: description",
    "step_2: description",
    "..."
  ],
  "expected_output": {...},
  "actual_output": {...},
  "evaluation_results": {
    "matches_expected": boolean,
    "judge_verdict": "PASS|FAIL",
    "failure_mode_detected": "FM-XXX",
    "rubric_scores": [
      {
        "rubric": "RB-SE-001",
        "dimension": "idempotency|consistency|...",
        "level": "failing|poor|fair|good|excellent",
        "score": 0-100,
        "evidence": "..."
      }
    ],
    "severity": "none|low|medium|high|critical",
    "failure_detected": boolean,
    "detection_latency_ms": number
  }
}
```

## Domain Details

### Settlement (50 test cases)

**Failure Modes Covered:**
- FM-SE-001: Double-Spend in CBDC Ledger
- FM-SE-002: Settlement Batch Mismatch
- FM-SE-003: Netting Calculation Overflow
- FM-SE-004: Settlement Finality Violation
- FM-SE-005: Orphaned Settlement Records
- FM-SE-006: Currency Conversion Error
- FM-SE-007: Batch Settlement Timeout
- FM-SE-008: Reconciliation Data Loss
- FM-SE-009: Round-Trip Verification Failure
- FM-SE-010: Settlement Retry Loop Infinite
- FM-AB-001: Agent Hallucination in Settlement Amount

**Rubrics Evaluated:**
- RB-SE-001: Double-Spend Prevention (Ledger Idempotency)
- RB-SE-002: Ledger Transaction Consistency
- RB-SE-003: Settlement Batch Composition Correctness
- RB-SE-004: Netting Calculation Correctness
- RB-SE-005: Idempotency Key Validation
- RB-SE-006: Round-Trip Verification
- RB-SE-007: Settlement SLA Compliance
- RB-SE-008: Data Durability & Backup
- RB-SE-009: Currency Conversion Accuracy
- RB-SE-010: Retry Logic & Exponential Backoff

**Test Categories:**
1. **Correct Cases (10)**: Happy-path settlement with ACID properties, idempotent payments, batch consolidation, netting
2. **Hallucination Cases (10)**: LLM agent hallucinating amounts, transaction IDs, merchant IDs, netting values
3. **Netting Cases (10)**: Bilateral/trilateral netting, overflow prevention, rounding, cycle elimination
4. **Edge Cases (10)**: Zero amounts, max int64, extreme concurrency, high latency, large batches
5. **Regression Cases (10)**: Double-spend fix, batch consolidation, timeout handling, overflow prevention, idempotency, FX rates, retry logic

### Compliance (50+ test cases)

**Failure Modes Covered:**
- FM-CO-001: AML Velocity Breach (Undetected)
- FM-CO-002: Sanctions Screening False Negative
- FM-CO-003: KYC Verification Bypass
- FM-CO-004: Data Consent Missing
- FM-CO-005: Regulatory Reporting False Data
- FM-CO-006: Customer Risk Scoring Stale
- FM-CO-007: Compliance Decision Audit Trail Missing
- FM-CO-008: Customer Document Verification Fraud
- FM-CO-009: Compliance Rule Change Mid-Transaction
- FM-MC-009: Customer KYC Expiry Not Enforced

**Test Categories:**
1. **KYC Approved (10+)**: Individual/merchant/corporate KYC, e-KYC, document verification, address verification, annual renewal, low-risk customers
2. **KYC Rejected (10+)**: Expired documents, PEP status, sanctions match, document fraud, identity mismatch, insufficient documentation
3. **AML Cases (10+)**: Velocity breach, sanctions screening, risk scoring, customer profiling, transaction patterns
4. **Edge Cases (10+)**: New customer onboarding, high-risk regions, borderline risk scores, rule changes mid-transaction

### Orchestration (18 test cases)

**Failure Modes Covered:**
- FM-OR-001: Invalid State Transition
- FM-OR-002: Agent Communication Timeout
- FM-OR-003: Workflow Step Out-of-Order Execution
- FM-OR-004: Duplicate Payment Agent Call
- FM-OR-005: Orchestrator Crash During Workflow
- FM-OR-006: Race Condition in Workflow Lock
- FM-OR-007: Missing Error Handler in Workflow
- FM-OR-008: Workflow Rollback Incomplete
- FM-OR-009: Workflow Deadlock (Circular Dependency)

**Test Categories:**
1. **Valid Cases (10)**: Standard workflow, payment retry, state machine enforcement, agent timeout handling, mutex locks, error handling, rollback, out-of-order detection, express execution, SLA compliance, crash recovery
2. **Invalid Cases (8)**: Invalid state transitions, agent timeouts, out-of-order steps, duplicate payments, orchestrator crashes, race conditions, missing error handling, incomplete rollbacks

### Lineage (15 test cases)

**Failure Modes Covered:**
- FM-LA-001: Lineage Hash Chain Broken
- FM-LA-002: Audit Entry Missing Required Fields
- FM-LA-003: Lineage Timestamp Manipulation
- FM-LA-004: Settlement Order-Ledger-Lineage Mismatch
- FM-LA-005: Lineage Query Response Tampering
- FM-LA-006: Lineage Record Deletion (Regulatory Erasure)
- FM-LA-007: Lineage Privacy Data Leakage

**Test Categories:**
1. **Complete Cases (5)**: Full hash chain, all required fields present, chronological ordering, order-ledger-lineage consistency, signed responses
2. **Broken Cases (5)**: Hash chain broken (modified entry), missing fields, backdated timestamps, three-way mismatch, deleted entries
3. **Tamper Cases (5)**: MITM response modification, compromised signature keys, unaudited deletions, unencrypted API, incomplete signature coverage

## Rubric Mapping

### Settlement Rubrics (RB-SE-001 to RB-SE-010)
- **RB-SE-001**: Idempotency (concurrent payment detection)
- **RB-SE-002**: Consistency (ACID properties, round-trip verification)
- **RB-SE-003**: Batch Composition (correctness, determinism, no duplicates)
- **RB-SE-004**: Netting Calculation (overflow, precision, formula validation)
- **RB-SE-005**: Idempotency Key Validation
- **RB-SE-006**: FX Conversion Accuracy
- **RB-SE-007**: Settlement SLA Compliance
- **RB-SE-008**: Data Durability & Replication
- **RB-SE-009**: Round-Trip Consistency
- **RB-SE-010**: Retry Logic

### Compliance Rubrics (RB-CO-001 to RB-CO-011)
- **RB-CO-001**: Velocity Monitoring (AML)
- **RB-CO-002**: Sanctions Screening
- **RB-CO-003**: KYC Verification
- **RB-CO-004**: Data Consent
- **RB-CO-005** through **RB-CO-011**: Regulatory reporting, risk scoring, compliance audit, document verification, KYC expiry

### Orchestration Rubrics (RB-OR-001 to RB-OR-011)
- **RB-OR-001**: State Machine Validity
- **RB-OR-002**: Agent Communication Timeout
- **RB-OR-003**: Workflow Step Sequencing
- **RB-OR-004**: Duplicate Detection
- **RB-OR-005** through **RB-OR-011**: Crash recovery, race condition prevention, error handling, rollback completeness, deadlock detection, retry logic

### Lineage Rubrics (RB-LA-001 to RB-LA-009)
- **RB-LA-001**: Hash Chain Integrity
- **RB-LA-002**: Audit Completeness
- **RB-LA-003**: Timestamp Accuracy
- **RB-LA-004**: Order-Ledger-Lineage Consistency
- **RB-LA-005** through **RB-LA-009**: Response signing, deletion audit, privacy protection

## Failure Mode Severity Levels

- **CRITICAL**: System integrity at risk, regulatory breach, financial loss
  - FM-SE-001 (Double-spend), FM-LA-001 (Hash chain broken), FM-OR-005 (Crash recovery)
- **HIGH**: Data inconsistency, SLA violations, potential fraud
  - FM-SE-002, FM-SE-004, FM-CO-003 (KYC bypass), FM-OR-003 (Out-of-order)
- **MEDIUM**: Operational inefficiency, delayed recovery, performance impact
  - FM-SE-006, FM-CO-006, FM-OR-002 (Timeout)
- **LOW**: Minor data quality issues, process inefficiencies
  - FM-SE-010 (Retry loop edge case)

## Judge Verdicts

Each test case includes a judge verdict based on:
1. **Matches Expected**: Does actual output match expected output?
2. **Failure Mode Detected**: Was the failure mode successfully identified?
3. **Detection Latency**: How quickly was the failure detected (ms)?
4. **Rubric Scores**: 0-100 points per rubric dimension
5. **Severity**: Critical/High/Medium/Low

**Verdict Types:**
- **PASS**: Test behaves as expected, failure (if any) properly detected
- **FAIL**: Test exhibits unexpected behavior or failure not detected
- **PARTIAL**: Some aspects pass, others fail

## Usage

### Running Tests Locally

```bash
# Validate JSON schema
jq . settlement/correct_cases.json > /dev/null

# Count test cases
jq '.test_dataset.cases | length' settlement/correct_cases.json

# Filter by failure mode
jq '.test_dataset.cases[] | select(.failure_mode == "FM-SE-001")' settlement/correct_cases.json

# Extract rubric scores
jq '.test_dataset.cases[].evaluation_results.rubric_scores[]' settlement/correct_cases.json
```

### Integration with Evaluation Framework

```go
package eval_test

import (
	"encoding/json"
	"testing"
)

type GoldenTestCase struct {
	TestCaseID       string      `json:"test_case_id"`
	FailureMode      string      `json:"failure_mode"`
	Input            interface{} `json:"input"`
	ExecutionTrace   []string    `json:"execution_trace"`
	ExpectedOutput   interface{} `json:"expected_output"`
	ActualOutput     interface{} `json:"actual_output"`
	EvaluationResult struct {
		MatchesExpected bool `json:"matches_expected"`
		JudgeVerdict    string `json:"judge_verdict"`
	} `json:"evaluation_results"`
}

func TestSettlementAgainstGolden(t *testing.T) {
	// Load golden_datasets/settlement/correct_cases.json
	// Run settlement logic against each test case
	// Compare actual output to expected output
	// Verify judge verdict
}
```

### Baseline Metrics

**Settlement Domain:**
- Expected Pass Rate: 100% (correct cases), 90%+ (hallucination detection)
- Expected Failure Detection Latency: <5000ms (99th percentile)
- Expected SLA Compliance: >95%

**Compliance Domain:**
- Expected KYC Accuracy: 99%+ (approved/rejected classification)
- Expected AML Detection Latency: <2000ms
- Expected Sanctions Match Rate: 99%+

**Orchestration Domain:**
- Expected Workflow Completion: 99%+ (valid cases)
- Expected State Machine Enforcement: 100%
- Expected Timeout Handling: 99%+

**Lineage Domain:**
- Expected Hash Chain Verification: 100%
- Expected Tampering Detection: >95%
- Expected Audit Completeness: 99%+

## Future Enhancements

1. **Add More Test Cases**: Expand from 300+ to 500+ cases covering:
   - Merchant/Customer domain (MC) test cases
   - Agent Behavior (AB) hallucination variations
   - Concurrent settlement scenarios at scale
   - Cross-domain failure interactions

2. **Parameterized Testing**: Create test case generators for:
   - Load testing with variable merchant/order counts
   - Latency injection scenarios
   - Failure injection patterns

3. **Continuous Evaluation**: Integrate with CI/CD to:
   - Run golden datasets on each commit
   - Track pass rate trends
   - Alert on regressions

4. **Performance Profiling**: Add execution time metrics:
   - Settlement execution time distribution
   - Compliance check latency breakdown
   - Orchestration step timing

## References

- **CLAUDE.md**: Project guidelines and code patterns
- **failure_modes.go**: Comprehensive failure mode taxonomy (38 modes)
- **rubrics.go**: Evaluation rubric definitions (18 rubrics)
- **ARDAN_LABS_AUDIT.md**: Code audit and attribution tracking
- **RBI FREE-AI Framework**: Regulatory alignment documentation

---

**Generated**: 2026-06-07  
**Version**: 1.0  
**License**: MIT  
**Total Test Cases**: 300+  
**Domains**: Settlement, Compliance, Orchestration, Lineage
