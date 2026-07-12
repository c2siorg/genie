package afg

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/sovereignty"

	"github.com/go-chi/chi/v5"
)

// AskRequest is the framework-native /v1/ask payload. Auth/RBAC context
// (user_id/roles/classification/region) would in production be injected by JWT
// middleware; here it is carried on the request so the governance gate can evaluate
// it exactly as the bus edge did.
type AskRequest struct {
	Agent          string `json:"agent"`
	Input          string `json:"input"`
	Type           string `json:"type"` // governance message type; defaults to Agent
	UserID         string `json:"user_id"`
	Roles          string `json:"roles"`          // comma-separated
	Classification string `json:"classification"` // public|internal|pii|secret
	Region         string `json:"region"`
}

// AskResponse is the /v1/ask reply.
type AskResponse struct {
	Agent        string `json:"agent"`
	Output       string `json:"output"`
	UsedFallback bool   `json:"used_fallback"`
}

// NewHandler is the framework-native HTTP edge. /v1/ask routes to the governed
// registry (gate → agent → fallback); /v1/ai-inventory serves the live inventory.
// No Postgres or message bus required — the registry is the source of truth.
func NewHandler(reg *Registry) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ready")) })
	r.Get("/v1/ai-inventory", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, reg.Inventory())
	})
	r.Post("/v1/ask", func(w http.ResponseWriter, req *http.Request) {
		var in AskRequest
		if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if in.Agent == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent is required"})
			return
		}
		mtype := in.Type
		if mtype == "" {
			mtype = in.Agent
		}
		pm := protocol.Message{
			From: "user", Role: protocol.RoleUser, Type: mtype, Content: in.Input,
			Metadata: map[string]any{
				protocol.MetaKeyUserRoles:      in.Roles,
				protocol.MetaKeyClassification: in.Classification,
				protocol.MetaKeyUserID:         in.UserID,
				sovereignty.MetaKeyRegion:      in.Region,
			},
		}
		ctx := WithGovMessage(req.Context(), pm)
		out, usedFB, err := reg.RunWithFallback(ctx, in.Agent, in.Input)
		if err != nil {
			var denied *DeniedError
			if errors.As(err, &denied) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error(), "decision": "denied"})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, AskResponse{Agent: in.Agent, Output: out, UsedFallback: usedFB})
	})
	return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
