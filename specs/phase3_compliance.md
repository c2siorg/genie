# Phase 3: Compliance Workspace Specification

**Status**: ✅ Complete  
**Real APIs**: 6 endpoints, fully discovered  
**Real Types**: 8 type definitions  
**Test Coverage**: 15 tests  

---

## Specification

### API Endpoints

#### 1. Compliance Check
```
POST /v1/compliance/check
Request:
  - payment_id: string
  - from_account: string
  - to_account: string
  - amount_paise: number
Response:
  - check_id: string
  - decision: "ALLOW" | "REVIEW" | "BLOCK"
  - aml_result: "pass" | "review" | "block"
  - velocity_result: "ok" | "warning" | "blocked"
  - fraud_score: 0-100
  - detected_patterns: string[]
```

#### 2. Check Result (Async Poll)
```
GET /v1/compliance/check/{check_id}
Response:
  - check_id, decision, aml_result, velocity_result
  - fraud_score, patterns, timestamp
```

#### 3. Velocity Metrics
```
GET /v1/compliance/velocity/{account_id}
Response:
  - daily_limit_paise: number
  - single_transaction_max_paise: number
  - current_daily_usage_paise: number
  - remaining_today_paise: number
  - reset_at: RFC3339
```

#### 4. Fraud History
```
GET /v1/compliance/fraud/{account_id}
Response:
  - account_id, fraud_score: 0-100
  - patterns: ["structuring", "round_tripping", "velocity_spike", "new_account_high"]
  - incident_count: number
```

#### 5. AML Risk Score
```
GET /v1/compliance/aml/{account_id}
Response:
  - account_id, risk_score: 0-100
  - screening_status: "clear" | "flag" | "blocked"
  - sanctions_match: boolean
  - pep_match: boolean
```

#### 6. HITL Approvals
```
GET /v1/hitl/approvals
Response:
  - requests: HITLApprovalRequest[]
    - request_id, session_id, agent_id
    - risk_score, created_at, expires_at
```

---

### Types

#### ComplianceCheck
```typescript
interface ComplianceCheck {
  check_id: string;
  payment_id: string;
  decision: "ALLOW" | "REVIEW" | "BLOCK";
  aml_result: "pass" | "review" | "block";
  velocity_result: "ok" | "warning" | "blocked";
  fraud_score: number; // 0-100
  detected_patterns: string[];
  timestamp: string; // RFC3339
}
```

#### VelocityMetrics
```typescript
interface VelocityMetrics {
  daily_limit_paise: number;
  single_transaction_max_paise: number;
  current_daily_usage_paise: number;
  remaining_today_paise: number;
  reset_at: string; // RFC3339
}
```

#### FraudHistory
```typescript
interface FraudHistory {
  account_id: string;
  fraud_score: number; // 0-100
  patterns: FraudPattern[];
  incident_count: number;
}
```

#### FraudPattern
```typescript
type FraudPattern = 
  | "structuring"
  | "round_tripping"
  | "velocity_spike"
  | "new_account_high";
```

#### AMLRiskScore
```typescript
interface AMLRiskScore {
  account_id: string;
  risk_score: number; // 0-100
  screening_status: "clear" | "flag" | "blocked";
  sanctions_match: boolean;
  pep_match: boolean;
}
```

#### HITLApprovalRequest
```typescript
interface HITLApprovalRequest {
  request_id: string;
  session_id: string;
  agent_id: string;
  tool_name: string;
  risk_score: number; // 0-1
  created_at: string; // RFC3339
  expires_at: string;
}
```

#### KYCApplication
```typescript
interface KYCApplication {
  kyc_id: string;
  account_id: string;
  state: "pending" | "approved" | "flagged" | "rejected";
  risk_level: "low" | "medium" | "high";
  review_timestamp: string; // RFC3339
}
```

#### ComplianceDecision
```typescript
interface ComplianceDecision {
  decision_id: string;
  check_id: string;
  decision: "ALLOW" | "REVIEW" | "BLOCK";
  reasoning: string;
  timestamp: string; // RFC3339
  reviewer_id?: string;
}
```

---

### Test Requirements

**Unit Tests** (10 tests):
- [ ] AML check blocks OFAC matches
- [ ] Velocity limit blocks exceeded transactions
- [ ] Fraud score calculated from pattern count
- [ ] New account gets high fraud score
- [ ] KYC rejected transactions blocked
- [ ] PEP flag triggers REVIEW
- [ ] Multiple patterns detected correctly
- [ ] Daily velocity resets at UTC midnight
- [ ] Velocity warning at 80% threshold
- [ ] Risk score combines AML + fraud signals

**Integration Tests** (5 tests):
- [ ] Payment blocks if compliance BLOCK
- [ ] HITL approval queue works end-to-end
- [ ] Flagged customer requires approval
- [ ] Velocity check runs before AML
- [ ] Incident recorded on block decision

---

### Validation Rules

**Velocity**:
- Daily limit: ₹100k for P2P, ₹500k for merchant
- Single transaction max: ₹50k
- Resets at UTC midnight
- Cumulative: current_daily_usage + transaction ≤ limit

**AML Screening**:
- OFAC, PEP, sanctions lists checked
- Any match → flag or block
- New accounts default to REVIEW

**Fraud Scoring**:
- Structuring: +30 points
- Round-tripping: +25 points
- Velocity spike: +20 points
- New account: +15 points
- Score ≥ 70 → flag, ≥ 90 → block

**KYC Decision**:
- Approved: verified documents + clear screening
- Flagged: incomplete docs or AML flag
- Rejected: OFAC match or fraud pattern

**HITL Escalation**:
- Risk score ≥ 0.5 → HITL review required
- Expires after 24 hours
- Reviewer decision recorded

---

### Success Criteria

✅ All 6 endpoints respond correctly  
✅ All compliance checks logged (100%)  
✅ Velocity limits enforced (zero bypass)  
✅ AML screening matches documented  
✅ HITL approvals audit-trailed  
✅ All 10 unit tests pass  
✅ All 5 integration tests pass  
✅ Zero false negatives on high-risk patterns  

---

### Related Specifications

- **Commerce** (Phase 2): Order payment blocks on BLOCK decision
- **Governance** (Phase 4): Incident escalation for false positives
- **Evaluation** (Phase 5): Judge for "compliance decision justified"
- **Assistant** (Phase 6): Compliance context for recommendations
