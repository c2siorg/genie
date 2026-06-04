package merchant

import (
	"context"
	"encoding/json"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/merchant"
)

const (
	AgentID    = "merchant_supervisor"
	AgentName  = "Merchant Onboarding Agent"
	Capability = "manage_merchant_onboarding"
)

// Agent implements the Merchant Onboarding Agent for the Genie platform.
type Agent struct {
	manager  merchant.MerchantManager
	workflow merchant.OnboardingWorkflow
}

// New constructs a Merchant Agent with dependencies.
func New(manager merchant.MerchantManager, workflow merchant.OnboardingWorkflow) *Agent {
	if manager == nil {
		manager = merchant.NewInMemoryMerchantManager()
	}
	if workflow == nil {
		workflow = merchant.NewInMemoryOnboardingWorkflow(manager)
	}
	return &Agent{
		manager:  manager,
		workflow: workflow,
	}
}

// ID implements agent.Agent interface.
func (a *Agent) ID() string {
	return AgentID
}

// Name implements agent.Agent interface.
func (a *Agent) Name() string {
	return AgentName
}

// Capabilities implements agent.Agent interface.
func (a *Agent) Capabilities() []string {
	return []string{Capability}
}

// HandleMessage implements agent.Agent interface.
// Supported message types:
// - onboard_merchant: create merchant and initiate KYC
// - get_merchant_status: retrieve merchant profile and onboarding status
// - submit_kyc: submit KYC documents and start verification
// - update_limits: update merchant transaction limits
// - suspend_merchant: suspend a merchant account
// - approve_merchant: approve onboarding (HITL)
// - reject_merchant: reject onboarding (HITL)
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	switch msg.Type {
	case "onboard_merchant":
		return a.handleOnboardMerchant(ctx, msg, env)
	case "get_merchant_status":
		return a.handleGetMerchantStatus(ctx, msg, env)
	case "submit_kyc":
		return a.handleSubmitKYC(ctx, msg, env)
	case "verify_kyc":
		return a.handleVerifyKYC(ctx, msg, env)
	case "check_compliance":
		return a.handleCheckCompliance(ctx, msg, env)
	case "evaluate_approval":
		return a.handleEvaluateApproval(ctx, msg, env)
	case "update_limits":
		return a.handleUpdateLimits(ctx, msg, env)
	case "suspend_merchant":
		return a.handleSuspendMerchant(ctx, msg, env)
	case "approve_merchant":
		return a.handleApproveMerchant(ctx, msg, env)
	case "reject_merchant":
		return a.handleRejectMerchant(ctx, msg, env)
	default:
		return nil, nil
	}
}

// handleOnboardMerchant creates a merchant and initiates onboarding.
func (a *Agent) handleOnboardMerchant(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req OnboardMerchantRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		env.Logf("[merchant_agent] error decoding onboard_merchant: %v", err)
		return nil, err
	}

	profile, err := a.manager.CreateMerchant(ctx, req.BusinessName, req.OwnerID, merchant.BusinessType(req.BusinessType))
	if err != nil {
		env.Logf("[merchant_agent] error creating merchant: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] created merchant: %s (%s)", profile.ID, profile.BusinessName)

	res := OnboardMerchantResponse{
		MerchantID: profile.ID,
		Status:     string(profile.Status),
		Message:    "Merchant registered. Please submit KYC documents.",
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "merchant_created", string(body), msg.Metadata),
	}, nil
}

// handleGetMerchantStatus retrieves merchant profile and onboarding status.
func (a *Agent) handleGetMerchantStatus(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req GetMerchantStatusRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		env.Logf("[merchant_agent] error decoding get_merchant_status: %v", err)
		return nil, err
	}

	profile, err := a.manager.GetMerchant(ctx, req.MerchantID)
	if err != nil {
		env.Logf("[merchant_agent] merchant not found: %s", req.MerchantID)
		return a.errorResponse(msg, err.Error())
	}

	res := GetMerchantStatusResponse{
		MerchantID:        profile.ID,
		BusinessName:      profile.BusinessName,
		Status:            string(profile.Status),
		DailyLimitPaise:   profile.DailyLimitPaise,
		SettlementAccount: profile.SettlementAccount,
		CreatedAt:         profile.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         profile.UpdatedAt.Format(time.RFC3339),
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "merchant_status", string(body), msg.Metadata),
	}, nil
}

// handleSubmitKYC submits KYC documents and initiates verification.
func (a *Agent) handleSubmitKYC(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req SubmitKYCRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		env.Logf("[merchant_agent] error decoding submit_kyc: %v", err)
		return nil, err
	}

	docs := make([]merchant.OnboardingDocument, len(req.Documents))
	for i, doc := range req.Documents {
		docs[i] = merchant.OnboardingDocument{
			ID:               doc.ID,
			Type:             doc.Type,
			URL:              doc.URL,
			UploadedAt:       time.Now(),
			ExtractionResult: doc.ExtractionFields,
		}
	}

	onboardReq, err := a.workflow.SubmitKYC(ctx, req.MerchantID, docs)
	if err != nil {
		env.Logf("[merchant_agent] error submitting KYC: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] submitted KYC for merchant %s: onboarding_id=%s", req.MerchantID, onboardReq.ID)

	res := SubmitKYCResponse{
		OnboardingID:  onboardReq.ID,
		MerchantID:    onboardReq.MerchantID,
		Status:        string(onboardReq.State),
		DocumentCount: len(docs),
		Message:       "KYC documents submitted. Verification in progress.",
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "kyc_submitted", string(body), msg.Metadata),
	}, nil
}

// handleVerifyKYC performs KYC verification.
func (a *Agent) handleVerifyKYC(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req VerifyKYCRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	kycResult, err := a.workflow.VerifyKYC(ctx, req.OnboardingID)
	if err != nil {
		env.Logf("[merchant_agent] error verifying KYC: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] KYC verification: status=%s", kycResult.Status)

	res := VerifyKYCResponse{
		OnboardingID:     req.OnboardingID,
		Status:           kycResult.Status,
		GSTVerified:      kycResult.GSTVerified,
		PANVerified:      kycResult.PANVerified,
		AddressConfirmed: kycResult.BusinessAddressConfirmed,
		IdentityVerified: kycResult.OwnerIdentityVerified,
		Issues:           kycResult.Issues,
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "kyc_verified", string(body), msg.Metadata),
	}, nil
}

// handleCheckCompliance runs AML and sanctions screening.
func (a *Agent) handleCheckCompliance(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req CheckComplianceRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	compResult, err := a.workflow.CheckCompliance(ctx, req.OnboardingID)
	if err != nil {
		env.Logf("[merchant_agent] error checking compliance: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] compliance check: status=%s risk_level=%s", compResult.Status, compResult.RiskLevel)

	res := CheckComplianceResponse{
		OnboardingID:   req.OnboardingID,
		Status:         compResult.Status,
		RiskLevel:      compResult.RiskLevel,
		AMLPassed:      compResult.AMLResult,
		SanctionsCheck: compResult.SanctionsCheck,
		FlagCount:      len(compResult.Flags),
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "compliance_checked", string(body), msg.Metadata),
	}, nil
}

// handleEvaluateApproval applies policy rules.
func (a *Agent) handleEvaluateApproval(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req EvaluateApprovalRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	approval, err := a.workflow.EvaluateApproval(ctx, req.OnboardingID)
	if err != nil {
		env.Logf("[merchant_agent] error evaluating approval: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] approval decision: %s (risk=%.2f)", approval.Decision, approval.RiskScore)

	res := EvaluateApprovalResponse{
		OnboardingID: req.OnboardingID,
		Decision:     approval.Decision,
		RiskScore:    approval.RiskScore,
		Reason:       approval.Reason,
	}

	body, _ := json.Marshal(res)

	msgType := "approval_evaluated"
	if approval.Decision == "auto_approve" {
		msgType = "auto_approved"
	} else if approval.Decision == "auto_reject" {
		msgType = "auto_rejected"
	}

	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, msgType, string(body), msg.Metadata),
	}, nil
}

// handleUpdateLimits updates merchant transaction limits.
func (a *Agent) handleUpdateLimits(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req UpdateLimitsRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	limits := merchant.MerchantLimits{
		DailyP2PPaise:        req.DailyP2PPaise,
		DailySettlementPaise: req.DailySettlementPaise,
		SingleTxnPaise:       req.SingleTxnPaise,
		MonthlyTotalPaise:    req.MonthlyTotalPaise,
		EffectiveFrom:        time.Now(),
		Notes:                req.Notes,
	}

	if err := a.manager.UpdateLimits(ctx, req.MerchantID, limits); err != nil {
		env.Logf("[merchant_agent] error updating limits: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] updated limits for merchant %s", req.MerchantID)

	res := UpdateLimitsResponse{
		MerchantID:           req.MerchantID,
		DailyP2PPaise:        limits.DailyP2PPaise,
		DailySettlementPaise: limits.DailySettlementPaise,
		Message:              "Limits updated successfully",
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "limits_updated", string(body), msg.Metadata),
	}, nil
}

// handleSuspendMerchant suspends a merchant account.
func (a *Agent) handleSuspendMerchant(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req SuspendMerchantRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	if err := a.manager.Suspend(ctx, req.MerchantID, req.Reason); err != nil {
		env.Logf("[merchant_agent] error suspending merchant: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] suspended merchant %s: reason=%s", req.MerchantID, req.Reason)

	res := map[string]string{
		"merchant_id": req.MerchantID,
		"status":      "suspended",
		"reason":      req.Reason,
		"message":     "Merchant account suspended",
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "merchant_suspended", string(body), msg.Metadata),
	}, nil
}

// handleApproveMerchant approves a merchant onboarding (HITL).
func (a *Agent) handleApproveMerchant(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req ApproveMerchantRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	approved, reason, err := a.workflow.ApproveOnboarding(ctx, req.OnboardingID)
	if err != nil {
		env.Logf("[merchant_agent] error approving merchant: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	if !approved {
		env.Logf("[merchant_agent] approval denied: %s", reason)
		return a.errorResponse(msg, reason)
	}

	env.Logf("[merchant_agent] approved merchant onboarding: %s", req.OnboardingID)

	res := map[string]string{
		"onboarding_id": req.OnboardingID,
		"status":        "approved",
		"message":       "Merchant approved and activated",
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "merchant_approved", string(body), msg.Metadata),
	}, nil
}

// handleRejectMerchant rejects a merchant onboarding (HITL).
func (a *Agent) handleRejectMerchant(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	var req RejectMerchantRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}

	if err := a.workflow.RejectOnboarding(ctx, req.OnboardingID, req.Reason); err != nil {
		env.Logf("[merchant_agent] error rejecting merchant: %v", err)
		return a.errorResponse(msg, err.Error())
	}

	env.Logf("[merchant_agent] rejected merchant onboarding: %s reason=%s", req.OnboardingID, req.Reason)

	res := map[string]string{
		"onboarding_id": req.OnboardingID,
		"status":        "rejected",
		"reason":        req.Reason,
		"message":       "Merchant onboarding rejected",
	}

	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "merchant_rejected", string(body), msg.Metadata),
	}, nil
}

// errorResponse generates an error response message.
func (a *Agent) errorResponse(msg agent.Message, errorMsg string) ([]agent.Message, error) {
	res := map[string]string{
		"error": errorMsg,
	}
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(AgentID, msg.From, agent.RoleAgent, "error", string(body), msg.Metadata),
	}, nil
}

// Request/Response types

type OnboardMerchantRequest struct {
	BusinessName string `json:"business_name"`
	OwnerID      string `json:"owner_id"`
	BusinessType string `json:"business_type"`
}

type OnboardMerchantResponse struct {
	MerchantID string `json:"merchant_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

type GetMerchantStatusRequest struct {
	MerchantID string `json:"merchant_id"`
}

type GetMerchantStatusResponse struct {
	MerchantID        string `json:"merchant_id"`
	BusinessName      string `json:"business_name"`
	Status            string `json:"status"`
	DailyLimitPaise   int64  `json:"daily_limit_paise"`
	SettlementAccount string `json:"settlement_account"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type SubmitKYCRequest struct {
	MerchantID string `json:"merchant_id"`
	Documents  []struct {
		ID               string            `json:"id"`
		Type             string            `json:"type"`
		URL              string            `json:"url"`
		ExtractionFields map[string]string `json:"extraction_fields"`
	} `json:"documents"`
}

type SubmitKYCResponse struct {
	OnboardingID  string `json:"onboarding_id"`
	MerchantID    string `json:"merchant_id"`
	Status        string `json:"status"`
	DocumentCount int    `json:"document_count"`
	Message       string `json:"message"`
}

type VerifyKYCRequest struct {
	OnboardingID string `json:"onboarding_id"`
}

type VerifyKYCResponse struct {
	OnboardingID     string   `json:"onboarding_id"`
	Status           string   `json:"status"`
	GSTVerified      bool     `json:"gst_verified"`
	PANVerified      bool     `json:"pan_verified"`
	AddressConfirmed bool     `json:"address_confirmed"`
	IdentityVerified bool     `json:"identity_verified"`
	Issues           []string `json:"issues,omitempty"`
}

type CheckComplianceRequest struct {
	OnboardingID string `json:"onboarding_id"`
}

type CheckComplianceResponse struct {
	OnboardingID   string `json:"onboarding_id"`
	Status         string `json:"status"`
	RiskLevel      string `json:"risk_level"`
	AMLPassed      bool   `json:"aml_passed"`
	SanctionsCheck bool   `json:"sanctions_check"`
	FlagCount      int    `json:"flag_count"`
}

type EvaluateApprovalRequest struct {
	OnboardingID string `json:"onboarding_id"`
}

type EvaluateApprovalResponse struct {
	OnboardingID string  `json:"onboarding_id"`
	Decision     string  `json:"decision"`
	RiskScore    float64 `json:"risk_score"`
	Reason       string  `json:"reason"`
}

type UpdateLimitsRequest struct {
	MerchantID           string `json:"merchant_id"`
	DailyP2PPaise        int64  `json:"daily_p2p_paise"`
	DailySettlementPaise int64  `json:"daily_settlement_paise"`
	SingleTxnPaise       int64  `json:"single_txn_paise"`
	MonthlyTotalPaise    int64  `json:"monthly_total_paise"`
	Notes                string `json:"notes,omitempty"`
}

type UpdateLimitsResponse struct {
	MerchantID           string `json:"merchant_id"`
	DailyP2PPaise        int64  `json:"daily_p2p_paise"`
	DailySettlementPaise int64  `json:"daily_settlement_paise"`
	Message              string `json:"message"`
}

type SuspendMerchantRequest struct {
	MerchantID string `json:"merchant_id"`
	Reason     string `json:"reason"`
}

type ApproveMerchantRequest struct {
	OnboardingID string `json:"onboarding_id"`
}

type RejectMerchantRequest struct {
	OnboardingID string `json:"onboarding_id"`
	Reason       string `json:"reason"`
}
