// mcptools.go — Lesson 12: MCP Tool Bridge
//
// MCPTools auto-discovers every tool advertised by a remote MCP server and
// wraps each one as an agenttools.Tool so the agent can use them without any
// special handling — the MCP protocol is transparent.
//
// Usage:
//
//	client := mcp.NewClient("https://mcp.example.com/mcp")
//	reg, err := agenttools.MCPTools(ctx, client)
//	runner := agentic.Runner{Registry: reg}
//
// To merge MCP tools with local tools use Registry.Merge (or register them
// both into a single registry manually):
//
//	local  := agenttools.FileTools()
//	remote, _ := agenttools.MCPTools(ctx, client)
//	for _, def := range remote.Definitions() {
//	    local.Register(remote.ToolByName(def["function"].(map[string]any)["name"].(string)))
//	}
package agenttools

import (
	"context"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/mcp"
)

// MCPTools initialises the MCP client, fetches the server's tool catalogue,
// and returns a Registry where each remote tool is wrapped as a local Tool.
//
// The client's Initialize handshake is performed here. Call once per process.
// The returned Registry can be shared across Runners safely (all calls are
// stateless reads after Initialize).
func MCPTools(ctx context.Context, client *mcp.Client) (*Registry, error) {
	if err := client.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("mcp bridge: initialize: %w", err)
	}
	tools, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp bridge: list tools: %w", err)
	}

	r := NewRegistry()
	for _, t := range tools {
		tool := t // capture loop variable
		r.Register(mcpToolAdapter(client, tool))
	}
	return r, nil
}

// mcpToolAdapter wraps a single mcp.Tool as an agenttools.Tool.
// The adapter forwards every Execute call to client.CallTool and converts the
// MCP ToolResult back into a plain string for the LLM.
func mcpToolAdapter(client *mcp.Client, t mcp.Tool) Tool {
	// Use InputSchema directly as the JSON Schema for the tool. MCP servers
	// provide this in the standard JSON Schema format that Ollama/OpenAI expect.
	schema := t.InputSchema
	if schema == nil {
		schema = map[string]any{"type": "object", "properties": map[string]any{}}
	}

	return &ToolDef{
		ToolName:   t.Name,
		ToolDescription:   t.Description,
		ToolSchema: schema,
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			res, err := client.CallTool(ctx, t.Name, args)
			if err != nil {
				// Return error as a string rather than a Go error so the agent
				// can reason about it and decide whether to retry or escalate.
				return fmt.Sprintf("mcp tool error (%s): %v", t.Name, err), nil
			}
			return mcpResultText(res), nil
		},
	}
}

// mcpResultText joins all text content items from an MCP ToolResult.
// Image/audio/binary items are noted but their data is not included.
func mcpResultText(res mcp.ToolResult) string {
	var parts []string
	for _, c := range res.Content {
		switch c.Type {
		case "text":
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		case "image":
			parts = append(parts, "[image content — not rendered]")
		default:
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
	}
	if len(parts) == 0 {
		return "(empty result)"
	}
	return strings.Join(parts, "\n")
}

// AppendMCPTools adds all tools from an MCP client into an existing Registry.
// Use this to combine local tools with remote ones:
//
//	reg := agenttools.AllTools()
//	if err := agenttools.AppendMCPTools(ctx, reg, mcpClient); err != nil {
//	    log.Printf("mcp unavailable: %v", err)
//	}
//	runner := agentic.Runner{Registry: reg}
func AppendMCPTools(ctx context.Context, reg *Registry, client *mcp.Client) error {
	if err := client.Initialize(ctx); err != nil {
		return fmt.Errorf("mcp bridge: initialize: %w", err)
	}
	tools, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("mcp bridge: list tools: %w", err)
	}
	for _, t := range tools {
		tool := t
		reg.Register(mcpToolAdapter(client, tool))
	}
	return nil
}
