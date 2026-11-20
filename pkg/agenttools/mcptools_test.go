package agenttools_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/mcp"
)

// stubTool builds a simple mcp.ServerTool for use in tests.
func stubTool(name, desc string, fn func(map[string]any) (string, error)) mcp.ServerTool {
	return mcp.ServerTool{
		Name:        name,
		Description: desc,
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			text, err := fn(args)
			if err != nil {
				return mcp.ToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, err
			}
			return mcp.ToolResult{Content: []mcp.ToolContent{{Type: "text", Text: text}}}, nil
		},
	}
}

func TestMCPTools_DiscoverAndCall(t *testing.T) {
	srv := mcp.NewServer(
		stubTool("greet", "Say hello", func(args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			if name == "" {
				name = "world"
			}
			return "Hello, " + name + "!", nil
		}),
		stubTool("add", "Add two numbers", func(args map[string]any) (string, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return fmt.Sprintf("%.0f", a+b), nil
		}),
	)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	client := mcp.NewClient(ts.URL)
	reg, err := agenttools.MCPTools(context.Background(), client)
	if err != nil {
		t.Fatalf("MCPTools: %v", err)
	}

	if len(reg.Definitions()) != 2 {
		t.Errorf("expected 2 tools, got %d", len(reg.Definitions()))
	}

	result, err := reg.Execute(context.Background(), "greet", map[string]any{"name": "Pratik"})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result from greet")
	}
}

func TestAppendMCPTools_AddsToExisting(t *testing.T) {
	srv := mcp.NewServer(
		stubTool("ping", "Ping pong", func(_ map[string]any) (string, error) { return "pong", nil }),
	)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	reg := agenttools.FileTools()
	before := len(reg.Definitions())

	client := mcp.NewClient(ts.URL)
	if err := agenttools.AppendMCPTools(context.Background(), reg, client); err != nil {
		t.Fatal(err)
	}

	after := len(reg.Definitions())
	if after != before+1 {
		t.Errorf("expected %d tools, got %d", before+1, after)
	}
}
