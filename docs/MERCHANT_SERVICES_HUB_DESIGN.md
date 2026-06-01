# Merchant Services Hub Design Document

**Status:** Design Phase (Ready for Implementation)  
**Version:** 1.0  
**Last Updated:** 2026-05-31

---

## Table of Contents

1. [Overview](#overview)
2. [Multi-Agent Workflow Architecture](#multi-agent-workflow-architecture)
3. [Core Components](#core-components)
4. [Data Models](#data-models)
5. [Integration Architecture](#integration-architecture)
6. [Workflow Orchestration](#workflow-orchestration)
7. [Test Scenarios](#test-scenarios)
8. [Implementation Roadmap](#implementation-roadmap)

---

## Overview

The **Merchant Services Hub** is a multi-agent platform that orchestrates the complete lifecycle of merchant payment settlements, reconciliation, and dispute resolution. It extends Genie's existing settlement infrastructure with three specialized agents operating under a supervisor:

1. **Settlement Coordinator** — executes settlement instructions and tracks execution
2. **Reconciliation Agent** — matches expected vs. actual bank confirmations
3. **Dispute/Chargeback Agent** — handles chargebacks, collects evidence, routes to HITL

The system is designed for **independent callability** (each agent can be invoked directly) and **supervisory orchestration** (the MerchantServicesHubSupervisor coordinates multi-step flows, maintains session state, and escalates to human analysts when needed).

### Key Principles

- **Separation of Concerns:** Each agent owns one phase of the settlement lifecycle
- **Event-Driven:** Agents communicate via messages, enabling loose coupling
- **HITL-First:** High-risk decisions (chargebacks, fraud flags, policy violations) route to human review
- **Audit Trail:** All decisions carry lineage metadata (who decided, when, why)
- **Idempotency:** Settlement operations are keyed to prevent duplicate submissions
- **Extensibility:** Settlement providers (bank APIs, correspondent banks) are pluggable

---

## Multi-Agent Workflow Architecture

### Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Merchant Settlement Lifecycle                     │
└─────────────────────────────────────────────────────────────────────┘

    SettlementRequest
    (from merchant, payment processor, or intra-day reconciliation)
            │
            ▼
    ┌──────────────────────────────────────────────────┐
    │  Settlement Coordinator Agent                     │
    │  - Validates settlement request                   │
    │  - Routes to settlement provider (bank/PSP)      │
    │  - Records execution_id + timestamp              │
    │  - Returns ExecutedSettlement                     │
    └──────────────────────────────────────────────────┘
            │
            ▼
         LEDGER: pending_settlement → settled
            │
            ▼
    ┌──────────────────────────────────────────────────┐
    │  Reconciliation Agent                            │
    │  - Waits for bank confirmation (polling/webhook) │
    │  - Matches expected vs. actual amounts            │
    │  - Detects timing mismatches, underpay, overpay  │
    │  - Returns ReconciliationResult                   │
    └──────────────────────────────────────────────────┘
            │
         ┌──┴──────────────────────────┬─────────────────┐
         │ Matched / OK                 │ Mismatch Found  │
         ▼                              ▼                 │
    LEDGER: reconciled         Escalation Queue           │
         │                                │               │
         │                                ▼               │
         │                     (HITL Review / Auto-flag)  │
         │                                │               │
         ▼                                ▼               │
    ┌──────────────────────────────────────────────────┐
    │  Dispute/Chargeback Agent                        │
    │  - Monitors incoming chargebacks (from card nets)│
    │  - Collects evidence (tx receipt, settlement proof)
    │  - Evaluates: is chargeback legitimate? (customer fraud vs. merchant error)
    │  - Routes to HITL for merchant response + decision
    │  - Escalates to card networks if needed           │
    │  - Returns DisputeResolution                      │
    └──────────────────────────────────────────────────┘
            │
            ▼
    LEDGER: disputes → resolved
```

### Workflow States

| State | Agent | Trigger | Output | Next State |
|-------|-------|---------|--------|-----------|
| `pending_settlement` | Settlement Coordinator | SettlementRequest received | ExecutedSettlement | `settled` |
| `settled` | Reconciliation Agent | Bank confirmation received | ReconciliationResult | `reconciled` OR `needs_investigation` |
| `reconciled` | — | Match confirmed, no action | — | `completed` |
| `needs_investigation` | HITL Handler | Mismatch detected | ManualReviewRequest | `escalated` |
| `escalated` | Dispute Agent | High-risk or chargeback | DisputeEvidenceRequest | `dispute_open` |
| `dispute_open` | HITL Merchant Handler | Chargeback received | MerchantResponseRequest | `dispute_in_response` |
| `dispute_in_response` | Dispute Agent | Merchant provided evidence | DisputeResolution | `dispute_resolved` |
| `dispute_resolved` | — | Final decision made | Resolution record | `completed` |

---

## Core Components

### 1. Settlement Coordinator Agent

**Responsibility:** Accept a settlement request, route it to the settlement provider, and record execution details.

```go
// agents/merchant/settlement_coordinator.go (skeleton)

package merchant

import (
    "context"
    "time"
    "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

// SettlementCoordinatorAgent executes settlements and tracks execution state.
type SettlementCoordinatorAgent struct {
    SettlementProvider SettlementProvider // pluggable: bank API, PSP, correspondent
    Store              SettlementStore    // persists ExecutedSettlement records
    Auditor            AuditLogger        // lineage tracking
}

// ID returns "merchant_settlement_coordinator"
func (a *SettlementCoordinatorAgent) ID() string { }

// Name returns human-friendly name
func (a *SettlementCoordinatorAgent) Name() string { }

// Capabilities returns ["execute_settlement"]
func (a *SettlementCoordinatorAgent) Capabilities() []string { }

// HandleMessage receives:
//   - msg.Type == "settlement_request"
//   - msg.Content: JSON(SettlementRequest)
// Returns:
//   - msg.Type == "settlement_executed"
//   - msg.Content: JSON(ExecutedSettlement)
func (a *SettlementCoordinatorAgent) HandleMessage(
    ctx context.Context,
    msg agent.Message,
    env agent.Environment,
) ([]agent.Message, error) {
    // 1. Parse SettlementRequest
    // 2. Validate: amount > 0, merchant exists, idempotency_key unique
    // 3. Call SettlementProvider.Execute(ctx, request)
    // 4. Record ExecutedSettlement with execution_id + timestamp
    // 5. Emit settlement_executed message
    // 6. Log to audit trail: who called, what settled, when
}

// Execute is the internal method that does the settlement routing.
// Separated from HandleMessage for testability.
func (a *SettlementCoordinatorAgent) Execute(
    ctx context.Context,
    req SettlementRequest,
) (*ExecutedSettlement, error) {
    // Provider-specific logic (could be bank API, UPI, IMPS, NEFT, etc.)
}
```

**Key Behaviors:**

- **Idempotency:** If the same `idempotency_key` is received twice, return the cached `execution_id` instead of executing again
- **Async Provider Calls:** Settlement provider calls may be async; use a polling mechanism or webhook to confirm execution
- **Risk Assessment:** High-amount settlements (>₹100k) should be flagged for HITL approval before submission
- **Audit Logging:** Every settlement execution is recorded with context (merchant_id, amount, provider, timestamp, who triggered)

---

### 2. Reconciliation Agent

**Responsibility:** Match expected settlements against actual bank confirmations, detect discrepancies.

```go
// agents/merchant/reconciliation_agent.go (skeleton)

package merchant

import (
    "context"
    "time"
)

// ReconciliationAgent matches expected vs. actual settlement amounts.
type ReconciliationAgent struct {
    Store          SettlementStore
    BankDataSource BankConfirmationSource // webhook or polling integration
    Auditor        AuditLogger
}

// HandleMessage receives:
//   - msg.Type == "bank_confirmation_received"
//   - msg.Content: JSON(BankConfirmation)
// Returns:
//   - msg.Type == "reconciliation_result"
//   - msg.Content: JSON(ReconciliationResult)
func (a *ReconciliationAgent) HandleMessage(
    ctx context.Context,
    msg agent.Message,
    env agent.Environment,
) ([]agent.Message, error) {
    // 1. Parse BankConfirmation
    // 2. Fetch corresponding ExecutedSettlement by settlement_id
    // 3. Compare: expected_amount vs actual_amount
    // 4. Verify: expected_settlement_date ~= actual_settlement_date (tolerance: +/- 2 business days)
    // 5. Return ReconciliationResult: matched | underpaid | overpaid | missing | timing_mismatch
}

// Reconcile is the core matching logic.
func (a *ReconciliationAgent) Reconcile(
    ctx context.Context,
    executed *ExecutedSettlement,
    actual *BankConfirmation,
) *ReconciliationResult {
    // Field-by-field comparison
    // Tolerance: amount within ±100 rupees (configurable)
    // Tolerance: date within ±2 business days
}

// DetectAnomalies identifies patterns that warrant escalation.
func (a *ReconciliationAgent) DetectAnomalies(
    ctx context.Context,
    merchantID string,
    result *ReconciliationResult,
) *AnomalyFlag {
    // Multiple consecutive mismatches
    // Recurring underpayments
    // Timing drift increasing over time
}
```

**Key Behaviors:**

- **Matching Logic:**
  - Amount match: exact or within ±100 rupees (configurable threshold)
  - Date match: expected_settlement_date within ±2 business days of actual_settlement_date
  - Reference matching: settlement_id or transaction reference must match
  
- **Mismatch Categories:**
  - `underpaid`: actual_amount < expected_amount - 100
  - `overpaid`: actual_amount > expected_amount + 100
  - `missing`: no bank confirmation received within SLA (default: 3 business days)
  - `timing_mismatch`: date discrepancy > 2 business days
  - `reference_mismatch`: settlement_id doesn't match bank records

- **Auto-Escalation Triggers:**
  - Any mismatch → escalate to HITL for analyst review
  - 3+ consecutive mismatches for same merchant → account review required
  - Total discrepancy > 10% of daily settlement volume → risk alert

---

### 3. Dispute/Chargeback Agent

**Responsibility:** Receive chargeback notifications, evaluate legitimacy, collect evidence, route to merchant for response.

```go
// agents/merchant/dispute_agent.go (skeleton)

package merchant

import (
    "context"
    "time"
)

// DisputeAgent handles incoming chargebacks and escalations.
type DisputeAgent struct {
    Store          SettlementStore
    CardNetSource  CardNetworkDataSource // Mastercard, Visa, RuPay APIs
    EvidenceStore  EvidenceStore          // stores supporting docs
    Auditor        AuditLogger
}

// HandleMessage receives:
//   - msg.Type == "chargeback_received"
//   - msg.Content: JSON(Chargeback)
// OR
//   - msg.Type == "merchant_response_received"
//   - msg.Content: JSON(MerchantResponse)
// Returns:
//   - msg.Type == "dispute_evidence_request" (asking merchant for docs)
//   - msg.Type == "dispute_resolution" (final decision)
func (a *DisputeAgent) HandleMessage(
    ctx context.Context,
    msg agent.Message,
    env agent.Environment,
) ([]agent.Message, error) {
    // 1. Parse incoming chargeback or merchant response
    // 2. Fetch associated settlement and transaction
    // 3. Evaluate legitimacy (see EvaluateLegitimacy below)
    // 4. If needs merchant input: emit dispute_evidence_request → route to HITL
    // 5. When merchant responds: collect evidence, re-evaluate
    // 6. Emit final dispute_resolution
}

// EvaluateLegitimacy assesses whether a chargeback is likely legitimate.
// Returns: (legitimate bool, confidence float64, reasoning string)
func (a *DisputeAgent) EvaluateLegitimacy(
    ctx context.Context,
    dispute *Chargeback,
    settlement *ExecutedSettlement,
    tx *Transaction,
) (bool, float64, string) {
    // Rules engine:
    // - "Customer initiated fraud": check transaction details against account history
    // - "Unrecognized transaction": check IP geolocation, device fingerprint, merchant category
    // - "Processing error": verify settlement amount matches invoice amount
    // - "Merchant error": check if merchant acknowledged the issue in comms
    //
    // Confidence score: 0.0–1.0 based on evidence strength
}

// CollectEvidence gathers supporting documents for the chargeback response.
func (a *DisputeAgent) CollectEvidence(
    ctx context.Context,
    dispute *Chargeback,
    settlement *ExecutedSettlement,
    tx *Transaction,
) *EvidencePackage {
    // - Original transaction receipt
    // - Settlement confirmation from bank
    // - Merchant communication logs (if any)
    // - Customer interaction history (if merchant has it)
    // - Device fingerprint / geolocation data (if available)
}

// RequestMerchantResponse generates a HITL task for the merchant.
func (a *DisputeAgent) RequestMerchantResponse(
    ctx context.Context,
    dispute *Chargeback,
    evidence *EvidencePackage,
    merchant *Merchant,
) *MerchantDisputeRequest {
    // Emit structured request to HITL system:
    // - Chargeback reason
    // - Evidence collected so far
    // - Deadline for merchant response (typically 7 days)
    // - Expected response format (merchant must provide counterevidence)
}
```

**Key Behaviors:**

- **Chargeback Lifecycle:**
  1. Card network sends chargeback notification (via webhook or polling)
  2. Dispute agent receives and validates
  3. If auto-decisible (clear fraud pattern, or clear merchant error): auto-resolve
  4. Otherwise: emit dispute_evidence_request → HITL → merchant gets 7 days to respond
  5. On merchant response: Dispute agent re-evaluates with new evidence
  6. Emit final dispute_resolution to card network

- **Legitimacy Rules:**
  - `customer_fraud` (70% confidence): IP location mismatches, new device, unrecognized beneficiary
  - `merchant_error` (85% confidence): settlement amount doesn't match invoice; duplicate charge
  - `processing_error` (90% confidence): both merchant and customer agree on the issue
  - `unclear` (0–50% confidence): escalate to merchant for response

- **Evidence Collection Priority:**
  1. Original transaction receipt (must have)
  2. Settlement proof (bank confirmation, ledger entry)
  3. Merchant communication logs (optional)
  4. Customer history (optional)

---

## Data Models

### ExecutedSettlement

Represents a settlement that has been submitted to the provider.

```go
type ExecutedSettlement struct {
    // Immutable identity
    ID              string    `json:"id"`                // UUID, generated by coordinator
    SettlementID    string    `json:"settlement_id"`     // Reference from provider (bank response)
    IdempotencyKey  string    `json:"idempotency_key"`   // Deduplication key
    
    // Source
    MerchantID      string    `json:"merchant_id"`
    SettlementDate  time.Time `json:"settlement_date"`   // When merchant expects funds
    
    // Amount info
    Amount          int64     `json:"amount"`            // in rupees (multiplied by 100)
    Currency        string    `json:"currency"`          // "INR"
    
    // Execution info
    ExecutedAt      time.Time `json:"executed_at"`       // When coordinator submitted
    Provider        string    `json:"provider"`          // "icici_bank" | "hdfc_bank" | "upi" | etc.
    ProviderTxnID   string    `json:"provider_txn_id"`   // Receipt from provider
    
    // Status tracking
    Status          string    `json:"status"`            // "pending" | "settled" | "reconciled" | "escalated"
    
    // Audit
    CreatedBy       string    `json:"created_by"`        // User or system ID that triggered
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
    LineageID       string    `json:"lineage_id"`        // Audit trail reference
}
```

### BankConfirmation

Represents actual bank settlement confirmation received via webhook or polling.

```go
type BankConfirmation struct {
    ID              string    `json:"id"`                // UUID
    SettlementID    string    `json:"settlement_id"`     // Must match ExecutedSettlement.SettlementID
    
    // Actual amounts and dates from bank
    ActualAmount    int64     `json:"actual_amount"`     // What bank actually settled
    ActualDate      time.Time `json:"actual_date"`       // When it actually hit the account
    
    // Bank details
    BankReference   string    `json:"bank_reference"`    // Bank's internal reference
    BankName        string    `json:"bank_name"`
    
    // Status
    ConfirmedAt     time.Time `json:"confirmed_at"`
    Status          string    `json:"status"`            // "confirmed" | "pending" | "failed"
    FailureReason   string    `json:"failure_reason,omitempty"`
}
```

### ReconciliationResult

Outcome of matching ExecutedSettlement against BankConfirmation.

```go
type ReconciliationResult struct {
    ID                   string    `json:"id"`
    ExecutedSettlementID string    `json:"executed_settlement_id"`
    BankConfirmationID   string    `json:"bank_confirmation_id"`
    
    // Matching outcome
    Status               string    `json:"status"`            // "matched" | "underpaid" | "overpaid" | "missing" | "timing_mismatch"
    
    // Discrepancy details
    ExpectedAmount       int64     `json:"expected_amount"`
    ActualAmount         int64     `json:"actual_amount"`
    AmountDifference     int64     `json:"amount_difference"`  // actual - expected
    
    ExpectedDate         time.Time `json:"expected_date"`
    ActualDate           time.Time `json:"actual_date"`
    DateDifference       int       `json:"date_difference"`    // days
    
    // Analysis
    Notes                string    `json:"notes"`              // Human-readable summary
    AnomaliesDetected    []string  `json:"anomalies_detected"` // Pattern flags
    
    MatchedAt            time.Time `json:"matched_at"`
    LineageID            string    `json:"lineage_id"`
}
```

### Chargeback

Incoming chargeback notification from card network.

```go
type Chargeback struct {
    ID                   string    `json:"id"`
    SettlementID         string    `json:"settlement_id"`      // Links to ExecutedSettlement
    MerchantID           string    `json:"merchant_id"`
    
    // Chargeback details
    ChargebackID         string    `json:"chargeback_id"`      // Card network reference
    Reason               string    `json:"reason"`             // "customer_denies" | "unauthorized" | "processing_error" | "fraud" | etc.
    Amount               int64     `json:"amount"`
    Currency             string    `json:"currency"`
    
    // Dates
    ReceivedAt           time.Time `json:"received_at"`        // When we got the notification
    ResponseDeadline     time.Time `json:"response_deadline"`  // When we must respond
    
    // Card network source
    CardNetwork          string    `json:"card_network"`       // "mastercard" | "visa" | "rupay"
    DisputeReference     string    `json:"dispute_reference"`  // Card network's case number
    
    Status               string    `json:"status"`             // "open" | "in_response" | "resolved" | "escalated"
    
    LineageID            string    `json:"lineage_id"`
}
```

### Dispute (Extended Chargeback Record)

Full dispute lifecycle record.

```go
type Dispute struct {
    ID                   string         `json:"id"`
    ChargebackID         string         `json:"chargeback_id"`
    MerchantID           string         `json:"merchant_id"`
    SettlementID         string         `json:"settlement_id"`
    
    // Chargeback metadata
    Reason               string         `json:"reason"`
    Amount               int64          `json:"amount"`
    
    // Analysis phase
    AutoDecisible        bool           `json:"auto_decisible"`      // Can be resolved without merchant input
    LegitimacyScore      float64        `json:"legitimacy_score"`    // 0.0–1.0 confidence
    EvaluatedReasoning   string         `json:"evaluated_reasoning"`
    
    // Evidence collection
    Evidence             *EvidencePackage `json:"evidence,omitempty"`
    
    // Merchant response phase
    MerchantResponse     *MerchantResponse `json:"merchant_response,omitempty"`
    MerchantResponseAt   *time.Time       `json:"merchant_response_at,omitempty"`
    
    // Resolution
    FinalDecision        string         `json:"final_decision"`      // "approved" | "denied" | "escalated"
    FinalReasoning       string         `json:"final_reasoning"`
    ResolvedAt           *time.Time     `json:"resolved_at,omitempty"`
    ResolvedBy           string         `json:"resolved_by,omitempty"` // HITL analyst ID
    
    Status               string         `json:"status"`
    LineageID            string         `json:"lineage_id"`
}

type EvidencePackage struct {
    TransactionReceipt     *Document    `json:"transaction_receipt"`
    SettlementProof        *Document    `json:"settlement_proof"`
    MerchantComms          []Document   `json:"merchant_comms"`      // Emails, notes, etc.
    DeviceFingerprint      *string      `json:"device_fingerprint,omitempty"`
    GeoLocation            *GeoPoint    `json:"geo_location,omitempty"`
}

type MerchantResponse struct {
    DisputeID              string       `json:"dispute_id"`
    MerchantID             string       `json:"merchant_id"`
    ResponseText           string       `json:"response_text"`
    ProvidedEvidence       []Document   `json:"provided_evidence"`
    SubmittedAt            time.Time    `json:"submitted_at"`
}
```

### MerchantDisputePolicy

Configurable rules for auto-escalation and handling.

```go
type MerchantDisputePolicy struct {
    MerchantID                 string  `json:"merchant_id"`
    
    // Auto-accept thresholds (amounts below these are auto-approved)
    AutoAcceptLimitRupees      int64   `json:"auto_accept_limit_rupees"`      // e.g., ₹1000
    
    // Manual review thresholds (amounts requiring analyst review)
    ManualReviewLimitRupees    int64   `json:"manual_review_limit_rupees"`    // e.g., ₹50k
    
    // Escalation triggers
    EscalationThreshold        int     `json:"escalation_threshold"`          // e.g., 3+ disputes in 30d
    EscalationThresholdWindow  string  `json:"escalation_threshold_window"`   // e.g., "30d", "90d"
    
    // Response deadline for merchant
    MerchantResponseDeadlineHours int  `json:"merchant_response_deadline_hours"` // e.g., 168 (7 days)
    
    // Risk flags
    AutoRejectPatterns         []string `json:"auto_reject_patterns"`         // Fraud patterns
    
    // Timestamps
    CreatedAt                  time.Time `json:"created_at"`
    UpdatedAt                  time.Time `json:"updated_at"`
}
```

---

## Integration Architecture

### 1. Settlement Provider Interface

Abstracting different settlement methods (bank APIs, PSPs, correspondent banks).

```go
// pkg/settlement/provider.go (skeletal signature)

package settlement

import "context"

// SettlementProvider is the pluggable interface for different settlement rails.
type SettlementProvider interface {
    // Execute submits a settlement request to the provider.
    // Returns the provider's transaction ID (for tracking).
    Execute(ctx context.Context, req SettlementRequest) (*SettlementExecutionResult, error)
    
    // PollStatus queries the provider for the settlement's current status.
    // Used for async providers.
    PollStatus(ctx context.Context, executionID string) (*SettlementStatus, error)
    
    // Name returns the provider identifier ("icici_bank", "upi", "imps", etc.)
    Name() string
}

type SettlementRequest struct {
    IdempotencyKey   string
    MerchantID       string
    Amount           int64
    Currency         string
    SettlementDate   time.Time
    BeneficiaryInfo  map[string]string // varies by provider
}

type SettlementExecutionResult struct {
    ExecutionID      string    // Provider's reference
    Status           string    // "submitted" | "processing" | "settled" | "failed"
    SubmittedAt      time.Time
    FailureReason    string
}

type SettlementStatus struct {
    ExecutionID      string
    Status           string
    SettledAmount    int64
    SettledAt        time.Time
}
```

### 2. Bank Confirmation Source Interface

Abstracting how confirmations arrive (webhooks, polling, batch files).

```go
// pkg/settlement/bank_source.go (skeletal)

package settlement

// BankConfirmationSource abstracts how we receive bank confirmations.
type BankConfirmationSource interface {
    // Listen starts a long-lived listener for incoming confirmations.
    // Sends confirmations on the returned channel.
    Listen(ctx context.Context) (<-chan BankConfirmation, error)
    
    // FetchRange polls for confirmations in a date range.
    // Used for reconciliation of missed confirmations.
    FetchRange(ctx context.Context, start, end time.Time) ([]BankConfirmation, error)
}
```

### 3. HITL Integration

Routing to human analysts for:
- High-risk settlements (via PolicyApprover)
- Reconciliation mismatches
- Dispute evidence requests
- Merchant response collection

```go
// How merchant_settlement_coordinator uses HITL

import "github.com/c2siorg/genie/pkg/hitl"

// Before submitting a large settlement:
approvalRequest := hitl.ApprovalRequest{
    ID:        uuid.New().String(),
    SessionID: traceID,
    AgentID:   "merchant_settlement_coordinator",
    ToolName:  "submit_settlement",
    Args: map[string]any{
        "merchant_id":   req.MerchantID,
        "amount":        req.Amount,
        "settlement_id": settlement.ID,
    },
    RiskScore: 0.8,  // High amount
    CreatedAt: env.Now(),
    ExpiresAt: env.Now().Add(30 * time.Minute),
}

approved, err := approver.RequestApproval(ctx, approvalRequest)
if !approved {
    // Reject the settlement
    return nil, fmt.Errorf("settlement rejected by human review")
}

// Proceed with execution...
```

### 4. Lineage/Audit Integration

All decisions logged via `pkg/lineage` (or similar audit package).

```go
// Example lineage logging in each agent

import "github.com/c2siorg/genie/pkg/lineage"

// In Settlement Coordinator.Execute():
auditor.LogDecision(ctx, lineage.DecisionRecord{
    TraceID:     traceID,
    EntityID:    settlement.ID,
    EntityType:  "ExecutedSettlement",
    Decision:    "SUBMIT_SETTLEMENT",
    DecisionBy:  "merchant_settlement_coordinator",
    Reason:      fmt.Sprintf("Amount=%.2f, Merchant=%s", amount, merchantID),
    Timestamp:   env.Now(),
    Metadata:    map[string]string{"provider": provider, "idempotency": idempKey},
})
```

---

## Workflow Orchestration

### Supervisor Pattern

The **MerchantServicesHubSupervisor** (similar to `agents/supervisor`) orchestrates multi-step flows.

```go
// agents/merchant/supervisor.go (skeleton)

package merchant

import (
    "context"
    "sync"
    "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type MerchantServicesHubSupervisor struct {
    mu       sync.Mutex
    sessions map[string]*SupervisorSession
}

type SupervisorSession struct {
    TraceID                  string
    MerchantID               string
    InitialSettlementRequest SettlementRequest
    
    // Collected responses from sub-agents
    ExecutedSettlement       *ExecutedSettlement
    ReconciliationResult     *ReconciliationResult
    DisputeResolution        *DisputeResolution
    
    // Session state
    Phase                    string  // "settlement" | "reconciliation" | "dispute"
    CompletedAt              *time.Time
}

func (sup *MerchantServicesHubSupervisor) ID() string     { return "merchant_supervisor" }
func (sup *MerchantServicesHubSupervisor) Name() string   { return "Merchant Services Supervisor" }
func (sup *MerchantServicesHubSupervisor) Capabilities() []string {
    return []string{"orchestrate_merchant_settlement_lifecycle"}
}

// HandleMessage routes incoming messages to sub-agents and aggregates responses.
func (sup *MerchantServicesHubSupervisor) HandleMessage(
    ctx context.Context,
    msg agent.Message,
    env agent.Environment,
) ([]agent.Message, error) {
    traceID, _ := msg.Metadata["trace_id"].(string)
    if traceID == "" {
        traceID = msg.ID
    }
    
    switch msg.Type {
    case "settlement_request":
        // Phase 1: Kick off settlement coordinator
        sup.mu.Lock()
        sup.sessions[traceID] = &SupervisorSession{
            TraceID:    traceID,
            MerchantID: extractMerchantID(msg),
            Phase:      "settlement",
        }
        sup.mu.Unlock()
        
        return []agent.Message{
            agent.NewMessage(sup.ID(), "merchant_settlement_coordinator", agent.RoleAgent,
                "settlement_request", msg.Content, msg.Metadata),
        }, nil
        
    case "settlement_executed":
        // Phase 2: Settlement coordinator returned; save and wait for bank confirmation
        sup.mu.Lock()
        session := sup.sessions[traceID]
        // Parse and store ExecutedSettlement
        session.Phase = "reconciliation"
        sup.mu.Unlock()
        
        // Emit a message to wait for bank confirmation
        // (This could be a scheduled polling task or webhook listener)
        return nil, nil
        
    case "bank_confirmation_received":
        // Route to reconciliation agent
        sup.mu.Lock()
        session := sup.sessions[traceID]
        sup.mu.Unlock()
        
        return []agent.Message{
            agent.NewMessage(sup.ID(), "merchant_reconciliation_agent", agent.RoleAgent,
                "bank_confirmation_received", msg.Content, msg.Metadata),
        }, nil
        
    case "reconciliation_result":
        // Phase 3: Check reconciliation outcome
        sup.mu.Lock()
        session := sup.sessions[traceID]
        // Parse and store ReconciliationResult
        
        // If matched, session is done (no dispute phase)
        // If mismatch, escalate to HITL or dispute agent
        if isMatched(msg) {
            session.CompletedAt = timePtr(env.Now())
            sup.mu.Unlock()
            
            // Emit completion message
            return []agent.Message{
                agent.NewMessage(sup.ID(), "reporter", agent.RoleAgent,
                    "merchant_settlement_completed", marshalJSON(session), msg.Metadata),
            }, nil
        }
        
        // Mismatch detected: escalate
        sup.mu.Unlock()
        return []agent.Message{
            agent.NewMessage(sup.ID(), "hitl_handler", agent.RoleAgent,
                "reconciliation_mismatch_alert", msg.Content, msg.Metadata),
        }, nil
        
    case "chargeback_received":
        // Phase 4: Dispute lifecycle
        sup.mu.Lock()
        session := sup.sessions[traceID]
        session.Phase = "dispute"
        sup.mu.Unlock()
        
        return []agent.Message{
            agent.NewMessage(sup.ID(), "merchant_dispute_agent", agent.RoleAgent,
                "chargeback_received", msg.Content, msg.Metadata),
        }, nil
    }
    
    return nil, nil
}
```

### Independent Agent Callability

While the supervisor orchestrates end-to-end flows, each agent can be called independently:

```go
// Direct invocation example (from CLI, webhook handler, scheduled job, etc.)

// 1. Call Settlement Coordinator directly
coordinator := merchant.NewSettlementCoordinatorAgent(provider, store, auditor)
settledMsg, _ := coordinator.HandleMessage(ctx, settlementMsg, env)

// 2. Call Reconciliation Agent directly (for a previously settled transaction)
reconciler := merchant.NewReconciliationAgent(store, bankSource, auditor)
reconcileMsg, _ := reconciler.HandleMessage(ctx, bankConfMsg, env)

// 3. Call Dispute Agent directly (when chargeback comes in)
disputer := merchant.NewDisputeAgent(store, cardNetSource, evidenceStore, auditor)
disputeMsg, _ := disputer.HandleMessage(ctx, chargebackMsg, env)
```

---

## Test Scenarios

### Scenario 1: Happy Path (Settlement Matches)

**Given:**
- Merchant requests ₹100,000 settlement for 2026-05-31
- Bank confirms ₹100,000 on 2026-06-01

**When:**
1. Settlement Coordinator executes request → ExecutedSettlement recorded
2. Reconciliation Agent receives bank confirmation → compares amounts
3. Amounts match (100,000 == 100,000) and date within tolerance

**Then:**
- ReconciliationResult.Status = "matched"
- Settlement marked as "reconciled"
- No escalation or dispute

---

### Scenario 2: Timing Mismatch (But Acceptable)

**Given:**
- Merchant requests settlement for 2026-05-31 (T+1 expected)
- Bank confirms ₹100,000 on 2026-06-03 (T+3 actual)

**When:**
1. Settlement Coordinator submits on 2026-05-31
2. Reconciliation Agent waits until 2026-06-03 for confirmation
3. Amounts match but settlement date is 2 days late

**Then:**
- ReconciliationResult.Status = "timing_mismatch"
- AnomalyFlag raised (late settlement)
- Escalate to HITL analyst for confirmation this is acceptable
- If analyst approves: mark as "reconciled_with_note"

---

### Scenario 3: Legitimate Chargeback (Customer Denies)

**Given:**
- Transaction: Merchant sold goods, settled ₹50,000
- Customer disputes: "I didn't authorize this transaction"
- Evidence: customer is in different country (IP geolocation mismatch)

**When:**
1. Chargeback received from Mastercard
2. Dispute Agent evaluates: customer in Mumbai, transaction IP in US → fraud pattern
3. LegitimacyScore = 0.75 (likely legitimate fraud)
4. Auto-decisible but not auto-deny (uncertain); escalate to merchant for response

**Then:**
- Emit MerchantDisputeRequest to HITL
- Merchant notified: "Customer claims unauthorized. Provide evidence of authorization (shipping proof, customer comms, etc.)"
- 7-day deadline for response
- If merchant provides shipping proof + delivery confirmation: DisputeResolution = "APPROVED" (merchant wins)
- If merchant has no evidence: DisputeResolution = "DENIED" (customer wins, chargeback approved)

---

### Scenario 4: Merchant Fraud (Underpayment)

**Given:**
- Settlement request: ₹100,000
- Bank confirmation: ₹95,000 (actual settled)
- Merchant invoice: ₹100,000

**When:**
1. Settlement Coordinator executes with expected amount ₹100,000
2. Bank confirms ₹95,000 received
3. Reconciliation Agent detects: actual < expected by ₹5,000

**Then:**
- ReconciliationResult.Status = "underpaid"
- AnomalyFlag: "Consistent underpayment pattern detected"
- Escalate to HITL analyst
- Analyst investigates: is this a bank issue, or is the merchant taking a cut?
- Decision: contact bank for reversal, or flag merchant account for review

---

### Scenario 5: Multiple Chargebacks Pattern Detection

**Given:**
- Merchant M1 receives 3 chargebacks within 30 days
- Chargeback reasons: "customer fraud" (1), "processing error" (1), "unauthorized" (1)

**When:**
1. Dispute Agent receives 3rd chargeback
2. Detects pattern: MerchantDisputePolicy.EscalationThreshold = 3 in 30 days triggered
3. Triggers account review escalation

**Then:**
- Emit "account_review_required" message to compliance/HITL
- Flag merchant account as "high_risk"
- All future settlements for this merchant require manual approval
- HITL analyst reviews merchant's history, KYC, transaction patterns
- Decision: continue with enhanced monitoring, or suspend merchant account

---

## Implementation Roadmap

### Phase 1: Foundation (Week 1–2)

- [ ] Create data models in `pkg/merchant/types.go`
  - ExecutedSettlement, BankConfirmation, ReconciliationResult, Chargeback, Dispute, MerchantDisputePolicy

- [ ] Create settlement provider interface in `pkg/settlement/provider.go`
  - Abstract settlement execution and status polling

- [ ] Create settlement store interface in `pkg/merchant/store.go`
  - Persist and query ExecutedSettlement, BankConfirmation, ReconciliationResult records

- [ ] Create HITL policy and routes in `pkg/merchant/policy.go`
  - Auto-acceptance rules, manual review thresholds, escalation triggers

### Phase 2: Agents (Week 2–3)

- [ ] Implement Settlement Coordinator Agent (`agents/merchant/settlement_coordinator.go`)
  - Parse SettlementRequest
  - Call SettlementProvider.Execute()
  - Record ExecutedSettlement with execution_id
  - Emit settlement_executed message
  - Tests: happy path, duplicate idempotency key, provider failure

- [ ] Implement Reconciliation Agent (`agents/merchant/reconciliation_agent.go`)
  - Receive BankConfirmation
  - Match against ExecutedSettlement
  - Detect amount/date mismatches
  - Emit reconciliation_result
  - Tests: exact match, underpay, overpay, missing, timing mismatch

- [ ] Implement Dispute Agent (`agents/merchant/dispute_agent.go`)
  - Receive chargeback notification
  - Evaluate legitimacy score
  - Collect evidence package
  - Emit dispute_evidence_request or dispute_resolution
  - Tests: auto-resolvable fraud, merchant response needed, pattern escalation

### Phase 3: Orchestration (Week 3–4)

- [ ] Implement Supervisor (`agents/merchant/supervisor.go`)
  - Multi-phase session management (settlement → reconciliation → dispute)
  - Route messages to sub-agents
  - Aggregate responses
  - Finalize settlements

- [ ] Integration with HITL (`pkg/hitl/` + merchant handlers)
  - Approval flows for high-risk settlements
  - Merchant response collection for disputes
  - Analyst decision routing

- [ ] Integration with Lineage/Audit
  - Log all decisions with context (who, what, when, why)

### Phase 4: Testing & Documentation (Week 4)

- [ ] Write integration tests for complete flows
  - Happy path: settlement → reconciliation → completion
  - Chargeback flow: received → escalated → merchant response → resolved
  - Pattern detection: multiple disputes → account review

- [ ] Create runbook for operators
  - How to manually reconcile a mismatch
  - How to override an auto-decision
  - How to escalate a dispute to card network

- [ ] Performance & load testing
  - Settlement throughput (target: 1000 settlements/min)
  - Reconciliation latency (target: <1s per confirmation)

---

## Summary

The **Merchant Services Hub** is a three-layer agent system:

1. **Settlement Coordinator** — submits, executes, records
2. **Reconciliation Agent** — matches, detects anomalies, escalates on mismatch
3. **Dispute Agent** — evaluates legitimacy, collects evidence, routes to HITL

Orchestrated by a **Supervisor** that maintains session state and routes messages between agents, with **HITL integration** for high-risk decisions and human oversight.

**Key Design Wins:**
- **Independent callability:** Each agent can be invoked directly or via supervisor
- **Extensibility:** Settlement providers and bank sources are pluggable interfaces
- **Audit trail:** All decisions are logged with full lineage context
- **Pattern detection:** Anomaly flags trigger automatic account reviews
- **Merchant-first:** Dispute evidence requests are routed to merchant with clear deadlines
- **Risk-based routing:** HITL approval gates applied proportionally to transaction risk

This architecture aligns with FREE-AI recommendations (Rec 16: human oversight, Rec 18: disclosure, Rec 22: audit trail) and provides a production-ready foundation for merchant settlement lifecycle management.

---

**Next Steps:**
1. Review this design with stakeholders (compliance, operations, engineering)
2. Finalize provider adapters (which settlement rails first?)
3. Start Phase 1 implementation (data models + interfaces)
4. Parallel: design HITL forms for merchant dispute responses
