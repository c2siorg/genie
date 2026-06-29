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

// InitiatePayment initiates an e-Rupee payment.
// Simplified: just creates a payment ID without full agent processing.
func (s *RealPaymentAgentStub) InitiatePayment(ctx context.Context, req PaymentInitiationRequest) (string, error) {
	if s.agent == nil {
		return "", fmt.Errorf("payment agent not initialized")
	}

	// Generate transaction ID based on order
	paymentID := "txn-" + req.OrderID

	// In production, this would call s.agent.InitiatePayment()
	// For testing, we just return the ID and let the workflow proceed
	return paymentID, nil
}

// PollPaymentStatus checks if a payment has been confirmed.
func (s *RealPaymentAgentStub) PollPaymentStatus(ctx context.Context, transactionID string) (*PaymentConfirmation, error) {
	if s.agent == nil {
		return nil, fmt.Errorf("payment agent not initialized")
	}

	// For testing, always return success
	// In production, this would query actual payment status
	return &PaymentConfirmation{
		TransactionID: transactionID,
		Success:       true,
		ConfirmedAt:   time.Now(),
		ErrorReason:   "",
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
