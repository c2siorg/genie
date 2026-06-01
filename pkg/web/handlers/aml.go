// aml.go — HTTP surface for AML Risk Scoring module.
//
// Routes wired by pkg/web/router.go:
//   POST /v1/aml/score — Score transaction (amount, parties, jurisdiction)
//   GET  /v1/aml/score/{score_id} — Get risk score result
//   GET  /v1/aml/account/{account_id}/history — Risk history
//
// All endpoints require authentication.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/aml"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
	"github.com/go-chi/chi/v5"
)

// AMLHandler wraps AML risk scoring operations.
type AMLHandler struct {
	// Scorer evaluates transactions for AML risk.
	Scorer aml.AMLScorer
	// Config holds AML rule weights and thresholds.
	Config *aml.AMLConfig
}

// ─── Request/response types ────────────────────────────────────────────────

// scoreTransactionRequest is the POST /v1/aml/score body.
type scoreTransactionRequest struct {
	// UserID is the originator of the transaction.
	UserID string `json:"user_id"`
	// Amount is the transaction amount in cents/minor units.
	Amount int64 `json:"amount"`
	// BeneficiaryID identifies the recipient.
	BeneficiaryID string `json:"beneficiary_id"`
	// BeneficiaryName is the recipient's name.
	BeneficiaryName string `json:"beneficiary_name"`
	// BeneficiaryCountry is the ISO-2 country code.
	BeneficiaryCountry string `json:"beneficiary_country"`
	// Description explains the transaction.
	Description string `json:"description,omitempty"`
	// Metadata holds additional context.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// scoreTransactionResponse wraps the RiskScore result.
type scoreTransactionResponse struct {
	ScoreID        string                `json:"score_id"`
	Score          float64               `json:"score"`
	Level          aml.RiskLevel         `json:"level"`
	TriggeredRules []string              `json:"triggered_rules"`
	Evidence       map[string]string     `json:"evidence"`
	PolicyOverride bool                  `json:"policy_override"`
	ScoredAt       string                `json:"scored_at"`
}

// getRiskScoreResponse returns a previously computed risk score.
type getRiskScoreResponse struct {
	ScoreID        string                `json:"score_id"`
	UserID         string                `json:"user_id"`
	Amount         int64                 `json:"amount"`
	BeneficiaryID  string                `json:"beneficiary_id"`
	BeneficiaryCountry string            `json:"beneficiary_country"`
	Score          float64               `json:"score"`
	Level          aml.RiskLevel         `json:"level"`
	TriggeredRules []string              `json:"triggered_rules"`
	Evidence       map[string]string     `json:"evidence"`
	ScoredAt       string                `json:"scored_at"`
}

// getRiskHistoryResponse lists recent risk scores for an account.
type getRiskHistoryResponse struct {
	AccountID string                   `json:"account_id"`
	Scores    []getRiskScoreResponse   `json:"scores"`
	Summary   map[string]interface{}   `json:"summary,omitempty"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// ScoreTransaction handles POST /v1/aml/score.
// Evaluates a transaction for AML risk and returns the score.
func (h *AMLHandler) ScoreTransaction(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	var body scoreTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if body.UserID == "" || body.Amount == 0 || body.BeneficiaryCountry == "" {
		http.Error(w, "user_id, amount, and beneficiary_country required", http.StatusBadRequest)
		return
	}

	// In production, look up the risk profile from a database
	profile := &aml.RiskProfile{
		UserID:              body.UserID,
		RiskLevel:           aml.RiskLevelApprove,
		DailyLimit:          500000, // 5000.00 in major units
		VelocityLimit:       10,
		FirstSeen:           time.Now().AddDate(0, 0, -30),
		AnomalyBaseline:     "baseline-hash",
		PolicyOverrides:     make(map[string]float64),
	}

	// Create transaction for scoring
	txn := &aml.Transaction{
		ID:                 generateID("txn"),
		UserID:             body.UserID,
		Amount:             body.Amount,
		BeneficiaryID:      body.BeneficiaryID,
		BeneficiaryName:    body.BeneficiaryName,
		BeneficiaryCountry: body.BeneficiaryCountry,
		Timestamp:          time.Now(),
		Description:        body.Description,
		Metadata:           body.Metadata,
	}

	// Score the transaction
	var score aml.RiskScore
	if h.Scorer != nil {
		var err error
		score, err = h.Scorer.ScoreTransaction(r.Context(), *txn, *profile)
		if err != nil {
			http.Error(w, "scoring failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Default mock score
		score = aml.RiskScore{
			Score:          25.0,
			Level:          aml.RiskLevelApprove,
			TriggeredRules: []string{},
			Evidence:       make(map[string]string),
			PolicyOverride: false,
			ScoredAt:       time.Now(),
		}
	}

	scoreID := generateID("score")
	respondJSON(w, http.StatusCreated, scoreTransactionResponse{
		ScoreID:        scoreID,
		Score:          score.Score,
		Level:          score.Level,
		TriggeredRules: score.TriggeredRules,
		Evidence:       score.Evidence,
		PolicyOverride: score.PolicyOverride,
		ScoredAt:       score.ScoredAt.Format(time.RFC3339),
	})
}

// GetScore handles GET /v1/aml/score/{score_id}.
// Returns a previously computed risk score.
func (h *AMLHandler) GetScore(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	scoreID := chi.URLParam(r, "score_id")
	if scoreID == "" {
		http.Error(w, "score_id required", http.StatusBadRequest)
		return
	}

	// In production, fetch from database
	score := getRiskScoreResponse{
		ScoreID:            scoreID,
		UserID:             "user-123",
		Amount:             100000,
		BeneficiaryID:      "ben-456",
		BeneficiaryCountry: "US",
		Score:              25.0,
		Level:              aml.RiskLevelApprove,
		TriggeredRules:     []string{},
		Evidence:           make(map[string]string),
		ScoredAt:           time.Now().Format(time.RFC3339),
	}

	respondJSON(w, http.StatusOK, score)
}

// GetHistory handles GET /v1/aml/account/{account_id}/history.
// Returns recent risk scores for an account.
func (h *AMLHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	accountID := chi.URLParam(r, "account_id")
	if accountID == "" {
		http.Error(w, "account_id required", http.StatusBadRequest)
		return
	}

	// In production, query the audit/risk history database
	scores := []getRiskScoreResponse{
		{
			ScoreID:            generateID("score"),
			UserID:             accountID,
			Amount:             100000,
			BeneficiaryID:      "ben-1",
			BeneficiaryCountry: "US",
			Score:              25.0,
			Level:              aml.RiskLevelApprove,
			TriggeredRules:     []string{},
			Evidence:           make(map[string]string),
			ScoredAt:           time.Now().AddDate(0, 0, -1).Format(time.RFC3339),
		},
		{
			ScoreID:            generateID("score"),
			UserID:             accountID,
			Amount:             250000,
			BeneficiaryID:      "ben-2",
			BeneficiaryCountry: "CN",
			Score:              45.0,
			Level:              aml.RiskLevelMonitor,
			TriggeredRules:     []string{"jurisdiction_high_risk"},
			Evidence:           map[string]string{"jurisdiction_high_risk": "Beneficiary in high-risk country"},
			ScoredAt:           time.Now().Format(time.RFC3339),
		},
	}

	summary := map[string]interface{}{
		"total_transactions": len(scores),
		"avg_risk_score":     35.0,
		"max_risk_level":     "monitor",
		"recent_alerts":      1,
	}

	respondJSON(w, http.StatusOK, getRiskHistoryResponse{
		AccountID: accountID,
		Scores:    scores,
		Summary:   summary,
	})
}
