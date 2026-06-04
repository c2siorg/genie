# Workflow State Terminology Reference

**Purpose**: Clarify the distinction between OrderStatus (visible state) and WorkflowStep (internal audit events) throughout the Genie codebase.

---

## Overview

Genie tracks e-Rupee commerce workflows with two complementary state systems:

| Concept | Scope | Purpose | Values |
|---------|-------|---------|--------|
| **OrderStatus** | Customer-visible | What state is the order in? | `pending`, `paid`, `fulfilled`, `payment_failed`, `cancelled` |
| **WorkflowStep** | Internal audit | What events happened? | `OrderCreated`, `PaymentInitiated`, `PaymentConfirmed`, `ComplianceCheckTriggered`, `ComplianceDecision`, `SettlementInitiated`, `SettlementCompleted`, `Fulfilled` |

---

## OrderStatus Values

**Definition**: The public-facing state machine showing order progression to the customer.

### States

- **`pending`**
  - Initial state when order is created
  - Payment not yet initiated
  - Duration: Order creation → Payment initiation
  - Example: Customer places order, awaiting payment processing

- **`paid`**
  - Payment confirmed AND compliance check passed
  - Funds reserved in merchant's settlement account (via CBDC ledger)
  - Duration: Payment confirmed → Order fulfillment
  - Example: Payment processed successfully, order ready to ship

- **`fulfilled`**
  - Order completed (shipped/delivered)
  - Settlement finalized and audit trail closed
  - Duration: End state
  - Example: Goods delivered, order closed

- **`payment_failed`**
  - Payment rejected (insufficient funds, compliance block, timeout)
  - Order cannot proceed
  - Duration: End state (error)
  - Example: Velocity limit exceeded, payment rejected

- **`cancelled`**
  - Customer or merchant cancelled before payment complete
  - No funds moved, settlement account unmarked
  - Duration: End state (cancelled)
  - Example: Customer requested refund before payment confirmed

### Transitions

```
pending ──[Payment OK + Compliance OK]──> paid ──[Fulfillment]──> fulfilled
    │
    ├──[Payment Failed]──────────────────> payment_failed
    │
    └──[Customer Cancels]────────────────> cancelled
```

---

## WorkflowStep Events

**Definition**: Internal audit trail events capturing detailed workflow execution for compliance and debugging.

### Event Types

Each WorkflowStep represents a discrete action taken during order processing:

| Event | When | Data Captured |
|-------|------|---------------|
| `OrderCreated` | Order created via API | order_id, merchant_id, customer_id, amount, items[] |
| `PaymentInitiated` | Payment request sent to payment agent | payment_id, from_account, to_account, amount_paise, timestamp |
| `PaymentConfirmed` | Payment agent confirms successful debit/credit | payment_id, transaction_hash, ledger_block_height |
| `ComplianceCheckTriggered` | Compliance check launched (AML, KYC, velocity) | check_type, rules_applied[], customer_velocity, limits[] |
| `ComplianceDecision` | Compliance check result (approved/blocked) | decision (approved\|blocked), reason, hold_reason (if blocked) |
| `SettlementInitiated` | Order batch consolidated for settlement | batch_id, order_count, consolidated_amount, netting_applied |
| `SettlementCompleted` | Settlement committed to CBDC ledger | settlement_id, net_amount, cbdc_block_height, merkle_root |
| `Fulfilled` | Order marked complete | fulfillment_timestamp, final_audit_trail_hash |

---

## Key Distinctions

### OrderStatus is NOT the same as WorkflowStep

❌ **WRONG**: "Order status is PaymentInitiated"
- `PaymentInitiated` is a WorkflowStep event, not an OrderStatus

✅ **CORRECT**: "Order status is pending while PaymentInitiated event is being processed"
- OrderStatus = `pending` (customer view)
- WorkflowStep = `PaymentInitiated` (internal event)

### Multiple WorkflowSteps map to one OrderStatus

Example for order with status `paid`:

```
Order Status: paid
├─ WorkflowStep: OrderCreated (t=0)
├─ WorkflowStep: PaymentInitiated (t=5s)
├─ WorkflowStep: PaymentConfirmed (t=10s)
├─ WorkflowStep: ComplianceCheckTriggered (t=11s)
└─ WorkflowStep: ComplianceDecision (t=15s, approved)
   → OrderStatus transitions to: paid
```

### WorkflowStep is the source of truth for audit

The audit trail (lineage) captures all WorkflowStep events with:
- Timestamp
- Event type
- Associated data
- Hash-chain link to previous event

This enables compliance verification: "Prove all compliance checks passed before payment was confirmed"

---

## Code Locations

### OrderStatus Definition

```go
// pkg/commerce/types.go
type OrderStatus string

const (
    OrderStatusPending       OrderStatus = "pending"
    OrderStatusPaid          OrderStatus = "paid"
    OrderStatusFulfilled     OrderStatus = "fulfilled"
    OrderStatusPaymentFailed OrderStatus = "payment_failed"
    OrderStatusCancelled     OrderStatus = "cancelled"
)
```

### WorkflowStep Definition

```go
// pkg/lineage/types.go (or similar)
type WorkflowStep struct {
    EventType   string        // "OrderCreated", "PaymentInitiated", etc.
    OrderID     string
    Timestamp   time.Time
    Data        map[string]interface{}  // Event-specific details
    PrevHash    string        // For hash-chain integrity
}
```

### Order Model

```go
// pkg/commerce/order.go
type Order struct {
    ID              string
    Status          OrderStatus      // What the customer sees
    WorkflowSteps   []WorkflowStep   // Full audit trail
    CreatedAt       time.Time
    // ... other fields
}
```

---

## Deployment Guide Corrections

When documenting workflows, use this terminology:

### ✅ CORRECT PHRASING

- "Order status transitions from `pending` to `paid` after payment confirmation"
- "The audit trail captures a `ComplianceCheckTriggered` event when the compliance agent runs"
- "WorkflowSteps include: OrderCreated → PaymentInitiated → PaymentConfirmed → ComplianceCheckTriggered"
- "Verify payment succeeded by checking for `PaymentConfirmed` in the WorkflowSteps array"

### ❌ WRONG PHRASING

- "Order status is PaymentInitiated" (use: status is `pending` while `PaymentInitiated` event occurs)
- "Order moved through states: pending → payment_initiated → paid" (use: status pending → paid; events: OrderCreated → PaymentInitiated → PaymentConfirmed)
- "Check the PaymentInitiated status field" (use: check WorkflowSteps for `PaymentInitiated` event)

---

## API Response Examples

### GET /v1/commerce/order/{order_id}

```json
{
  "id": "order_abc123",
  "status": "paid",           ← OrderStatus (what customer sees)
  "amount": 100000,
  "workflow_steps": [         ← WorkflowStep events (audit trail)
    {
      "event_type": "OrderCreated",
      "timestamp": "2026-06-01T10:00:00Z",
      "data": { "merchant_id": "merchant_xyz", "items": [...] }
    },
    {
      "event_type": "PaymentInitiated",
      "timestamp": "2026-06-01T10:00:05Z",
      "data": { "payment_id": "pay_123", "amount_paise": 10000000 }
    },
    {
      "event_type": "PaymentConfirmed",
      "timestamp": "2026-06-01T10:00:10Z",
      "data": { "ledger_block_height": 42, "txn_hash": "0x..." }
    },
    {
      "event_type": "ComplianceCheckTriggered",
      "timestamp": "2026-06-01T10:00:11Z",
      "data": { "check_type": "velocity", "customer_id": "cust_001" }
    },
    {
      "event_type": "ComplianceDecision",
      "timestamp": "2026-06-01T10:00:15Z",
      "data": { "decision": "approved", "reason": "within velocity limits" }
    }
  ]
}
```

---

## Testing

### Unit Test Example

```go
func TestOrderStatus_WorkflowStepSequence(t *testing.T) {
    // Arrange
    order := &commerce.Order{
        ID:     "order_test1",
        Status: commerce.OrderStatusPending,
    }
    
    // Act: Simulate payment confirmation
    order.Status = commerce.OrderStatusPaid
    order.WorkflowSteps = append(order.WorkflowSteps,
        lineage.WorkflowStep{
            EventType: "PaymentConfirmed",
            Timestamp: time.Now(),
        },
    )
    
    // Assert
    if order.Status != commerce.OrderStatusPaid {
        t.Errorf("expected status paid, got %v", order.Status)
    }
    if len(order.WorkflowSteps) == 0 {
        t.Error("expected WorkflowSteps to be recorded")
    }
}
```

### Integration Test Example

```go
func TestE2E_OrderStatus_RefectsWorkflowSteps(t *testing.T) {
    // Create order (status: pending)
    order := createOrder(t, orderData)
    if order.Status != OrderStatusPending {
        t.Fatalf("initial status should be pending")
    }
    
    // Execute workflow (should add WorkflowSteps and update status)
    executeWorkflow(t, order.ID)
    
    // Verify status is now paid AND WorkflowSteps captured events
    updated := getOrder(t, order.ID)
    if updated.Status != OrderStatusPaid {
        t.Errorf("status should be paid after successful workflow")
    }
    if !hasWorkflowStep(updated, "PaymentConfirmed") {
        t.Error("WorkflowSteps should contain PaymentConfirmed event")
    }
}
```

---

## Summary

**Remember**:
1. **OrderStatus** = Customer-facing state (`pending`, `paid`, `fulfilled`, etc.)
2. **WorkflowStep** = Internal audit events (`OrderCreated`, `PaymentInitiated`, etc.)
3. Multiple WorkflowSteps can occur while OrderStatus is unchanged
4. Use the terminology correctly in API docs, deployment guides, and code comments
5. Audit trail queries should look at WorkflowStep events, not OrderStatus values

---

**Created**: June 1, 2026  
**Applies to**: Phase 1 (Completed), Phase 2 (Planned)  
**Updated by**: Documentation Audit
