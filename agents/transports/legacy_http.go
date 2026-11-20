package transports

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	pkgagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// LegacyHTTPServer exposes a pkg/agent.Agent (the HandleMessage contract used by
// the ~34 registered agents) over HTTP via POST /handle. Unlike HTTPServer
// (which serves the agents/core.Agent Execute contract for the advisor agents),
// this server preserves the full protocol.Message — From/To/Role/Type/Content/
// Metadata — in both directions, so an agent's routing fields survive the wire.
//
// Wire contract:
//
//	POST /handle  body: protocol.Message            -> []protocol.Message
//	GET  /health                                    -> {"status":"healthy"}
//	GET  /info                                      -> {"id","name","capabilities"}
//
// The backend-side counterpart is pkg/registry.HTTPRegistryAgent, which marshals
// the orchestrator's message into this endpoint and republishes the returned
// messages — preserving sub-orchestration (e.g. supervisor → analyzer → …).
type LegacyHTTPServer struct {
	agent   pkgagent.Agent
	mux     *http.ServeMux
	token   string
	maxBody int64
}

// NewLegacyHTTPServer wraps a pkg/agent.Agent. Options reuse the HTTPServer
// Option type (WithToken, WithMaxBodyBytes) for a consistent surface.
func NewLegacyHTTPServer(agent pkgagent.Agent, opts ...Option) *LegacyHTTPServer {
	// Reuse the Option plumbing by applying options to a temporary HTTPServer.
	tmp := &HTTPServer{maxBody: defaultMaxBodyBytes}
	for _, o := range opts {
		o(tmp)
	}
	s := &LegacyHTTPServer{
		agent:   agent,
		mux:     http.NewServeMux(),
		token:   tmp.token,
		maxBody: tmp.maxBody,
	}
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /handle", s.handleHandle)
	s.mux.HandleFunc("GET /info", s.handleInfo)
	return s
}

// ServeHTTP enforces the shared-secret token on every path except /health.
func (s *LegacyHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/health" && s.token != "" {
		got := r.Header.Get("X-Agent-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "missing or invalid X-Agent-Token"})
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

// legacyEnv is the minimal pkg/agent.Environment supplied to the wrapped agent.
type legacyEnv struct{}

func (legacyEnv) Now() time.Time                  { return time.Now() }
func (legacyEnv) Logf(format string, args ...any) {} // transport-level logging handled elsewhere

func (s *LegacyHTTPServer) handleHandle(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)

	var msg protocol.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid message: " + err.Error()})
		return
	}

	out, err := s.agent.HandleMessage(r.Context(), msg, legacyEnv{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	// Always return a JSON array (never null) so the client can decode cleanly.
	if out == nil {
		out = []protocol.Message{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *LegacyHTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "healthy", "agent": s.agent.ID()})
}

func (s *LegacyHTTPServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           s.agent.ID(),
		"name":         s.agent.Name(),
		"capabilities": s.agent.Capabilities(),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
