// Package transports implements transport layers for agents.
// Agents can run: locally (memory), via HTTP, via gRPC, etc.
//
// This file: HTTP transport - expose agents as HTTP endpoints.
// Usage: Wrap an Agent, get HTTP handlers that serve it.
package transports

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	"github.com/google/uuid"
)

// defaultMaxBodyBytes caps request bodies to protect agent pods from oversized
// or malicious payloads. 10 MiB is generous for a JSON agent message.
const defaultMaxBodyBytes = 10 << 20 // 10 MiB

// HTTPServer wraps an agent and exposes it via HTTP.
type HTTPServer struct {
	agent   core.Agent
	mux     *http.ServeMux
	token   string // shared-secret X-Agent-Token; empty disables auth
	maxBody int64
}

// Option configures an HTTPServer.
type Option func(*HTTPServer)

// WithToken requires every non-health request to present a matching
// X-Agent-Token header (validated in constant time). An empty token disables
// auth (in-cluster dev only).
func WithToken(token string) Option { return func(s *HTTPServer) { s.token = token } }

// WithMaxBodyBytes overrides the request body size cap.
func WithMaxBodyBytes(n int64) Option {
	return func(s *HTTPServer) {
		if n > 0 {
			s.maxBody = n
		}
	}
}

// NewHTTPServer creates a new HTTP server for an agent.
func NewHTTPServer(agent core.Agent, opts ...Option) *HTTPServer {
	s := &HTTPServer{
		agent:   agent,
		mux:     http.NewServeMux(),
		maxBody: defaultMaxBodyBytes,
	}
	for _, o := range opts {
		o(s)
	}

	// Register routes
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /execute", s.handleExecute)
	s.mux.HandleFunc("GET /info", s.handleInfo)

	return s
}

// ServeHTTP implements http.Handler. It enforces the shared-secret token on
// every path except /health (which must stay open for Kubernetes liveness and
// readiness probes).
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": "missing or invalid X-Agent-Token"})
		return
	}
	s.mux.ServeHTTP(w, r)
}

// authorized reports whether the request may proceed. Health checks are always
// allowed; otherwise, when a token is configured, the X-Agent-Token header must
// match in constant time.
func (s *HTTPServer) authorized(r *http.Request) bool {
	if r.URL.Path == "/health" || s.token == "" {
		return true
	}
	got := r.Header.Get("X-Agent-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}

// handleHealth checks agent health.
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.agent.Health(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"agent":  s.agent.Name(),
	})
}

// handleInfo returns agent metadata.
func (s *HTTPServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    s.agent.Name(),
		"version": s.agent.Version(),
	})
}

// handleExecute runs the agent.
// Request body: {"user_id": "...", "payload": {...}}
// Response: {"agent_name": "...", "status": "success", "result": {...}}
func (s *HTTPServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	// Cap the request body to defend against oversized payloads.
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)

	// Parse request
	var req core.AgentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	// Extract/create execution context
	ctx := r.Context()
	if req.TraceID == "" {
		req.TraceID = generateTraceID(s.agent.Name())
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	// Create execution context and embed in request context
	execCtx := core.ExecutionContext{
		UserID:   req.UserID,
		TraceID:  req.TraceID,
		Metadata: req.Metadata,
	}
	ctx = core.WithContext(ctx, execCtx)

	// Execute agent
	result, err := s.agent.Execute(ctx, req.Payload)

	// Build response
	resp := core.AgentOutput{
		AgentName: s.agent.Name(),
		TraceID:   req.TraceID,
	}

	if err != nil {
		resp.Status = "error"
		resp.Error = err.Error()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		resp.Status = "success"
		resp.Result = result
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(resp)
}

// StreamHTTPServer is like HTTPServer but supports Server-Sent Events for streaming.
// Useful for agents that generate recommendations with progress events.
type StreamHTTPServer struct {
	agent core.Agent
	mux   *http.ServeMux
}

// NewStreamHTTPServer creates a streaming HTTP server.
func NewStreamHTTPServer(agent core.Agent) *StreamHTTPServer {
	s := &StreamHTTPServer{
		agent: agent,
		mux:   http.NewServeMux(),
	}

	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /execute-stream", s.handleExecuteStream)
	s.mux.HandleFunc("GET /info", s.handleInfo)

	return s
}

// ServeHTTP implements http.Handler.
func (s *StreamHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *StreamHTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.agent.Health(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"agent":  s.agent.Name(),
	})
}

func (s *StreamHTTPServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    s.agent.Name(),
		"version": s.agent.Version(),
	})
}

// handleExecuteStream runs agent and streams result via SSE.
func (s *StreamHTTPServer) handleExecuteStream(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req core.AgentInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf(`{"error":"invalid request: %v"}`, err))
		return
	}

	// Set up SSE. NOTE: no wildcard CORS header — these are internal,
	// token-authenticated agent endpoints, not browser-facing. A permissive
	// Access-Control-Allow-Origin would let any origin probe them.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"error":"streaming not supported"}`)
		return
	}

	// Extract/create execution context
	ctx := r.Context()
	if req.TraceID == "" {
		req.TraceID = generateTraceID(s.agent.Name())
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	execCtx := core.ExecutionContext{
		UserID:   req.UserID,
		TraceID:  req.TraceID,
		Metadata: req.Metadata,
	}
	ctx = core.WithContext(ctx, execCtx)

	// Send initial event
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(map[string]interface{}{
		"event":    "started",
		"trace_id": req.TraceID,
	}))
	flusher.Flush()

	// Execute agent
	result, err := s.agent.Execute(ctx, req.Payload)

	// Send result
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", mustMarshal(map[string]interface{}{
			"event": "error",
			"error": err.Error(),
		}))
	} else {
		fmt.Fprintf(w, "data: %s\n\n", mustMarshal(map[string]interface{}{
			"event":  "completed",
			"result": result,
		}))
	}
	flusher.Flush()
}

// Helpers

func mustMarshal(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// generateTraceID builds a globally-unique trace identifier for a request that
// arrives without one. Previously this used a hardcoded constant, which caused
// every request to the same agent to collide on TraceID (tr-<agent>-1000),
// making audit trails and distributed traces unsearchable. A UUID guarantees
// uniqueness per request. Mirrors the idiom in pkg/mcp/server.go.
func generateTraceID(agentName string) string {
	return fmt.Sprintf("tr-%s-%s", agentName, uuid.NewString())
}
