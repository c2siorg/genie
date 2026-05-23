package mcp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServer_ListAndCall(t *testing.T) {
	s := NewServer(ServerTool{
		Name:        "ping",
		Description: "ping",
		InputSchema: map[string]any{"type": "object"},
		Handler: func(ctx context.Context, args map[string]any) (ToolResult, error) {
			return ToolResult{Content: []ToolContent{{Type: "text", Text: "pong"}}}, nil
		},
	})
	srv := httptest.NewServer(s)
	defer srv.Close()
	c := NewClient(srv.URL)
	if err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "ping" {
		t.Fatalf("got %+v", tools)
	}
	res, err := c.CallTool(context.Background(), "ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content[0].Text, "pong") {
		t.Fatalf("unexpected: %+v", res)
	}
}
