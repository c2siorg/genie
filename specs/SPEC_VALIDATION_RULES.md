# Genie Specification Validation Rules

Comprehensive validation framework for spec-driven development. Use these rules to validate implementations against specifications.

---

## Phase 2: Commerce

### Edge Cases to Test

**Order Creation**:
- [ ] Zero items → reject with 400
- [ ] Item quantity = 0 → reject  
- [ ] Negative unit_price_paise → reject
- [ ] total_paise overflow (>max int64) → reject
- [ ] Merchant not found → reject with 404
- [ ] Customer UUID invalid format → reject with 400
- [ ] Concurrent creations same merchant → both succeed with different order_ids

**Payment Processing**:
- [ ] Payment amount < order total → reject
- [ ] Payment amount > order total → accept (change handled)
- [ ] Double payment same order → first succeeds, second fails (idempotency check)
- [ ] Payment timeout (8s) → async, return pending status
- [ ] From account == to account → reject (self-payment)

**Settlement**:
- [ ] Empty order list → reject
- [ ] Order already settled → reject
- [ ] Partial netting (2 of 3 orders) → calculate net correctly
- [ ] Merchant not found in orders → reject
- [ ] Settlement exceeds merchant daily limit → reject

### Compliance Mappings

| Failure Mode | Test Case | RBI Concern |
|--------------|-----------|-------------|
| Settlement amount hallucinated | Order ₹100k, settlement ₹105k | Rec 3 (CBDC ledger accuracy) |
| Double-spend | Payment confirmed twice | Rec 5 (No double-spends) |
| Lost transaction | Settlement initiated but not recorded | Rec 7 (Lineage completeness) |
| Merchant not settled | Orders placed but no settlement | Rec 9 (Settlement finality) |

---

## Phase 3: Compliance

### Edge Cases to Test

**Velocity Checks**:
- [ ] Daily limit = 0 → all transactions blocked
- [ ] Current usage = limit → next transaction blocked
- [ ] Current usage = limit - 1 paise → next transaction (exactly limit) succeeds
- [ ] Reset at UTC midnight → usage resets to 0 at 00:00:00 UTC
- [ ] Timezone edge case → transaction at 23:59:59 UTC vs 00:00:01 UTC
- [ ] Concurrent transactions same account → sum checked atomically

**AML Screening**:
- [ ] OFAC match on partial name → flag (not block, allow review)
- [ ] PEP match → block immediately
- [ ] Sanctions list expired → ignore old matches
- [ ] New account (age < 30 days) → automatic REVIEW flag
- [ ] Multiple matches same account → highest severity wins

**Fraud Detection**:
- [ ] Structuring (4 × ₹25k in 1 hour) → flag pattern
- [ ] Round-tripping (A→B→C→A in 10 min) → flag pattern
- [ ] Velocity spike (10× normal daily spend in 1 hour) → flag
- [ ] Pattern combination (velocity + new account) → cumulative scoring

**HITL Approvals**:
- [ ] Approval expires after 24h → auto-deny on expiry
- [ ] Reviewer rejects → log reason + decision
- [ ] No response in 24h → escalate to supervisor
- [ ] Concurrent approvals same request → first wins

### Compliance Mappings

| Failure Mode | Test Case | RBI Concern |
|--------------|-----------|-------------|
| Velocity bypass | ₹500k limit, ₹600k transaction approved | Rec 4 (Velocity enforcement) |
| AML false positive | Legitimate merchant blocked forever | Rec 8 (Graded liability) |
| KYC incomplete | Customer approved despite missing docs | Rec 6 (KYC due diligence) |
| Bias in flagging | Customer A flagged, Customer B (same profile) not | FREE-AI Sutra 3 (Fairness) |

---

## Phase 4: Governance

### Edge Cases to Test

**Agent Tier Progression**:
- [ ] Sketch → prototype: requires >0.7 trust score
- [ ] Prototype → beta: requires >0.85 trust score
- [ ] Beta → production: requires >0.95 trust score
- [ ] Demotion: failure rate spike → auto-demote to lower tier
- [ ] Tier change logged → audit entry created

**Kill-Switch**:
- [ ] Global activate → all agents stop within 100ms
- [ ] Agent-specific → only target agent stops, others continue
- [ ] Capability-specific → e.g., "delete_*" blocked, "read_*" allowed
- [ ] Deactivate → operations resume (not from backup, from normal state)
- [ ] Activation logged with reason + actor

**Hash-Chained Audit**:
- [ ] Entry N+1 contains SHA256(N's row_hash + N+1's content)
- [ ] Missing entry → chain broken
- [ ] Entry modified → hash mismatch detected
- [ ] Concurrent writes → sequence number enforced (no gaps)

**Incident Severity**:
- [ ] Critical (e.g., double-spend) → auto-page on-call
- [ ] High (e.g., AML bypass) → email owner + Slack
- [ ] Moderate (e.g., slow response) → log in dashboard
- [ ] Low (e.g., typo in message) → monthly batch review

### Compliance Mappings

| Failure Mode | Test Case | RBI Concern |
|--------------|-----------|-------------|
| Agent uncontrolled | Tier-4 agent modifies compliance rules | FREE-AI Sutra 1 (Authority) |
| Audit tampering | Incident log modified after creation | Rec 7 (Lineage integrity) |
| Silent incident | Breach occurs, not reported | Annexure VI (Timely disclosure) |
| Kill-switch delayed | Takes >1s to stop harmful agent | Rec 11 (Safety mechanisms) |

---

## Phase 5: Evaluation

### Edge Cases to Test

**Judge Calibration**:
- [ ] Test set (40% of 300 traces) → judges evaluated on held-out data
- [ ] TPR = TP / (TP + FN) → compute correctly
- [ ] TNR = TN / (TN + FP) → compute correctly
- [ ] 95% CI via 10k bootstrap samples → bounds tight (±2-5%)
- [ ] Judge disagreement → 2 out of 3 judges threshold

**Coverage Status**:
- [ ] Failure mode with 0 test cases → "critical" status
- [ ] Failure mode with 2-4 cases (of 5 minimum) → "warning"
- [ ] Failure mode with 5+ cases → "ok"
- [ ] Total coverage ≥ 100% → golden dataset complete

**Drift Detection**:
- [ ] Pass rate drops 5% week-over-week → drift alert
- [ ] New failure mode appears (not in previous 30 days) → drift alert
- [ ] Agent version changed → reset baseline (new 30-day window)
- [ ] Drift reason populated with root cause → e.g., "new LLM model"

**Multi-Rater Agreement**:
- [ ] Cohen's Kappa per rubric → κ ≥ 0.6 means substantial agreement
- [ ] κ < 0.6 → rubric needs refinement or raters realigned
- [ ] 3 raters → majority vote (2 of 3) breaks ties

### Compliance Mappings

| Failure Mode | Test Case | RBI Concern |
|--------------|-----------|-------------|
| Judge inaccuracy | Judge TPR = 0.70 (target: ≥0.90) | FREE-AI Sutra 6 (Competence) |
| Coverage gap | Failure mode FM-SE-001 has 0 test cases | Rec 13 (Comprehensive testing) |
| Drift undetected | Pass rate drops 10%, no alert | Rec 12 (Continuous monitoring) |
| Inconsistent evaluation | Same scenario scores differently by rater | Sutra 3 (Fairness) |

---

## Phase 6: Assistant

### Edge Cases to Test

**Request Validation**:
- [ ] Question = "" → reject with 400
- [ ] Question > 2000 chars → reject
- [ ] document_id invalid UUID → reject with 400
- [ ] document_id not found → reject with 404
- [ ] provider not in [anthropic, ollama, openai, gemini] → reject
- [ ] max_tokens > 32000 → reject

**Streaming**:
- [ ] Client cancels mid-stream → close gracefully
- [ ] LLM timeout (8s for /ask) → return partial report
- [ ] LLM unavailable (circuit open) → fallback to Ollama
- [ ] Event ordering: ai_disclosure must be first, report last
- [ ] WebSocket disconnect → cleanup connection, allow reconnect

**Token Budget**:
- [ ] Budget = 0 → all requests rejected
- [ ] Budget = 100 tokens, request uses 150 → reject ("budget exceeded")
- [ ] Budget daily resets → 00:00 UTC usage resets to 0
- [ ] Cost tracking → emitted to OTel (genie.llm.cost_micros)

**Data Residency**:
- [ ] Document classification = "pii" → only Ollama allowed
- [ ] Document classification = "secret" → only Ollama allowed
- [ ] Document classification = "internal" → any provider allowed
- [ ] Provider not in region → reject with 403 (forbidden)

### Compliance Mappings

| Failure Mode | Test Case | RBI Concern |
|--------------|-----------|-------------|
| Hallucination undetected | Assistant claims customer age, data missing | Sutra 6 (Accuracy) |
| PII leak to external LLM | Customer SSN sent to OpenAI for "pii" doc | Rec 10 (Data protection) |
| Budget bypass | Token budget = 0, request still processed | Rec 4 (Usage control) |
| Disclosure skipped | No AI banner shown to user | Recommendation 18 (Transparency) |

---

## Cross-Phase Validation

### Data Flow Integrity

**Commerce → Compliance**:
- [ ] Order created → compliance check runs before payment
- [ ] Order amount used in velocity check
- [ ] Merchant KYC status blocks order if not approved

**Compliance → Governance**:
- [ ] AML flag → incident created with "aml_breach" type
- [ ] HITL approval needed → logged in audit trail
- [ ] False positive → incident marked, trust score penalty

**Governance → Evaluation**:
- [ ] Incident severity mapped to failure mode
- [ ] Judge accuracy correlates with incident detection rate
- [ ] Coverage validated per failure mode

**Evaluation → Assistant**:
- [ ] Judge verdict influences response confidence
- [ ] Hallucination judge flags assistant responses
- [ ] Coverage status shown in assistant context

### State Machine Transitions

**Order Lifecycle**:
```
created 
  → [compliance check]
  → payment_initiated 
  → [payment confirm]
  → payment_confirmed 
  → [settlement request]
  → settlement_initiated
  → [settlement execution]
  → settlement_completed
  → [fulfillment]
  → fulfilled
```

No skips, no reverse transitions. Lineage records each step.

**Incident Lifecycle**:
```
occurred 
  → detected 
  → [HITL review if needed]
  → response_actions
  → [monitoring]
  → resolved/escalated
```

All transitions timestamped + actor recorded.

---

## CI/CD Validation Script

```bash
#!/bin/bash
# validate-all-specs.sh — Run complete spec validation

echo "=== Phase 2: Commerce ==="
# Endpoints exist
curl -s http://localhost:8080/v1/commerce/order/test-123 | jq .
# Types match
go test ./pkg/commerce -v -run TypeOrder

echo "=== Phase 3: Compliance ==="
# AML check working
curl -s -X POST http://localhost:8080/v1/compliance/check -d '...'
# Velocity enforced
go test ./pkg/compliance -v -run TestVelocityLimit

echo "=== Phase 4: Governance ==="
# Audit chain unbroken
go test ./pkg/storage -v -run TestAuditHashChain
# Kill-switch responsive
curl -s http://localhost:8080/governance/killswitch | jq .

echo "=== Phase 5: Evaluation ==="
# Judge accuracy ≥ 0.90
go test ./pkg/eval -v -run TestJudgeAccuracy
# Coverage validated
go test ./pkg/eval -v -run TestCoverageStatus

echo "=== Phase 6: Assistant ==="
# Streaming works
curl -s -N http://localhost:8080/v1/ask/stream | head -20
# Token budget enforced
go test ./pkg/llm -v -run TestBudgetEnforced

echo "=== All validations complete ==="
```

---

## Appendix: RBI FREE-AI Mapping

| Sutra | Validation Rule | Test Case |
|-------|-----------------|-----------|
| 1: Authority | Agent tier restricts capabilities | Sketch agent cannot delete |
| 2: People First | AI disclosure shown before interaction | Disclosure banner on /ask |
| 3: Fairness | No demographic bias in flagging | Same profile, same decision |
| 4: Transparency | Explainability for decisions | HITL approval reason logged |
| 5: Consent | User consent recorded (audit trail) | Lineage: actor + action |
| 6: Competence | Judge accuracy ≥ 0.90 | TPR/TNR calibrated |
| 7: Compliance | RBI limits enforced | Velocity blocked at limit |

---

## Status

All 6 phases have comprehensive validation rules documented. Phase 7+ should follow the same framework.
