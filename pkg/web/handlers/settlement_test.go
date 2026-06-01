package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/settlement"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

func TestSettlementHandler_CreateRequest(t *testing.T) {
	handler := &SettlementHandler{
		AuditLog: settlement.NewInMemoryAuditLog(),
	}

	body := createSettlementRequest{
		Transactions: []settlement.Transaction{
			{
				ID:               "txn-1",
				FromCounterparty: "bank-a",
				ToCounterparty:   "bank-b",
				Amount:           1000000,
				Currency:         "USD",
				SettlementDate:   time.Now().AddDate(0, 0, 1),
			},
		},
		Metadata: map[string]string{
			"batch_id": "batch-123",
		},
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/settlement/request", bytes.NewReader(bodyBytes))

	// Add auth claims to context
	claims := auth.Claims{
		Subject: "user-123",
		Roles:   []auth.Role{auth.RoleUser},
	}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.CreateRequest(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp createSettlementResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ID == "" {
		t.Error("expected settlement ID")
	}
	if resp.State != settlement.StatePending {
		t.Errorf("expected state pending, got %s", resp.State)
	}
}

func TestSettlementHandler_CreateRequest_InvalidJSON(t *testing.T) {
	handler := &SettlementHandler{}

	req := httptest.NewRequest("POST", "/v1/settlement/request", bytes.NewReader([]byte("invalid")))
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.CreateRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSettlementHandler_CreateRequest_NoTransactions(t *testing.T) {
	handler := &SettlementHandler{}

	body := createSettlementRequest{
		Transactions: []settlement.Transaction{},
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/settlement/request", bytes.NewReader(bodyBytes))
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.CreateRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSettlementHandler_CreateRequest_Unauthenticated(t *testing.T) {
	handler := &SettlementHandler{}

	body := createSettlementRequest{
		Transactions: []settlement.Transaction{},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/settlement/request", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	handler.CreateRequest(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSettlementHandler_GetRequest(t *testing.T) {
	handler := &SettlementHandler{}

	req := httptest.NewRequest("GET", "/v1/settlement/request/sreq-123", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	// Simulate chi URL param
	ctx := req.Context()
	ctx = setupChiContext(ctx, "request_id", "sreq-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp settlementStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ID != "sreq-123" {
		t.Errorf("expected ID sreq-123, got %s", resp.ID)
	}
}

func TestSettlementHandler_ExecuteSettlement(t *testing.T) {
	handler := &SettlementHandler{
		AuditLog: settlement.NewInMemoryAuditLog(),
	}

	body := executeSettlementRequest{
		ApprovalID: "approval-456",
		Reason:     "Approved by admin",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/settlement/request/sreq-123/execute", bytes.NewReader(bodyBytes))
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	ctx := req.Context()
	ctx = setupChiContext(ctx, "request_id", "sreq-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ExecuteSettlement(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp settlementStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.State != settlement.StateExecuted {
		t.Errorf("expected state executed, got %s", resp.State)
	}
}

func TestSettlementHandler_GetAuditLog(t *testing.T) {
	auditLog := settlement.NewInMemoryAuditLog()

	// Log an entry
	auditLog.Log(nil, settlement.AuditEntry{
		SettlementID: "sreq-123",
		Agent:        "test-agent",
		Action:       "create",
		Timestamp:    time.Now(),
	})

	handler := &SettlementHandler{
		AuditLog: auditLog,
	}

	req := httptest.NewRequest("GET", "/v1/settlement/request/sreq-123/audit", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	ctx := req.Context()
	ctx = setupChiContext(ctx, "request_id", "sreq-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetAuditLog(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp auditLogResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.SettlementID != "sreq-123" {
		t.Errorf("expected settlement ID sreq-123, got %s", resp.SettlementID)
	}
	if len(resp.Entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(resp.Entries))
	}
}
