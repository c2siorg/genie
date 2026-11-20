// Package erupeepayment provides the PaymentAgent for e-Rupee (CBDC) fund transfers.
// The agent enforces account validation, balance checks, and amount validation,
// then integrates with the CBDC Bridge for ledger commitment with laminar tracing.
package erupeepayment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "erupeepayment"
	Capability = "initiate_payment"
	TypeIn     = "payment_initiation"
	TypeOut    = "payment_result"
	NextAgent  = "cbdc_bridge" // Integrates with CBDC Bridge for ledger commitment

	// MaxPaymentAmount: hardcoded ceiling for e-Rupee single transaction (₹10L in paise)
	MaxPaymentAmountPaise = 10_000_000
)

// PaymentInitiationRequest is the inbound message from clients.
type PaymentInitiationRequest struct {
	FromAccount string            `json:"from_account"`
	ToAccount   string            `json:"to_account"`
	AmountPaise int64             `json:"amount_paise"`
	Reference   string            `json:"reference,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PaymentResult is the outbound message indicating success or failure.
type PaymentResult struct {
	PaymentID     string        `json:"payment_id"`
	Status        PaymentStatus `json:"status"`
	Error         string        `json:"error,omitempty"`
	CorrelationID string        `json:"correlation_id"` // for tracing
	Timestamp     time.Time     `json:"timestamp"`
}

// PaymentAgent orchestrates e-Rupee fund transfers with validation and account management.
type PaymentAgent struct {
	accountMgr AccountManager
	txnLog     TransactionLog
	idGen      func() string    // injectable ID generator
	clock      func() time.Time // injectable clock for testing
	cbdcBridge CBDCBridge       // for ledger commitment (can be nil for testing)
}

// CBDCBridge interface allows pluggable ledger backends.
type CBDCBridge interface {
	CommitLedger(ctx context.Context, fromAccount, toAccount string, amountPaise int64, paymentID string) (ledgerID string, err error)
}

// NewPaymentAgent creates a payment agent with the given account and transaction managers.
func NewPaymentAgent(accountMgr AccountManager, txnLog TransactionLog) *PaymentAgent {
	return &PaymentAgent{
		accountMgr: accountMgr,
		txnLog:     txnLog,
		idGen: func() string {
			return fmt.Sprintf("pay_%d", time.Now().UnixNano())
		},
		clock: time.Now,
	}
}

// WithIDGenerator sets a custom ID generator (for testing).
func (a *PaymentAgent) WithIDGenerator(gen func() string) *PaymentAgent {
	a.idGen = gen
	return a
}

// WithClock sets a custom clock (for testing).
func (a *PaymentAgent) WithClock(clock func() time.Time) *PaymentAgent {
	a.clock = clock
	return a
}

// WithCBDCBridge sets the CBDC Bridge backend.
func (a *PaymentAgent) WithCBDCBridge(bridge CBDCBridge) *PaymentAgent {
	a.cbdcBridge = bridge
	return a
}

// ID returns the agent's unique identifier.
func (a *PaymentAgent) ID() string { return ID }

// Name returns a human-readable name.
func (a *PaymentAgent) Name() string { return "e-Rupee Payment Agent" }

// Capabilities returns the agent's capabilities.
func (a *PaymentAgent) Capabilities() []string { return []string{Capability} }

// RiskLevel indicates this is a high-risk agent (financial transaction).
func (a *PaymentAgent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

// HandleMessage processes an incoming payment initiation request.
func (a *PaymentAgent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}

	var req PaymentInitiationRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	result := a.InitiatePayment(ctx, req, env)
	env.Logf("[erupeepayment] payment_id=%s status=%s from=%s to=%s amount=%d",
		result.PaymentID, result.Status, req.FromAccount, req.ToAccount, req.AmountPaise)

	body, _ := json.Marshal(result)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// InitiatePayment processes a payment request with full validation.
// Returns PaymentResult with pending status on success.
func (a *PaymentAgent) InitiatePayment(ctx context.Context, req PaymentInitiationRequest, env agent.Environment) PaymentResult {
	paymentID := a.idGen()
	correlationID := fmt.Sprintf("%s_%d", paymentID, time.Now().UnixNano())
	now := a.clock()

	// Validation 1: From and To accounts must be provided
	if req.FromAccount == "" || req.ToAccount == "" {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         "from_account and to_account are required",
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Validation 2: From and To must differ
	if req.FromAccount == req.ToAccount {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         "cannot transfer to the same account",
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Validation 3: Amount must be positive
	if req.AmountPaise <= 0 {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         "amount must be positive",
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Validation 4: Amount must not exceed maximum
	if req.AmountPaise > MaxPaymentAmountPaise {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         fmt.Sprintf("amount exceeds maximum (%d paise)", MaxPaymentAmountPaise),
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Validation 5: Sender account must exist and be active
	fromAcct := a.accountMgr.GetAccount(req.FromAccount)
	if fromAcct == nil {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         "sender account not found",
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}
	if fromAcct.Status != StatusActive {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         fmt.Sprintf("sender account is %s", fromAcct.Status),
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Validation 6: Receiver account must exist and be active
	toAcct := a.accountMgr.GetAccount(req.ToAccount)
	if toAcct == nil {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         "recipient account not found",
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}
	if toAcct.Status != StatusActive {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         fmt.Sprintf("recipient account is %s", toAcct.Status),
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Validation 7: Sender must have sufficient balance
	if fromAcct.BalancePaise < req.AmountPaise {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         fmt.Sprintf("insufficient balance: have %d, need %d", fromAcct.BalancePaise, req.AmountPaise),
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Idempotency check: if this exact payment was already recorded, return pending
	existingStatus, found := a.txnLog.GetStatus(paymentID)
	if found && existingStatus == StatusPending {
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusPending,
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// Record the pending transaction
	txnRecord := &TransactionRecord{
		PaymentID:   paymentID,
		FromAccount: req.FromAccount,
		ToAccount:   req.ToAccount,
		Amount:      req.AmountPaise,
		Timestamp:   now,
		Status:      StatusPending,
	}
	if err := a.txnLog.Record(txnRecord); err != nil {
		env.Logf("[erupeepayment] failed to record transaction: %v", err)
		return PaymentResult{
			PaymentID:     paymentID,
			Status:        StatusFailed,
			Error:         fmt.Sprintf("transaction recording failed: %v", err),
			CorrelationID: correlationID,
			Timestamp:     now,
		}
	}

	// If CBDC Bridge is available, attempt ledger commitment (async, confirming state)
	if a.cbdcBridge != nil {
		go func() {
			ledgerID, err := a.cbdcBridge.CommitLedger(context.Background(), req.FromAccount, req.ToAccount, req.AmountPaise, paymentID)
			if err != nil {
				// Record failure
				failedTxn := &TransactionRecord{
					PaymentID:   paymentID,
					FromAccount: req.FromAccount,
					ToAccount:   req.ToAccount,
					Amount:      req.AmountPaise,
					Timestamp:   a.clock(),
					Status:      StatusFailed,
				}
				a.txnLog.Record(failedTxn)
			} else {
				// Record confirmation
				confirmedTxn := &TransactionRecord{
					PaymentID:   paymentID,
					FromAccount: req.FromAccount,
					ToAccount:   req.ToAccount,
					Amount:      req.AmountPaise,
					Timestamp:   a.clock(),
					Status:      StatusConfirmed,
					LedgerID:    ledgerID,
				}
				a.txnLog.Record(confirmedTxn)
				// Update balances
				a.accountMgr.UpdateBalance(req.FromAccount, -req.AmountPaise)
				a.accountMgr.UpdateBalance(req.ToAccount, req.AmountPaise)
			}
		}()
	}

	// Return pending status with correlation ID for client tracing
	return PaymentResult{
		PaymentID:     paymentID,
		Status:        StatusPending,
		CorrelationID: correlationID,
		Timestamp:     now,
	}
}

// GetPaymentStatus queries the current status of a payment.
func (a *PaymentAgent) GetPaymentStatus(paymentID string) (PaymentStatus, bool) {
	return a.txnLog.GetStatus(paymentID)
}
