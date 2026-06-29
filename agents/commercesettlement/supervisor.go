// Package commercesettlement implements the Commerce Settlement Agent supervisor.
// It orchestrates batch creation, netting, and settlement execution for e-Rupee
// merchant settlements. The agent exposes three main capabilities:
//   - create_settlement_batch: aggregate merchant receivables for a settlement date
//   - settle_batch: apply netting and execute settlement payments
//   - get_settlement_status: query batch status and netting savings
//
// The agent integrates with:
//   - Merchant receivables aggregation (from Commerce Workflow)
//   - Payment Agent + CBDC Bridge (for e-Rupee settlement)
//   - Audit trail for compliance
package commercesettlement

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/commercesettlement"
)

const (
	ID         = "commerce_settlement"
	Capability = "settle_commerce"
	NextAgent  = "financial_supervisor"
)

// Agent is the Commerce Settlement Agent supervisor.
type Agent struct {
	batchManager SettlementBatchManager
	nettingCalc  NettingCalculator
	executor     SettlementExecutor
	auditLog     *AuditLog
}

// Interface definitions (re-exported from pkg for convenience).
type (
	SettlementBatchManager = commercesettlement.SettlementBatchManager
	NettingCalculator      = commercesettlement.NettingCalculator
	SettlementExecutor     = commercesettlement.SettlementExecutor
	AuditLog               = commercesettlement.AuditLog
)

// NewAgent creates a new Commerce Settlement Agent with dependencies.
func NewAgent(bm SettlementBatchManager, nc NettingCalculator, se SettlementExecutor) *Agent {
	return &Agent{
		batchManager: bm,
		nettingCalc:  nc,
		executor:     se,
		auditLog:     commercesettlement.NewAuditLog(),
	}
}

// ID returns the agent's unique identifier.
func (a *Agent) ID() string { return ID }

// Name returns a human-readable name.
func (a *Agent) Name() string { return "Commerce Settlement Agent" }

// Capabilities lists the agent's capabilities.
func (a *Agent) Capabilities() []string { return []string{Capability} }

// RiskLevel declares this agent handles high-value settlements.
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

// HandleMessage processes incoming messages and routes to appropriate handler.
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	env.Logf("[%s] Received message type: %s from %s", ID, msg.Type, msg.From)

	var result interface{}
	var err error

	switch msg.Type {
	case "create_settlement_batch":
		result, err = a.handleCreateBatch(msg, env)
	case "settle_batch":
		result, err = a.handleSettleBatch(msg, env)
	case "get_settlement_status":
		result, err = a.handleGetStatus(msg, env)
	default:
		return nil, nil // ignore unknown types
	}

	if err != nil {
		env.Logf("[%s] Error: %v", ID, err)
		return []agent.Message{
			a.errorMessage(msg, err),
		}, nil
	}

	body, _ := json.Marshal(result)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, msg.Type+"_result", string(body), msg.Metadata),
	}, nil
}

// handleCreateBatch processes a create_settlement_batch request.
type CreateBatchRequest struct {
	SettlementDate string           `json:"settlement_date"` // YYYY-MM-DD
	MerchantIDs    []string         `json:"merchant_ids"`
	AmountsPaise   map[string]int64 `json:"amounts_paise"` // merchant_id -> amount in paise
}

type CreateBatchResponse struct {
	BatchID          string `json:"batch_id"`
	SettlementDate   string `json:"settlement_date"`
	TotalAmountPaise int64  `json:"total_amount_paise"`
	MerchantCount    int    `json:"merchant_count"`
	Status           string `json:"status"`
	Message          string `json:"message"`
}

func (a *Agent) handleCreateBatch(msg agent.Message, env agent.Environment) (interface{}, error) {
	var req CreateBatchRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, fmt.Errorf("invalid create_settlement_batch request: %v", err)
	}

	// Parse settlement date.
	settlementDate, err := time.Parse("2006-01-02", req.SettlementDate)
	if err != nil {
		return nil, fmt.Errorf("invalid settlement_date format (expected YYYY-MM-DD): %v", err)
	}

	// Create batch.
	batch, err := a.batchManager.CreateBatch(settlementDate, req.MerchantIDs)
	if err != nil {
		return nil, err
	}

	// Add merchant amounts.
	if len(req.AmountsPaise) > 0 {
		if err := a.batchManager.AddMerchantAmounts(batch.ID, req.AmountsPaise); err != nil {
			return nil, err
		}
		// Refresh batch to get updated totals.
		batch, _ = a.batchManager.GetBatch(batch.ID)
	}

	// Log to audit trail.
	a.auditLog.LogAction(batch.ID, "batch_created", map[string]any{
		"merchant_count": len(batch.Entries),
		"total_amount":   batch.TotalAmountPaise,
	})

	env.Logf("[%s] Created batch %s with %d merchants, total %.2f paise", ID, batch.ID, len(batch.Entries), float64(batch.TotalAmountPaise))

	return CreateBatchResponse{
		BatchID:          batch.ID,
		SettlementDate:   batch.SettlementDate.Format("2006-01-02"),
		TotalAmountPaise: batch.TotalAmountPaise,
		MerchantCount:    len(batch.Entries),
		Status:           string(batch.Status),
		Message:          fmt.Sprintf("Batch created successfully with %d merchants", len(batch.Entries)),
	}, nil
}

// handleSettleBatch processes a settle_batch request.
type SettleBatchRequest struct {
	BatchID string `json:"batch_id"`
}

type SettleBatchResponse struct {
	BatchID             string           `json:"batch_id"`
	Status              string           `json:"status"`
	GrossAmountPaise    int64            `json:"gross_amount_paise"`
	NetAmountPaise      int64            `json:"net_amount_paise"`
	NettingSavingsPaise int64            `json:"netting_savings_paise"`
	SettlementTxnCount  int              `json:"settlement_txn_count"`
	SettlementTxnIDs    []string         `json:"settlement_txn_ids"`
	NetPositions        map[string]int64 `json:"net_positions"`
	Message             string           `json:"message"`
}

func (a *Agent) handleSettleBatch(msg agent.Message, env agent.Environment) (interface{}, error) {
	var req SettleBatchRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, fmt.Errorf("invalid settle_batch request: %v", err)
	}

	// Retrieve batch.
	batch, err := a.batchManager.GetBatch(req.BatchID)
	if err != nil {
		return nil, err
	}

	if len(batch.Entries) == 0 {
		return nil, fmt.Errorf("batch has no entries to settle")
	}

	// Convert entries to array.
	entries := make([]*commercesettlement.SettlementEntry, 0, len(batch.Entries))
	for _, entry := range batch.Entries {
		entries = append(entries, entry)
	}

	// Calculate net positions.
	pool, err := a.nettingCalc.CalculateNetPositionsWithPool(batch.SettlementDate, entries)
	if err != nil {
		return nil, err
	}

	env.Logf("[%s] Batch %s: gross=%.2f paise, net=%.2f paise, savings=%.2f paise",
		ID, req.BatchID, float64(pool.GrossAmount), float64(pool.NetAmount), float64(pool.NettingSavings))

	// Mark batch as netted.
	a.batchManager.UpdateBatchStatus(req.BatchID, commercesettlement.StatusNetted)
	batch.NettedPositions = pool.NetPosition
	batch.NettingSavings = pool.NettingSavings

	// Execute settlement.
	success, txnIDs, err := a.executor.ExecuteBatch(req.BatchID, pool.NetPosition)
	if err != nil {
		a.auditLog.LogAction(req.BatchID, "settlement_execution_failed", map[string]any{
			"error": err.Error(),
		})
		return nil, err
	}

	if !success {
		return nil, fmt.Errorf("settlement execution failed for batch %s", req.BatchID)
	}

	// Log to audit trail.
	a.auditLog.LogAction(req.BatchID, "settlement_executed", map[string]any{
		"txn_count":       len(txnIDs),
		"netting_savings": pool.NettingSavings,
	})

	env.Logf("[%s] Settlement executed: %d transactions, %.2f paise savings", ID, len(txnIDs), float64(pool.NettingSavings))

	return SettleBatchResponse{
		BatchID:             req.BatchID,
		Status:              string(batch.Status),
		GrossAmountPaise:    pool.GrossAmount,
		NetAmountPaise:      pool.NetAmount,
		NettingSavingsPaise: pool.NettingSavings,
		SettlementTxnCount:  len(txnIDs),
		SettlementTxnIDs:    txnIDs,
		NetPositions:        pool.NetPosition,
		Message:             fmt.Sprintf("Settlement completed: %d merchants, %.2f paise saved via netting", len(batch.Entries), float64(pool.NettingSavings)),
	}, nil
}

// handleGetStatus processes a get_settlement_status request.
type GetStatusRequest struct {
	BatchID string `json:"batch_id"`
}

type GetStatusResponse struct {
	BatchID             string   `json:"batch_id"`
	SettlementDate      string   `json:"settlement_date"`
	Status              string   `json:"status"`
	MerchantCount       int      `json:"merchant_count"`
	GrossAmountPaise    int64    `json:"gross_amount_paise"`
	NetAmountPaise      int64    `json:"net_amount_paise"`
	NettingSavingsPaise int64    `json:"netting_savings_paise"`
	SettlementTxnIDs    []string `json:"settlement_txn_ids"`
	CreatedAt           string   `json:"created_at"`
	SettledAt           *string  `json:"settled_at,omitempty"`
	IsIdempotent        bool     `json:"is_idempotent"`
}

func (a *Agent) handleGetStatus(msg agent.Message, env agent.Environment) (interface{}, error) {
	var req GetStatusRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, fmt.Errorf("invalid get_settlement_status request: %v", err)
	}

	// Retrieve batch.
	batch, err := a.batchManager.GetBatch(req.BatchID)
	if err != nil {
		return nil, err
	}

	// Check if idempotent (has been executed before).
	isIdempotent, _ := a.executor.VerifySettlement(req.BatchID)

	response := GetStatusResponse{
		BatchID:             batch.ID,
		SettlementDate:      batch.SettlementDate.Format("2006-01-02"),
		Status:              string(batch.Status),
		MerchantCount:       len(batch.Entries),
		GrossAmountPaise:    batch.TotalAmountPaise,
		NetAmountPaise:      calculateNetAmount(batch.NettedPositions),
		NettingSavingsPaise: batch.NettingSavings,
		SettlementTxnIDs:    batch.SettlementTxnIDs,
		CreatedAt:           batch.CreatedAt.Format("2006-01-02T15:04:05Z"),
		IsIdempotent:        isIdempotent,
	}

	if batch.SettledAt != nil {
		settledAtStr := batch.SettledAt.Format("2006-01-02T15:04:05Z")
		response.SettledAt = &settledAtStr
	}

	env.Logf("[%s] Status for batch %s: %s", ID, req.BatchID, batch.Status)

	return response, nil
}

// errorMessage creates an error response message.
func (a *Agent) errorMessage(msg agent.Message, err error) agent.Message {
	errResp := map[string]string{
		"error": err.Error(),
	}
	body, _ := json.Marshal(errResp)
	return agent.NewMessage(ID, msg.From, agent.RoleAgent, "error", string(body), msg.Metadata)
}

// calculateNetAmount computes total net amount from positions.
func calculateNetAmount(netPositions map[string]int64) int64 {
	var total int64
	for _, amount := range netPositions {
		if amount > 0 {
			total += amount
		}
	}
	return total
}
