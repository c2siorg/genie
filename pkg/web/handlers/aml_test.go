package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/aml"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

func TestAMLHandler_ScoreTransaction(t *testing.T) {
	handler := &AMLHandler{
		Config: aml.DefaultAMLConfig(),
	}

	body := scoreTransactionRequest{
		UserID:             "user-123",
		Amount:             100000,
		BeneficiaryID:      "ben-456",
		BeneficiaryName:    "John Doe",
		BeneficiaryCountry: "US",
		Description:        "Invoice payment",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/aml/score", bytes.NewReader(bodyBytes))
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.ScoreTransaction(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp scoreTransactionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ScoreID == "" {
		t.Error("expected score ID")
	}
	if resp.Score == 0 {
		t.Error("expected non-zero score")
	}
	if resp.Level != aml.RiskLevelApprove {
		t.Errorf("expected approval, got %s", resp.Level)
	}
}

func TestAMLHandler_ScoreTransaction_InvalidJSON(t *testing.T) {
	handler := &AMLHandler{}

	req := httptest.NewRequest("POST", "/v1/aml/score", bytes.NewReader([]byte("invalid")))
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.ScoreTransaction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAMLHandler_ScoreTransaction_MissingUserID(t *testing.T) {
	handler := &AMLHandler{}

	body := scoreTransactionRequest{
		UserID:             "",
		Amount:             100000,
		BeneficiaryCountry: "US",
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/aml/score", bytes.NewReader(bodyBytes))
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.ScoreTransaction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAMLHandler_ScoreTransaction_Unauthenticated(t *testing.T) {
	handler := &AMLHandler{}

	body := scoreTransactionRequest{
		UserID:             "user-123",
		Amount:             100000,
		BeneficiaryCountry: "US",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/aml/score", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	handler.ScoreTransaction(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAMLHandler_GetScore(t *testing.T) {
	handler := &AMLHandler{}

	req := httptest.NewRequest("GET", "/v1/aml/score/score-123", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	ctx := req.Context()
	ctx = setupChiContext(ctx, "score_id", "score-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetScore(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp getRiskScoreResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ScoreID != "score-123" {
		t.Errorf("expected score ID score-123, got %s", resp.ScoreID)
	}
}

func TestAMLHandler_GetScore_Unauthenticated(t *testing.T) {
	handler := &AMLHandler{}

	req := httptest.NewRequest("GET", "/v1/aml/score/score-123", nil)
	w := httptest.NewRecorder()
	handler.GetScore(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAMLHandler_GetHistory(t *testing.T) {
	handler := &AMLHandler{}

	req := httptest.NewRequest("GET", "/v1/aml/account/user-123/history", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	ctx := req.Context()
	ctx = setupChiContext(ctx, "account_id", "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp getRiskHistoryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.AccountID != "user-123" {
		t.Errorf("expected account ID user-123, got %s", resp.AccountID)
	}
	if len(resp.Scores) == 0 {
		t.Error("expected at least one score in history")
	}
	if resp.Summary["total_transactions"] == nil {
		t.Error("expected summary field total_transactions")
	}
}

func TestAMLHandler_GetHistory_Unauthenticated(t *testing.T) {
	handler := &AMLHandler{}

	req := httptest.NewRequest("GET", "/v1/aml/account/user-123/history", nil)
	w := httptest.NewRecorder()
	handler.GetHistory(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
