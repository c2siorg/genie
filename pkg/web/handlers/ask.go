package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supervisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/busio"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// Ask publishes a finance question onto the bus, waits for the supervisor's
// final report, and returns it.
type Ask struct {
	Bus        comm.Bus
	Correlator *busio.Correlator
	Documents  postgres.DocumentRepo
	Encryptor  *crypto.Encryptor
	Timeout    time.Duration

	// AIDisclosureBanner is prepended to every response, satisfying Sutra 2
	// (People First) and Recommendation 18 (Consumer Protection): consumers
	// must be told when they are interacting with AI.
	AIDisclosureBanner string
}

type askRequest struct {
	Question   string `json:"question"`
	DocumentID string `json:"document_id"`
}

type askResponse struct {
	TraceID         string `json:"trace_id"`
	Report          string `json:"report"`
	AIDisclosure    string `json:"ai_disclosure,omitempty"`
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
	ch := h.Correlator.Await(traceID)

	roleStrings := make([]string, len(claims.Roles))
	for i, r := range claims.Roles {
		roleStrings[i] = string(r)
	}

	timeout := h.Timeout
	if timeout == 0 {
		timeout = 8 * time.Second
	}

	question := agent.NewMessage("user", supervisor.ID, agent.RoleUser, supervisor.TypeQuestion, req.Question, map[string]any{
		"trace_id":                       traceID,
		"account_id":                     claims.Subject,
		"csv":                            string(plain),
		protocol.MetaKeyUserID:           claims.Subject,
		protocol.MetaKeyUserRoles:        roleStrings,
		protocol.MetaKeyClassification:   string(doc.Classification),
	})
	h.Bus.Publish(r.Context(), question)

	select {
	case msg := <-ch:
		respondJSON(w, http.StatusOK, askResponse{
			TraceID:      traceID,
			Report:       msg.Content,
			AIDisclosure: h.AIDisclosureBanner,
		})
	case <-time.After(timeout):
		h.Correlator.Cancel(traceID)
		http.Error(w, "timed out waiting for report", http.StatusGatewayTimeout)
	case <-r.Context().Done():
		h.Correlator.Cancel(traceID)
	}
}
