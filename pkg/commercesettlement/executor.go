package commercesettlement

import (
	"fmt"
	"sync"
	"time"
)

// SettlementExecutor defines operations for executing settlement batches.
type SettlementExecutor interface {
	// ExecuteBatch executes settlement for a batch, creating e-Rupee payments
	// via the Payment Agent + CBDC Bridge. Returns success flag, settlement txn IDs, and error.
	ExecuteBatch(batchID string, netPositions map[string]int64) (bool, []string, error)

	// VerifySettlement checks if a batch settlement is idempotent (same batch_id = no duplicates).
	VerifySettlement(batchID string) (bool, error)
}

// PaymentBridgeAdapter defines the interface for calling the Payment Agent / CBDC Bridge.
type PaymentBridgeAdapter interface {
	// CreatePayment creates an e-Rupee payment transaction.
	// Returns the settlement transaction ID and any error.
	CreatePayment(fromMerchant, toMerchant string, amountPaise int64, metadata map[string]any) (string, error)
}

// CBDCPaymentAdapter is a mock implementation of PaymentBridgeAdapter for testing.
// In production, this would integrate with the Payment Agent + CBDC Bridge.
type CBDCPaymentAdapter struct {
	mu                sync.Mutex
	settlementHistory map[string][]string // batch_id -> []txn_ids (idempotency check)
}

// NewCBDCPaymentAdapter creates a new CBDC payment adapter.
func NewCBDCPaymentAdapter() *CBDCPaymentAdapter {
	return &CBDCPaymentAdapter{
		settlementHistory: make(map[string][]string),
	}
}

// CreatePayment simulates creating a payment via CBDC Bridge.
func (c *CBDCPaymentAdapter) CreatePayment(fromMerchant, toMerchant string, amountPaise int64, metadata map[string]any) (string, error) {
	if amountPaise <= 0 {
		return "", fmt.Errorf("invalid amount: %d paise", amountPaise)
	}
	if fromMerchant == "" || toMerchant == "" {
		return "", fmt.Errorf("missing merchant IDs")
	}
	// Simulate CBDC transaction ID generation.
	txnID := "txn-" + time.Now().Format("20060102150405") + "-" + randomSuffix(12)
	return txnID, nil
}

// DefaultSettlementExecutor implements SettlementExecutor with idempotency safeguards.
type DefaultSettlementExecutor struct {
	batchManager SettlementBatchManager
	paymentBridge PaymentBridgeAdapter
	mu           sync.Mutex
	executed     map[string]bool // idempotency check: batch_id -> executed
}

// NewDefaultSettlementExecutor creates a new settlement executor.
func NewDefaultSettlementExecutor(bm SettlementBatchManager, pb PaymentBridgeAdapter) *DefaultSettlementExecutor {
	return &DefaultSettlementExecutor{
		batchManager: bm,
		paymentBridge: pb,
		executed:     make(map[string]bool),
	}
}

// ExecuteBatch executes settlement for a batch.
// For each net position (merchant that owes others), create a payment.
// Idempotency: if batch_id is already executed, return the previous txn_ids.
func (e *DefaultSettlementExecutor) ExecuteBatch(batchID string, netPositions map[string]int64) (bool, []string, error) {
	e.mu.Lock()
	if e.executed[batchID] {
		e.mu.Unlock()
		// Return cached result (idempotent).
		batch, err := e.batchManager.GetBatch(batchID)
		if err != nil {
			return false, nil, err
		}
		return true, batch.SettlementTxnIDs, nil
	}
	e.mu.Unlock()

	// Validate net positions (conservation of value).
	if err := ValidateNetting(netPositions); err != nil {
		return false, nil, err
	}

	batch, err := e.batchManager.GetBatch(batchID)
	if err != nil {
		return false, nil, err
	}

	var settlementTxnIDs []string
	var paymentErrors []string

	// Execute payments for each net position.
	// Convention: positive balance = merchant owes to the settlement authority (or pool).
	// Negative balance = merchant is owed by the settlement authority.
	// We execute payments from net debtors to net creditors.
	for merchant, netAmount := range netPositions {
		// Only execute for net debtors (positive amounts owed).
		if netAmount <= 0 {
			continue
		}

		// In a real system, we'd identify the creditor(s) and execute bilateral payments.
		// For this implementation, we'll create a payment from the debtor to a
		// central settlement pool (represented as "settlement_pool").
		// In production, this would be more sophisticated (multi-legged settlement).

		metadata := map[string]any{
			"settlement_batch_id": batchID,
			"settlement_date":     batch.SettlementDate.Format("2006-01-02"),
			"purpose":             "commerce_settlement",
		}

		txnID, err := e.paymentBridge.CreatePayment(merchant, "settlement_pool", netAmount, metadata)
		if err != nil {
			paymentErrors = append(paymentErrors, fmt.Sprintf("merchant %s: %v", merchant, err))
			continue
		}

		settlementTxnIDs = append(settlementTxnIDs, txnID)

		// Update the settlement entry with the transaction ID.
		if entry, ok := batch.Entries[merchant]; ok {
			entry.SettlementTxnID = txnID
		}
	}

	// If any payments failed, mark batch as failed.
	if len(paymentErrors) > 0 {
		batch.Status = StatusFailed
		e.batchManager.UpdateBatchStatus(batchID, StatusFailed)
		return false, settlementTxnIDs, fmt.Errorf("settlement execution partial failure: %v", paymentErrors)
	}

	// Mark batch as settled - update directly first, then via manager.
	batch.SettlementTxnIDs = settlementTxnIDs
	batch.NettedPositions = netPositions
	batch.Status = StatusSettled
	now := time.Now()
	batch.SettledAt = &now

	// UpdateBatchStatus may also set SettledAt, but we already did it above.
	err = e.batchManager.UpdateBatchStatus(batchID, StatusSettled)
	if err != nil {
		return false, nil, err
	}

	// Verify batch was updated.
	batch, _ = e.batchManager.GetBatch(batchID)

	// Mark as executed for idempotency.
	e.mu.Lock()
	e.executed[batchID] = true
	e.mu.Unlock()

	return true, settlementTxnIDs, nil
}

// VerifySettlement checks idempotency: has this batch already been executed?
func (e *DefaultSettlementExecutor) VerifySettlement(batchID string) (bool, error) {
	e.mu.Lock()
	executed := e.executed[batchID]
	e.mu.Unlock()

	batch, err := e.batchManager.GetBatch(batchID)
	if err != nil {
		return false, err
	}

	// If already marked as settled, it's been executed.
	return batch.Status == StatusSettled || executed, nil
}

// SettlementAuditEntry records a settlement action for compliance.
type SettlementAuditEntry struct {
	ID          string            `json:"id"`
	BatchID     string            `json:"batch_id"`
	Action      string            `json:"action"`
	Details     map[string]any    `json:"details"`
	ExecutedAt  time.Time         `json:"executed_at"`
	ExecutedBy  string            `json:"executed_by"`
}

// AuditLog manages audit records for settlement operations (thread-safe).
type AuditLog struct {
	mu      sync.RWMutex
	entries []*SettlementAuditEntry
}

// NewAuditLog creates a new audit log.
func NewAuditLog() *AuditLog {
	return &AuditLog{
		entries: []*SettlementAuditEntry{},
	}
}

// LogAction records a settlement action.
func (al *AuditLog) LogAction(batchID, action string, details map[string]any) {
	al.mu.Lock()
	defer al.mu.Unlock()

	entry := &SettlementAuditEntry{
		ID:         "audit-" + randomSuffix(16),
		BatchID:    batchID,
		Action:     action,
		Details:    details,
		ExecutedAt: time.Now(),
		ExecutedBy: "system",
	}
	al.entries = append(al.entries, entry)
}

// GetEntries retrieves all audit entries for a batch.
func (al *AuditLog) GetEntries(batchID string) []*SettlementAuditEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var result []*SettlementAuditEntry
	for _, entry := range al.entries {
		if entry.BatchID == batchID {
			result = append(result, entry)
		}
	}
	return result
}
