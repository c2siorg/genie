# Phase 2: Commerce Workspace Specification

**Status**: ✅ Complete  
**Real APIs**: 8 endpoints, fully discovered  
**Real Types**: 6 type definitions  
**Test Coverage**: 18 tests  

---

## Specification

### API Endpoints

#### 1. Create Order
```
POST /v1/commerce/order
Request:
  - merchant_id: string (required)
  - customer_id: string (required)
  - items: OrderItem[] (required, min 1)
  
Response:
  - order_id: string
  - status: "pending" | "paid" | "fulfilled" | "cancelled" | "payment_failed"
  - total_paise: number (₹ in paise units)
  - created_at: unix timestamp
  
Validation:
  - merchant_id must exist in merchant registry
  - customer_id must be valid UUID
  - total_paise must match sum of items
```

#### 2. Get Order
```
GET /v1/commerce/order/{order_id}
Response:
  - order_id, merchant_id, customer_id, status
  - items: OrderItem[] with SKU, description, quantity, unit_price_paise
  - total_paise, workflow_status, created_at
```

#### 3. Execute Order
```
POST /v1/commerce/order/{order_id}/execute
Request:
  - action: "pay" | "settle" | "fulfill"
Response:
  - order_id, status, workflow_status, timestamp
  
State Machine:
  created → payment_initiated → payment_confirmed → settlement → fulfilled
```

#### 4. Get Order Audit Trail
```
GET /v1/commerce/order/{order_id}/audit
Response:
  - entries: AuditEntry[]
    - step: string (e.g., "order_created", "payment_initiated")
    - timestamp: RFC3339
    - error?: string
```

#### 5. Initiate Payment
```
POST /v1/payment/initiate
Request:
  - order_id: string
  - from_account: string
  - to_account: string
  - amount_paise: number
Response:
  - payment_id: string
  - status: "pending" | "confirmed" | "failed"
  - trace_id: string
```

#### 6. Get Payment Status
```
GET /v1/payment/{payment_id}
Response:
  - payment_id, order_id, status, amount_paise
  - from_account, to_account, trace_id
```

#### 7. Request Settlement
```
POST /v1/settlement/request
Request:
  - order_ids: string[]
  - merchant_id: string
Response:
  - settlement_id: string
  - total_paise: number
  - status: "pending" | "calculated" | "routed" | "executed"
```

#### 8. Get Settlement Status
```
GET /v1/settlement/request/{settlement_id}
Response:
  - settlement_id, merchant_id, total_paise
  - status, orders_included: string[]
```

---

### Types

#### Order
```typescript
interface Order {
  order_id: string;
  merchant_id: string;
  customer_id: string;
  items: OrderItem[];
  total_paise: number;
  status: "pending" | "paid" | "fulfilled" | "cancelled" | "payment_failed";
  workflow_status: string;
  created_at: number;
}
```

#### OrderItem
```typescript
interface OrderItem {
  sku: string;
  description: string;
  quantity: number;
  unit_price_paise: number;
}
```

#### Payment
```typescript
interface Payment {
  payment_id: string;
  order_id: string;
  from_account: string;
  to_account: string;
  amount_paise: number;
  status: "pending" | "confirmed" | "failed";
  trace_id: string;
}
```

#### Settlement
```typescript
interface Settlement {
  settlement_id: string;
  merchant_id: string;
  orders: string[];
  total_paise: number;
  status: "pending" | "calculated" | "routed" | "executed";
}
```

#### AuditEntry
```typescript
interface AuditEntry {
  step: string;
  timestamp: string; // RFC3339
  error?: string;
}
```

#### CreateOrderRequest
```typescript
interface CreateOrderRequest {
  merchant_id: string;
  customer_id: string;
  items: OrderItem[];
}
```

---

### Test Requirements

**Unit Tests** (12 tests):
- [ ] Create order with valid items
- [ ] Create order rejects invalid merchant_id
- [ ] Create order calculates total_paise correctly
- [ ] Get order returns full details
- [ ] Get order returns 404 for missing order
- [ ] Payment initiation creates payment_id
- [ ] Payment status updates correctly
- [ ] Settlement consolidation groups by merchant
- [ ] Settlement netting applies correctly
- [ ] Audit trail records all state transitions
- [ ] Order status reflects workflow_status
- [ ] Concurrent modifications don't corrupt state

**Integration Tests** (6 tests):
- [ ] Full workflow: Create → Pay → Settle → Fulfill
- [ ] Multiple orders settle together
- [ ] Netting calculation matches ledger
- [ ] Lineage recorded for all transitions
- [ ] Compliance check integrated before payment
- [ ] Settlement reconciliation passes

---

### Validation Rules

**Amount Validation**:
- Order total_paise = sum of (quantity × unit_price_paise)
- Settlement total_paise = sum of order totals
- No negative amounts

**State Transitions**:
- Only valid: created → payment_initiated → payment_confirmed → settlement → fulfilled
- No skipped steps
- No reverse transitions

**Merchant/Customer**:
- Merchant must be onboarded (KYC verified)
- Customer must have valid account
- Customer velocity limits enforced

**Audit Trail**:
- Every state transition logged
- Timestamp and error (if any) recorded
- Hash-chained for integrity

---

### Success Criteria

✅ All 8 endpoints respond with correct status codes  
✅ All 6 types match TypeScript definitions  
✅ All 12 unit tests pass  
✅ All 6 integration tests pass  
✅ Order total matches settlement total  
✅ Lineage recorded for 100% of transitions  
✅ Zero silent failures (all errors logged)  

---

### Related Specifications

- **Compliance** (Phase 3): Payment compliance check integration
- **Governance** (Phase 4): Incident tracking for settlement failures
- **Evaluation** (Phase 5): Settlement amount hallucination detection
- **Assistant** (Phase 6): Financial context derived from order data
