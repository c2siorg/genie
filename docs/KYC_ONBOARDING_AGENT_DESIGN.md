# KYC/Onboarding Agent Design Document

**Version:** 1.0  
**Status:** Design (No Code Yet)  
**Date:** May 31, 2026  
**Author:** Pratik Dhanave  
**Alignment:** FREE-AI Rec 8 (Graded Liability), Rec 14 (Board Policy), Rec 18 (Disclosure), Rec 22 (Audit)

---

## Executive Summary

The KYC/Onboarding Agent is a multi-step, stateful workflow orchestrator that processes customer onboarding from document upload through identity verification, sanctions screening, and human approval. It implements a state machine (`pending → documents_uploaded → identity_verified → sanctions_checked → approved|flagged`) with pluggable sub-agents for each verification task, integrated HITL approval for risk-flagged cases, and full audit trails for regulatory compliance.

**Key Design Principles:**
1. **State Machine Clarity** — explicit states, transitions, and rollback semantics
2. **Sub-agent Composition** — DocumentProcessor, IdentityVerifier, SanctionsChecker, OnboardingApprover as independent agents
3. **Policy-Driven Decisions** — auto-approve, manual-review, and auto-flag rules configurable per jurisdiction/risk tier
4. **Audit & Lineage** — every check and decision recorded with actor, timestamp, confidence, and rationale
5. **Pure Functions** — core decision logic testable without external services (mock OCR, identity, sanctions checks)

---

## 1. Multi-Step Verification Workflow

### 1.1 Workflow Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ONBOARDING REQUEST INTAKE                             │
│  OnboardingRequest created, State = PENDING, assigned unique UUID            │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│               STEP 1: DOCUMENT UPLOAD & OCR EXTRACTION                       │
│  Sub-agent: DocumentProcessor                                               │
│  Input: raw files (passport, ID, bank statements)                           │
│  Output: extracted_text, document_confidence_score, storage_reference       │
│  State: DOCUMENTS_UPLOADED                                                  │
│  Failure Path: Request retry, escalate to manual upload                      │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│         STEP 2: IDENTITY VERIFICATION (Name, DOB, Address)                  │
│  Sub-agent: IdentityVerifier                                                │
│  Input: extracted_text, identity_info (name, dob, address from form)       │
│  Output: field_matches (yes/no), confidence_score (0-1), verification_date  │
│  State: IDENTITY_VERIFIED                                                   │
│  Decision Gate: Confidence ≥ 95% → proceed | 80-95% → manual review         │
│                 < 80% → auto-flag for high risk                             │
│  Failure Path: OCR low-confidence, request document reupload or manual entry │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│      STEP 3: SANCTIONS SCREENING (OFAC, UN, MHA Watchlists)                │
│  Sub-agent: SanctionsChecker                                                │
│  Input: name, dob, jurisdiction from identity_verification                 │
│  Output: is_sanctioned (bool), watchlists_checked (array), risk_level       │
│  State: SANCTIONS_CHECKED                                                   │
│  Decision Gate: Sanctioned → AUTO-FLAG & REJECT                             │
│                 Not sanctioned → proceed to approval policy check            │
│  Failure Path: MCP call timeout, log and continue with "unverified" status  │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│             STEP 4: APPROVAL POLICY EVALUATION & ESCALATION                 │
│  Logic: Pure function applied to all prior results                          │
│  Auto-Approve: low-risk jurisdiction + no sanctions + identity conf > 95%   │
│  Manual Review: moderate-risk jurisdiction OR 80-95% confidence             │
│  Auto-Flag: sanctioned OR high-risk jurisdiction OR < 80% confidence        │
│  State: APPROVAL_PENDING (→ APPROVED | FLAGGED based on policy)             │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                 ┌───────────────────┼───────────────────┐
                 │                   │                   │
      AUTO-APPROVE           MANUAL REVIEW         AUTO-FLAG
            │                   │                   │
            ▼                   ▼                   ▼
    ┌──────────────┐   ┌──────────────────┐   ┌──────────────┐
    │  APPROVED    │   │  HITL APPROVER   │   │   FLAGGED    │
    │  State change│   │  SLA: 24h review │   │  State change│
    │  Audit trail │   │  Analyst decision│   │  Audit trail │
    └──────────────┘   │  or request more │   └──────────────┘
                       │  documentation   │
                       │  Actor logged    │
                       └──────────────────┘
```

### 1.2 State Machine

```go
// OnboardingState defines the finite states of a request
type OnboardingState string

const (
    StatePending           OnboardingState = "pending"
    StatesDocumentUpload   OnboardingState = "documents_uploaded"
    StateIdentityVerified  OnboardingState = "identity_verified"
    StateSanctionsChecked  OnboardingState = "sanctions_checked"
    StateApprovalPending   OnboardingState = "approval_pending"
    StateApproved          OnboardingState = "approved"
    StateFlagged           OnboardingState = "flagged"
    StateRejected          OnboardingState = "rejected"
)

// Transitions:
pending → documents_uploaded (DocumentProcessor success)
documents_uploaded → identity_verified (IdentityVerifier success)
identity_verified → sanctions_checked (SanctionsChecker success)
sanctions_checked → approval_pending (Policy evaluation complete)
approval_pending → approved (Auto-approve OR HITL approves)
approval_pending → flagged (Auto-flag OR HITL holds/rejects)
any state → rejected (Terminal: sanctions hit, document fraud, etc.)
```

### 1.3 Failure & Rollback

- **OCR Failure (low confidence):** Request document reupload or manual data entry. State remains `documents_uploaded`.
- **Identity Verification Timeout:** Log event, continue with `is_verified = false, confidence = 0.0`. State advances with degraded signal.
- **Sanctions Check Timeout:** Log event, advance to `approval_pending` with `sanctions_status = "unverified"`. Policy treats as elevated risk.
- **Analyst Timeout (> 24h):** Auto-escalate to compliance manager. System generates notification.

---

## 2. Sub-Agent Architecture

### 2.1 DocumentProcessor Agent

**Responsibility:** File ingestion, storage, OCR extraction, format validation.

```go
// Pseudo-signature
type DocumentProcessor interface {
    // ProcessDocument receives raw file, validates format, stores securely,
    // runs OCR, and returns extracted text with confidence.
    ProcessDocument(
        ctx context.Context,
        docType string, // "passport" | "driving_license" | "bank_statement"
        rawFileBytes []byte,
        metadata map[string]any, // user_id, request_id
    ) (DocumentProcessingResult, error)
}

type DocumentProcessingResult struct {
    DocumentID          string    `json:"document_id"`
    DocumentType        string    `json:"document_type"`
    ExtractedText       string    `json:"extracted_text"`
    OCRConfidence       float64   `json:"ocr_confidence_0_1"`
    FormattedFields     map[string]any `json:"formatted_fields"`
    StorageReference    string    `json:"storage_reference"`
    ProcessedAt         time.Time `json:"processed_at"`
    IssuesDetected      []string  `json:"issues_detected"`
}

// DocumentProcessorAgent implements pkg/agent.Agent
// ID: "document_processor"
// Capabilities: ["extract_identity_document", "parse_financial_statement"]
// RiskLevel: RiskMedium (handles PII, file I/O)
```

**Integration Points:**
- MCP or library for OCR (mock: return static text for testing)
- Cloud storage (S3/GCS) for document archival and compliance holds
- Database: `Documents` table with encryption at rest

---

### 2.2 IdentityVerifier Agent

**Responsibility:** Validate extracted identity fields against expected patterns, optionally call third-party APIs (biometric, address, etc.).

```go
// Pseudo-signature
type IdentityVerifier interface {
    // VerifyIdentity checks name/DOB/address consistency.
    // Returns field-level match confidence and an overall confidence score.
    VerifyIdentity(
        ctx context.Context,
        extractedText string,
        formData map[string]any,
        documentType string,
    ) (IdentificationResult, error)
}

type IdentificationResult struct {
    RequestID           string                 `json:"request_id"`
    FieldMatches        FieldMatchDetails      `json:"field_matches"`
    OverallConfidence   float64                `json:"overall_confidence_0_1"`
    VerificationMethod  string                 `json:"verification_method"`
    VerifiedAt          time.Time              `json:"verified_at"`
    VerifierID          string                 `json:"verifier_id"`
    Issues              []string               `json:"issues"`
}

type FieldMatchDetails struct {
    Name            MatchResult `json:"name"`
    DOB             MatchResult `json:"dob"`
    Address         MatchResult `json:"address"`
    IDNumber        MatchResult `json:"id_number"`
}

type MatchResult struct {
    IsMatch    bool    `json:"is_match"`
    Confidence float64 `json:"confidence_0_1"`
    Extracted  string  `json:"extracted"`
    Expected   string  `json:"expected"`
    Note       string  `json:"note,omitempty"`
}

// IdentityVerifierAgent implements pkg/agent.Agent
// ID: "identity_verifier"
// Capabilities: ["verify_identity", "check_biometric"]
// RiskLevel: RiskHigh (PII processing)
```

**Confidence Scoring:**
- Name match (fuzzy, Levenshtein distance): 0-1
- DOB match: binary + variance tolerance (± 3 days for data entry error)
- Address match: token overlap + postcode validation
- Overall: weighted average or worst-of-three

---

### 2.3 SanctionsChecker Agent

**Responsibility:** Screen customer name/DOB/jurisdiction against OFAC SDN, UN, and MHA watchlists via MCP pattern.

```go
// Pseudo-signature
type SanctionsChecker interface {
    // CheckSanctions screens name against multiple watchlists.
    // Returns hit status, list of matched records, and risk level.
    CheckSanctions(
        ctx context.Context,
        name string,
        dob *time.Time,
        jurisdiction string,
    ) (SanctionsResult, error)
}

type SanctionsResult struct {
    RequestID          string              `json:"request_id"`
    IsSanctioned       bool                `json:"is_sanctioned"`
    Hits               []SanctionsHit      `json:"hits"`
    WatchlistsChecked  []string            `json:"watchlists_checked"`
    RiskLevel          string              `json:"risk_level"` // "green" | "yellow" | "red"
    LastCheckedAt      time.Time           `json:"last_checked_at"`
    CheckerID          string              `json:"checker_id"`
    Note               string              `json:"note,omitempty"`
}

type SanctionsHit struct {
    Watchlist      string    `json:"watchlist"`
    MatchedName    string    `json:"matched_name"`
    Confidence     float64   `json:"confidence_0_1"`
    RecordID       string    `json:"record_id"`
    DateOfListing  time.Time `json:"date_of_listing"`
    Type           string    `json:"type"` // "individual" | "entity"
}

// SanctionsCheckerAgent implements pkg/agent.Agent
// ID: "sanctions_checker"
// Capabilities: ["screen_sanctions"]
// RiskLevel: RiskHigh (regulatory, audit-critical)
```

**Watchlist Coverage:**
- OFAC SDN (Specially Designated Nationals) — US Treasury
- UN Security Council Consolidated List (1267, 1988, 1989)
- MHA National Security List (India-specific, if applicable)

---

### 2.4 OnboardingApprover Agent

**Responsibility:** HITL interface for analysts to review flagged cases, request additional docs, approve, or reject.

```go
// Pseudo-signature
type OnboardingApprover interface {
    // RequestApproval sends a case to HITL for analyst review.
    // Blocks until analyst submits decision (or deadline expires).
    RequestApproval(
        ctx context.Context,
        request *OnboardingRequest,
        riskFactors []string,
        sla time.Duration,
    ) (ApprovalDecision, error)
}

type ApprovalDecision struct {
    OnboardingRequestID string    `json:"onboarding_request_id"`
    Decision            string    `json:"decision"` // "approve" | "hold" | "reject" | "request_docs"
    Reason              string    `json:"reason"`
    DecidedBy           string    `json:"decided_by"`
    DecidedAt           time.Time `json:"decided_at"`
    RequestedDocs       []string  `json:"requested_docs,omitempty"`
    Notes               string    `json:"notes,omitempty"`
}

// OnboardingApproverAgent implements pkg/agent.Agent
// ID: "onboarding_approver"
// Capabilities: ["approve_onboarding", "request_additional_documents"]
// RiskLevel: RiskHigh (human authority, irreversible decision)
```

**Integration Points:**
- `pkg/hitl.AsyncApprover` — HTTP endpoint for analyst UI to submit decisions
- `pkg/hitl.PolicyApprover` — Optional rules to auto-approve/deny before sending to human
- Websocket or polling for real-time analyst notifications

---

## 3. Data Model

### 3.1 OnboardingRequest (Core Entity)

```go
type OnboardingRequest struct {
    ID                string                 `json:"id"`        // UUID
    UserID            string                 `json:"user_id"`
    RequestedAt       time.Time              `json:"requested_at"`
    State             OnboardingState        `json:"state"`
    UpdatedAt         time.Time              `json:"updated_at"`
    ApplicantInfo     ApplicantInfo          `json:"applicant_info"`
    Documents         []Document             `json:"documents"`
    IdentityResult    *IdentificationResult  `json:"identity_result"`
    SanctionsResult   *SanctionsResult       `json:"sanctions_result"`
    ApprovalPolicy    ApprovalPolicyResult   `json:"approval_policy"`
    ApprovalDecision  *ApprovalDecision      `json:"approval_decision"`
    AuditLog          []compliance.AuditEntry `json:"audit_log"`
    LineageRecords    []string               `json:"lineage_records"`
}

type ApplicantInfo struct {
    FullName        string    `json:"full_name"`
    DateOfBirth     time.Time `json:"date_of_birth"`
    Address         string    `json:"address"`
    Jurisdiction    string    `json:"jurisdiction"`    // ISO 3166-1 alpha-2
    Email           string    `json:"email"`
    PhoneNumber     string    `json:"phone_number"`
    OccupationCode  string    `json:"occupation_code"`
}

func (or *OnboardingRequest) RiskScore() float64 {
    // Synthetic score based on all verifications, used for policy routing
    // Returns 0.0 (lowest risk) to 1.0 (highest risk)
}
```

### 3.2 Document

```go
type Document struct {
    ID                  string            `json:"id"`           // UUID
    OnboardingRequestID string            `json:"request_id"`
    Type                string            `json:"type"`         // "passport" | "driving_license" | "bank_statement"
    UploadedAt          time.Time         `json:"uploaded_at"`
    StorageReference    string            `json:"storage_ref"`
    OCRExtractedText    string            `json:"ocr_text"`
    OCRConfidence       float64           `json:"ocr_confidence_0_1"`
    FormattedFields     map[string]any    `json:"formatted_fields"`
    VerificationResults map[string]any    `json:"verification_results"`
    Hash                string            `json:"hash"`         // SHA256 for tamper detection
}
```

---

## 4. Policy & Escalation Rules

### 4.1 Decision Matrix

```
+─────────────────────┬────────────────────┬──────────────────┬─────────────────┐
│ Jurisdiction Risk   │ Identity Conf      │ Sanctions Status │ Recommended     │
├─────────────────────┼────────────────────┼──────────────────┼─────────────────┤
│ Low (Tier A)        │ ≥ 95%              │ Clean            │ AUTO_APPROVE    │
│ Low (Tier A)        │ 80-95%             │ Clean            │ MANUAL_REVIEW   │
│ Low (Tier A)        │ < 80%              │ Clean            │ AUTO_FLAG       │
│ Moderate (Tier B)   │ ≥ 95%              │ Clean            │ MANUAL_REVIEW   │
│ Moderate (Tier B)   │ < 95%              │ Clean            │ MANUAL_REVIEW   │
│ High (Tier C)       │ any                │ Clean            │ AUTO_FLAG       │
│ any                 │ any                │ Hit              │ AUTO_FLAG       │
│ any                 │ any                │ Unverified*      │ MANUAL_REVIEW   │
└─────────────────────┴────────────────────┴──────────────────┴─────────────────┘

* Sanctions check timeout or unavailable
```

### 4.2 Risk Scoring Function

```go
func ComputeRiskScore(req *OnboardingRequest) float64 {
    score := 0.0

    // Identity verification confidence (negative signal: lower conf = higher risk)
    if req.IdentityResult != nil {
        score += (1.0 - req.IdentityResult.OverallConfidence) * 0.30
    }

    // Sanctions (positive signal: hit = max risk)
    if req.SanctionsResult != nil && req.SanctionsResult.IsSanctioned {
        score += 1.0
    }

    // Jurisdiction risk tier (weights from policy YAML)
    if req.ApplicantInfo.Jurisdiction != "" {
        if tier, ok := policyJurisdictionTiers[req.ApplicantInfo.Jurisdiction]; ok {
            score += tier.RiskWeight * 0.20
        }
    }

    // Occupation high-risk (FATF guidance)
    if isHighRiskOccupation(req.ApplicantInfo.OccupationCode) {
        score += 0.10
    }

    // Document issues
    if len(req.Documents) == 0 {
        score += 0.15
    }
    for _, doc := range req.Documents {
        if doc.OCRConfidence < 0.70 {
            score += 0.05
        }
    }

    // Clamp to [0, 1]
    if score < 0 {
        score = 0
    }
    if score > 1 {
        score = 1
    }

    return score
}
```

### 4.3 SLA & Escalation

| Scenario                           | SLA        | Escalation Path                  |
|------------------------------------|------------|---------------------------------|
| Auto-approve                       | Immediate  | None                            |
| Manual review assigned to analyst  | 24 hours   | After 12h: reminder; after 24h: escalate to manager |
| Auto-flag                          | Immediate  | Log to audit trail; notify compliance |
| Sanctions hit                      | < 1 hour   | Notify compliance, prepare STR if confirmed |
| Document reupload requested        | 7 days     | Auto-reject if deadline missed  |

---

## 5. Integration Points

### 5.1 Existing Genie Packages

**pkg/agent** — Message-driven orchestration, agent interface

**pkg/hitl** — Human-in-the-loop approval, async HTTP + policy-based routing

**pkg/compliance** — Audit logging with hash-chain tamper detection

**pkg/lineage** — Data provenance and access auditing

**pkg/agentic** — LLM tool-calling (optional for analyst summaries)

### 5.2 External Services (Pluggable Interfaces)

**DocumentStore** — S3, GCS, or encrypted DB blob storage

**OCRService** — Google Vision, local Tesseract, or mock for tests

**BiometricVerifier** — V-CIP, Aadhaar verification (optional)

**AddressValidator** — Postcode/district lookup (optional)

**SanctionsService** — OFAC API, UN List API, or internal watchlist store (MCP pattern)

---

## 6. Test Scenarios

### 6.1 Happy Path
Valid passport, clean sanctions, auto-approve → State progresses: pending → documents_uploaded → identity_verified → sanctions_checked → approval_pending → approved

### 6.2 Moderate Risk
Expired document, identity confidence 88%, clean sanctions → manual_review state, analyst approves

### 6.3 High Risk
Sanctioned party → auto_flag, incident logged, state → flagged

### 6.4 OCR Failure
Low-quality passport (confidence 0.55) → request reupload, auto-reject if deadline missed

### 6.5 Sanctions Timeout
MCP unavailable → continue with sanctions_status = "unverified", policy routes to manual_review

---

## 7. File Structure (Ready to Code)

```
agents/kyc/
├── supervisor.go              # OnboardingAgentSupervisor
├── supervisor_test.go
├── document_processor.go       # DocumentProcessor agent
├── document_processor_test.go
├── identity_verifier.go        # IdentityVerifier agent
├── identity_verifier_test.go
├── sanctions_checker.go        # SanctionsChecker agent
├── sanctions_checker_test.go
├── onboarding_approver.go      # OnboardingApprover agent (HITL wrapper)
└── onboarding_approver_test.go

pkg/kyc/
├── types.go                    # OnboardingRequest, Document, etc.
├── policy.go                   # EvaluateApprovalPolicy, ComputeRiskScore
├── policy_test.go
├── store.go                    # In-memory + DB persistence interface
├── store_test.go
└── const.go                    # Constants: states, policies, thresholds
```

---

## 8. Method Signatures (Skeleton)

### agents/kyc/supervisor.go

```go
type OnboardingAgentSupervisor struct {
    docProc       DocumentProcessor
    idVerifier    IdentityVerifier
    sanctions     SanctionsChecker
    approver      OnboardingApprover
    store         OnboardingStore
    auditLog      compliance.AuditLog
}

func (s *OnboardingAgentSupervisor) ID() string
func (s *OnboardingAgentSupervisor) Name() string
func (s *OnboardingAgentSupervisor) Capabilities() []string
func (s *OnboardingAgentSupervisor) RiskLevel() agent.RiskClass

func (s *OnboardingAgentSupervisor) HandleMessage(
    ctx context.Context,
    msg agent.Message,
    env agent.Environment,
) ([]agent.Message, error)

func (s *OnboardingAgentSupervisor) Orchestrate(
    ctx context.Context,
    request *kyc.OnboardingRequest,
    env agent.Environment,
) (*kyc.OnboardingRequest, error)
```

### agents/kyc/document_processor.go

```go
type DocumentProcessorAgent struct {
    ocr      OCRService
    store    DocumentStore
    auditLog compliance.AuditLog
}

func (a *DocumentProcessorAgent) ProcessDocument(
    ctx context.Context,
    docType string,
    rawFileBytes []byte,
) (*kyc.Document, error)
```

### agents/kyc/identity_verifier.go

```go
type IdentityVerifierAgent struct {
    biometric BiometricVerifier
    address   AddressValidator
    auditLog  compliance.AuditLog
}

func (a *IdentityVerifierAgent) VerifyIdentity(
    ctx context.Context,
    extractedText string,
    formData map[string]any,
    docType string,
) (*kyc.VerificationResult, error)
```

### agents/kyc/sanctions_checker.go

```go
type SanctionsCheckerAgent struct {
    sanctions SanctionsService
    cache     *MCPResultCache
    auditLog  compliance.AuditLog
}

func (a *SanctionsCheckerAgent) CheckSanctions(
    ctx context.Context,
    name string,
    dob *time.Time,
    jurisdiction string,
) (*kyc.SanctionsResult, error)
```

### agents/kyc/onboarding_approver.go

```go
type OnboardingApproverAgent struct {
    hitlApprover hitl.Approver
    store        OnboardingStore
    auditLog     compliance.AuditLog
}

func (a *OnboardingApproverAgent) RequestApproval(
    ctx context.Context,
    request *kyc.OnboardingRequest,
    riskFactors []string,
    sla time.Duration,
) (*kyc.ApprovalDecision, error)
```

### pkg/kyc/policy.go

```go
func EvaluateApprovalPolicy(
    ctx context.Context,
    request *OnboardingRequest,
    policy *ApprovalPolicy,
) (*ApprovalPolicyResult, error)

func ComputeRiskScore(req *OnboardingRequest) float64

func isHighRiskJurisdiction(jurisdiction string) bool

func isHighRiskOccupation(occupationCode string) bool
```

### pkg/kyc/store.go

```go
type OnboardingStore interface {
    Create(ctx context.Context, request *OnboardingRequest) error
    Get(ctx context.Context, id string) (*OnboardingRequest, error)
    Update(ctx context.Context, request *OnboardingRequest) error
    ListByUserID(ctx context.Context, userID string) ([]*OnboardingRequest, error)
    Archive(ctx context.Context, id string) error
}

type InMemoryOnboardingStore struct { }
type DBOnboardingStore struct { }
```

---

## 9. Regulatory Alignment

| Requirement | Implementation |
|-------------|-----------------|
| RBI Master Direction on KYC | Applied rules from sections I-VII, including SDD/EDD routing |
| FATF High-Risk Jurisdiction Detection | Jurisdiction tier lookup in policy YAML, auto-flag for Tier C |
| PEP Screening | SanctionsChecker MCP includes PEP lists (UN, MHA) |
| Sanctions List Coverage | OFAC SDN, UN Consolidated List, MHA list |
| Document Validation | DocumentProcessor format validation, hash-chain audit |
| Audit Trail (Annexure VI Compliance) | Hash-chained compliance.AuditLog, incident payloads on reject |
| HITL Oversight (FREE-AI Rec 16) | OnboardingApprover wraps hitl.AsyncApprover, analyst review |
| Disclosure (FREE-AI Rec 18) | Verdict includes disclaimer on risk assessment caveats |

---

## 10. Open Design Questions

1. **Biometric Mandatory?** Should V-CIP be mandatory for non-Tier A jurisdictions, or optional?
2. **PEP Refresh Cadence** Should existing customers be re-screened? (Annual? Monthly?)
3. **EDD Workflow** What additional checks define Enhanced Due Diligence? Beneficial ownership lookup?
4. **Batch Processing** Support bulk onboarding for B2B customers? SLA per record?
5. **Archival** After approval, encrypt & archive documents, or delete per GDPR?

---

## Conclusion

This design establishes a **production-grade, audit-ready KYC platform** that decomposes complexity across focused sub-agents, grounds decisions in explicit policy rules, integrates audit trails and HITL approval, and supports testing in-memory without external services. The next phase is **implementation** following these patterns and skeletons.
