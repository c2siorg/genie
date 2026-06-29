// lineage.go — HTTP surface for Data Lineage (Audit Trail) module.
//
// Routes wired by pkg/web/router.go:
//
//	GET /v1/lineage/query — Query lineage entries (user, resource, since, until)
//	GET /v1/lineage/verify — Verify hash chain integrity
//	GET /v1/lineage/export — Export audit trail (csv/json)
//
// All endpoints require authentication.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/lineage"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// LineageHandler wraps data lineage operations.
type LineageHandler struct {
	// Recorder stores and retrieves lineage entries.
	Recorder lineage.Recorder
}

// ─── Request/response types ────────────────────────────────────────────────

// lineageEntryResponse represents a lineage entry for HTTP responses.
type lineageEntryResponse struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	UserID       string `json:"user_id"`
	ResourceID   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	Action       string `json:"action"`
	Decision     string `json:"decision"`
	ReasonCode   string `json:"reason_code"`
	PolicyRule   string `json:"policy_rule,omitempty"`
	AgentID      string `json:"agent_id,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	TraceID      string `json:"trace_id,omitempty"`
	Hash         string `json:"hash,omitempty"`
	PrevHash     string `json:"prev_hash,omitempty"`
}

// lineageQueryResponse wraps a set of lineage entries.
type lineageQueryResponse struct {
	Total   int                    `json:"total"`
	Entries []lineageEntryResponse `json:"entries"`
}

// lineageVerifyResponse indicates hash chain integrity status.
type lineageVerifyResponse struct {
	Valid        bool   `json:"valid"`
	TotalEntries int    `json:"total_entries"`
	BrokenAt     string `json:"broken_at,omitempty"`
	Error        string `json:"error,omitempty"`
}

// lineageExportResponse contains the exported audit trail.
type lineageExportResponse struct {
	Format     string `json:"format"` // csv or json
	Data       string `json:"data"`
	ByteCount  int    `json:"byte_count"`
	EntryCount int    `json:"entry_count"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// QueryLineage handles GET /v1/lineage/query.
// Queries lineage entries with optional filters (user, resource, since, until).
func (h *LineageHandler) QueryLineage(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	query := lineage.LineageQuery{
		UserID:       r.URL.Query().Get("user_id"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		ResourceType: r.URL.Query().Get("resource_type"),
	}

	// Parse action filter
	if actionStr := r.URL.Query().Get("action"); actionStr != "" {
		query.Action = lineage.Action(actionStr)
	}

	// Parse decision filter
	if decisionStr := r.URL.Query().Get("decision"); decisionStr != "" {
		query.Decision = lineage.Decision(decisionStr)
	}

	// Parse time filters
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			query.Since = t
		}
	}
	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			query.Until = t
		}
	}

	// Parse pagination
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		_ = json.Unmarshal([]byte(limitStr), &query.Limit)
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		_ = json.Unmarshal([]byte(offsetStr), &query.Offset)
	}

	// Query the recorder
	responseEntries := make([]lineageEntryResponse, 0)
	if h.Recorder != nil {
		entries, err := h.Recorder.Query(r.Context(), query)
		if err != nil {
			http.Error(w, "failed to query lineage: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert to response format
		responseEntries = make([]lineageEntryResponse, len(entries))
		for i, e := range entries {
			responseEntries[i] = lineageEntryResponse{
				ID:           e.ID,
				Timestamp:    e.Timestamp.Format(time.RFC3339),
				UserID:       e.UserID,
				ResourceID:   e.ResourceID,
				ResourceType: e.ResourceType,
				Action:       string(e.Action),
				Decision:     string(e.Decision),
				ReasonCode:   e.ReasonCode,
				PolicyRule:   e.PolicyRule,
				AgentID:      e.AgentID,
				SessionID:    e.SessionID,
				TraceID:      e.TraceID,
				Hash:         e.Hash,
				PrevHash:     e.PrevHash,
			}
		}
	}

	respondJSON(w, http.StatusOK, lineageQueryResponse{
		Total:   len(responseEntries),
		Entries: responseEntries,
	})
}

// VerifyIntegrity handles GET /v1/lineage/verify.
// Verifies the integrity of the lineage hash chain.
func (h *LineageHandler) VerifyIntegrity(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	// Verify the hash chain
	var result lineage.LineageIntegrityResult
	if h.Recorder != nil {
		result = h.Recorder.Verify(r.Context())
	} else {
		// Mock result
		result = lineage.LineageIntegrityResult{
			Valid:        true,
			TotalEntries: 0,
		}
	}

	// Convert to response
	resp := lineageVerifyResponse{
		Valid:        result.Valid,
		TotalEntries: result.TotalEntries,
		BrokenAt:     result.BrokenAt,
	}
	if result.Error != nil {
		resp.Error = result.Error.Error()
	}

	respondJSON(w, http.StatusOK, resp)
}

// ExportAuditTrail handles GET /v1/lineage/export.
// Exports the audit trail in CSV or JSON format.
func (h *LineageHandler) ExportAuditTrail(w http.ResponseWriter, r *http.Request) {
	_, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	// Parse format parameter (default: json)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	if format != "csv" && format != "json" {
		http.Error(w, "format must be csv or json", http.StatusBadRequest)
		return
	}

	// Parse optional query for filtering
	query := lineage.LineageQuery{
		UserID:       r.URL.Query().Get("user_id"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		ResourceType: r.URL.Query().Get("resource_type"),
	}

	// Parse time filters
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			query.Since = t
		}
	}
	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			query.Until = t
		}
	}

	// Export the audit trail
	var data string
	var entryCount int

	if h.Recorder != nil {
		entries, err := h.Recorder.Query(r.Context(), query)
		if err != nil {
			http.Error(w, "failed to query lineage: "+err.Error(), http.StatusInternalServerError)
			return
		}
		entryCount = len(entries)

		// Format the export data
		if format == "json" {
			bytes, _ := json.Marshal(entries)
			data = string(bytes)
		} else {
			// Simple CSV format
			data = "id,timestamp,user_id,resource_id,action,decision,reason_code\n"
			for _, e := range entries {
				data += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
					e.ID, e.Timestamp.Format(time.RFC3339), e.UserID, e.ResourceID,
					string(e.Action), string(e.Decision), e.ReasonCode)
			}
		}
	} else {
		// Mock empty export
		if format == "json" {
			data = "[]"
		} else {
			data = "id,timestamp,user_id,resource_id,action,decision,reason_code\n"
		}
	}

	// Prepare response
	resp := lineageExportResponse{
		Format:     format,
		Data:       data,
		ByteCount:  len(data),
		EntryCount: entryCount,
	}

	// Set content-type header based on format
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=audit-trail.%s", format))
	w.WriteHeader(http.StatusOK)

	// Always send JSON response body (with data field containing CSV if format=csv)
	json.NewEncoder(w).Encode(resp)
}
