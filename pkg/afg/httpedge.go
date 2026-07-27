package afg

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/sovereignty"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"

	"github.com/go-chi/chi/v5"
)

// AskRequest is the framework-native /v1/ask payload. Identity (user id,
// roles) is NOT carried here — it comes from the authenticated JWT claims on
// the request context (see identityFromContext in edge_auth.go). Classification
// and region remain request attributes: they describe the request being made,
// not who is making it.
type AskRequest struct {
	Agent          string `json:"agent"`
	Input          string `json:"input"`
	Type           string `json:"type"`           // governance message type; defaults to Agent
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
//
// /healthz and /readyz stay public (no auth, no rate limit — k8s probes must
// not be throttled or challenged). /v1/ask and /v1/ai-inventory sit behind the
// real production chain: RequestID → Recovery → AccessLog → mid.Auth(issuer) →
// rate limit, mirroring the ordering in pkg/web/router.go.
func NewHandler(reg *Registry, issuer *auth.Issuer) http.Handler {
	r := chi.NewRouter()
	r.Use(mid.RequestID)
	r.Use(mid.Recovery(nil))
	r.Use(mid.AccessLog(nil))

	// Public routes — no auth, no rate limit.
	r.Group(func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
		r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ready")) })
	})

	// Protected routes — real JWT auth, then rate limiting.
	limiter := mid.NewRateLimit(60, 1.0) // 60-req burst, 1/sec refill
	r.Group(func(r chi.Router) {
		r.Use(mid.Auth(issuer))
		r.Use(limiter.Middleware)

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
			userID, rolesCSV, _ := identityFromContext(req.Context())
			pm := protocol.Message{
				From: "user", Role: protocol.RoleUser, Type: mtype, Content: in.Input,
				Metadata: map[string]any{
					protocol.MetaKeyUserRoles:      rolesCSV,
					protocol.MetaKeyClassification: in.Classification,
					protocol.MetaKeyUserID:         userID,
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
	})

	return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
