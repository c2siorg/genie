package fx

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentpkg "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/fx"
)

// MockEnvironment implements agent.Environment for testing.
type MockEnvironment struct {
	logs []string
}

func (m *MockEnvironment) Now() time.Time             { return time.Now() }
func (m *MockEnvironment) Logf(format string, args ...interface{}) {
	m.logs = append(m.logs, "log")
}

func TestFXRouterHandleMessage(t *testing.T) {
	quoter := fx.NewMockQuoter()
	poolManager := fx.NewMockLiquidityPoolManager()
	optimizer := fx.NewMockPathOptimizer(quoter, poolManager)
	agent := New(quoter, optimizer, poolManager)

	ctx := context.Background()

	tests := []struct {
		name          string
		messageType   string
		requestBody   interface{}
		shouldErr     bool
		expectPaths   bool
	}{
		{
			name:        "Valid settlement request",
			messageType: TypeSettlementIn,
			requestBody: SettlementRequest{
				RequestID: "req-001",
				NetAmounts: []fx.NetAmount{
					{Currency: "USD", Amount: -100_000.0},
					{Currency: "INR", Amount: 5_000_000.0},
				},
				PreferredRoute: "cost",
			},
			shouldErr:   false,
			expectPaths: true,
		},
		{
			name:        "Empty net amounts",
			messageType: TypeSettlementIn,
			requestBody: SettlementRequest{
				RequestID:  "req-002",
				NetAmounts: []fx.NetAmount{},
			},
			shouldErr:   false,
			expectPaths: false,
		},
		{
			name:        "Wrong message type",
			messageType: "unknown_type",
			requestBody: SettlementRequest{
				RequestID: "req-003",
			},
			shouldErr:   false,
			expectPaths: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			msg := agentpkg.NewMessage("test", ID, agentpkg.RoleAgent, tt.messageType, string(body), nil)

			env := &MockEnvironment{}
			responses, err := agent.HandleMessage(ctx, msg, env)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(responses) > 0 {
				var resp SettlementResponse
				if err := json.Unmarshal([]byte(responses[0].Content), &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				if tt.expectPaths && len(resp.Paths) == 0 {
					t.Errorf("expected paths, got none")
				}
				if !tt.expectPaths && len(resp.Paths) > 0 {
					t.Errorf("expected no paths, got %d", len(resp.Paths))
				}
			}
		})
	}
}

func TestFXRouterChooseRecommendedPath(t *testing.T) {
	quoter := fx.NewMockQuoter()
	poolManager := fx.NewMockLiquidityPoolManager()
	optimizer := fx.NewMockPathOptimizer(quoter, poolManager)
	agent := New(quoter, optimizer, poolManager)

	paths := []fx.SettlementPath{
		{
			Route:           fx.RouteDirect,
			EstimatedCostBps: 100,
			ExecutionTimeMs: 14400000,
		},
		{
			Route:           fx.RouteCorrespondent,
			EstimatedCostBps: 75,
			ExecutionTimeMs: 28800000,
		},
		{
			Route:           fx.RouteNettingPool,
			EstimatedCostBps: 50,
			ExecutionTimeMs: 86400000,
		},
	}

	tests := []struct {
		name          string
		policy        string
		expectedRoute fx.SettlementRoute
	}{
		{"Cost policy", "cost", fx.RouteDirect},           // First path (lowest cost)
		{"Speed policy", "speed", fx.RouteDirect},         // Fastest execution
		{"Balanced policy", "balanced", fx.RouteDirect},   // Prefers direct
		{"Unknown policy", "unknown", fx.RouteDirect},     // Falls back to cost
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chosen := agent.chooseRecommendedPath(paths, tt.policy)
			if chosen.Route != tt.expectedRoute {
				t.Errorf("expected route %v, got %v", tt.expectedRoute, chosen.Route)
			}
		})
	}
}

func TestFXRouterIntegration(t *testing.T) {
	// Full integration test: request -> route -> response
	quoter := fx.NewMockQuoter()
	poolManager := fx.NewMockLiquidityPoolManager()
	optimizer := fx.NewMockPathOptimizer(quoter, poolManager)
	fxAgent := New(quoter, optimizer, poolManager)

	ctx := context.Background()

	// Real-world scenario: settlement of multi-currency flows
	request := SettlementRequest{
		RequestID: "settlement-20240531-001",
		NetAmounts: []fx.NetAmount{
			{Currency: "USD", Amount: -100_000.0},  // Outflow
			{Currency: "EUR", Amount: 50_000.0},    // Inflow
			{Currency: "GBP", Amount: 30_000.0},    // Inflow
			{Currency: "INR", Amount: -5_000_000.0}, // Outflow
		},
		PreferredRoute: "cost",
	}

	body, _ := json.Marshal(request)
	msg := agentpkg.NewMessage("settlement", ID, agentpkg.RoleAgent, TypeSettlementIn, string(body), nil)

	env := &MockEnvironment{}
	responses, err := fxAgent.HandleMessage(ctx, msg, env)

	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	var resp SettlementResponse
	if err := json.Unmarshal([]byte(responses[0].Content), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.RequestID != request.RequestID {
		t.Errorf("request ID mismatch")
	}

	if len(resp.Paths) == 0 {
		t.Errorf("expected settlement paths")
	}

	if resp.Recommend.Route == "" {
		t.Errorf("expected recommended path")
	}

	// Log the results for inspection
	t.Logf("Found %d settlement paths:", len(resp.Paths))
	for i, path := range resp.Paths {
		t.Logf("  Path %d: route=%v cost=%d bps execution=%d ms",
			i, path.Route, path.EstimatedCostBps, path.ExecutionTimeMs)
	}

	t.Logf("Recommended: route=%v cost=%d bps execution=%d ms",
		resp.Recommend.Route, resp.Recommend.EstimatedCostBps, resp.Recommend.ExecutionTimeMs)
}

func TestFXRouterIDAndCapabilities(t *testing.T) {
	quoter := fx.NewMockQuoter()
	poolManager := fx.NewMockLiquidityPoolManager()
	optimizer := fx.NewMockPathOptimizer(quoter, poolManager)
	fxAgent := New(quoter, optimizer, poolManager)

	if fxAgent.ID() != ID {
		t.Errorf("agent ID mismatch: expected %s, got %s", ID, fxAgent.ID())
	}

	capabilities := fxAgent.Capabilities()
	if len(capabilities) != 1 || capabilities[0] != CapRouteSettlement {
		t.Errorf("unexpected capabilities: %v", capabilities)
	}

	if fxAgent.Name() == "" {
		t.Errorf("agent should have a name")
	}
}
