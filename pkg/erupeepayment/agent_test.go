package erupeepayment

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Now() }
func (testEnv) Logf(format string, args ...any) {}

type mockCBDCBridge struct {
	mu         sync.Mutex
	commits    []map[string]interface{}
	shouldFail bool
}

func (m *mockCBDCBridge) CommitLedger(ctx context.Context, fromAccount, toAccount string, amountPaise int64, paymentID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commits = append(m.commits, map[string]interface{}{
		"from_account": fromAccount,
		"to_account":   toAccount,
		"amount_paise": amountPaise,
		"payment_id":   paymentID,
	})
	if m.shouldFail {
		return "", nil
	}
	return "ledger_" + paymentID, nil
}

func setupPaymentAgent() (*PaymentAgent, AccountManager, TransactionLog) {
	idCounter := 0
	idGen := func() string {
		idCounter++
		return "id_" + string(rune(idCounter))
	}

	acctMgr := NewInMemoryAccountManager(idGen)
	txnLog := NewInMemoryTransactionLog(idGen)
	agent := NewPaymentAgent(acctMgr, txnLog).
		WithIDGenerator(func() string {
			idCounter++
			return "pay_" + string(rune(idCounter))
		})
	return agent, acctMgr, txnLog
}

func TestPaymentInitiationSuccess(t *testing.T) {
	payAgent, acctMgr, txnLog := setupPaymentAgent()
	env := testEnv{}

	// Setup accounts
	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 50_000)

	// Initiate payment
	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 10_000,
		Reference:   "test_001",
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, result.Status)
	}
	if result.PaymentID == "" {
		t.Errorf("payment ID not assigned")
	}
	if result.CorrelationID == "" {
		t.Errorf("correlation ID not assigned")
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}

	// Verify transaction was recorded
	status, found := txnLog.GetStatus(result.PaymentID)
	if !found {
		t.Errorf("payment not found in transaction log")
	}
	if status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, status)
	}
}

func TestPaymentInitiationMissingFromAccount(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	req := PaymentInitiationRequest{
		FromAccount: "",
		ToAccount:   to.ID,
		AmountPaise: 10_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
	if result.Error == "" {
		t.Errorf("expected error message")
	}
}

func TestPaymentInitiationSameAccount(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	acct, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	acctMgr.UpdateBalance(acct.ID, 10_000)

	req := PaymentInitiationRequest{
		FromAccount: acct.ID,
		ToAccount:   acct.ID,
		AmountPaise: 5_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationNegativeAmount(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: -1_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationZeroAmount(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 0,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationExceedsMaximum(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 1_000_000_000) // 1 billion paise

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: MaxPaymentAmountPaise + 1,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationSenderNotFound(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)

	req := PaymentInitiationRequest{
		FromAccount: "nonexistent",
		ToAccount:   to.ID,
		AmountPaise: 10_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationRecipientNotFound(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 10_000)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   "nonexistent",
		AmountPaise: 10_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationSenderFrozen(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 10_000)
	acctMgr.UpdateStatus(from.ID, StatusFrozen)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 5_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationRecipientFrozen(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 10_000)
	acctMgr.UpdateStatus(to.ID, StatusFrozen)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 5_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
}

func TestPaymentInitiationInsufficientBalance(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 5_000)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 10_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
	if result.Error == "" {
		t.Errorf("expected error message")
	}
}

func TestPaymentIdempotency(t *testing.T) {
	payAgent, acctMgr, txnLog := setupPaymentAgent()
	_ = txnLog // txnLog is used below
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 50_000)

	// Manually create a pending transaction with a known payment ID
	paymentID := "pay_manual_001"
	txn := &TransactionRecord{
		PaymentID:   paymentID,
		FromAccount: from.ID,
		ToAccount:   to.ID,
		Amount:      10_000,
		Status:      StatusPending,
	}
	txnLog.Record(txn)

	// Now try to initiate a payment with the same ID
	payAgent.WithIDGenerator(func() string { return paymentID })
	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 10_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	// Should return pending status (idempotency)
	if result.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, result.Status)
	}
}

func TestGetPaymentStatus(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 10_000)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 5_000,
	}
	result := payAgent.InitiatePayment(context.Background(), req, env)

	status, found := payAgent.GetPaymentStatus(result.PaymentID)
	if !found {
		t.Errorf("payment not found")
	}
	if status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, status)
	}
}

func TestHandleMessage(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 10_000)

	req := PaymentInitiationRequest{
		FromAccount: from.ID,
		ToAccount:   to.ID,
		AmountPaise: 5_000,
	}
	body, _ := json.Marshal(req)
	msg := agent.NewMessage("client", ID, agent.RoleUser, TypeIn, string(body), nil)

	out, err := payAgent.HandleMessage(context.Background(), msg, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].Type != TypeOut {
		t.Fatalf("expected one output message of type %s", TypeOut)
	}

	var result PaymentResult
	json.Unmarshal([]byte(out[0].Content), &result)
	if result.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, result.Status)
	}
}

func TestConcurrentPayments(t *testing.T) {
	payAgent, acctMgr, _ := setupPaymentAgent()
	env := testEnv{}

	// Setup accounts
	from, _ := acctMgr.CreateAccount("holder_001", TypePersonal)
	to, _ := acctMgr.CreateAccount("holder_002", TypePersonal)
	acctMgr.UpdateBalance(from.ID, 1_000_000) // 1M paise

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			req := PaymentInitiationRequest{
				FromAccount: from.ID,
				ToAccount:   to.ID,
				AmountPaise: 10_000,
			}
			result := payAgent.InitiatePayment(context.Background(), req, env)
			if result.Status != StatusPending && result.Status != StatusFailed {
				t.Errorf("unexpected status: %s", result.Status)
			}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestAgentMetadata(t *testing.T) {
	payAgent := NewPaymentAgent(NewInMemoryAccountManager(nil), NewInMemoryTransactionLog(nil))
	if payAgent.ID() != ID {
		t.Errorf("ID mismatch: got %s, want %s", payAgent.ID(), ID)
	}
	if payAgent.Name() == "" {
		t.Errorf("agent name is empty")
	}
	capabilities := payAgent.Capabilities()
	if len(capabilities) == 0 || capabilities[0] != Capability {
		t.Errorf("capabilities mismatch")
	}
	if payAgent.RiskLevel() != agent.RiskHigh {
		t.Errorf("risk level should be RiskHigh")
	}
}
