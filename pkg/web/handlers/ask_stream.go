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

// AskStream is the SSE variant of Ask. After the agent-framework cutover the
// question pipeline runs inline (no bus, no event tap), so the stream emits the
// AI disclosure, a trace id, a single `agent.handle` progress event while the
// governed pipeline runs, and finally the `report` event.
//
// Honest limitation: the legacy bus edge streamed one `agent.handle` event per
// agent hop (analyzer, forecaster, ...). The afg QAService runs the pipeline as
// one governed call, so per-hop streaming is collapsed into a single progress
// event. The SSE event vocabulary (ai_disclosure/trace/agent.handle/report) is
// preserved for the console; restoring per-hop progress is a tracked follow-up.
type AskStream struct {
	QA        *afg.QAService
	Documents postgres.DocumentRepo
	Encryptor *crypto.Encryptor
	Timeout   time.Duration

	AIDisclosureBanner string
}

type askStreamRequest struct {
	Question   string `json:"question"`
	DocumentID string `json:"document_id"`
}

// Post streams progress over text/event-stream.
func (h *AskStream) Post(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	var req askStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	doc, err := h.Documents.GetByID(r.Context(), req.DocumentID)
	if err != nil || doc.UserID != claims.Subject {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}
	plain, err := h.Encryptor.Decrypt(doc.Payload)
	if err != nil {
		http.Error(w, "decrypt failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	traceID := fmt.Sprintf("tr-%d", time.Now().UnixNano())
	if h.AIDisclosureBanner != "" {
		send("ai_disclosure", h.AIDisclosureBanner)
	}
	send("trace", traceID)

	roleStrings := make([]string, len(claims.Roles))
	for i, role := range claims.Roles {
		roleStrings[i] = string(role)
	}
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
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(afg.WithGovMessage(r.Context(), gov), timeout)
	defer cancel()

	progress, _ := json.Marshal(map[string]any{"from": "financial_supervisor", "type": "pipeline_running", "trace_id": traceID})
	send("agent.handle", string(progress))

	report, err := h.QA.Answer(ctx, string(plain), req.Question)
	if err != nil {
		var denied *afg.DeniedError
		switch {
		case errors.As(err, &denied):
			send("error", "denied by governance policy")
		case errors.Is(err, context.DeadlineExceeded):
			send("error", "timeout")
		default:
			send("error", err.Error())
		}
		return
	}
	send("report", report)
}
