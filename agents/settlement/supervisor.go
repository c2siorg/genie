// Package settlement implements the Settlement Coordinator multi-agent system
// (Lesson 11: Supervisor pattern + Lesson 02: Tools + Lesson 09: HITL).
//
// SettlementSupervisor orchestrates the complete settlement lifecycle:
//
//  1. FetchTransactions() — fetcher agent retrieves pending transactions
//  2. CalculateNets() — calculator agent aggregates by (counterparty, currency, date)
//  3. RouteSettlements() — router agent selects settlement paths
//  4. ApproveAndExecute() — HITL approval or auto-approval, then execution
//
// Each step is guarded by state machine enforcement and audit logging.
package settlement

import (
	"context"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agentic"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/hitl"
	sett "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/settlement"
)

// SettlementSupervisor routes tasks to specialized sub-agents.
// It maintains the settlement state machine and enforces valid transitions.
type SettlementSupervisor struct {
	// Supervisor is the underlying multi-agent router
	Supervisor *agentic.Supervisor
	// Policy defines approval and routing rules
	Policy sett.SettlementPolicy
	// AuditLog records all transitions and agent actions
	AuditLog sett.AuditLog
	// Approver gates HITL decisions (can be nil for auto-approval)
	Approver hitl.Approver
	// Store maps settlement ID → SettlementRequest (in-memory for demo)
	Store map[string]*sett.SettlementRequest
}

// NewSettlementSupervisor creates a fully configured settlement orchestrator.
// Pass nil for Policy to use defaults, nil for AuditLog to disable audit trails.
func NewSettlementSupervisor(approver hitl.Approver, policy *sett.SettlementPolicy,
	auditLog sett.AuditLog) *SettlementSupervisor {

	if policy == nil {
		p := sett.DefaultPolicy()
		policy = &p
	}

	// Build sub-agent registries (lesson 02: tools)
	fetcherReg := buildFetcherRegistry()
	calcReg := buildCalculatorRegistry()
	routerReg := buildRouterRegistry()

	sup := &agentic.Supervisor{
		Agents: []agentic.SubAgent{
			{
				Name:         "fetcher",
				Description:  "Fetches pending transactions from the source system by date range",
				SystemPrompt: fetcherSystemPrompt,
				Registry:     fetcherReg,
				Approver:     nil, // fetcher has no side effects, always auto-approve
			},
			{
				Name:         "calculator",
				Description:  "Aggregates transactions by (counterparty, currency, date) to compute net amounts",
				SystemPrompt: calculatorSystemPrompt,
				Registry:     calcReg,
				Approver:     nil, // calculator is deterministic
			},
			{
				Name:         "router",
				Description:  "Routes net settlements via direct bank, correspondent, or netting pool",
				SystemPrompt: routerSystemPrompt,
				Registry:     routerReg,
				Approver:     nil, // routing is rule-based
			},
		},
		Config:        agentic.DefaultConfig(),
		FallbackAgent: "fetcher",
	}

	return &SettlementSupervisor{
		Supervisor: sup,
		Policy:     *policy,
		AuditLog:   auditLog,
		Approver:   approver,
		Store:      make(map[string]*sett.SettlementRequest),
	}
}

// CreateRequest initializes a new settlement request with a unique ID.
func (s *SettlementSupervisor) CreateRequest(ctx context.Context, metadata map[string]string) (*sett.SettlementRequest, error) {
	id := fmt.Sprintf("sett-%d", time.Now().UnixNano())
	req := &sett.SettlementRequest{
		ID:        id,
		State:     sett.StatePending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  metadata,
	}
	s.Store[id] = req
	return req, nil
}

// FetchTransactions transitions from pending → fetched by delegating to the fetcher agent.
func (s *SettlementSupervisor) FetchTransactions(ctx context.Context, reqID string, dateRange string) error {
	req, ok := s.Store[reqID]
	if !ok {
		return fmt.Errorf("settlement: request %q not found", reqID)
	}

	// Enforce state machine: only fetch from pending
	if req.State != sett.StatePending {
		return fmt.Errorf("settlement: cannot fetch from state %q", req.State)
	}

	// Ask the fetcher agent to retrieve transactions
	task := fmt.Sprintf("Fetch pending transactions for date range: %s", dateRange)
	_, _, _, err := s.Supervisor.Run(ctx, task, nil)
	if err != nil {
		sett.LogToolExecution(ctx, s.AuditLog, reqID, "fetcher", "fetch_transactions", dateRange, nil, err)
		return fmt.Errorf("settlement: fetcher failed: %w", err)
	}

	// In a real system, parse the LLM response to extract transactions.
	// For now, simulate fetching 3 sample transactions.
	req.Transactions = []sett.Transaction{
		{
			ID:              fmt.Sprintf("txn-%d-1", time.Now().UnixNano()),
			FromCounterparty: "BANK_A",
			ToCounterparty:   "BANK_B",
			Amount:          100_000,
			Currency:        "USD",
			SettlementDate:  time.Now().Add(24 * time.Hour),
			CreatedAt:       time.Now(),
		},
		{
			ID:              fmt.Sprintf("txn-%d-2", time.Now().UnixNano()),
			FromCounterparty: "BANK_B",
			ToCounterparty:   "BANK_C",
			Amount:          75_000,
			Currency:        "USD",
			SettlementDate:  time.Now().Add(24 * time.Hour),
			CreatedAt:       time.Now(),
		},
		{
			ID:              fmt.Sprintf("txn-%d-3", time.Now().UnixNano()),
			FromCounterparty: "BANK_C",
			ToCounterparty:   "BANK_A",
			Amount:          50_000,
			Currency:        "USD",
			SettlementDate:  time.Now().Add(24 * time.Hour),
			CreatedAt:       time.Now(),
		},
	}

	// Collect unique counterparties
	cpMap := make(map[string]bool)
	for _, txn := range req.Transactions {
		cpMap[txn.FromCounterparty] = true
		cpMap[txn.ToCounterparty] = true
	}
	for cp := range cpMap {
		req.Counterparties = append(req.Counterparties, cp)
	}

	// Transition state
	oldState := req.State
	req.State = sett.StateFetched
	req.UpdatedAt = time.Now()

	sett.LogToolExecution(ctx, s.AuditLog, reqID, "fetcher", "fetch_transactions",
		dateRange, len(req.Transactions), nil)
	sett.LogStateTransition(ctx, s.AuditLog, reqID, "fetcher", oldState, req.State)

	return nil
}

// CalculateNets transitions from fetched → calculated by delegating to the calculator agent.
func (s *SettlementSupervisor) CalculateNets(ctx context.Context, reqID string) error {
	req, ok := s.Store[reqID]
	if !ok {
		return fmt.Errorf("settlement: request %q not found", reqID)
	}

	// Enforce state machine: only calculate from fetched
	if req.State != sett.StateFetched {
		return fmt.Errorf("settlement: cannot calculate from state %q", req.State)
	}

	// Ask the calculator agent
	task := fmt.Sprintf("Calculate net amounts from %d transactions, aggregating by counterparty and currency",
		len(req.Transactions))
	_, _, _, err := s.Supervisor.Run(ctx, task, nil)
	if err != nil {
		sett.LogToolExecution(ctx, s.AuditLog, reqID, "calculator", "calculate_net_amounts",
			len(req.Transactions), nil, err)
		return fmt.Errorf("settlement: calculator failed: %w", err)
	}

	// Simulate netting calculation: aggregate by (cp, currency, date)
	type netKey struct {
		cp   string
		curr string
		date string
	}
	netMap := make(map[netKey]float64)
	for _, txn := range req.Transactions {
		key := netKey{txn.FromCounterparty, txn.Currency, txn.SettlementDate.Format("2006-01-02")}
		netMap[key] -= txn.Amount // we send out

		key.cp = txn.ToCounterparty
		netMap[key] += txn.Amount // we receive
	}

	// Convert to NetAmount structs
	req.NetAmounts = []sett.NetAmount{}
	for key, amt := range netMap {
		if amt != 0 { // skip zero nets
			req.NetAmounts = append(req.NetAmounts, sett.NetAmount{
				Counterparty:   key.cp,
				Currency:       key.curr,
				Amount:         amt,
				SettlementDate: time.Now().Add(24 * time.Hour),
				ComputedAt:     time.Now(),
				// Path will be set by router
			})
		}
	}

	// Transition state
	oldState := req.State
	req.State = sett.StateCalculated
	req.UpdatedAt = time.Now()

	sett.LogToolExecution(ctx, s.AuditLog, reqID, "calculator", "calculate_net_amounts",
		len(req.Transactions), len(req.NetAmounts), nil)
	sett.LogStateTransition(ctx, s.AuditLog, reqID, "calculator", oldState, req.State)

	return nil
}

// RouteSettlements transitions from calculated → routed by delegating to the router agent.
func (s *SettlementSupervisor) RouteSettlements(ctx context.Context, reqID string) error {
	req, ok := s.Store[reqID]
	if !ok {
		return fmt.Errorf("settlement: request %q not found", reqID)
	}

	// Enforce state machine: only route from calculated
	if req.State != sett.StateCalculated {
		return fmt.Errorf("settlement: cannot route from state %q", req.State)
	}

	// Ask the router agent
	task := fmt.Sprintf("Route %d net settlements. Policy: prefer %v. Select direct for <500k, correspondent for <2M, netting for >=2M",
		len(req.NetAmounts), s.Policy.PreferredPaths)
	_, _, _, err := s.Supervisor.Run(ctx, task, nil)
	if err != nil {
		sett.LogToolExecution(ctx, s.AuditLog, reqID, "router", "route_settlement",
			len(req.NetAmounts), nil, err)
		return fmt.Errorf("settlement: router failed: %w", err)
	}

	// Apply simple routing rules
	for i := range req.NetAmounts {
		absAmt := req.NetAmounts[i].Amount
		if absAmt < 0 {
			absAmt = -absAmt
		}

		// Simple heuristic: use direct for small, correspondent for medium, netting for large
		if absAmt < 500_000 {
			req.NetAmounts[i].Path = sett.PathDirect
		} else if absAmt < 2_000_000 {
			req.NetAmounts[i].Path = sett.PathCorrespondent
		} else {
			req.NetAmounts[i].Path = sett.PathNettingPool
		}
	}

	// Transition state
	oldState := req.State
	req.State = sett.StateRouted
	req.UpdatedAt = time.Now()

	sett.LogToolExecution(ctx, s.AuditLog, reqID, "router", "route_settlement",
		len(req.NetAmounts), len(req.NetAmounts), nil)
	sett.LogStateTransition(ctx, s.AuditLog, reqID, "router", oldState, req.State)

	return nil
}

// ApproveAndExecute transitions from routed → approved → executed via HITL or policy.
func (s *SettlementSupervisor) ApproveAndExecute(ctx context.Context, reqID string, approverID string) error {
	req, ok := s.Store[reqID]
	if !ok {
		return fmt.Errorf("settlement: request %q not found", reqID)
	}

	// Enforce state machine: only approve from routed
	if req.State != sett.StateRouted {
		return fmt.Errorf("settlement: cannot approve from state %q", req.State)
	}

	// Compute total for policy decision
	totalAmt := 0.0
	for _, net := range req.NetAmounts {
		if net.Amount > 0 {
			totalAmt += net.Amount // sum obligations
		}
	}

	// Apply policy
	approved := false
	if totalAmt < s.Policy.AutoApproveLimit {
		approved = true
	} else if totalAmt > s.Policy.AutoRejectLimit {
		approved = false
	} else if s.Approver != nil {
		// Manual review via HITL (lesson 09)
		decision, err := s.Approver.RequestApproval(ctx, hitl.ApprovalRequest{
			ID:      reqID,
			AgentID: "settlement_supervisor",
		})
		if err != nil {
			return fmt.Errorf("settlement: approval failed: %w", err)
		}
		approved = decision
		// In a real system, approverID would come from the HITL system
		if approved {
			approverID = "human_approver"
		}
	}

	sett.LogApproval(ctx, s.AuditLog, reqID, approverID, approved, "policy evaluation")

	if !approved {
		// State stays routed; can retry later
		return fmt.Errorf("settlement: approval rejected")
	}

	// Transition approved
	oldState := req.State
	req.State = sett.StateApproved
	req.ApprovalID = approverID
	req.UpdatedAt = time.Now()
	sett.LogStateTransition(ctx, s.AuditLog, reqID, "supervisor", oldState, req.State)

	// Execute (in real system, call banking APIs)
	oldState = req.State
	req.State = sett.StateExecuted
	req.UpdatedAt = time.Now()
	sett.LogStateTransition(ctx, s.AuditLog, reqID, "supervisor", oldState, req.State)

	return nil
}

// GetRequest returns the current settlement request.
func (s *SettlementSupervisor) GetRequest(reqID string) (*sett.SettlementRequest, error) {
	req, ok := s.Store[reqID]
	if !ok {
		return nil, fmt.Errorf("settlement: request %q not found", reqID)
	}
	return req, nil
}

// System prompts for each sub-agent

const fetcherSystemPrompt = `You are the Fetcher agent. Your job is to:
1. Acknowledge the date range and scope
2. Summarize the fetched transactions

You DO NOT actually connect to a database; the system will handle the fetch.
Just confirm what was requested and provide a brief report of what would be fetched.`

const calculatorSystemPrompt = `You are the Calculator agent. Your job is to:
1. Understand transaction inflow/outflow
2. Compute net amounts by (counterparty, currency, settlement_date)
3. Identify netting opportunities

Explain your logic clearly so the router can make informed decisions.`

const routerSystemPrompt = `You are the Router agent. Your job is to:
1. Review the policy preferences for settlement paths
2. Apply rules: direct for small amounts, correspondent for medium, netting for large
3. Justify each path selection

Return a summary of routing decisions without modifying the actual settlement (the system does that).`

// buildFetcherRegistry creates tools for the fetcher agent.
func buildFetcherRegistry() *agenttools.Registry {
	reg := agenttools.NewRegistry()

	// Placeholder tool: search_pending_transactions
	reg.Register(&agenttools.ToolDef{
		ToolName: "search_pending_transactions",
		ToolDescription: "Search for pending transactions by date range and settlement status. " +
			"Returns transaction list (stubbed for demo; real implementation calls GraphRAG Query tool).",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"date_range": map[string]any{
					"type":        "string",
					"description": "ISO 8601 date range, e.g. '2024-05-01:2024-05-31'",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Settlement status: 'pending', 'in_flight', 'settled'",
				},
			},
			"required": []string{"date_range", "status"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			dateRange, _ := args["date_range"].(string)
			status, _ := args["status"].(string)
			return fmt.Sprintf("Searched for transactions: date_range=%s status=%s; found 3 pending transactions", dateRange, status), nil
		},
	})

	return reg
}

// buildCalculatorRegistry creates tools for the calculator agent.
func buildCalculatorRegistry() *agenttools.Registry {
	reg := agenttools.NewRegistry()

	reg.Register(&agenttools.ToolDef{
		ToolName: "calculate_net_amounts",
		ToolDescription: "Aggregate transactions by (counterparty, currency, settlement_date) " +
			"to compute net bilaterals and identify netting opportunities.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"transactions": map[string]any{
					"type":        "array",
					"description": "List of transactions to aggregate",
				},
				"include_netting": map[string]any{
					"type":        "boolean",
					"description": "Consider netting pool consolidation if true",
				},
			},
			"required": []string{"transactions"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			return "Calculated nets: BANK_A owe USD 50k to BANK_B, BANK_B owe USD 25k to BANK_C, etc.", nil
		},
	})

	return reg
}

// buildRouterRegistry creates tools for the router agent.
func buildRouterRegistry() *agenttools.Registry {
	reg := agenttools.NewRegistry()

	reg.Register(&agenttools.ToolDef{
		ToolName: "route_settlement",
		ToolDescription: "Select settlement path (direct_bank, correspondent, netting_pool) " +
			"based on amount, policy, and counterparty ratings.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"net_amounts": map[string]any{
					"type":        "array",
					"description": "List of net amounts to route",
				},
				"policy": map[string]any{
					"type":        "object",
					"description": "Settlement policy (auto_approve_limit, manual_review_limit, etc.)",
				},
			},
			"required": []string{"net_amounts"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			return "Routed settlements: BANK_A -> BANK_B via direct_bank, BANK_B -> BANK_C via correspondent, etc.", nil
		},
	})

	return reg
}
