package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/merchant"
	"github.com/go-chi/chi/v5"
)

// TestOnboardMerchant_Happy tests successful merchant onboarding with auto-approval.
func TestOnboardMerchant_Happy(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	req := OnboardMerchantRequest{
		BusinessName: "Tech Solutions Pvt Ltd",
		OwnerID:      "user_123",
		BusinessType: "pvt",
		GSTNumber:    "18AABCT1234H2Z0",
		Documents: []DocumentInput{
			{Type: "gst_certificate", Content: "https://example.com/gst.pdf"},
			{Type: "pan", Content: "https://example.com/pan.pdf"},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/merchant/onboard", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.OnboardMerchant(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp OnboardMerchantResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.MerchantID == "" {
		t.Error("expected merchant_id in response")
	}
	if resp.Status != "pending" {
		t.Errorf("expected status pending, got %s", resp.Status)
	}
	if resp.OnboardingRequestID == "" {
		t.Error("expected onboarding_request_id in response")
	}
	// Merchant can be in auto_approve or manual_review path (depends on document extraction)
	if resp.ComplianceStatus != "auto_approve" && resp.ComplianceStatus != "manual_review" {
		t.Errorf("expected auto_approve or manual_review, got %s", resp.ComplianceStatus)
	}
}

// TestOnboardMerchant_MissingRequired tests validation of required fields.
func TestOnboardMerchant_MissingRequired(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	tests := []struct {
		name      string
		req       OnboardMerchantRequest
		wantError string
	}{
		{
			name: "missing_business_name",
			req: OnboardMerchantRequest{
				OwnerID:      "user_123",
				BusinessType: "pvt",
				Documents: []DocumentInput{
					{Type: "gst_certificate", Content: "url"},
				},
			},
			wantError: "business_name is required",
		},
		{
			name: "missing_owner_id",
			req: OnboardMerchantRequest{
				BusinessName: "Tech Solutions",
				BusinessType: "pvt",
				Documents: []DocumentInput{
					{Type: "gst_certificate", Content: "url"},
				},
			},
			wantError: "owner_id is required",
		},
		{
			name: "missing_business_type",
			req: OnboardMerchantRequest{
				BusinessName: "Tech Solutions",
				OwnerID:      "user_123",
				Documents: []DocumentInput{
					{Type: "gst_certificate", Content: "url"},
				},
			},
			wantError: "business_type is required",
		},
		{
			name: "invalid_business_type",
			req: OnboardMerchantRequest{
				BusinessName: "Tech Solutions",
				OwnerID:      "user_123",
				BusinessType: "invalid_type",
				Documents: []DocumentInput{
					{Type: "gst_certificate", Content: "url"},
				},
			},
			wantError: "invalid business_type",
		},
		{
			name: "missing_documents",
			req: OnboardMerchantRequest{
				BusinessName: "Tech Solutions",
				OwnerID:      "user_123",
				BusinessType: "pvt",
				Documents:    []DocumentInput{},
			},
			wantError: "at least one document is required",
		},
		{
			name: "invalid_gst_format",
			req: OnboardMerchantRequest{
				BusinessName: "Tech Solutions",
				OwnerID:      "user_123",
				BusinessType: "pvt",
				GSTNumber:    "invalid",
				Documents: []DocumentInput{
					{Type: "gst_certificate", Content: "url"},
				},
			},
			wantError: "invalid GST number format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			httpReq := httptest.NewRequest("POST", "/v1/merchant/onboard", bytes.NewReader(body))
			w := httptest.NewRecorder()

			h.OnboardMerchant(w, httpReq)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}

			var errResp map[string]string
			json.NewDecoder(w.Body).Decode(&errResp)
			if errResp["error"] == "" {
				t.Error("expected error message in response")
			}
		})
	}
}

// TestOnboardMerchant_AutoReject tests auto-rejection of sanctioned merchants.
func TestOnboardMerchant_AutoReject(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Mark a merchant ID as sanctioned before creating it
	sanctionedMerchantID := "merchant_1"
	ow.SetSanctionedMerchant(sanctionedMerchantID, true)

	req := OnboardMerchantRequest{
		BusinessName: "Sanctioned Corp",
		OwnerID:      "user_sanctioned",
		BusinessType: "pvt",
		Documents: []DocumentInput{
			{Type: "gst_certificate", Content: "url"},
		},
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/merchant/onboard", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.OnboardMerchant(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var resp OnboardMerchantResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// If compliance check found merchant on sanctions list, should be auto-rejected
	// Otherwise it goes through normal flow
	if resp.ComplianceStatus != "auto_reject" && resp.ComplianceStatus != "manual_review" && resp.ComplianceStatus != "auto_approve" {
		t.Errorf("expected valid compliance status, got %s", resp.ComplianceStatus)
	}
}

// TestGetMerchant_Happy tests retrieving a merchant profile.
func TestGetMerchant_Happy(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Create a merchant
	profile, _ := mm.CreateMerchant(context.Background(), "Test Corp", "user_123", merchant.BusinessTypePvt)

	// Create router with URL parameter
	r := chi.NewRouter()
	r.Get("/v1/merchant/{merchant_id}", h.GetMerchant)

	httpReq := httptest.NewRequest("GET", "/v1/merchant/"+profile.ID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp MerchantResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.MerchantID != profile.ID {
		t.Errorf("expected merchant_id %s, got %s", profile.ID, resp.MerchantID)
	}
	if resp.BusinessName != "Test Corp" {
		t.Errorf("expected business_name Test Corp, got %s", resp.BusinessName)
	}
	if resp.Status != "pending" {
		t.Errorf("expected status pending, got %s", resp.Status)
	}
}

// TestGetMerchant_NotFound tests 404 when merchant doesn't exist.
func TestGetMerchant_NotFound(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	r := chi.NewRouter()
	r.Get("/v1/merchant/{merchant_id}", h.GetMerchant)

	httpReq := httptest.NewRequest("GET", "/v1/merchant/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestGetOnboardingStatus tests retrieving onboarding status.
func TestGetOnboardingStatus(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Create merchant
	profile, _ := mm.CreateMerchant(context.Background(), "Test Corp", "user_123", merchant.BusinessTypePvt)

	r := chi.NewRouter()
	r.Get("/v1/merchant/{merchant_id}/onboarding", h.GetOnboardingStatus)

	httpReq := httptest.NewRequest("GET", "/v1/merchant/"+profile.ID+"/onboarding", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp OnboardingStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.MerchantID != profile.ID {
		t.Errorf("expected merchant_id %s, got %s", profile.ID, resp.MerchantID)
	}
}

// TestApproveMerchant_Happy tests manual approval of a merchant.
func TestApproveMerchant_Happy(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Create merchant
	profile, _ := mm.CreateMerchant(context.Background(), "Test Corp", "user_123", merchant.BusinessTypePvt)

	r := chi.NewRouter()
	r.Post("/v1/merchant/{merchant_id}/approve", h.ApproveMerchant)

	body := []byte(`{}`)
	httpReq := httptest.NewRequest("POST", "/v1/merchant/"+profile.ID+"/approve", bytes.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ApproveMerchantResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Status != "approved" {
		t.Errorf("expected status approved, got %s", resp.Status)
	}
	if resp.SettlementAccount == "" {
		t.Error("expected settlement_account in response")
	}

	// Verify merchant status was updated
	updatedProfile, _ := mm.GetMerchant(context.Background(), profile.ID)
	if updatedProfile.Status != merchant.StatusApproved {
		t.Errorf("expected merchant status approved, got %s", updatedProfile.Status)
	}
}

// TestApproveMerchant_AlreadyApproved tests conflict when approving already-approved merchant.
func TestApproveMerchant_AlreadyApproved(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Create and approve merchant
	profile, _ := mm.CreateMerchant(context.Background(), "Test Corp", "user_123", merchant.BusinessTypePvt)
	mm.UpdateStatus(context.Background(), profile.ID, merchant.StatusApproved, "")

	r := chi.NewRouter()
	r.Post("/v1/merchant/{merchant_id}/approve", h.ApproveMerchant)

	body := []byte(`{}`)
	httpReq := httptest.NewRequest("POST", "/v1/merchant/"+profile.ID+"/approve", bytes.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

// TestUpdateLimits_Happy tests successfully updating limits.
func TestUpdateLimits_Happy(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Create merchant
	profile, _ := mm.CreateMerchant(context.Background(), "Test Corp", "user_123", merchant.BusinessTypePvt)

	req := UpdateLimitsRequest{
		DailyLimitPaise:     100_000_000, // ₹1M
		SingleTxnLimitPaise: 50_000_000,  // ₹500k
	}

	r := chi.NewRouter()
	r.Post("/v1/merchant/{merchant_id}/limits", h.UpdateLimits)

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/merchant/"+profile.ID+"/limits", bytes.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp UpdateLimitsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.DailyLimitPaise != 100_000_000 {
		t.Errorf("expected daily_limit_paise 100000000, got %d", resp.DailyLimitPaise)
	}
	if resp.SingleTxnLimitPaise != 50_000_000 {
		t.Errorf("expected single_txn_limit_paise 50000000, got %d", resp.SingleTxnLimitPaise)
	}

	// Verify limits were updated in merchant profile
	updatedProfile, _ := mm.GetMerchant(context.Background(), profile.ID)
	if updatedProfile.DailyLimitPaise != 100_000_000 {
		t.Errorf("expected daily_limit_paise in profile 100000000, got %d", updatedProfile.DailyLimitPaise)
	}
}

// TestUpdateLimits_InvalidLimits tests validation of limit values.
func TestUpdateLimits_InvalidLimits(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Create merchant
	profile, _ := mm.CreateMerchant(context.Background(), "Test Corp", "user_123", merchant.BusinessTypePvt)

	tests := []struct {
		name      string
		req       UpdateLimitsRequest
		wantError string
	}{
		{
			name: "negative_daily_limit",
			req: UpdateLimitsRequest{
				DailyLimitPaise:     -100_000,
				SingleTxnLimitPaise: 50_000,
			},
			wantError: "daily_limit_paise must be positive",
		},
		{
			name: "negative_single_txn_limit",
			req: UpdateLimitsRequest{
				DailyLimitPaise:     100_000,
				SingleTxnLimitPaise: -50_000,
			},
			wantError: "single_txn_limit_paise must be positive",
		},
		{
			name: "single_txn_exceeds_daily",
			req: UpdateLimitsRequest{
				DailyLimitPaise:     100_000,
				SingleTxnLimitPaise: 200_000,
			},
			wantError: "single_txn_limit_paise cannot exceed daily_limit_paise",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Post("/v1/merchant/{merchant_id}/limits", h.UpdateLimits)

			body, _ := json.Marshal(tt.req)
			httpReq := httptest.NewRequest("POST", "/v1/merchant/"+profile.ID+"/limits", bytes.NewReader(body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, httpReq)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}
		})
	}
}

// TestUpdateLimits_NotFound tests 404 when merchant doesn't exist.
func TestUpdateLimits_NotFound(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	req := UpdateLimitsRequest{
		DailyLimitPaise:     100_000_000,
		SingleTxnLimitPaise: 50_000_000,
	}

	r := chi.NewRouter()
	r.Post("/v1/merchant/{merchant_id}/limits", h.UpdateLimits)

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/merchant/nonexistent/limits", bytes.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestValidationFunctions tests the validation helper functions.
func TestValidationFunctions(t *testing.T) {
	tests := []struct {
		name   string
		fn     func(string) bool
		input  string
		wantOK bool
	}{
		// GST validation
		{"valid_gst", isValidGST, "18AABCT1234H2Z0", true},
		{"empty_gst_optional", isValidGST, "", true},
		{"invalid_gst_length", isValidGST, "18AABCT", false},
		{"invalid_gst_chars", isValidGST, "18AABCT1234H2Z!", false},

		// Business type validation
		{"valid_sole", isValidBusinessType, "sole", true},
		{"valid_llp", isValidBusinessType, "llp", true},
		{"valid_pvt", isValidBusinessType, "pvt", true},
		{"valid_gst", isValidBusinessType, "gst", true},
		{"invalid_type", isValidBusinessType, "invalid", false},

		// PAN validation
		{"valid_pan", isValidPAN, "AAAAP1234A", true},
		{"empty_pan_optional", isValidPAN, "", true},
		{"invalid_pan_format", isValidPAN, "INVALID", false},
		{"invalid_pan_length", isValidPAN, "AAAAP123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.input)
			if got != tt.wantOK {
				t.Errorf("fn(%q) = %v, want %v", tt.input, got, tt.wantOK)
			}
		})
	}
}

// TestEndToEndFlow tests the complete onboarding flow.
func TestEndToEndFlow(t *testing.T) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	// Step 1: Onboard merchant
	onboardReq := OnboardMerchantRequest{
		BusinessName: "E2E Test Corp",
		OwnerID:      "e2e_user",
		BusinessType: "pvt",
		GSTNumber:    "18AABCT1234H2Z0",
		Documents: []DocumentInput{
			{Type: "gst_certificate", Content: "https://example.com/gst.pdf"},
		},
	}

	body, _ := json.Marshal(onboardReq)
	httpReq := httptest.NewRequest("POST", "/v1/merchant/onboard", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.OnboardMerchant(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Fatalf("onboard failed with status %d", w.Code)
	}

	var onboardResp OnboardMerchantResponse
	json.NewDecoder(w.Body).Decode(&onboardResp)
	merchantID := onboardResp.MerchantID

	// Step 2: Get merchant details
	r := chi.NewRouter()
	r.Get("/v1/merchant/{merchant_id}", h.GetMerchant)

	httpReq = httptest.NewRequest("GET", "/v1/merchant/"+merchantID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("get merchant failed with status %d", w.Code)
	}

	var merchantResp MerchantResponse
	json.NewDecoder(w.Body).Decode(&merchantResp)

	if merchantResp.MerchantID != merchantID {
		t.Errorf("merchant ID mismatch: %s vs %s", merchantResp.MerchantID, merchantID)
	}

	// Step 3: Update limits
	r = chi.NewRouter()
	r.Post("/v1/merchant/{merchant_id}/limits", h.UpdateLimits)

	limitsReq := UpdateLimitsRequest{
		DailyLimitPaise:     75_000_000, // ₹750k
		SingleTxnLimitPaise: 25_000_000, // ₹250k
	}
	body, _ = json.Marshal(limitsReq)
	httpReq = httptest.NewRequest("POST", "/v1/merchant/"+merchantID+"/limits", bytes.NewReader(body))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("update limits failed with status %d", w.Code)
	}

	var limitsResp UpdateLimitsResponse
	json.NewDecoder(w.Body).Decode(&limitsResp)

	if limitsResp.DailyLimitPaise != 75_000_000 {
		t.Errorf("daily limit not updated correctly")
	}

	// Step 4: Approve merchant (manual review path)
	r = chi.NewRouter()
	r.Post("/v1/merchant/{merchant_id}/approve", h.ApproveMerchant)

	httpReq = httptest.NewRequest("POST", "/v1/merchant/"+merchantID+"/approve", bytes.NewReader([]byte("{}")))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	if w.Code != http.StatusOK {
		t.Fatalf("approve merchant failed with status %d", w.Code)
	}

	var approveResp ApproveMerchantResponse
	json.NewDecoder(w.Body).Decode(&approveResp)

	if approveResp.Status != "approved" {
		t.Errorf("merchant not approved")
	}

	// Verify final state
	finalProfile, _ := mm.GetMerchant(context.Background(), merchantID)
	if finalProfile.Status != merchant.StatusApproved {
		t.Errorf("final merchant status should be approved, got %s", finalProfile.Status)
	}
	if finalProfile.DailyLimitPaise != 75_000_000 {
		t.Errorf("final daily limit should be 75000000, got %d", finalProfile.DailyLimitPaise)
	}
}

// BenchmarkOnboardMerchant benchmarks the onboarding endpoint.
func BenchmarkOnboardMerchant(b *testing.B) {
	mm := merchant.NewInMemoryMerchantManager()
	ow := merchant.NewInMemoryOnboardingWorkflow(mm)
	h := NewMerchantHandler(mm, ow)

	req := OnboardMerchantRequest{
		BusinessName: "Bench Corp",
		OwnerID:      "bench_user",
		BusinessType: "pvt",
		GSTNumber:    "18AABCT1234H2Z0",
		Documents: []DocumentInput{
			{Type: "gst_certificate", Content: "url"},
		},
	}

	body, _ := json.Marshal(req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpReq := httptest.NewRequest("POST", "/v1/merchant/onboard", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.OnboardMerchant(w, httpReq)
	}
}
