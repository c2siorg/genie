# Transaction Settlement Coordinator Agent — Implementation Plan

**Context**: Genie has an agentic `Runner` (tool-calling loop), `Registry` (tool discovery), and `Supervisor` (agent-of-agents orchestration pattern).

**Objective**: Design a **settlement orchestration workflow** where a Supervisor agent manages three specialist sub-agents (Fetcher, Calculator, Router), with HITL approval gates and audit logging at each state transition.

---

## 1. Supervisor Agent Logic (Orchestration & State Transitions)

### Architecture Overview

The **Settlement Coordinator Supervisor** is the orchestration hub that:
- Accepts a settlement request (date range, counterparties, optional netting pool)
- Routes to sub-agents in sequence: Fetcher → Calculator → Router → Approval → Execution
- Maintains settlement state machine: `pending` → `calculated` → `routed` → `approved` → `executed`
- Logs each step for audit compliance

### State Machine

```
[pending]
    ↓ (invoke Fetcher)
[fetched: X transactions loaded]
    ↓ (invoke Calculator)
[calculated: net amounts computed by counterparty/currency/settlement_date]
    ↓ (invoke Router)
[routed: settlement paths assigned (direct/correspondent/netting_pool)]
    ↓ (HITL approval gate)
[approved: human signed off on instructions]
    ↓ (invoke Executor — out of scope for this design)
[executed: bank transfers submitted]
    ↓
[completed]
```

### Supervisor Implementation Skeleton

```go
type SettlementCoordinator struct {
    Supervisor *agentic.Supervisor
    AuditLog   *AuditLog              // logs state transitions
    Store      *SettlementStore       // persists settlement state
}

type SettlementRequest struct {
    ID              string    `json:"settlement_id"`        // UUID
    StartDate       string    `json:"start_date"`           // YYYY-MM-DD
    EndDate         string    `json:"end_date"`
    Counterparties  []string  `json:"counterparties"`       // optional filter
    NettingPoolID   string    `json:"netting_pool_id"`      // optional
    RequestedBy     string    `json:"requested_by"`         // user ID
    CreatedAt       time.Time `json:"created_at"`
}

type SettlementState struct {
    ID            string                    `json:"settlement_id"`
    Status        string                    `json:"status"`       // pending|calculated|routed|approved|executed
    Transactions  []Transaction             `json:"transactions"` // from Fetcher
    NetAmounts    map[string]NetAmount      `json:"net_amounts"`  // from Calculator
    Routes        map[string]SettlementPath `json:"routes"`       // from Router
    ApprovedAt    *time.Time                `json:"approved_at"`
    ApprovedBy    string                    `json:"approved_by"`
    ExecutedAt    *time.Time                `json:"executed_at"`
    Idempotency   string                    `json:"idempotency"` // prevents double-payment
}

type NetAmount struct {
    Counterparty  string   `json:"counterparty"`
    Currency      string   `json:"currency"`
    NetCents      int64    `json:"net_cents"`       // aggregated; sign indicates direction
    SettlementDay string   `json:"settlement_day"`  // YYYY-MM-DD
    TxnCount      int      `json:"transaction_count"`
}

type SettlementPath struct {
    Counterparty    string  `json:"counterparty"`
    Method          string  `json:"method"`         // direct_bank|correspondent|netting_pool
    BankCode        string  `json:"bank_code,omitempty"`
    CorrespondentID string  `json:"correspondent_id,omitempty"`
    NettingPoolID   string  `json:"netting_pool_id,omitempty"`
    Amount          int64   `json:"amount_cents"`
    Currency        string  `json:"currency"`
}

// Run orchestrates the full settlement workflow
func (c *SettlementCoordinator) Run(
    ctx context.Context,
    req SettlementRequest,
) (*SettlementState, error) {
    // 1. Initialize state
    state := &SettlementState{
        ID:            req.ID,
        Status:        "pending",
        CreatedAt:     time.Now(),
    }
    
    // 2. Fetch transactions (delegates to Fetcher sub-agent)
    // 3. Calculate net amounts (delegates to Calculator sub-agent)
    // 4. Route settlement paths (delegates to Router sub-agent)
    // 5. HITL approval gate
    // 6. Audit log each step
    
    return state, nil
}
```

---

## 2. Sub-Agent Definitions (Fetcher, Calculator, Router)

Each sub-agent is registered as a `SubAgent` in the Supervisor's agent pool. Each performs one step and is callable by the LLM via tool invocation from the Supervisor's Runner.

### 2.1 Fetcher Sub-Agent

**Role**: Query GraphRAG for pending transactions by date range, counterparty, status.

**Tools**:
- `query_pending_transactions` — search GraphRAG by date, counterparty, status
- `get_transaction_details` — fetch full transaction record by ID

```go
type FetcherRequest struct {
    StartDate      string   `json:"start_date"`       // YYYY-MM-DD
    EndDate        string   `json:"end_date"`
    Counterparties []string `json:"counterparties"`   // nil = all
    Status         string   `json:"status"`           // "pending" (default) or "all"
}

type FetcherResponse struct {
    Transactions   []Transaction `json:"transactions"`
    Count          int           `json:"count"`
    TotalAmountCents int64       `json:"total_amount_cents"`
    Currency       string        `json:"currency"`       // INR
}

// queryPendingTransactions tool
func queryPendingTransactions(ctx context.Context, args map[string]any) (string, error) {
    // 1. Extract args: start_date, end_date, counterparties[], status
    // 2. Build GraphRAG query: 
    //    Match (t:Transaction {settlement_status: 'pending'})
    //    Where t.date >= startDate AND t.date <= endDate
    // 3. Filter by counterparty if provided
    // 4. Return JSON response with transaction list
}
```

### 2.2 Calculator Sub-Agent

**Role**: Aggregate transactions by (counterparty, currency, settlement_date), compute net amounts, detect netting opportunities.

**Tools**:
- `aggregate_transactions` — group by counterparty/currency, compute net
- `detect_netting` — identify cross-currency or multi-leg netting saves
- `compute_fees` — estimate settlement fees per method

```go
type CalculatorRequest struct {
    Transactions []Transaction `json:"transactions"`
    NettingPoolID string       `json:"netting_pool_id"` // optional
}

type CalculatorResponse struct {
    NetAmounts    []NetAmount    `json:"net_amounts"`
    NettingOpportunities []Netting `json:"netting_opportunities"`
    EstimatedFees map[string]int64 `json:"estimated_fees_cents"`
}

type Netting struct {
    PartyA    string `json:"party_a"`
    PartyB    string `json:"party_b"`
    Currency  string `json:"currency"`
    SavesCents int64 `json:"saves_cents"`  // gross – netting amount
}

// aggregateTransactions tool
func aggregateTransactions(ctx context.Context, args map[string]any) (string, error) {
    // 1. Unmarshal []Transaction from args
    // 2. Group by (counterparty, currency, settlement_day)
    // 3. Sum net amounts (credits – debits)
    // 4. Return JSON with NetAmount slice
}
```

### 2.3 Router Sub-Agent

**Role**: Decide settlement path (direct bank transfer, correspondent, netting pool) based on amount, counterparty tier, liquidity constraints, regulatory rules.

**Tools**:
- `get_counterparty_profile` — fetch tier, settlement method capabilities
- `check_liquidity` — verify sufficient funds available
- `select_settlement_path` — apply routing rules (thresholds, affinity, cost)

```go
type RouterRequest struct {
    NetAmounts []NetAmount `json:"net_amounts"`
    RulesVersion string    `json:"rules_version"` // for audit
}

type RouterResponse struct {
    Routes []SettlementPath `json:"routes"`
    ReasonsForPath map[string][]string `json:"reasons_for_path"` // per counterparty
    TotalOutstandingCents int64 `json:"total_outstanding_cents"`
}

// selectSettlementPath tool
func selectSettlementPath(ctx context.Context, args map[string]any) (string, error) {
    // 1. Fetch counterparty profile (tier, methods, max_amount)
    // 2. Apply routing rules:
    //    - Direct bank if tier=A and amount <= threshold
    //    - Correspondent if amount > direct limit
    //    - Netting pool if available and saves fees
    // 3. Return JSON with SettlementPath for each counterparty
}
```

---

## 3. GraphRAG Schema Extension (Settlement Status Field)

Add a `settlement_status` field to the Transaction node in the knowledge graph.

### Cypher Schema Patch

```cypher
-- Add settlement_status property to existing Transaction nodes
MATCH (t:Transaction)
SET t.settlement_status = 'pending'

-- Create index for fast filtering in Fetcher
CREATE INDEX settlement_status_idx FOR (t:Transaction) ON (t.settlement_status, t.date)

-- Optional: Create a SettlementBatch node to track aggregate instructions
CREATE (sb:SettlementBatch {
    id: uuid(),
    status: 'pending',
    created_at: timestamp(),
    settlement_date: date(),
    total_amount_cents: 0,
    counterparty_count: 0
})
```

### Extended Transaction Model

```go
// In pkg/finance/types.go, extend Transaction:
type Transaction struct {
    TransactionID      string       `json:"transaction_id"`
    AccountID          string       `json:"account_id"`
    Date               string       `json:"date"`                  // ISO-8601
    AmountCents        int64        `json:"amount_cents"`
    Currency           string       `json:"currency"`
    Direction          Direction    `json:"direction"`
    Description        string       `json:"description"`
    Merchant           string       `json:"merchant,omitempty"`
    
    // ← NEW FIELDS FOR SETTLEMENT ←
    SettlementStatus   string       `json:"settlement_status"`     // pending|calculated|routed|approved|executed
    SettlementDate     string       `json:"settlement_date"`       // YYYY-MM-DD; T+N offset
    Counterparty       string       `json:"counterparty"`          // SWIFT code or identifier
    NetPositionID      string       `json:"net_position_id"`       // FK to NetAmount aggregate
    SettlementBatchID  string       `json:"settlement_batch_id"`   // FK to SettlementBatch
    Idempotency        string       `json:"idempotency"`           // UUID; prevents double-settlement
    UpdatedAt          time.Time    `json:"updated_at"`
}
```

---

## 4. HITL Integration (Approval Endpoints & Policy Rules)

### 4.1 Approval Flow

After routing completes, the Supervisor **pauses** and requests HITL approval before executing settlement instructions.

```go
type SettlementApprovalRequest struct {
    SettlementID    string               `json:"settlement_id"`
    Routes          []SettlementPath     `json:"routes"`
    TotalAmountCents int64               `json:"total_amount_cents"`
    CounterpartyCount int                `json:"counterparty_count"`
    RiskLevel       string               `json:"risk_level"`   // low|medium|high
    Reason          string               `json:"reason"`       // why approval is needed
}

type SettlementApprovalDecision struct {
    SettlementID    string                `json:"settlement_id"`
    Approved        bool                  `json:"approved"`
    Reason          string                `json:"reason,omitempty"`
    ApprovedBy      string                `json:"approved_by"`          // user ID or role
    ApprovedAt      time.Time             `json:"approved_at"`
    Modifications   *SettlementModifications `json:"modifications,omitempty"` // optional changes
}

type SettlementModifications struct {
    RemovedCounterparties []string          `json:"removed_counterparties"`
    ReducedAmounts        map[string]int64  `json:"reduced_amounts_cents"`
    DelayedUntil          *time.Time        `json:"delayed_until,omitempty"`
}
```

### 4.2 Approval Policy Rules

Implement tiered risk-based approval using `hitl.PolicyApprover`:

```go
type SettlementApprovalPolicy struct {
    // Auto-approve: total < 1M INR, all counterparties tier-A
    AutoApproveThresholdCents int64
    AutoApproveTierMax        string  // "A"
    
    // Ask human: total 1M–10M, or any tier-B counterparty
    ManualReviewThresholdCents int64
    ManualReviewTiers          []string
    
    // Auto-deny: total > 100M, unverified counterparties, non-business-hours
    AutoDenyThresholdCents int64
    AutoDenyCounterparties []string
}

func (p *SettlementApprovalPolicy) Evaluate(
    req SettlementApprovalRequest,
) (action string, reason string) {
    // action: "Allow" | "Deny" | "AskHuman"
    
    // 1. Check total amount
    if req.TotalAmountCents > p.AutoDenyThresholdCents {
        return "Deny", "exceeds maximum settlement limit"
    }
    
    // 2. Check counterparty tiers
    for _, route := range req.Routes {
        if contains(p.AutoDenyCounterparties, route.Counterparty) {
            return "Deny", fmt.Sprintf("counterparty %s not authorized", route.Counterparty)
        }
    }
    
    // 3. Auto-approve small, low-risk batches
    if req.TotalAmountCents < p.AutoApproveThresholdCents && req.RiskLevel == "low" {
        return "Allow", "auto-approved: within threshold and low-risk"
    }
    
    // 4. Manual review for medium amounts or mixed tiers
    if req.TotalAmountCents < p.ManualReviewThresholdCents {
        return "AskHuman", fmt.Sprintf("manual review required: %s risk", req.RiskLevel)
    }
    
    return "AskHuman", "requires human approval"
}
```

### 4.3 Approval HTTP Endpoints

```go
// POST /settlement/approval/request
// Supervisor calls this to ask for approval
func (h *SettlementHandler) RequestApproval(w http.ResponseWriter, r *http.Request) {
    var req SettlementApprovalRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // Store request in async store (hitl.AsyncApprover)
    decision := h.approver.RequestApproval(r.Context(), hitl.ApprovalRequest{
        ID:        uuid.New().String(),
        ToolName:  "settle_transactions",
        Args:      map[string]any{"routes": req.Routes, "total": req.TotalAmountCents},
    })
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "request_id": decision.RequestID,
        "status":     "pending_human_review",
    })
}

// POST /settlement/approval/:requestId/decide
// Human submits their decision
func (h *SettlementHandler) SubmitApprovalDecision(w http.ResponseWriter, r *http.Request) {
    requestID := chi.URLParam(r, "requestId")
    var decision SettlementApprovalDecision
    json.NewDecoder(r.Body).Decode(&decision)
    
    // Store decision in hitl store; unblock Supervisor
    h.approvalStore.Put(requestID, decision)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "decision_recorded",
        "settlement_id": decision.SettlementID,
    })
}
```

---

## 5. Audit Log (State Transitions & Approval Checkpoints)

Every step of the settlement workflow is logged for compliance and dispute resolution.

```go
type AuditLogEntry struct {
    ID              string            `json:"id"`                 // UUID
    SettlementID    string            `json:"settlement_id"`
    Timestamp       time.Time         `json:"timestamp"`
    Agent           string            `json:"agent"`              // "fetcher"|"calculator"|"router"|"hitl"|"executor"
    Action          string            `json:"action"`             // "fetch_started"|"fetch_completed"|"approved"|"executed"|"rejected"
    Status          string            `json:"status"`             // "pending"|"success"|"error"
    Input           map[string]any    `json:"input,omitempty"`
    Output          map[string]any    `json:"output,omitempty"`
    ApprovedBy      string            `json:"approved_by,omitempty"`
    Reason          string            `json:"reason,omitempty"`
}

type AuditLog struct {
    entries []AuditLogEntry
    mu      sync.RWMutex
}

func (al *AuditLog) Log(entry AuditLogEntry) {
    al.mu.Lock()
    defer al.mu.Unlock()
    entry.ID = uuid.New().String()
    entry.Timestamp = time.Now()
    al.entries = append(al.entries, entry)
}

func (al *AuditLog) GetBySettlementID(settlementID string) []AuditLogEntry {
    al.mu.RLock()
    defer al.mu.RUnlock()
    var result []AuditLogEntry
    for _, e := range al.entries {
        if e.SettlementID == settlementID {
            result = append(result, e)
        }
    }
    return result
}
```

---

## 6. End-to-End Test Scenario (T+0 Settlement for 3 Counterparties)

### Test Setup

```go
func TestSettlementCoordinator_T0_ThreeCounterparties(t *testing.T) {
    ctx := context.Background()
    
    // Create test transactions: 3 counterparties, 2 currencies, net debit/credit
    txns := []finance.Transaction{
        {
            TransactionID: "TXN001",
            Date: "2026-05-31",
            AmountCents: 500_000,   // ₹5,000 credit
            Currency: "INR",
            Direction: finance.DirectionCredit,
            Counterparty: "CITIUS",
            SettlementStatus: "pending",
        },
        {
            TransactionID: "TXN002",
            Date: "2026-05-31",
            AmountCents: 200_000,   // ₹2,000 debit
            Currency: "INR",
            Direction: finance.DirectionDebit,
            Counterparty: "CITIUS",
            SettlementStatus: "pending",
        },
        // HSBC: net credit of ₹3,000
        {
            TransactionID: "TXN003",
            Date: "2026-05-31",
            AmountCents: 400_000,
            Currency: "INR",
            Direction: finance.DirectionCredit,
            Counterparty: "HSBCINDIA",
            SettlementStatus: "pending",
        },
        {
            TransactionID: "TXN004",
            Date: "2026-05-31",
            AmountCents: 100_000,
            Currency: "INR",
            Direction: finance.DirectionDebit,
            Counterparty: "HSBCINDIA",
            SettlementStatus: "pending",
        },
        // Axis: net debit of ₹1,500
        {
            TransactionID: "TXN005",
            Date: "2026-05-31",
            AmountCents: 200_000,
            Currency: "INR",
            Direction: finance.DirectionDebit,
            Counterparty: "AXISBANK",
            SettlementStatus: "pending",
        },
        {
            TransactionID: "TXN006",
            Date: "2026-05-31",
            AmountCents: 50_000,
            Currency: "INR",
            Direction: finance.DirectionCredit,
            Counterparty: "AXISBANK",
            SettlementStatus: "pending",
        },
    }
    
    // Initialize Coordinator
    coord := NewSettlementCoordinator()
    
    // Run settlement workflow
    req := SettlementRequest{
        ID: "SETL001",
        StartDate: "2026-05-31",
        EndDate: "2026-05-31",
        Counterparties: []string{"CITIUS", "HSBCINDIA", "AXISBANK"},
        RequestedBy: "treasury_user_1",
    }
    
    state, err := coord.Run(ctx, req)
    require.NoError(t, err)
    
    // Verify state transitions
    require.Equal(t, state.Status, "pending")
    require.Equal(t, len(state.Transactions), 6)
    
    // Simulate Fetcher step
    state.Status = "fetched"
    require.Equal(t, len(state.Transactions), 6)
    
    // Simulate Calculator step
    netAmounts := coord.Calculate(state.Transactions)
    // Expected: CITIUS +3000, HSBCINDIA +3000, AXISBANK -1500
    require.Equal(t, len(netAmounts), 3)
    require.Equal(t, netAmounts[0].Counterparty, "CITIUS")
    require.Equal(t, netAmounts[0].NetCents, 300_000)  // ₹3,000
    
    state.NetAmounts = netAmounts
    state.Status = "calculated"
    
    // Simulate Router step
    routes := coord.Route(netAmounts)
    // Expected: CITIUS direct (tier-A, <10k), HSBCINDIA correspondent, AXISBANK direct
    require.Equal(t, len(routes), 3)
    require.Equal(t, routes[0].Method, "direct_bank")
    require.Equal(t, routes[1].Method, "correspondent")
    require.Equal(t, routes[2].Method, "direct_bank")
    
    state.Routes = routes
    state.Status = "routed"
    
    // Simulate HITL approval (auto-approve: total ₹4,500, all tier-A/B)
    decision := coord.RequestHITLApproval(ctx, state)
    require.True(t, decision.Approved)
    require.Equal(t, decision.ApprovedBy, "policy_engine")
    
    state.Status = "approved"
    state.ApprovedAt = &decision.ApprovedAt
    state.ApprovedBy = decision.ApprovedBy
    
    // Verify audit log has all steps
    auditEntries := coord.AuditLog.GetBySettlementID(req.ID)
    require.GreaterOrEqual(t, len(auditEntries), 4)  // fetch, calc, route, approval
}
```

### Expected Log Output

```
[2026-05-31T10:15:23Z] SETL001 | Fetcher     | fetch_started      | success | 6 txns in [2026-05-31, 2026-05-31]
[2026-05-31T10:15:24Z] SETL001 | Calculator  | calculate_started  | success | 3 counterparties, 2 currencies
[2026-05-31T10:15:25Z] SETL001 | Router      | route_started      | success | 3 routes selected
[2026-05-31T10:15:26Z] SETL001 | HITL        | approval_requested | success | policy auto-approved
[2026-05-31T10:15:27Z] SETL001 | Executor    | execute_started    | success | 3 payment instructions submitted
```

---

## 7. Idempotency & Re-run Safety

To prevent double-payment if the coordinator re-runs the same settlement:

1. **Idempotency Key**: Every settlement gets a UUID. Transactions are linked to it.
2. **State Persistence**: Store settlement state in a database with status field.
3. **Lookup Before Fetch**: Before Fetcher runs, check if `SettlementID` already exists. If status >= `calculated`, skip Fetcher and use cached transactions.

```go
func (c *SettlementCoordinator) Run(ctx context.Context, req SettlementRequest) (*SettlementState, error) {
    // Idempotency: check if this settlement already started
    existing, err := c.Store.GetByID(ctx, req.ID)
    if err == nil && existing != nil {
        // Settlement in progress or completed; resume from last good state
        if existing.Status == "executed" {
            return existing, fmt.Errorf("settlement already executed; cannot re-run")
        }
        // Resume from last step
        return c.resumeFrom(ctx, existing)
    }
    
    // New settlement: proceed normally
    ...
}
```

---

## 8. File Structure & Registration

### New Packages

```
agents/settlement_coordinator/
├── settlement_coordinator.go          # Supervisor implementation
├── settlement_coordinator_test.go
├── fetcher/
│   ├── fetcher.go                     # Fetcher sub-agent
│   └── fetcher_test.go
├── calculator/
│   ├── calculator.go                  # Calculator sub-agent
│   └── calculator_test.go
└── router/
    ├── router.go                      # Router sub-agent
    └── router_test.go

pkg/settlement/
├── types.go                           # SettlementState, NetAmount, SettlementPath
├── store.go                           # Persistence layer
├── audit.go                           # AuditLog implementation
├── idempotency.go                     # Idempotency check helpers
└── policy.go                          # Settlement approval policy rules
```

### Registry Registration

```go
// In main.go or agent registry initialization:
reg := agenttools.NewRegistry()

// Register sub-agents as tools
reg.Register(fetcher.NewTool(ragIndex))
reg.Register(calculator.NewTool())
reg.Register(router.NewTool(counterpartyService))

// Create Supervisor with sub-agents
coordinator := &agentic.Supervisor{
    Agents: []agentic.SubAgent{
        {
            Name:        "settlement_fetcher",
            Description: "Queries pending transactions by date and counterparty",
            Registry:    fetcher.Registry(),
        },
        {
            Name:        "settlement_calculator",
            Description: "Aggregates transactions into net amounts by counterparty/currency",
            Registry:    calculator.Registry(),
        },
        {
            Name:        "settlement_router",
            Description: "Decides settlement path (direct/correspondent/netting) per counterparty",
            Registry:    router.Registry(),
        },
    },
    Config:       agentic.DefaultConfig(),
}
```

---

## Summary

| Component | Pattern | Key Details |
|-----------|---------|-------------|
| **Supervisor** | `agentic.Supervisor` | Routes to Fetcher → Calculator → Router in sequence |
| **Fetcher** | Sub-agent tool | Queries GraphRAG by date/counterparty; returns Transaction slice |
| **Calculator** | Sub-agent tool | Aggregates by (counterparty, currency, date); returns NetAmount slice |
| **Router** | Sub-agent tool | Applies policy rules; returns SettlementPath per counterparty |
| **GraphRAG** | Schema extension | Add `settlement_status`, `settlement_date`, `counterparty`, `idempotency` fields |
| **HITL** | `hitl.PolicyApprover` | Auto-approve small/low-risk, manual review medium, auto-deny high-risk |
| **Audit** | `AuditLog` struct | Log each step, agent, input/output, approver identity, timestamp |
| **Idempotency** | UUID + state store | Lookup before re-run; prevent double-payment |
| **Test Scenario** | 6 transactions, 3 counterparties | Net amounts: +3k (CITIUS), +3k (HSBC), -1.5k (Axis) |

All patterns follow existing Genie conventions: ToolDef, Registry, Supervisor, HITL, audit logging.
