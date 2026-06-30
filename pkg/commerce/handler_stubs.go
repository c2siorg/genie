// Package commerce provides end-to-end workflow orchestration for e-Rupee commerce operations.
// This file contains handler stubs that bridge the workflow orchestrator to actual business logic.
//
// Author: Phase 2 Integration Testing Implementation
// License: MIT (see root LICENSE file)
package commerce

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/erupeepayment"
)

// noopEnv is a minimal agent.Environment that discards all log output.
// Used when calling PaymentAgent.InitiatePayment outside of a real bus dispatch.
type noopEnv struct{}

func (noopEnv) Now() time.Time                          { return time.Now() }
func (noopEnv) Logf(_ string, _ ...interface{})         {}

// RealPaymentAgentStub bridges the workflow orchestrator to the actual PaymentAgent.
// Instead of HTTP calls, it directly processes payments through the agent logic.
type RealPaymentAgentStub struct {
	agent *erupeepayment.PaymentAgent
}

// NewRealPaymentAgentStub creates a payment stub that delegates to the real payment agent.
func NewRealPaymentAgentStub(agent *erupeepayment.PaymentAgent) *RealPaymentAgentStub {
	return &RealPaymentAgentStub{
		agent: agent,
	}
}

// InitiatePayment initiates a real e-Rupee payment through the actual PaymentAgent.
// Maps commerce fields (CustomerID/MerchantID) to the agent's from/to account model.
func (s *RealPaymentAgentStub) InitiatePayment(ctx context.Context, req PaymentInitiationRequest) (string, error) {
	if s.agent == nil {
		return "", fmt.Errorf("payment agent not initialized")
	}

	result := s.agent.InitiatePayment(ctx, erupeepayment.PaymentInitiationRequest{
		FromAccount: req.CustomerID,  // customer account is debited
		ToAccount:   req.MerchantID,  // merchant account is credited
		AmountPaise: req.AmountPaise,
		Reference:   req.OrderID,
	}, noopEnv{})

	if result.Status == erupeepayment.StatusFailed {
		return "", fmt.Errorf("payment rejected: %s", result.Error)
	}

	return result.PaymentID, nil
}

// PollPaymentStatus queries the real PaymentAgent for confirmation status.
// Returns success for pending/any non-failed status (optimistic confirm for in-process workflow).
func (s *RealPaymentAgentStub) PollPaymentStatus(ctx context.Context, transactionID string) (*PaymentConfirmation, error) {
	if s.agent == nil {
		return nil, fmt.Errorf("payment agent not initialized")
	}

	status, found := s.agent.GetPaymentStatus(transactionID)
	if !found {
		return &PaymentConfirmation{
			TransactionID: transactionID,
			Success:       false,
			ErrorReason:   "payment not found in log",
		}, nil
	}

	if status == erupeepayment.StatusFailed {
		return &PaymentConfirmation{
			TransactionID: transactionID,
			Success:       false,
			ConfirmedAt:   time.Now(),
			ErrorReason:   "payment failed",
		}, nil
	}

	return &PaymentConfirmation{
		TransactionID: transactionID,
		Success:       true,
		ConfirmedAt:   time.Now(),
	}, nil
}

// RealSettlementAgentStub bridges the workflow orchestrator to settlement execution.
// It directly invokes settlement logic without HTTP calls.
type RealSettlementAgentStub struct {
	executor interface {
		ExecuteBatch(ctx context.Context, batchID string, positions map[string]int64) error
		GetBatchStatus(ctx context.Context, batchID string) (string, error)
	}
}

// NewRealSettlementAgentStub creates a settlement stub.
func NewRealSettlementAgentStub(executor interface {
	ExecuteBatch(ctx context.Context, batchID string, positions map[string]int64) error
	GetBatchStatus(ctx context.Context, batchID string) (string, error)
}) *RealSettlementAgentStub {
	return &RealSettlementAgentStub{
		executor: executor,
	}
}

// InitiateSettlement triggers a settlement batch for an order.
func (s *RealSettlementAgentStub) InitiateSettlement(ctx context.Context, req SettlementInitiationRequest) (string, error) {
	if s.executor == nil {
		return "", fmt.Errorf("settlement executor not initialized")
	}

	// Create settlement positions (simplified: just one merchant receiving)
	positions := map[string]int64{
		req.MerchantID: req.AmountPaise,
	}

	// Execute settlement batch
	batchID := "batch-" + req.OrderID
	err := s.executor.ExecuteBatch(ctx, batchID, positions)
	if err != nil {
		return "", err
	}

	return batchID, nil
}

// WaitSettlementCompletion polls for settlement completion.
func (s *RealSettlementAgentStub) WaitSettlementCompletion(ctx context.Context, settlementID string) (*SettlementResult, error) {
	if s.executor == nil {
		return nil, fmt.Errorf("settlement executor not initialized")
	}

	// Check settlement status
	status, err := s.executor.GetBatchStatus(ctx, settlementID)
	if err != nil {
		return nil, err
	}

	return &SettlementResult{
		SettlementID: settlementID,
		Success:      status == "executed",
		CompletedAt:  time.Now(),
	}, nil
}
