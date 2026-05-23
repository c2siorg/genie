// Package mcp implements a minimal Model Context Protocol surface: a client
// for calling remote MCP servers (Zerodha Kite, Plaid, GitHub, etc.) and a
// server that exposes Genie's read-only agents as MCP tools.
//
// MCP is JSON-RPC 2.0. Genie targets the "streamable HTTP" transport since
// that's what hosted MCP servers like mcp.kite.trade speak.
package mcp

import "encoding/json"

// ProtocolVersion is the MCP version Genie speaks. Bump when supporting newer wire features.
const ProtocolVersion = "2025-06-18"

// RPC envelope helpers — Genie keeps its own minimal types instead of pulling
// a third-party JSON-RPC library so the wire surface stays auditable.

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  any         `json:"params,omitempty"`
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

// Tool describes one tool advertised by an MCP server.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolResult is the structured response returned by tools/call.
type ToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent is the discriminated union an MCP server emits inside a result.
// Genie only cares about text payloads for now; binary/image is left as a
// follow-up and surfaces as a TODO if encountered.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// Data and MIMEType are populated for image/audio content; ignored today.
	Data     string `json:"data,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
}
