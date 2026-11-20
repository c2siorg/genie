package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// financeQuestionType is the governance message Type the /v1/ask flow evaluates
// against. It matches the RBAC key in config/ai-policy.example.yaml (gated to
// user/advisor/admin) — the same Type the legacy bus supervisor used, so the
// role gate behaves identically after the agent-framework cutover.
const financeQuestionType = "finance_question"

// Ask answers a finance question by running the governed agent-framework
// question pipeline (afg.QAService) end-to-end and returning the reporter's
// final report. This replaces the legacy bus + Correlator path: there is no
// message bus, no async wait — the pipeline runs inline and returns.
type Ask struct {
	QA        *afg.QAService
	Documents postgres.DocumentRepo
	Encryptor *crypto.Encryptor
	Timeout   time.Duration

	// AIDisclosureBanner is returned with every response, satisfying Sutra 2
	// (People First) and Recommendation 18: consumers must be told when they
	// are interacting with AI.
	AIDisclosureBanner string
}

type askRequest struct {
	Question   string `json:"question"`
	DocumentID string `json:"document_id"`
}

type askResponse struct {
	TraceID      string `json:"trace_id"`
	Report       string `json:"report"`
	AIDisclosure string `json:"ai_disclosure,omitempty"`
}

func (h *Ask) Post(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Question == "" || req.DocumentID == "" {
		http.Error(w, "question and document_id required", http.StatusBadRequest)
		return
	}

	doc, err := h.Documents.GetByID(r.Context(), req.DocumentID)
	if err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}
	if doc.UserID != claims.Subject {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	plain, err := h.Encryptor.Decrypt(doc.Payload)
	if err != nil {
		http.Error(w, "decrypt failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	traceID := fmt.Sprintf("tr-%d", time.Now().UnixNano())

	roleStrings := make([]string, len(claims.Roles))
	for i, role := range claims.Roles {
		roleStrings[i] = string(role)
	}

	// Governance context for the run. Content is left EMPTY on purpose: each
	// governed stage's gate falls back to evaluating its own payload (the CSV,
	// the enriched transactions, ...), reproducing the legacy per-hop content
	// evaluation, while Type + roles drive the RBAC/classification gates.
	gov := protocol.Message{
		From: "user", Role: protocol.RoleUser, Type: financeQuestionType,
		Metadata: map[string]any{
			"trace_id":                     traceID,
			protocol.MetaKeyUserID:         claims.Subject,
			protocol.MetaKeyUserRoles:      roleStrings,
			protocol.MetaKeyClassification: string(doc.Classification),
		},
	}

	timeout := h.Timeout
	if timeout == 0 {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(afg.WithGovMessage(r.Context(), gov), timeout)
	defer cancel()

	report, err := h.QA.Answer(ctx, string(plain), req.Question)
	if err != nil {
		var denied *afg.DeniedError
		if errors.As(err, &denied) {
			http.Error(w, "request denied by governance policy: "+denied.Error(), http.StatusForbidden)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "timed out generating report", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "pipeline error: "+err.Error(), http.StatusBadGateway)
		return
	}

	respondJSON(w, http.StatusOK, askResponse{
		TraceID:      traceID,
		Report:       report,
		AIDisclosure: h.AIDisclosureBanner,
	})
}
