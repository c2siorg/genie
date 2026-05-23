package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/busio"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/google/uuid"
)

// ServerTool advertises one Genie capability over MCP. The Handler runs
// in-process and can either compute synchronously or publish onto the bus
// and await the response via a Correlator.
type ServerTool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ServerToolHandler
}

// ServerToolHandler runs a single MCP tool invocation.
type ServerToolHandler func(ctx context.Context, args map[string]any) (ToolResult, error)

// Server is a minimal MCP-over-HTTP server. Mount it on any HTTP path (e.g.
// /mcp) and it speaks JSON-RPC 2.0.
type Server struct {
	Tools []ServerTool

	mu       sync.Mutex
	sessions map[string]time.Time
}

// NewServer builds a Server with the given tools.
func NewServer(tools ...ServerTool) *Server {
	return &Server{Tools: tools, sessions: map[string]time.Time{}}
}

// ServeHTTP implements the streamable-HTTP MCP transport (JSON only).
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
	case "initialize":
		id := uuid.NewString()
		s.mu.Lock()
		s.sessions[id] = time.Now()
		s.mu.Unlock()
		w.Header().Set("Mcp-Session-Id", id)
		s.writeResult(w, req.ID, map[string]any{
			"protocolVersion": ProtocolVersion,
			"serverInfo":      map[string]any{"name": "genie-mcp", "version": "0.1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		})
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		tools := make([]Tool, 0, len(s.Tools))
		for _, t := range s.Tools {
			tools = append(tools, Tool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
		}
		s.writeResult(w, req.ID, map[string]any{"tools": tools})
	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]any         `json:"arguments"`
		}
		raw, _ := json.Marshal(req.Params)
		_ = json.Unmarshal(raw, &params)
		tool := s.findTool(params.Name)
		if tool == nil {
			s.writeError(w, req.ID, -32601, fmt.Sprintf("unknown tool %q", params.Name))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		res, err := tool.Handler(ctx, params.Arguments)
		if err != nil {
			s.writeResult(w, req.ID, ToolResult{
				IsError: true,
				Content: []ToolContent{{Type: "text", Text: err.Error()}},
			})
			return
		}
		s.writeResult(w, req.ID, res)
	default:
		s.writeError(w, req.ID, -32601, "method not found")
	}
}

func (s *Server) findTool(name string) *ServerTool {
	for i := range s.Tools {
		if s.Tools[i].Name == name {
			return &s.Tools[i]
		}
	}
	return nil
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

// BusTool wraps an agent so a single tool call publishes onto the bus and
// awaits the agent's first reply via the supplied Correlator.
//
// Use this to expose read-only agents (educator, currency, macro, rates) as
// MCP tools without duplicating their logic.
func BusTool(name, description string, schema map[string]any, bus comm.Bus, corr *busio.Correlator, target, msgType string) ServerTool {
	return ServerTool{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Handler: func(ctx context.Context, args map[string]any) (ToolResult, error) {
			content, _ := args["query"].(string)
			traceID := fmt.Sprintf("mcp-%s", uuid.NewString())
			ch := corr.Await(traceID)
			bus.Publish(ctx, agent.NewMessage("mcp", target, agent.RoleUser, msgType, content, map[string]any{
				"trace_id": traceID,
				// Bus replies are routed back to this corr by trace_id.
				protocol.MetaKeyClassification: string(protocol.ClassInternal),
			}))
			select {
			case msg := <-ch:
				return ToolResult{Content: []ToolContent{{Type: "text", Text: msg.Content}}}, nil
			case <-time.After(5 * time.Second):
				corr.Cancel(traceID)
				return ToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: "timeout"}}}, nil
			case <-ctx.Done():
				corr.Cancel(traceID)
				return ToolResult{IsError: true, Content: []ToolContent{{Type: "text", Text: ctx.Err().Error()}}}, nil
			}
		},
	}
}
