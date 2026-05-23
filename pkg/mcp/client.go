package mcp

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
)

// Client is a minimal MCP client over streamable HTTP.
//
// Lifecycle:
//
//  1. Construct with NewClient(endpoint).
//  2. Call Initialize() to negotiate the protocol version and capture the
//     Mcp-Session-Id header the server returns.
//  3. Use ListTools and CallTool. Both reuse the captured session id.
//
// The client is safe for concurrent use after Initialize returns. Use one
// Client per remote endpoint per process.
type Client struct {
	Endpoint string
	HTTP     *http.Client

	// Authorization is optional. If set it is sent as the Authorization
	// header on every request. Use "Bearer <token>" for hosted servers like
	// mcp.kite.trade.
	Authorization string

	// ClientInfo identifies Genie to the server. Defaults are filled in by
	// Initialize.
	ClientInfo ClientInfo

	mu        sync.Mutex
	sessionID string
	idCounter int
}

// ClientInfo is sent on initialize.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewClient builds a Client targeting the given JSON-RPC URL. For Zerodha
// hosted MCP use "https://mcp.kite.trade/mcp".
func NewClient(endpoint string) *Client {
	return &Client{
		Endpoint: endpoint,
		HTTP:     http.DefaultClient,
		ClientInfo: ClientInfo{
			Name:    "genie",
			Version: "0.1.0",
		},
	}
}

// SessionID returns the captured Mcp-Session-Id, useful for logging.
func (c *Client) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

// Initialize handshakes with the server and stores the session id for later
// calls. Most public MCP servers refuse tools/* before this.
func (c *Client) Initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      c.ClientInfo,
	}
	resp, _, err := c.do(ctx, "initialize", params, true)
	if err != nil {
		return err
	}
	// We don't need to parse the result yet — capabilities discovery is a
	// follow-up. The session id (if any) was captured by do().
	_ = resp
	// Per spec, send the "notifications/initialized" notification.
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("post-init notify: %w", err)
	}
	return nil
}

// ListTools returns the catalogue advertised by the server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	raw, _, err := c.do(ctx, "tools/list", map[string]any{}, false)
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode tools/list: %w", err)
	}
	return out.Tools, nil
}

// CallTool invokes a tool by name with the given JSON arguments.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (ToolResult, error) {
	raw, _, err := c.do(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	}, false)
	if err != nil {
		return ToolResult{}, err
	}
	var res ToolResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return ToolResult{}, fmt.Errorf("decode tools/call: %w", err)
	}
	if res.IsError {
		return res, fmt.Errorf("tool %q reported error", name)
	}
	return res, nil
}

// do is the JSON-RPC POST.  Captures the Mcp-Session-Id response header on the
// first call (typically initialize) and replays it on subsequent calls.
//
// The streamable HTTP transport may respond with either application/json
// (single response) or text/event-stream (SSE). We currently only consume the
// single-response form; SSE is left as a follow-up. Most query-style MCP
// servers (Kite included) reply with JSON for tool calls.
func (c *Client) do(ctx context.Context, method string, params any, capture bool) (json.RawMessage, http.Header, error) {
	c.mu.Lock()
	c.idCounter++
	id := c.idCounter
	sess := c.sessionID
	c.mu.Unlock()

	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	if c.Authorization != "" {
		req.Header.Set("Authorization", c.Authorization)
	}
	if sess != "" {
		req.Header.Set("Mcp-Session-Id", sess)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if capture {
		if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
			c.mu.Lock()
			c.sessionID = sid
			c.mu.Unlock()
		}
	}

	if resp.StatusCode == http.StatusAccepted {
		// Server accepted but is streaming the response over SSE — not
		// supported yet. Drain and return a friendly error.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, resp.Header, errors.New("mcp: server returned SSE stream which this client does not yet consume")
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, resp.Header, fmt.Errorf("mcp: http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var rpc rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return nil, resp.Header, fmt.Errorf("decode response: %w", err)
	}
	if rpc.Error != nil {
		return nil, resp.Header, fmt.Errorf("mcp rpc error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	return rpc.Result, resp.Header, nil
}

// notify sends a JSON-RPC notification (no id, no expected response).
func (c *Client) notify(ctx context.Context, method string, params any) error {
	body, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
	}{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	if c.Authorization != "" {
		req.Header.Set("Authorization", c.Authorization)
	}
	c.mu.Lock()
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	c.mu.Unlock()

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mcp notify http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
