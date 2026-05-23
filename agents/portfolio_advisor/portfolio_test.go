package portfolio_advisor

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/mcp"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
)

type fakeTokenRepo struct {
	stored postgres.MCPToken
}

func (f *fakeTokenRepo) Upsert(_ context.Context, t postgres.MCPToken) (postgres.MCPToken, error) {
	f.stored = t
	return t, nil
}
func (f *fakeTokenRepo) Get(_ context.Context, _, _ string) (postgres.MCPToken, error) {
	return f.stored, nil
}

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestPortfolio_HappyPath(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	t.Setenv("GENIE_KEK_BASE64", base64.StdEncoding.EncodeToString(key))
	enc := crypto.New(crypto.NewEnvKeyResolver("test-kek"))

	// Encrypt a fake bearer token.
	tokenPayload, err := enc.Encrypt([]byte("bearer-from-kite"))
	if err != nil {
		t.Fatal(err)
	}

	// Stand up a fake MCP server that returns canned holdings/positions.
	s := mcp.NewServer(
		mcp.ServerTool{
			Name: "get_holdings", Description: "h", InputSchema: map[string]any{"type": "object"},
			Handler: func(_ context.Context, _ map[string]any) (mcp.ToolResult, error) {
				return mcp.ToolResult{Content: []mcp.ToolContent{{Type: "text", Text: "RELIANCE:10"}}}, nil
			},
		},
		mcp.ServerTool{
			Name: "get_positions", Description: "p", InputSchema: map[string]any{"type": "object"},
			Handler: func(_ context.Context, _ map[string]any) (mcp.ToolResult, error) {
				return mcp.ToolResult{Content: []mcp.ToolContent{{Type: "text", Text: "NIFTY:1"}}}, nil
			},
		},
	)
	srv := httptest.NewServer(s)
	defer srv.Close()

	repo := &fakeTokenRepo{stored: postgres.MCPToken{
		UserID:   "u-1",
		Provider: ProviderKite,
		Endpoint: srv.URL,
		Payload:  tokenPayload,
	}}

	a := New(repo, enc)
	msg := agent.NewMessage("supervisor", ID, agent.RoleUser, TypeQuestion, "show portfolio", map[string]any{
		protocol.MetaKeyUserID: "u-1",
	})
	out, err := a.HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeSnapshot {
		t.Fatalf("unexpected output: %+v", out)
	}
	var snap Snapshot
	if err := json.Unmarshal([]byte(out[0].Content), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Holdings == "" || snap.Positions == "" {
		t.Fatalf("snapshot missing fields: %+v", snap)
	}
	if snap.Classification != protocol.ClassPII {
		t.Errorf("classification should be PII, got %s", snap.Classification)
	}
}
