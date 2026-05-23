// Package a2a implements the Agent-to-Agent protocol — a JSON-RPC surface
// that lets remote agents invoke each other as peers.
//
// The shape mirrors pkg/mcp: a small Client speaks JSON-RPC over HTTP, a
// matching Server hosts handlers. The split between MCP and A2A is
// deliberate:
//
//   - MCP exposes *tools* (function-like calls) for an LLM to invoke.
//   - A2A exposes *agents* (autonomous workers) for other agents to invoke.
//
// The RBI FREE-AI report explicitly names both as the emerging
// interoperability rails (para 2.1.8).
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ProtocolVersion is the A2A wire version Genie speaks.
const ProtocolVersion = "0.2.0"

// AgentCard describes one peer agent on the network. Similar to MCP's
// tool catalogue but at the agent-granularity instead of per-tool.
type AgentCard struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	URL         string         `json:"url"`
	Skills      []Skill        `json:"skills"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Skill is one capability advertised by an agent.
type Skill struct {
	ID          string         `json:"id"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Task is an A2A invocation. The naming follows Google's A2A reference.
type Task struct {
	ID      string         `json:"id"`
	SkillID string         `json:"skill_id"`
	Input   map[string]any `json:"input"`
}

// TaskResult is the typed response.
type TaskResult struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"` // "completed" | "failed" | "running"
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// JSON-RPC scaffolding — identical to pkg/mcp but kept separate so the two
// protocols can evolve independently.

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return e.Message }

// Client invokes a remote A2A endpoint.
type Client struct {
	Endpoint string
	HTTP     *http.Client
	Auth     string // "Bearer ..." optional

	mu        sync.Mutex
	idCounter int
}

// NewClient builds a Client.
func NewClient(endpoint string) *Client {
	return &Client{Endpoint: endpoint, HTTP: http.DefaultClient}
}

// GetAgentCard fetches the peer's AgentCard (analogous to MCP tools/list).
func (c *Client) GetAgentCard(ctx context.Context) (AgentCard, error) {
	raw, err := c.do(ctx, "agent/getCard", nil)
	if err != nil {
		return AgentCard{}, err
	}
	var card AgentCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return AgentCard{}, err
	}
	return card, nil
}

// SubmitTask sends a task and waits for the typed result.
func (c *Client) SubmitTask(ctx context.Context, task Task) (TaskResult, error) {
	raw, err := c.do(ctx, "task/submit", task)
	if err != nil {
		return TaskResult{}, err
	}
	var res TaskResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return TaskResult{}, err
	}
	if res.Status == "failed" {
		return res, errors.New("a2a task failed: " + res.Error)
	}
	return res, nil
}

func (c *Client) do(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.idCounter++
	id := c.idCounter
	c.mu.Unlock()

	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("A2A-Protocol-Version", ProtocolVersion)
	if c.Auth != "" {
		req.Header.Set("Authorization", c.Auth)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("a2a http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var rpc rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return nil, err
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("a2a rpc %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	return rpc.Result, nil
}

// Handler runs one skill server-side.
type Handler func(ctx context.Context, task Task) (TaskResult, error)

// Server exposes an A2A endpoint over HTTP.
type Server struct {
	Card     AgentCard
	Handlers map[string]Handler

	mu sync.Mutex
}

// NewServer builds a Server with the given AgentCard. Register handlers
// with Handle.
func NewServer(card AgentCard) *Server {
	return &Server{Card: card, Handlers: map[string]Handler{}}
}

// Handle installs (or replaces) a skill handler.
func (s *Server) Handle(skillID string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Handlers[skillID] = h
}

// ServeHTTP implements the A2A wire protocol.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	switch req.Method {
	case "agent/getCard":
		s.writeResult(w, req.ID, s.Card)
	case "task/submit":
		var t Task
		raw, _ := json.Marshal(req.Params)
		_ = json.Unmarshal(raw, &t)
		s.mu.Lock()
		h, ok := s.Handlers[t.SkillID]
		s.mu.Unlock()
		if !ok {
			s.writeError(w, req.ID, -32601, "unknown skill: "+t.SkillID)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		res, err := h(ctx, t)
		if err != nil {
			res.TaskID = t.ID
			res.Status = "failed"
			res.Error = err.Error()
		}
		if res.TaskID == "" {
			res.TaskID = t.ID
		}
		if res.Status == "" {
			res.Status = "completed"
		}
		s.writeResult(w, req.ID, res)
	default:
		s.writeError(w, req.ID, -32601, "method not found")
	}
}

func (s *Server) writeResult(w http.ResponseWriter, id any, result any) {
	raw, _ := json.Marshal(result)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: raw})
}

func (s *Server) writeError(w http.ResponseWriter, id any, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
}
