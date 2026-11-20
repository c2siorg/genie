// router.go — RouterAgent sub-agent (Lesson 02: Tools)
//
// RouterAgent applies settlement policy rules to assign each net amount
// to a settlement path (direct_bank, correspondent, or netting_pool).
// Path selection depends on amount, counterparty ratings, and policy preferences.
package settlement

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	sett "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/settlement"
)

// RouterAgent routes net settlements via appropriate channels.
type RouterAgent struct {
	// Policy defines routing rules
	Policy *sett.SettlementPolicy
}

// NewRouterAgent returns a router with the given policy.
func NewRouterAgent(policy *sett.SettlementPolicy) *RouterAgent {
	if policy == nil {
		p := sett.DefaultPolicy()
		policy = &p
	}
	return &RouterAgent{Policy: policy}
}

// Registry returns the tool set for the router.
func (ra *RouterAgent) Registry() *agenttools.Registry {
	reg := agenttools.NewRegistry()

	// route_settlement: select settlement path
	reg.Register(&agenttools.ToolDef{
		ToolName: "route_settlement",
		ToolDescription: "Route net settlements by selecting the optimal path (direct_bank, correspondent, or netting_pool) " +
			"based on amount, counterparty, and policy. Returns routed settlement request.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"net_amounts": map[string]any{
					"type":        "array",
					"description": "Array of net amounts to route",
				},
				"policy": map[string]any{
					"type":        "object",
					"description": "Settlement policy (preferred_paths, netting_threshold, etc.)",
				},
			},
			"required": []string{"net_amounts"},
		},
		Fn: ra.routeSettlement,
	})

	// validate_counterparty: check if counterparty is approved
	reg.Register(&agenttools.ToolDef{
		ToolName: "validate_counterparty",
		ToolDescription: "Validate that a counterparty is approved for settlement. " +
			"Returns approval status and any restrictions (e.g. max amount, correspondent required).",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"counterparty": map[string]any{
					"type":        "string",
					"description": "Counterparty BIC or identifier",
				},
			},
			"required": []string{"counterparty"},
		},
		Fn: ra.validateCounterparty,
	})

	return reg
}

// routeSettlement assigns each net amount to a settlement path.
func (ra *RouterAgent) routeSettlement(ctx context.Context, args map[string]any) (string, error) {
	netsRaw, ok := args["net_amounts"]
	if !ok {
		return "error: net_amounts is required", nil
	}

	var nets []sett.NetAmount
	data, _ := json.Marshal(netsRaw)
	if err := json.Unmarshal(data, &nets); err != nil {
		return fmt.Sprintf("error: failed to parse net_amounts: %v", err), nil
	}

	if len(nets) == 0 {
		return "error: no net amounts to route", nil
	}

	// Apply routing logic
	routed := []sett.NetAmount{}
	for _, net := range nets {
		absAmt := net.Amount
		if absAmt < 0 {
			absAmt = -absAmt
		}

		// Simple heuristic: small → direct, medium → correspondent, large → netting
		if absAmt < 500_000 {
			net.Path = sett.PathDirect
		} else if absAmt < 2_000_000 {
			net.Path = sett.PathCorrespondent
		} else {
			net.Path = sett.PathNettingPool
		}

		routed = append(routed, net)
	}

	// Sort by path for grouped output
	sort.Slice(routed, func(i, j int) bool {
		if routed[i].Path != routed[j].Path {
			return routed[i].Path < routed[j].Path
		}
		return routed[i].Counterparty < routed[j].Counterparty
	})

	// Format summary
	var result []string
	result = append(result, fmt.Sprintf("Routing %d net amounts:", len(routed)))
	result = append(result, "")

	pathGroups := make(map[sett.SettlementPath][]sett.NetAmount)
	for _, net := range routed {
		pathGroups[net.Path] = append(pathGroups[net.Path], net)
	}

	for _, path := range ra.Policy.PreferredPaths {
		if nets, ok := pathGroups[path]; ok {
			result = append(result, fmt.Sprintf("Path: %s (%d settlements)", path, len(nets)))
			for _, net := range nets {
				absAmt := net.Amount
				sign := "owes"
				if absAmt < 0 {
					sign = "is owed"
					absAmt = -absAmt
				}
				result = append(result, fmt.Sprintf("  - %s %s %.0f %s", net.Counterparty, sign, absAmt, net.Currency))
			}
			result = append(result, "")
		}
	}

	output := fmt.Sprintf("routing complete\n%s", fmt.Sprintf("%s", result))
	return output, nil
}

// validateCounterparty checks if a counterparty is settlement-approved.
func (ra *RouterAgent) validateCounterparty(ctx context.Context, args map[string]any) (string, error) {
	cp, ok := args["counterparty"].(string)
	if !ok || cp == "" {
		return "error: counterparty is required", nil
	}

	// Stub: approve common banks
	approvedList := map[string]bool{
		"BANK_A":        true,
		"BANK_B":        true,
		"BANK_C":        true,
		"SWIFT_POOL":    true,
		"CORRESPONDENT": true,
	}

	if approvedList[cp] {
		return fmt.Sprintf("counterparty %q is approved for settlement. No restrictions.", cp), nil
	}

	return fmt.Sprintf("counterparty %q not approved. Requires additional due diligence.", cp), nil
}
