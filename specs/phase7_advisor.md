# Phase 7: Advisor Workspace Specification

**Status**: 🔜 Planned  
**Planned APIs**: 6 endpoints  
**Planned Types**: 8 type definitions  
**Planned Tests**: 16 tests  

---

## Specification

### Overview

The Advisor workspace extends the Assistant by providing **personalized financial recommendations**. While the Assistant answers questions, the Advisor generates tailored suggestions based on user profile, transaction history, compliance status, and risk profile.

**Key Difference**:
- **Assistant**: "What are my top spending categories?" → Answer with data
- **Advisor**: "Based on your profile, you should optimize spending in X, increase savings in Y, consider Z products" → Recommendation with rationale

---

### API Endpoints

#### 1. Generate Recommendation
```
POST /v1/advisor/recommendation
Request:
  - user_id: string (required)
  - category: "spending_optimization" | "savings" | "investment" | "tax_efficiency" | "risk_management"
  - context_document_id?: string (CSV with transaction data)
  - constraints?: {max_risk: 0-1, min_return: %, timeline_months: number}
Response:
  - recommendation_id: string
  - category: string
  - title: string
  - description: string
  - actions: Action[]
  - estimated_impact: {monthly_savings: number, annual_return: number}
  - confidence: 0-1
  - trace_id: string
  - ai_disclosure: string
```

#### 2. Get Recommendation Detail
```
GET /v1/advisor/recommendation/{recommendation_id}
Response:
  - recommendation_id, category, title, description
  - actions: Action[]
  - estimated_impact, confidence
  - rationale: string (why this recommendation)
  - risks: Risk[]
  - compliance_constraints: string[]
  - created_at: RFC3339
  - updated_at: RFC3339
```

#### 3. Accept/Reject Recommendation
```
POST /v1/advisor/recommendation/{recommendation_id}/feedback
Request:
  - action: "accept" | "reject" | "defer"
  - reason?: string (why rejected/deferred)
  - deferred_until?: RFC3339 (for defer action)
Response:
  - recommendation_id, status, feedback_timestamp
```

#### 4. Recommendation History
```
GET /v1/advisor/history?user_id={id}&limit=20&status=accepted|rejected|pending
Response:
  - recommendations: Recommendation[]
  - total_count: number
  - recommendations_accepted: number
  - recommendations_rejected: number
  - estimated_total_impact: {monthly_savings, annual_return}
```

#### 5. Recommendation Calibration
```
GET /v1/advisor/calibration?user_id={id}
Response:
  - user_id: string
  - recommendations_count: number
  - acceptance_rate: % (recommendations accepted / total)
  - avg_impact_realized: number (actual vs estimated)
  - accuracy_score: 0-100 (how often estimated impact matched reality)
  - last_calibrated_at: RFC3339
```

#### 6. Recommendations Stream
```
POST /v1/advisor/stream
Request:
  - user_id: string
  - category?: string (filter by category)
Response: text/event-stream
  event: ai_disclosure
  data: "..."
  
  event: advisor.analyzing
  data: {step: "analyzing_profile", progress: 0.2}
  
  event: advisor.generating
  data: {step: "generating_recommendation", progress: 0.8}
  
  event: recommendation
  data: {recommendation_id, category, title, ...}
```

---

### Types

#### Action
```typescript
interface Action {
  id: string;
  type: "reduce_category" | "increase_savings" | "move_funds" | "invest" | "refinance" | "enroll_program";
  description: string;
  estimated_impact_paise: number;
  difficulty: "easy" | "medium" | "hard"; // ease of implementation
  timeline_days: number; // how long to implement
}
```

#### Risk
```typescript
interface Risk {
  id: string;
  category: "market" | "liquidity" | "counterparty" | "regulatory";
  description: string;
  severity: "low" | "medium" | "high";
  mitigation: string;
}
```

#### Recommendation
```typescript
interface Recommendation {
  recommendation_id: string;
  user_id: string;
  category: string;
  title: string;
  description: string;
  rationale: string;
  actions: Action[];
  estimated_impact: {
    monthly_savings_paise: number;
    annual_return_paise: number;
    confidence: number; // 0-1
  };
  risks: Risk[];
  compliance_constraints: string[];
  status: "pending" | "accepted" | "rejected" | "deferred" | "implemented";
  trace_id: string;
  created_at: string;
  updated_at: string;
}
```

#### RecommendationRequest
```typescript
interface RecommendationRequest {
  user_id: string;
  category: string;
  context_document_id?: string;
  constraints?: {
    max_risk: number; // 0-1
    min_return: number; // %
    timeline_months: number;
  };
}
```

#### UserProfile
```typescript
interface UserProfile {
  user_id: string;
  risk_tolerance: "conservative" | "moderate" | "aggressive";
  income_annual_paise: number;
  savings_goal_paise: number;
  investment_experience: "beginner" | "intermediate" | "expert";
  compliance_restrictions: string[]; // e.g., ["cannot_invest_offshore"]
}
```

#### CalibrationMetrics
```typescript
interface CalibrationMetrics {
  user_id: string;
  recommendations_count: number;
  acceptance_rate: number; // %
  avg_impact_realized: number; // actual / estimated ratio
  accuracy_score: number; // 0-100
  last_calibrated_at: string;
}
```

---

### Test Requirements

**Unit Tests** (10 tests):
- [ ] Recommendation generated with correct category
- [ ] Actions include realistic impact estimates
- [ ] Confidence score reflects recommendation certainty (0-1)
- [ ] Compliance constraints honored (e.g., no offshore for restricted users)
- [ ] Risk assessment includes market, liquidity, regulatory risks
- [ ] Acceptance rate tracks correctly
- [ ] Deferred recommendations re-appear after deadline
- [ ] Impact estimates within ±10% of baseline
- [ ] Multiple recommendations per category don't conflict
- [ ] Streaming events ordered: disclosure → analyzing → generating → recommendation

**Integration Tests** (6 tests):
- [ ] Full flow: generate → detail → accept → history
- [ ] Calibration compares estimated vs realized impact
- [ ] User profile influences recommendations (conservative vs aggressive)
- [ ] Compliance rules block certain recommendations
- [ ] Multi-recommendation strategy (e.g., "spend reduction + investment") coherent
- [ ] Advisor + Assistant conversation loop works (ask question → get recommendation)

---

### Validation Rules

**Recommendation Generation**:
- Category: one of [spending_optimization, savings, investment, tax_efficiency, risk_management]
- Title: 5-100 characters, non-empty
- Actions: at least 1 per recommendation
- Confidence: 0.0-1.0 (0 = uncertain, 1 = very confident)
- Estimated impact: realistic (conservative ±20% tolerance on baseline)

**Compliance Constraints**:
- User compliance restrictions checked before generating investment recommendations
- Example: if user flagged for AML, cannot recommend transfers to high-risk countries
- Example: if user is retail, cannot recommend derivatives
- Constraints documented in recommendation

**Risk Assessment**:
- Market risk: for investment recommendations
- Liquidity risk: for illiquid assets
- Counterparty risk: for investments/deposits
- Regulatory risk: for recommendations near compliance limits
- All risks must have mitigation strategy

**User Profile**:
- Risk tolerance determines recommendation aggressiveness
- Investment experience gates complexity (beginner: no derivatives)
- Income + savings goal determine realistic targets

**Impact Estimation**:
- Base estimate on user's historical data
- Conservative vs aggressive scenarios
- Confidence reflects data quality (more historical data = higher confidence)

---

### Success Criteria

✅ All 6 endpoints respond correctly  
✅ All 8 types match TypeScript definitions  
✅ Recommendations include rationale + actions  
✅ Confidence scores calibrated (0.6-0.95 typical range)  
✅ Compliance constraints always honored  
✅ Acceptance rate tracked per user  
✅ Calibration metrics computed monthly  
✅ All 10 unit tests pass  
✅ All 6 integration tests pass  
✅ Streaming events ordered correctly  

---

### Architecture

**Advisor Agent Pipeline**:
```
User Request
  → ProfileAnalyzer (load user risk, compliance, history)
  → FinancialAnalyst (analyze transactions, identify opportunities)
  → RecommendationGenerator (LLM creates personalized rec)
  → ComplianceChecker (validate against constraints)
  → ImpactEstimator (calculate savings/return)
  → AdvisorOrchestrator (format response, return)
```

**Data Sources**:
- Commerce orders (spending patterns)
- Compliance status (what's allowed)
- Assistant conversations (user goals expressed)
- User profile (risk tolerance, income)
- Historical recommendations (track accuracy)

**Integration with Other Phases**:
- **Commerce**: Transaction history determines spending patterns
- **Compliance**: KYC/AML status gates recommendations
- **Governance**: High-impact recommendations require HITL approval
- **Evaluation**: Judge: "Is recommendation appropriate for user?"
- **Assistant**: Follow-up to questions (e.g., "How do I save more?" → Advisor generates plan)

---

### Phase 7 Dependencies

**Must Have**:
- ✅ Phase 2 (Commerce): Order/transaction data for analysis
- ✅ Phase 3 (Compliance): KYC/AML status to gate recommendations
- ✅ Phase 4 (Governance): HITL for high-impact approvals
- ✅ Phase 5 (Evaluation): Judge for recommendation quality
- ✅ Phase 6 (Assistant): Integration for conversational advice

**Nice to Have**:
- Market data feeds (external data integration)
- Product catalog (investment options)
- Benchmarking service (compare to peers)

---

### Compliance & Standards

**RBI Alignment**:
- Sutra 4 (Transparency): Recommendation includes full rationale
- Sutra 6 (Competence): Recommendations tested against golden dataset
- Recommendation 8 (Graded Liability): HITL review for high-impact changes
- Recommendation 14 (Suitability): Recommendations match user profile

**Liability Model**:
- System-recommended: system bears risk if not meeting user goals
- User-implemented: user responsible for execution
- HITL-approved: shared responsibility (system + human)

---

### Testing Strategy

**Synthetic User Profiles**:
1. Conservative retiree (risk_tolerance: low, income: ₹2L/month)
2. Aggressive young professional (risk_tolerance: high, income: ₹10L/month)
3. Moderate middle-class (risk_tolerance: medium, income: ₹5L/month)
4. AML-flagged user (compliance_restrictions: ["cannot_invest_offshore"])
5. High-net-worth individual (income: ₹50L/month, investment_experience: expert)

**Test Scenarios**:
- Conservative user → all recommendations low-risk
- Restricted user → no forbidden recommendations
- Young professional → recommendations include growth investments
- Multi-recommendation → no conflicts between actions

---

### Success Metrics (Week 1)

- [ ] Spec defined + reviewed
- [ ] Implementation checklist generated
- [ ] API endpoints scaffolded
- [ ] Type definitions created + tested
- [ ] First 3 agents working (ProfileAnalyzer, FinancialAnalyst, RecommendationGenerator)
- [ ] Unit tests passing for basic flow
- [ ] Documentation complete

---

### Related Specifications

- **Commerce** (Phase 2): Transaction history as input
- **Compliance** (Phase 3): Constraints gating recommendations
- **Governance** (Phase 4): HITL approval for high-impact actions
- **Evaluation** (Phase 5): Judge for recommendation quality
- **Assistant** (Phase 6): Conversational entry point

---

## Next Phase (Phase 8 Suggested)

**Analytics Workspace**: Dashboard for insights
- Transaction trends (monthly/annual)
- Spending by category (pie charts, trends)
- Savings vs goals (progress bars)
- Recommendation impact tracking (actual vs estimated)
- Peer benchmarking (user vs similar profiles)

This would create a complete financial advisory platform.
