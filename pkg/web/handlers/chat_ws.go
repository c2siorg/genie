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
	"github.com/coder/websocket"
)

// ChatWS implements a bidirectional WebSocket chat. The client sends one or
// more `{"question":..., "document_id":...}` frames; the server runs the
// governed agent-framework question pipeline (afg.QAService) per request and
// replies with a `report` frame. The connection stays open for multiple turns.
//
// Honest limitation (same as AskStream): the legacy bus edge emitted one
// `agent.handle` frame per agent hop; the afg pipeline runs as one governed
// call, so a single `pipeline_running` progress frame stands in. The event
// vocabulary is preserved for the console.
type ChatWS struct {
	QA        *afg.QAService
	Documents postgres.DocumentRepo
	Encryptor *crypto.Encryptor
	Timeout   time.Duration

	AIDisclosureBanner string
}

type chatIncoming struct {
	Question   string `json:"question"`
	DocumentID string `json:"document_id"`
}

type chatEvent struct {
	Event   string          `json:"event"`
	TraceID string          `json:"trace_id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Serve upgrades to WS and runs the turn loop.
func (h *ChatWS) Serve(w http.ResponseWriter, r *http.Request) {
	claims, ok := mid.ClaimsFrom(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:  []string{"*"}, // tighten for production
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	timeout := h.Timeout
	if timeout == 0 {
		timeout = 12 * time.Second
	}

	if h.AIDisclosureBanner != "" {
		_ = writeJSON(r.Context(), conn, chatEvent{Event: "ai_disclosure", Data: jsonString(h.AIDisclosureBanner)})
	}

	roleStrings := make([]string, len(claims.Roles))
	for i, role := range claims.Roles {
		roleStrings[i] = string(role)
	}

	for {
		_, body, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var in chatIncoming
		if err := json.Unmarshal(body, &in); err != nil {
			_ = writeJSON(r.Context(), conn, chatEvent{Event: "error", Data: jsonString("invalid json")})
			continue
		}
		if err := h.runTurn(r.Context(), conn, claims.Subject, roleStrings, in, timeout); err != nil {
			_ = writeJSON(r.Context(), conn, chatEvent{Event: "error", Data: jsonString(err.Error())})
		}
	}
}

func (h *ChatWS) runTurn(ctx context.Context, conn *websocket.Conn, userID string, roles []string, in chatIncoming, timeout time.Duration) error {
	if in.Question == "" || in.DocumentID == "" {
		return fmt.Errorf("question and document_id required")
	}
	doc, err := h.Documents.GetByID(ctx, in.DocumentID)
	if err != nil || doc.UserID != userID {
		return fmt.Errorf("document not found")
	}
	plain, err := h.Encryptor.Decrypt(doc.Payload)
	if err != nil {
		return fmt.Errorf("decrypt failed: %w", err)
	}

	traceID := fmt.Sprintf("tr-%d", time.Now().UnixNano())
	_ = writeJSON(ctx, conn, chatEvent{Event: "trace", TraceID: traceID})

	gov := protocol.Message{
		From: "user", Role: protocol.RoleUser, Type: financeQuestionType,
		Metadata: map[string]any{
			"trace_id":                     traceID,
			protocol.MetaKeyUserID:         userID,
			protocol.MetaKeyUserRoles:      roles,
			protocol.MetaKeyClassification: string(doc.Classification),
		},
	}
	runCtx, cancel := context.WithTimeout(afg.WithGovMessage(ctx, gov), timeout)
	defer cancel()

	progress, _ := json.Marshal(map[string]any{"from": "financial_supervisor", "type": "pipeline_running", "trace_id": traceID})
	_ = writeJSON(ctx, conn, chatEvent{Event: "agent.handle", TraceID: traceID, Data: progress})

	report, err := h.QA.Answer(runCtx, string(plain), in.Question)
	if err != nil {
		var denied *afg.DeniedError
		switch {
		case errors.As(err, &denied):
			_ = writeJSON(ctx, conn, chatEvent{Event: "error", Data: jsonString("denied by governance policy")})
		case errors.Is(err, context.DeadlineExceeded):
			_ = writeJSON(ctx, conn, chatEvent{Event: "error", Data: jsonString("timeout")})
		default:
			_ = writeJSON(ctx, conn, chatEvent{Event: "error", Data: jsonString(err.Error())})
		}
		return nil
	}
	_ = writeJSON(ctx, conn, chatEvent{Event: "report", TraceID: traceID, Data: jsonString(report)})
	return nil
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, body)
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
