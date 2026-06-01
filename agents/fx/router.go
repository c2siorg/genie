// Package fx provides the FX Router Agent, which handles multi-currency settlement
// optimization. It uses the FX quoter and optimizer to find optimal cross-border
// payment routes and present cost/speed tradeoffs to the settlement coordinator.
//
// The agent integrates with Laminar tracing for observability and can be
// configured with policy rules (e.g., prefer direct routes for <$1M, prefer
// netting for >$10M).
package fx

import (
	"context"
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/fx"
)

const (
	ID                  = "fx_router"
	CapRouteSettlement  = "route_multi_currency_settlement"
	TypeSettlementIn    = "multi_currency_settlement_request"
	TypeSettlementOut   = "settlement_paths"
	NextAgent           = "settlement_coordinator"
)

// SettlementRequest is the inbound multi-currency settlement request.
type SettlementRequest struct {
	RequestID      string           `json:"request_id"`       // Correlation ID / trace_id
	NetAmounts     []fx.NetAmount   `json:"net_amounts"`      // Currency flows to settle
	PreferredRoute string           `json:"preferred_route"`  // "cost" | "speed" | "balanced"
	Metadata       map[string]any   `json:"metadata,omitempty"`
}

// SettlementResponse is the outbound set of ranked settlement paths.
type SettlementResponse struct {
	RequestID  string               `json:"request_id"`
	Paths      []fx.SettlementPath  `json:"paths"`            // Ranked by cost (lower first)
	Recommend  fx.SettlementPath    `json:"recommended"`      // Best path given policy
	Error      string               `json:"error,omitempty"`
}

// Agent routes multi-currency settlement requests through the FX optimization engine.
type Agent struct {
	quoter      fx.FXQuoter
	optimizer   fx.SettlementPathOptimizer
	poolManager fx.LiquidityPoolManager
}

// New creates an FX router agent with the given quoter, optimizer, and pool manager.
func New(quoter fx.FXQuoter, optimizer fx.SettlementPathOptimizer, poolManager fx.LiquidityPoolManager) *Agent {
	return &Agent{
		quoter:      quoter,
		optimizer:   optimizer,
		poolManager: poolManager,
	}
}

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "FX Router" }
func (a *Agent) Capabilities() []string { return []string{CapRouteSettlement} }

// HandleMessage processes settlement requests and returns ranked paths.
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeSettlementIn {
		return nil, nil
	}

	var req SettlementRequest
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return a.rejectRequest(msg, "Failed to parse settlement request", req.RequestID)
	}

	if len(req.NetAmounts) == 0 {
		return a.rejectRequest(msg, "No net amounts provided", req.RequestID)
	}

	env.Logf("[fx_router] request_id=%s currencies=%d preferred=%s",
		req.RequestID, len(req.NetAmounts), req.PreferredRoute)

	// Find best paths
	paths, err := a.optimizer.FindBestPath(ctx, req.NetAmounts)
	if err != nil {
		return a.rejectRequest(msg, "No viable settlement paths: "+err.Error(), req.RequestID)
	}

	// Choose recommended path based on policy
	recommended := a.chooseRecommendedPath(paths, req.PreferredRoute)

	// Check liquidity constraints
	for _, path := range paths {
		for _, liq := range path.Liquidity {
			available, _ := a.poolManager.GetAvailable(liq.Currency)
			env.Logf("[fx_router] liquidity_check currency=%s available=%.2f required=%.2f",
				liq.Currency, available, liq.AvailableBalance)
		}
	}

	response := SettlementResponse{
		RequestID: req.RequestID,
		Paths:     paths,
		Recommend: recommended,
	}

	body, _ := json.Marshal(response)

	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeSettlementOut, string(body), msg.Metadata),
	}, nil
}

// chooseRecommendedPath applies policy rules to select the best path.
func (a *Agent) chooseRecommendedPath(paths []fx.SettlementPath, policy string) fx.SettlementPath {
	if len(paths) == 0 {
		return fx.SettlementPath{}
	}

	switch policy {
	case "cost":
		// Already sorted by cost; return first
		return paths[0]
	case "speed":
		// Sort by execution time
		best := paths[0]
		for _, p := range paths {
			if p.ExecutionTimeMs < best.ExecutionTimeMs {
				best = p
			}
		}
		return best
	case "balanced":
		// Prefer direct routes for typical amounts, netting for large volumes
		for _, p := range paths {
			if p.Route == fx.RouteDirect {
				return p
			}
		}
		return paths[0]
	default:
		// Default: cost-optimal
		return paths[0]
	}
}

// rejectRequest builds an error response.
func (a *Agent) rejectRequest(msg agent.Message, errMsg string, requestID string) ([]agent.Message, error) {
	if requestID == "" {
		// Extract from message if not provided
		var req SettlementRequest
		if err := json.Unmarshal([]byte(msg.Content), &req); err == nil {
			requestID = req.RequestID
		}
	}

	response := SettlementResponse{
		RequestID: requestID,
		Paths:     []fx.SettlementPath{},
		Error:     errMsg,
	}

	body, _ := json.Marshal(response)

	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeSettlementOut, string(body), msg.Metadata),
	}, nil
}
