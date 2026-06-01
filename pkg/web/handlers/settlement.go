// settlement.go — HTTP surface for Settlement Coordinator module.
//
// Routes wired by pkg/web/router.go:
//   POST /v1/settlement/request — Create settlement request
//   GET  /v1/settlement/request/{request_id} — Get status
//   POST /v1/settlement/request/{request_id}/execute — Execute settlement
//   GET  /v1/settlement/request/{request_id}/audit — Get audit trail
//
// All endpoints require authentication. Settlement execution may require
// admin approval via router-level gates.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/settlement"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
	"github.com/go-chi/chi/v5"
)

// SettlementHandler wraps settlement coordinator operations.
type SettlementHandler struct {
	// In a real system, these would be injected service/supervisor interfaces.
	// For now, we store references to the audit log and settlement components.
	AuditLog settlement.AuditLog
}

// ─── Request/response types ────────────────────────────────────────────────

// createSettlementRequest is the POST /v1/settlement/request body.
type createSettlementRequest struct {
	// Transactions is a list of inbound payments to settle.
	Transactions []settlement.Transaction `json:"transactions"`
	// Metadata holds context (settlement date, batch ID, etc.)
	Metadata map[string]string `json:"metadata,omitempty"`
}

// createSettlementResponse wraps the created SettlementRequest.
type createSettlementResponse struct {
	ID        string                      `json:"id"`
	State     settlement.SettlementState  `json:"state"`
	CreatedAt string                      `json:"created_at"`
}

// executeSettlementRequest is the POST /v1/settlement/request/{id}/execute body.
type executeSettlementRequest struct {
	ApprovalID string `json:"approval_id,omitempty"` // Link to HITL approval
	Reason     string `json:"reason,omitempty"`      // Approver's justification
}

// settlementStatusResponse wraps a SettlementRequest for HTTP responses.
type settlementStatusResponse struct {
	ID             string                      `json:"id"`
	State          settlement.SettlementState  `json:"state"`
	Transactions   []settlement.Transaction    `json:"transactions,omitempty"`
	Counterparties []string                    `json:"counterparties,omitempty"`
	NetAmounts     []settlement.NetAmount      `json:"net_amounts,omitempty"`
	CreatedAt      string                      `json:"created_at"`
	UpdatedAt      string                      `json:"updated_at"`
	Metadata       map[string]string           `json:"metadata,omitempty"`
}

// auditLogResponse wraps a slice of AuditEntry for HTTP responses.
type auditLogResponse struct {
	SettlementID string                    `json:"settlement_id"`
	Entries      []settlement.AuditEntry   `json:"entries"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// CreateRequest handles POST /v1/settlement/request.
// Creates a new settlement request and records it in the audit log.
func (h *SettlementHandler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	var body createSettlementRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if len(body.Transactions) == 0 {
		http.Error(w, "transactions required", http.StatusBadRequest)
		return
	}

	// Generate settlement request ID (in production, use UUID)
	reqID := generateID("sreq")

	// Create the settlement request in pending state
	req := &settlement.SettlementRequest{
		ID:             reqID,
		Transactions:   body.Transactions,
		State:          settlement.StatePending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Metadata:       body.Metadata,
	}

	// Extract counterparties from transactions
	counterpartyMap := make(map[string]bool)
	for _, txn := range body.Transactions {
		counterpartyMap[txn.FromCounterparty] = true
		counterpartyMap[txn.ToCounterparty] = true
	}
	for cp := range counterpartyMap {
		req.Counterparties = append(req.Counterparties, cp)
	}

	// Log the creation
	err := settlement.LogToolExecution(
		r.Context(),
		h.AuditLog,
		reqID,
		"settlement-handler",
		"create_request",
		body,
		req,
		nil,
	)
	if err != nil {
		// Log errors are non-fatal; don't fail the request
	}

	respondJSON(w, http.StatusCreated, createSettlementResponse{
		ID:        req.ID,
		State:     req.State,
		CreatedAt: req.CreatedAt.Format(time.RFC3339),
	})
}

// GetRequest handles GET /v1/settlement/request/{request_id}.
// Returns the current status of a settlement request.
func (h *SettlementHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	requestID := chi.URLParam(r, "request_id")
	if requestID == "" {
		http.Error(w, "request_id required", http.StatusBadRequest)
		return
	}

	// In a real system, fetch from database. For now, return a mock response.
	// This would typically look up the SettlementRequest by ID.
	sr := &settlement.SettlementRequest{
		ID:    requestID,
		State: settlement.StateFetched,
		Transactions: []settlement.Transaction{
			{
				ID:               "txn-1",
				FromCounterparty: "bank-a",
				ToCounterparty:   "bank-b",
				Amount:           1000000,
				Currency:         "USD",
			},
		},
		Counterparties: []string{"bank-a", "bank-b"},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	respondJSON(w, http.StatusOK, settlementStatusResponse{
		ID:             sr.ID,
		State:          sr.State,
		Transactions:   sr.Transactions,
		Counterparties: sr.Counterparties,
		NetAmounts:     sr.NetAmounts,
		CreatedAt:      sr.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      sr.UpdatedAt.Format(time.RFC3339),
		Metadata:       sr.Metadata,
	})
}

// ExecuteSettlement handles POST /v1/settlement/request/{request_id}/execute.
// Executes a settlement request after approval.
// This is typically gated by router-level admin checks.
func (h *SettlementHandler) ExecuteSettlement(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	_ = claims // Used later in LogApproval

	requestID := chi.URLParam(r, "request_id")
	if requestID == "" {
		http.Error(w, "request_id required", http.StatusBadRequest)
		return
	}

	var body executeSettlementRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// In production, this would:
	// 1. Fetch the SettlementRequest by ID
	// 2. Verify it's in StateApproved
	// 3. Execute the settlement (transfer funds)
	// 4. Update state to StateExecuted
	// 5. Log the execution

	err := settlement.LogApproval(
		r.Context(),
		h.AuditLog,
		requestID,
		claims.Subject,
		true,
		body.Reason,
	)
	if err != nil {
		// Non-fatal; execution proceeds
	}

	sr := &settlement.SettlementRequest{
		ID:    requestID,
		State: settlement.StateExecuted,
	}

	respondJSON(w, http.StatusOK, settlementStatusResponse{
		ID:    sr.ID,
		State: sr.State,
	})
}

// GetAuditLog handles GET /v1/settlement/request/{request_id}/audit.
// Returns the complete audit trail for a settlement request.
func (h *SettlementHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	requestID := chi.URLParam(r, "request_id")
	if requestID == "" {
		http.Error(w, "request_id required", http.StatusBadRequest)
		return
	}

	// In production, fetch from the audit log
	var entries []settlement.AuditEntry
	if h.AuditLog != nil {
		var err error
		entries, err = h.AuditLog.GetBySettlementID(r.Context(), requestID)
		if err != nil {
			http.Error(w, "failed to fetch audit log", http.StatusInternalServerError)
			return
		}
	}

	respondJSON(w, http.StatusOK, auditLogResponse{
		SettlementID: requestID,
		Entries:      entries,
	})
}
