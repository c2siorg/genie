package merchant

import (
	"context"
	"testing"
	"time"
)

// TestCreateMerchant verifies merchant profile creation.
func TestCreateMerchant(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	profile, err := manager.CreateMerchant(ctx, "Tech Startup Inc", "owner_123", BusinessTypePvt)
	if err != nil {
		t.Fatalf("CreateMerchant failed: %v", err)
	}

	if profile.ID == "" {
		t.Error("merchant ID should not be empty")
	}
	if profile.BusinessName != "Tech Startup Inc" {
		t.Errorf("expected business name 'Tech Startup Inc', got %q", profile.BusinessName)
	}
	if profile.OwnerID != "owner_123" {
		t.Errorf("expected owner_id 'owner_123', got %q", profile.OwnerID)
	}
	if profile.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, profile.Status)
	}
	if profile.DailyLimitPaise != 50_000_000 {
		t.Errorf("expected daily limit 50M paise, got %d", profile.DailyLimitPaise)
	}
}

// TestCreateMerchantValidation verifies required fields are validated.
func TestCreateMerchantValidation(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	// Missing business name
	_, err := manager.CreateMerchant(ctx, "", "owner_123", BusinessTypeSole)
	if err == nil {
		t.Error("expected error for missing business_name")
	}

	// Missing owner ID
	_, err = manager.CreateMerchant(ctx, "Business", "", BusinessTypeSole)
	if err == nil {
		t.Error("expected error for missing owner_id")
	}
}

// TestGetMerchant verifies merchant retrieval.
func TestGetMerchant(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	created, _ := manager.CreateMerchant(ctx, "Business Co", "owner_123", BusinessTypeSole)
	retrieved, err := manager.GetMerchant(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetMerchant failed: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected ID %q, got %q", created.ID, retrieved.ID)
	}
	if retrieved.BusinessName != created.BusinessName {
		t.Errorf("expected name %q, got %q", created.BusinessName, retrieved.BusinessName)
	}
}

// TestGetMerchantNotFound verifies error handling for missing merchants.
func TestGetMerchantNotFound(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	_, err := manager.GetMerchant(ctx, "nonexistent_id")
	if err == nil {
		t.Error("expected error for non-existent merchant")
	}
}

// TestGetMerchantByOwner verifies retrieving all merchants for an owner.
func TestGetMerchantByOwner(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	// Create multiple merchants for the same owner
	manager.CreateMerchant(ctx, "Business 1", "owner_123", BusinessTypeSole)
	manager.CreateMerchant(ctx, "Business 2", "owner_123", BusinessTypeLLP)
	manager.CreateMerchant(ctx, "Business 3", "owner_456", BusinessTypePvt)

	merchants, err := manager.GetMerchantByOwner(ctx, "owner_123")
	if err != nil {
		t.Fatalf("GetMerchantByOwner failed: %v", err)
	}

	if len(merchants) != 2 {
		t.Errorf("expected 2 merchants for owner_123, got %d", len(merchants))
	}

	for _, m := range merchants {
		if m.OwnerID != "owner_123" {
			t.Errorf("expected OwnerID 'owner_123', got %q", m.OwnerID)
		}
	}
}

// TestUpdateLimits verifies limit updates.
func TestUpdateLimits(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	merchant, _ := manager.CreateMerchant(ctx, "Business", "owner_123", BusinessTypeSole)

	newLimits := MerchantLimits{
		DailyP2PPaise:        20_000_000,
		DailySettlementPaise: 15_000_000,
		SingleTxnPaise:       5_000_000,
		MonthlyTotalPaise:    100_000_000,
		EffectiveFrom:        time.Now(),
	}

	err := manager.UpdateLimits(ctx, merchant.ID, newLimits)
	if err != nil {
		t.Fatalf("UpdateLimits failed: %v", err)
	}

	updated, _ := manager.GetMerchant(ctx, merchant.ID)
	if updated.DailyLimitPaise != 20_000_000 {
		t.Errorf("expected daily limit 20M paise, got %d", updated.DailyLimitPaise)
	}
}

// TestUpdateStatus verifies status transitions.
func TestUpdateStatus(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	merchant, _ := manager.CreateMerchant(ctx, "Business", "owner_123", BusinessTypeSole)
	if merchant.Status != StatusPending {
		t.Fatalf("initial status should be pending")
	}

	// Transition to approved
	err := manager.UpdateStatus(ctx, merchant.ID, StatusApproved, "")
	if err != nil {
		t.Fatalf("UpdateStatus to approved failed: %v", err)
	}

	updated, _ := manager.GetMerchant(ctx, merchant.ID)
	if updated.Status != StatusApproved {
		t.Errorf("expected status approved, got %q", updated.Status)
	}
}

// TestInvalidStatusTransition verifies invalid transitions are rejected.
func TestInvalidStatusTransition(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	merchant, _ := manager.CreateMerchant(ctx, "Business", "owner_123", BusinessTypeSole)

	// Try invalid transition (pending -> suspended, should fail)
	err := manager.UpdateStatus(ctx, merchant.ID, StatusSuspended, "test")
	if err == nil {
		t.Error("expected error for invalid status transition")
	}
}

// TestSuspendMerchant verifies merchant suspension.
func TestSuspendMerchant(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	merchant, _ := manager.CreateMerchant(ctx, "Business", "owner_123", BusinessTypeSole)
	manager.UpdateStatus(ctx, merchant.ID, StatusApproved, "")

	err := manager.Suspend(ctx, merchant.ID, "Fraud detected")
	if err != nil {
		t.Fatalf("Suspend failed: %v", err)
	}

	updated, _ := manager.GetMerchant(ctx, merchant.ID)
	if updated.Status != StatusSuspended {
		t.Errorf("expected status suspended, got %q", updated.Status)
	}
	if updated.SuspensionReason != "Fraud detected" {
		t.Errorf("expected suspension reason 'Fraud detected', got %q", updated.SuspensionReason)
	}
}

// TestAssignSettlementAccount verifies settlement account assignment.
func TestAssignSettlementAccount(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	merchant, _ := manager.CreateMerchant(ctx, "Business", "owner_123", BusinessTypeSole)

	err := manager.AssignSettlementAccount(ctx, merchant.ID, "acct_0123456789")
	if err != nil {
		t.Fatalf("AssignSettlementAccount failed: %v", err)
	}

	updated, _ := manager.GetMerchant(ctx, merchant.ID)
	if updated.SettlementAccount != "acct_0123456789" {
		t.Errorf("expected settlement account 'acct_0123456789', got %q", updated.SettlementAccount)
	}
}

// TestDefaultMerchantLimits verifies default limit values.
func TestDefaultMerchantLimits(t *testing.T) {
	limits := DefaultMerchantLimits()

	if limits.DailyP2PPaise != 50_000_000 {
		t.Errorf("expected daily P2P 50M paise, got %d", limits.DailyP2PPaise)
	}
	if limits.SingleTxnPaise != 10_000_000 {
		t.Errorf("expected single txn 10M paise, got %d", limits.SingleTxnPaise)
	}
	if limits.MonthlyTotalPaise != 500_000_000 {
		t.Errorf("expected monthly 500M paise, got %d", limits.MonthlyTotalPaise)
	}
}

// TestRestrictedMerchantLimits verifies restricted limit values.
func TestRestrictedMerchantLimits(t *testing.T) {
	limits := RestrictedMerchantLimits()

	if limits.DailyP2PPaise != 10_000_000 {
		t.Errorf("expected daily P2P 10M paise, got %d", limits.DailyP2PPaise)
	}
	if limits.SingleTxnPaise != 2_500_000 {
		t.Errorf("expected single txn 2.5M paise, got %d", limits.SingleTxnPaise)
	}
	if limits.MonthlyTotalPaise != 50_000_000 {
		t.Errorf("expected monthly 50M paise, got %d", limits.MonthlyTotalPaise)
	}
}

// TestOnboardingWorkflow verifies the complete KYC workflow.
func TestOnboardingWorkflow(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	workflow := NewInMemoryOnboardingWorkflow(manager)
	ctx := context.Background()

	// Create merchant
	merchant, _ := manager.CreateMerchant(ctx, "Clean Merchant", "owner_123", BusinessTypeSole)

	// Submit KYC
	docs := []OnboardingDocument{
		{
			ID:   "doc_1",
			Type: "gst_cert",
			URL:  "https://example.com/gst.pdf",
			ExtractionResult: map[string]string{
				"gst_number": "27AAFCT1234H1Z0",
			},
		},
	}

	onboardReq, err := workflow.SubmitKYC(ctx, merchant.ID, docs)
	if err != nil {
		t.Fatalf("SubmitKYC failed: %v", err)
	}

	if onboardReq.State != OnboardingDocumentsReady {
		t.Errorf("expected state documents_ready, got %q", onboardReq.State)
	}

	// Verify KYC
	kycResult, err := workflow.VerifyKYC(ctx, onboardReq.ID)
	if err != nil {
		t.Fatalf("VerifyKYC failed: %v", err)
	}

	if kycResult.Status != "verified" {
		t.Errorf("expected KYC status verified, got %q", kycResult.Status)
	}

	// Check compliance
	compResult, err := workflow.CheckCompliance(ctx, onboardReq.ID)
	if err != nil {
		t.Fatalf("CheckCompliance failed: %v", err)
	}

	if compResult.Status != "clear" {
		t.Errorf("expected compliance status clear, got %q", compResult.Status)
	}

	// Evaluate approval
	approval, err := workflow.EvaluateApproval(ctx, onboardReq.ID)
	if err != nil {
		t.Fatalf("EvaluateApproval failed: %v", err)
	}

	if approval.Decision != "auto_approve" {
		t.Errorf("expected auto_approve, got %q", approval.Decision)
	}

	// Approve onboarding
	approved, reason, err := workflow.ApproveOnboarding(ctx, onboardReq.ID)
	if err != nil {
		t.Fatalf("ApproveOnboarding failed: %v", err)
	}

	if !approved {
		t.Errorf("expected approval to succeed, got: %s", reason)
	}

	// Verify merchant is now approved
	updated, _ := manager.GetMerchant(ctx, merchant.ID)
	if updated.Status != StatusApproved {
		t.Errorf("expected merchant status approved, got %q", updated.Status)
	}
}

// TestAutoRejectSanctioned verifies auto-rejection of sanctioned merchants.
func TestAutoRejectSanctioned(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	workflow := NewInMemoryOnboardingWorkflow(manager)
	ctx := context.Background()

	// Create merchant
	merchant, _ := manager.CreateMerchant(ctx, "Sanctioned Merchant", "owner_456", BusinessTypeSole)

	// Mark as sanctioned
	workflow.SetSanctionedMerchant(merchant.ID, true)

	// Submit KYC
	docs := []OnboardingDocument{
		{
			ID:   "doc_1",
			Type: "gst_cert",
			URL:  "https://example.com/gst.pdf",
		},
	}

	onboardReq, _ := workflow.SubmitKYC(ctx, merchant.ID, docs)
	workflow.VerifyKYC(ctx, onboardReq.ID)
	workflow.CheckCompliance(ctx, onboardReq.ID)

	// Evaluate approval
	approval, err := workflow.EvaluateApproval(ctx, onboardReq.ID)
	if err != nil {
		t.Fatalf("EvaluateApproval failed: %v", err)
	}

	if approval.Decision != "auto_reject" {
		t.Errorf("expected auto_reject for sanctioned merchant, got %q", approval.Decision)
	}

	// Attempt approval (should fail)
	approved, reason, _ := workflow.ApproveOnboarding(ctx, onboardReq.ID)
	if approved {
		t.Error("expected approval to fail for sanctioned merchant")
	}
	if reason == "" {
		t.Error("expected reason for rejection")
	}

	// Manually reject (since policy says auto-reject)
	workflow.RejectOnboarding(ctx, onboardReq.ID, "Sanctioned party")

	// Verify merchant is rejected
	updated, _ := manager.GetMerchant(ctx, merchant.ID)
	if updated.Status != StatusRejected {
		t.Errorf("expected merchant status rejected, got %q", updated.Status)
	}
}

// TestManualReviewFlow verifies merchants requiring manual review.
func TestManualReviewFlow(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	workflow := NewInMemoryOnboardingWorkflow(manager)
	ctx := context.Background()

	// Create merchant
	merchant, _ := manager.CreateMerchant(ctx, "Moderate Risk", "owner_789", BusinessTypeLLP)

	// Submit KYC with missing extraction
	docs := []OnboardingDocument{
		{
			ID:   "doc_1",
			Type: "business_reg",
			URL:  "https://example.com/reg.pdf",
			// Missing extraction result
		},
	}

	onboardReq, _ := workflow.SubmitKYC(ctx, merchant.ID, docs)
	workflow.VerifyKYC(ctx, onboardReq.ID)
	workflow.CheckCompliance(ctx, onboardReq.ID)

	// Evaluate approval
	approval, _ := workflow.EvaluateApproval(ctx, onboardReq.ID)
	if approval.Decision != "manual_review" {
		t.Errorf("expected manual_review, got %q", approval.Decision)
	}

	// State should be manual_review
	retrieved, _ := workflow.GetOnboardingRequest(ctx, onboardReq.ID)
	if retrieved.State != OnboardingManualReview {
		t.Errorf("expected state manual_review, got %q", retrieved.State)
	}
}

// TestLimitsEnforcement verifies limit enforcement logic.
func TestLimitsEnforcement(t *testing.T) {
	limits := DefaultMerchantLimits()

	// Verify all limits are set correctly
	tests := []struct {
		name     string
		limit    int64
		expected int64
	}{
		{"Daily P2P", limits.DailyP2PPaise, 50_000_000},
		{"Daily Settlement", limits.DailySettlementPaise, 50_000_000},
		{"Single Transaction", limits.SingleTxnPaise, 10_000_000},
		{"Monthly Total", limits.MonthlyTotalPaise, 500_000_000},
	}

	for _, tt := range tests {
		if tt.limit != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.name, tt.expected, tt.limit)
		}
	}
}

// TestAuditTrail verifies audit trail is recorded.
func TestAuditTrail(t *testing.T) {
	manager := NewInMemoryMerchantManager()
	workflow := NewInMemoryOnboardingWorkflow(manager)
	ctx := context.Background()

	merchant, _ := manager.CreateMerchant(ctx, "Audit Test", "owner_123", BusinessTypeSole)

	docs := []OnboardingDocument{
		{
			ID:   "doc_1",
			Type: "gst_cert",
			URL:  "https://example.com/gst.pdf",
		},
	}

	onboardReq, _ := workflow.SubmitKYC(ctx, merchant.ID, docs)

	if len(onboardReq.AuditTrail) == 0 {
		t.Error("expected audit trail entries")
	}

	// Verify first entry is the submit_kyc action
	if len(onboardReq.AuditTrail) > 0 {
		entry := onboardReq.AuditTrail[0]
		if entry.Action != "submit_kyc" {
			t.Errorf("expected action submit_kyc, got %q", entry.Action)
		}
		if entry.ActorID != "merchant_agent" {
			t.Errorf("expected actor merchant_agent, got %q", entry.ActorID)
		}
	}
}

// BenchmarkCreateMerchant measures merchant creation performance.
func BenchmarkCreateMerchant(b *testing.B) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CreateMerchant(ctx, "Benchmark Business", "owner_123", BusinessTypeSole)
	}
}

// BenchmarkUpdateLimits measures limit update performance.
func BenchmarkUpdateLimits(b *testing.B) {
	manager := NewInMemoryMerchantManager()
	ctx := context.Background()

	merchant, _ := manager.CreateMerchant(ctx, "Benchmark", "owner_123", BusinessTypeSole)
	limits := DefaultMerchantLimits()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.UpdateLimits(ctx, merchant.ID, limits)
	}
}
