package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/reporter"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supervisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/busio"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// AskStream is the SSE variant of Ask.
//
// Each bus event tagged with this request's trace_id is forwarded as an
// `agent.handle` SSE event. The final reporter output is sent as a `report`
// event and the connection closes.
type AskStream struct {
	Bus       comm.Bus
	Tap       *busio.EventTap
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

	traceID := fmt.Sprintf("tr-%d", time.Now().UnixNano())
	events := h.Tap.Subscribe(traceID)
	defer h.Tap.Unsubscribe(traceID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	if h.AIDisclosureBanner != "" {
		send("ai_disclosure", h.AIDisclosureBanner)
	}
	send("trace", traceID)

	roleStrings := make([]string, len(claims.Roles))
	for i, r := range claims.Roles {
		roleStrings[i] = string(r)
	}

	h.Bus.Publish(r.Context(), agent.NewMessage("user", supervisor.ID, agent.RoleUser, supervisor.TypeQuestion, req.Question, map[string]any{
		"trace_id":                     traceID,
		"account_id":                   claims.Subject,
		"csv":                          string(plain),
		protocol.MetaKeyUserID:         claims.Subject,
		protocol.MetaKeyUserRoles:      roleStrings,
		protocol.MetaKeyClassification: string(doc.Classification),
	}))

	timeout := h.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-deadline.C:
			send("error", "timeout")
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			// Terminal event: the final report is To=user, Type=final_report.
			if ev.To == "user" && ev.Type == reporter.TypeOut {
				send("report", ev.Content)
				return
			}
			// Progress event: skip raw payloads (CSV etc.) and just send a summary.
			summary := map[string]any{
				"from": ev.From, "to": ev.To, "type": ev.Type, "msg_id": ev.ID,
			}
			body, _ := json.Marshal(summary)
			send("agent.handle", string(body))
		}
	}
}
