package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeServer implements just enough of the MCP wire format to verify the client.
func fakeServer(t *testing.T, sessionID string) *httptest.Server {
	t.Helper()
	var initialized bool
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch req.Method {
		case "initialize":
			initialized = true
			w.Header().Set("Mcp-Session-Id", sessionID)
			writeRPCResult(w, req.ID, map[string]any{"protocolVersion": ProtocolVersion})
		case "notifications/initialized":
			// notification — no body required.
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if !initialized {
				http.Error(w, "not initialized", http.StatusBadRequest)
				return
			}
			if r.Header.Get("Mcp-Session-Id") != sessionID {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}
			writeRPCResult(w, req.ID, map[string]any{"tools": []Tool{
				{Name: "echo", Description: "echo the input", InputSchema: map[string]any{"type": "object"}},
			}})
		case "tools/call":
			writeRPCResult(w, req.ID, ToolResult{
				Content: []ToolContent{{Type: "text", Text: "hello world"}},
			})
		default:
			http.Error(w, "unknown method", http.StatusNotFound)
		}
	}))
}

// SSE-only handler used to verify the client refuses streaming responses.
func sseServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
}

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	w.Header().Set("Content-Type", "application/json")
	raw, _ := json.Marshal(result)
	_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: raw})
}

func TestClient_HappyPath(t *testing.T) {
	srv := fakeServer(t, "sess-1")
	defer srv.Close()
	c := NewClient(srv.URL)
	if err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.SessionID() != "sess-1" {
		t.Fatalf("session id: %q", c.SessionID())
	}
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	res, err := c.CallTool(context.Background(), "echo", map[string]any{"q": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) != 1 || !strings.Contains(res.Content[0].Text, "hello") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestClient_RefusesSSE(t *testing.T) {
	srv := sseServer(t)
	defer srv.Close()
	c := NewClient(srv.URL)
	if err := c.Initialize(context.Background()); err == nil {
		t.Fatal("expected SSE refusal")
	}
}
