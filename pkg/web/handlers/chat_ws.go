package handlers

import (
	"context"
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
	"github.com/coder/websocket"
)

// ChatWS implements a bidirectional WebSocket chat. The client sends one or
// more `{"question":..., "document_id":...}` frames; the server streams
// `{"event":"agent.handle", ...}` and finally `{"event":"report", ...}` per
// request. The connection stays open for multiple turns.
type ChatWS struct {
	Bus       comm.Bus
	Tap       *busio.EventTap
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
	for i, r := range claims.Roles {
		roleStrings[i] = string(r)
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
	events := h.Tap.Subscribe(traceID)
	defer h.Tap.Unsubscribe(traceID)
	_ = writeJSON(ctx, conn, chatEvent{Event: "trace", TraceID: traceID})

	h.Bus.Publish(ctx, agent.NewMessage("user", supervisor.ID, agent.RoleUser, supervisor.TypeQuestion, in.Question, map[string]any{
		"trace_id":                     traceID,
		"account_id":                   userID,
		"csv":                          string(plain),
		protocol.MetaKeyUserID:         userID,
		protocol.MetaKeyUserRoles:      roles,
		protocol.MetaKeyClassification: string(doc.Classification),
	}))

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-deadline.C:
			_ = writeJSON(ctx, conn, chatEvent{Event: "error", Data: jsonString("timeout")})
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			if ev.To == "user" && ev.Type == reporter.TypeOut {
				_ = writeJSON(ctx, conn, chatEvent{Event: "report", TraceID: traceID, Data: jsonString(ev.Content)})
				return nil
			}
			summary, _ := json.Marshal(map[string]any{
				"from": ev.From, "to": ev.To, "type": ev.Type, "msg_id": ev.ID,
			})
			_ = writeJSON(ctx, conn, chatEvent{Event: "agent.handle", TraceID: traceID, Data: summary})
		}
	}
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

