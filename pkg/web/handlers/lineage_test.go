package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

func TestLineageHandler_QueryLineage(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/query?user_id=user-123", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.QueryLineage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp lineageQueryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Total < 0 {
		t.Error("expected non-negative total")
	}
}

func TestLineageHandler_QueryLineage_Unauthenticated(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/query?user_id=user-123", nil)
	w := httptest.NewRecorder()
	handler.QueryLineage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestLineageHandler_QueryLineage_WithFilters(t *testing.T) {
	handler := &LineageHandler{}

	// Build query with filters
	sinceTime := time.Now().AddDate(0, 0, -1).Format(time.RFC3339)
	untilTime := time.Now().Format(time.RFC3339)

	url := "/v1/lineage/query?user_id=user-123&resource_type=message&since=" + sinceTime + "&until=" + untilTime + "&limit=50"
	req := httptest.NewRequest("GET", url, nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.QueryLineage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp lineageQueryResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Should have at least a valid response
	if resp.Entries == nil {
		t.Error("expected entries array")
	}
}

func TestLineageHandler_VerifyIntegrity(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/verify", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.VerifyIntegrity(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp lineageVerifyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Should have valid set
	if resp.Valid == false && resp.Error == "" {
		t.Error("expected either valid=true or an error message")
	}
	if resp.TotalEntries < 0 {
		t.Error("expected non-negative total entries")
	}
}

func TestLineageHandler_VerifyIntegrity_Unauthenticated(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/verify", nil)
	w := httptest.NewRecorder()
	handler.VerifyIntegrity(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestLineageHandler_ExportAuditTrail_JSON(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/export?format=json", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.ExportAuditTrail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("expected content-type application/json, got %s", contentType)
	}

	var resp lineageExportResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Format != "json" {
		t.Errorf("expected format json, got %s", resp.Format)
	}
}

func TestLineageHandler_ExportAuditTrail_CSV(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/export?format=csv", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.ExportAuditTrail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv; charset=utf-8" {
		t.Errorf("expected content-type text/csv, got %s", contentType)
	}

	var resp lineageExportResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Format != "csv" {
		t.Errorf("expected format csv, got %s", resp.Format)
	}
}

func TestLineageHandler_ExportAuditTrail_InvalidFormat(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/export?format=xml", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.ExportAuditTrail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestLineageHandler_ExportAuditTrail_DefaultFormat(t *testing.T) {
	handler := &LineageHandler{}

	// No format parameter should default to JSON
	req := httptest.NewRequest("GET", "/v1/lineage/export", nil)
	claims := auth.Claims{Subject: "user-123"}
	req = req.WithContext(mid.WithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	handler.ExportAuditTrail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("expected content-type application/json, got %s", contentType)
	}
}

func TestLineageHandler_ExportAuditTrail_Unauthenticated(t *testing.T) {
	handler := &LineageHandler{}

	req := httptest.NewRequest("GET", "/v1/lineage/export", nil)
	w := httptest.NewRecorder()
	handler.ExportAuditTrail(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
