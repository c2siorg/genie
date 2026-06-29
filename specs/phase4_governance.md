# Phase 4: Governance Workspace Specification

**Status**: ✅ Complete  
**Real APIs**: 9 endpoints, fully discovered  
**Real Types**: 12 type definitions  
**Test Coverage**: 20 tests  

---

## Specification

### API Endpoints

#### 1. Agent Fleet
```
GET /governance/agents
Response:
  - agents: AgentSummary[]
    - agent_id, name, capabilities[]
    - tier: "sketch" | "prototype" | "beta" | "production"
    - ring: "admin" | "standard" | "restricted" | "sandboxed"
    - trust_score: 0-100, status: "healthy" | "degraded" | "unhealthy"
```

#### 2. Agent Trust
```
GET /governance/trust/{agent_id}
Response:
  - agent_id, overall_score: 0-100
  - reliability: success rate %, compliance: policy adherence %
  - audited_at: RFC3339
```

#### 3. Incidents
```
GET /incidents?limit=20
Response:
  - incidents: Incident[]
    - id, occurred_at, detected_at, use_case, model
    - description, failure_mode, severity
    - affected_stakeholders, status, actor_id
```

#### 4. Create Incident
```
POST /incidents
Request:
  - use_case: string
  - model: string
  - description: string
  - affected_stakeholders: "internal" | "external" | "both"
Response:
  - incident_id: string, status: "ongoing"
```

#### 5. HITL Approvals
```
GET /hitl/approvals
Response:
  - approvals: HITLApprovalRequest[]
    - request_id, session_id, agent_id, tool_name
    - risk_score: 0-1, created_at, expires_at
```

#### 6. Compliance Limits
```
GET /compliance/limits/{account_id}
Response:
  - type: "p2p" | "merchant"
  - daily_limit_paise, single_transaction_max_paise
  - current_daily_usage_paise, remaining_today_paise
  - reset_at: RFC3339
```

#### 7. Kill-Switch
```
GET /governance/killswitch
Response:
  - is_active: boolean
  - activations: KillSwitchRequest[]
    - scope: "global" | "agent" | "capability"
    - target?, reason, activated_at, activated_by
  - last_change_at: RFC3339
```

#### 8. Activate Kill-Switch
```
POST /governance/killswitch
Request:
  - scope: "global" | "agent" | "capability"
  - target?: string (agent_id or capability_name)
  - reason: string
Response:
  - is_active: true, activation_record
```

#### 9. Audit Log
```
GET /governance/audit?agent_id={id}&action={name}&limit=50
Response:
  - entries: AuditEntry[]
    - seq: number, occurred_at, actor, action, target
    - details?, prev_hash, row_hash (hash-chained)
```

---

### Types

#### AgentSummary
```typescript
interface AgentSummary {
  agent_id: string;
  name: string;
  capabilities: string[];
  tier: "sketch" | "prototype" | "beta" | "production";
  ring: "admin" | "standard" | "restricted" | "sandboxed";
  trust_score: number; // 0-100
  status: "healthy" | "degraded" | "unhealthy";
  last_seen_at?: string;
}
```

#### TrustScore
```typescript
interface TrustScore {
  agent_id: string;
  overall_score: number; // 0-100
  reliability: number; // success rate %
  compliance: number; // policy adherence %
  audited_at: string; // RFC3339
}
```

#### Incident (RBI Annexure VI)
```typescript
interface Incident {
  id: string;
  occurred_at: string; // RFC3339
  detected_at: string;
  use_case: string;
  model: string;
  description: string;
  severity: "low" | "moderate" | "high" | "critical";
  failure_mode: string;
  affected_stakeholders: "internal" | "external" | "both";
  status: "ongoing" | "resolved";
  actor_id: string;
  metadata?: Record<string, unknown>;
}
```

#### FailureMode (38 types documented)
```typescript
type FailureMode = 
  | "double_spend" // FM-SE-001
  | "aml_breach" // FM-CO-001
  | "state_invalid" // FM-OR-001
  | "hallucination" | "bias" | "privacy_breach" | ...;
```

#### KillSwitchStatus
```typescript
interface KillSwitchStatus {
  is_active: boolean;
  activations: KillSwitchRequest[];
  last_change_at: string; // RFC3339
}
```

#### KillSwitchRequest
```typescript
interface KillSwitchRequest {
  scope: "global" | "agent" | "capability";
  target?: string;
  reason: string;
  activated_at: string; // RFC3339
  activated_by: string;
}
```

#### AuditEntry (hash-chained)
```typescript
interface AuditEntry {
  seq: number;
  occurred_at: string; // RFC3339
  actor: string; // user_id, agent_id, "system"
  action: string; // "consent.grant", "payment.settle"
  target: string;
  details?: Record<string, unknown>;
  prev_hash: string; // hex SHA256
  row_hash: string; // hex SHA256
}
```

#### ComplianceLimit
```typescript
interface ComplianceLimit {
  type: "p2p" | "merchant";
  daily_limit_paise: number;
  single_transaction_max_paise: number;
  current_daily_usage_paise: number;
  remaining_today_paise: number;
  reset_at?: string;
}
```

#### SLOReport
```typescript
interface SLOReport {
  agent_id: string;
  availability_percent: number; // target: 99.5%
  latency_p95_ms: number; // target: 10000ms
  reported_at: string;
  window_days: number;
}
```

#### PolicyRule
```typescript
interface PolicyRule {
  id: string;
  tool_pattern: string; // "delete_*", "read_*"
  action: "allow" | "deny" | "ask_human" | "rate_limit";
  reason: string;
  priority: number;
}
```

---

### Test Requirements

**Unit Tests** (12 tests):
- [ ] Agent tier progression: sketch → prototype → beta → production
- [ ] Trust score combines reliability + compliance
- [ ] Incident creates with correct severity
- [ ] Kill-switch can scope: global, agent, capability
- [ ] Audit trail hash-chain verified
- [ ] Compliance limits reset at UTC midnight
- [ ] SLO report calculates availability %
- [ ] HITL approval expires after 24h
- [ ] Policy rules prioritized correctly
- [ ] Agent ring restricts capabilities
- [ ] Incident escalation triggers at critical severity
- [ ] Audit entry includes prev_hash chain

**Integration Tests** (8 tests):
- [ ] Agent health degrades on repeated failures
- [ ] Kill-switch blocks all agent operations
- [ ] Audit log unbroken for 100% of actions
- [ ] HITL approval workflow end-to-end
- [ ] Incident reported, resolved, timestamped
- [ ] Policy enforcement blocks risky tools
- [ ] Compliance limits enforced across agents
- [ ] SLO metrics published to OTel

---

### Validation Rules

**Agent Tier**:
- sketch: development only, sandboxed
- prototype: alpha testing, restricted ring
- beta: limited production, standard ring
- production: full access, admin ring

**Trust Score**:
- Calculation: (reliability × 0.6) + (compliance × 0.4)
- Degradation: −5 points per error
- Recovery: +1 point per 100 successful ops

**Kill-Switch**:
- Global: stops all agents
- Agent: stops one agent
- Capability: blocks tool pattern (e.g., delete_*)
- Reversible: deactivate endpoint

**Audit Trail**:
- Every action logged (creation, deletion, modification)
- Hash-chained: row_hash based on prev_hash + content
- Tamper-detection: broken hash chain fails verification
- Immutable: entries never deleted

**Incident Response**:
- Critical → page on-call within 5 min
- High → notify owner within 30 min
- Moderate → review in daily standup
- Low → batch review weekly

---

### Success Criteria

✅ All 9 endpoints respond correctly  
✅ All incidents tracked (100%)  
✅ All actions audit-logged (100%)  
✅ Audit hash-chain unbroken  
✅ Trust scores update in real-time  
✅ Kill-switch blocks in <1s  
✅ All 12 unit tests pass  
✅ All 8 integration tests pass  

---

### Related Specifications

- **Commerce** (Phase 2): Payment failure incident logging
- **Compliance** (Phase 3): KYC rejection incident escalation
- **Evaluation** (Phase 5): Judge accuracy SLO tracking
- **Assistant** (Phase 6): Agent behavior incident detection
