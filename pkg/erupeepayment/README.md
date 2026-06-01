# e-Rupee Payment Agent

A robust payment agent for e-Rupee (CBDC) fund transfers with core account management and transaction tracking. Integrates with the CBDC Bridge for ledger commitment and provides laminar tracing for audit trails.

## Architecture

```
PaymentAgent (agent.go)
  ├─ AccountManager interface
  │  └─ InMemoryAccountManager (thread-safe, map-based)
  │     ├─ GetAccount(id)
  │     ├─ CreateAccount(holderID, type)
  │     ├─ UpdateBalance(id, delta)
  │     └─ UpdateStatus(id, status)
  │
  ├─ TransactionLog interface
  │  └─ InMemoryTransactionLog (append-only, queryable, immutable)
  │     ├─ Record(txn)
  │     ├─ Query(accountID, since, until)
  │     ├─ GetStatus(paymentID)
  │     └─ GetByPaymentID(paymentID)
  │
  └─ CBDCBridge interface (pluggable)
     └─ CommitLedger(ctx, from, to, amount, paymentID)
```

## Core Types

### PaymentStatus
Represents the lifecycle of a payment:
- **pending**: Initiated, awaiting ledger commitment
- **confirming**: Submitted to CBDC Bridge, awaiting confirmation
- **confirmed**: Ledger committed, payment final
- **failed**: Payment rejected (insufficient balance, invalid account, etc.)
- **reversed**: Payment reversed after confirmation (refund scenario)

### Account
Represents an e-Rupee wallet:
- **ID**: Unique account identifier
- **HolderID**: Links to account holder (person or merchant)
- **BalancePaise**: Current balance (in paise, not rupees, to avoid floating-point issues)
- **Type**: personal or merchant
- **Status**: active, frozen (AML hold), or closed
- **Timestamps**: CreatedAt, UpdatedAt

### TransactionRecord
Immutable ledger entry for audit trails:
- One PaymentRequest may generate two TransactionRecords (debit + credit)
- Includes LedgerID from CBDC Bridge for reconciliation
- Linked to originating PaymentID for traceability

## Key Features

### 1. Thread-Safe Account Management
- `InMemoryAccountManager` uses RWMutex for concurrent access
- All returned accounts are shallow copies to prevent external mutations
- Balance updates are atomic; rejects negative balances
- Supports status transitions (active → frozen → closed)

### 2. Immutable Transaction Log
- `InMemoryTransactionLog` is append-only
- All returned records are copies to prevent mutations
- Status cache maintains most recent state per payment ID
- Queryable by account ID and time range

### 3. Payment Validation Pipeline
Comprehensive 7-point validation:
1. From and To accounts must be provided and differ
2. Amount must be positive and within maximum (₹10L)
3. Sender account must exist and be active
4. Recipient account must exist and be active
5. Sender must have sufficient balance
6. Idempotency check (same payment ID = no duplicate charge)
7. Transaction record persistence

### 4. CBDC Bridge Integration
- Pluggable `CBDCBridge` interface for ledger backends
- Async commitment: payment returns pending status immediately
- Automatic balance updates on ledger confirmation
- Failure handling records transaction status as failed

### 5. Laminar Tracing
- Correlation ID per payment (paymentID + nanosecond timestamp)
- Agent logs include payment ID, status, accounts, and amounts
- Transaction records link to payment ID for audit trails

## Usage Example

```go
// Setup
accountMgr := NewInMemoryAccountManager(nil)
txnLog := NewInMemoryTransactionLog(nil)
paymentAgent := NewPaymentAgent(accountMgr, txnLog)

// Create accounts
sender, _ := accountMgr.CreateAccount("holder_001", TypePersonal)
receiver, _ := accountMgr.CreateAccount("holder_002", TypePersonal)

// Fund sender
accountMgr.UpdateBalance(sender.ID, 100_000) // 100,000 paise = ₹1,000

// Initiate payment
req := PaymentInitiationRequest{
    FromAccount: sender.ID,
    ToAccount:   receiver.ID,
    AmountPaise: 10_000,
    Reference:   "INV-2026-001",
}

result := paymentAgent.InitiatePayment(context.Background(), req, env)
// result.Status == StatusPending
// result.PaymentID contains the payment identifier
// result.CorrelationID for tracing

// Check status later
status, found := paymentAgent.GetPaymentStatus(result.PaymentID)
```

## Error Handling

All validation failures return `StatusFailed` with descriptive error messages:
- "from_account and to_account are required"
- "cannot transfer to the same account"
- "amount must be positive"
- "amount exceeds maximum"
- "sender account not found"
- "sender account is frozen"
- "insufficient balance: have 5000, need 10000"
- etc.

## Testing

Comprehensive test coverage (87.2%):
- **Account tests** (16): creation, retrieval, balance updates, status transitions, concurrent operations
- **Transaction tests** (12): recording, querying, status tracking, immutability, concurrent recording
- **Agent tests** (19): payment initiation, validation, idempotency, status queries, message handling, concurrent payments

All 47 tests pass.

## Integration Points

### CBDC Bridge
Implement the `CBDCBridge` interface to integrate ledger backends:
```go
type CBDCBridge interface {
    CommitLedger(ctx context.Context, fromAccount, toAccount string, 
                 amountPaise int64, paymentID string) (ledgerID string, err error)
}

agent.WithCBDCBridge(yourBridge)
```

### Message Bus
The agent implements the `agent.Agent` interface and integrates with Genie's message bus:
- Receives: `TypeIn` messages (payment_initiation)
- Emits: `TypeOut` messages (payment_result)
- Routes to: NextAgent = "cbdc_bridge"

## Performance Characteristics

- **Account Creation**: O(1)
- **Balance Update**: O(1)
- **Transaction Recording**: O(1)
- **Query by Account**: O(n) where n = all transactions for account
- **Query by Time Range**: O(n) where n = all transactions in range
- **Concurrent Operations**: Thread-safe via RWMutex; no lock contention at the map level for different accounts

## Security Considerations

1. **No Negative Balances**: Enforced at the UpdateBalance level
2. **Account Status**: Frozen/closed accounts cannot participate in transfers
3. **Immutable Ledger**: TransactionRecords cannot be modified after recording
4. **Atomicity**: Balance deductions are atomic; no partial transfers
5. **Audit Trail**: Every payment generates immutable transaction records with ledger IDs

## Future Enhancements

- Paging for large transaction queries
- Transaction filtering by status
- Bulk payment operations
- Dispute handling and reversals
- Fee calculations
- Rate limiting and quota management
- Account holders (identity linking)
