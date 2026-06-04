// Package settlement defines types and state machines for the Settlement Coordinator
// multi-agent system. Settlement orchestrates the collection, aggregation, and
// routing of interbank transactions through state transitions with audit trails.
package settlement

import (
	"encoding/json"
	"time"
)

// SettlementState represents the current phase of a settlement request.
type SettlementState string

const (
	// StatePending: settlement request created, awaiting transaction fetch
	StatePending SettlementState = "pending"
	// StateFetched: transactions retrieved from source system
	StateFetched SettlementState = "fetched"
	// StateCalculated: net amounts computed, netting opportunities identified
	StateCalculated SettlementState = "calculated"
	// StateRouted: settlement paths assigned (direct, correspondent, netting pool)
	StateRouted SettlementState = "routed"
	// StateApproved: human approval obtained or auto-approved via policy
	StateApproved SettlementState = "approved"
	// StateExecuted: settlement executed, funds transferred
	StateExecuted SettlementState = "executed"
)

// SettlementRequest orchestrates one complete settlement cycle.
type SettlementRequest struct {
	// ID uniquely identifies this settlement request
	ID string `json:"id"`
	// Transactions collected from the source system
	Transactions []Transaction `json:"transactions"`
	// Counterparties involved in this settlement batch
	Counterparties []string `json:"counterparties"`
	// NetAmounts computed and routed
	NetAmounts []NetAmount `json:"net_amounts"`
	// State tracks current phase
	State SettlementState `json:"state"`
	// CreatedAt records when the request was initiated
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt records the most recent state transition
	UpdatedAt time.Time `json:"updated_at"`
	// ApprovalID links to the human approver or approval policy if auto-approved
	ApprovalID string `json:"approval_id,omitempty"`
	// Metadata holds key-value context (settlement date, batch ID, etc.)
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Transaction is one inbound payment awaiting settlement.
type Transaction struct {
	// ID uniquely identifies the transaction
	ID string `json:"id"`
	// FromCounterparty sends the payment
	FromCounterparty string `json:"from_counterparty"`
	// ToCounterparty receives the payment
	ToCounterparty string `json:"to_counterparty"`
	// Amount in base currency
	Amount float64 `json:"amount"`
	// Currency of the amount (ISO 4217: USD, INR, EUR, etc.)
	Currency string `json:"currency"`
	// SettlementDate agreed settlement date
	SettlementDate time.Time `json:"settlement_date"`
	// CreatedAt records when the transaction entered the system
	CreatedAt time.Time `json:"created_at"`
}

// NetAmount represents aggregated, settled amounts between two counterparties.
type NetAmount struct {
	// Counterparty identifies the other party in the settlement
	Counterparty string `json:"counterparty"`
	// Currency of the net amount
	Currency string `json:"currency"`
	// Amount net total (positive = we owe, negative = we are owed)
	Amount float64 `json:"amount"`
	// SettlementDate when the net settlement should occur
	SettlementDate time.Time `json:"settlement_date"`
	// Path chosen for settlement execution
	Path SettlementPath `json:"path"`
	// Timestamp when this net was computed
	ComputedAt time.Time `json:"computed_at"`
}

// SettlementPath describes how the net amount will be settled.
type SettlementPath string

const (
	// PathDirect: direct bank-to-bank transfer (bilateral)
	PathDirect SettlementPath = "direct_bank"
	// PathCorrespondent: routed through correspondent banking network
	PathCorrespondent SettlementPath = "correspondent"
	// PathNettingPool: netting pool aggregates multiple parties for reduced settlement
	PathNettingPool SettlementPath = "netting_pool"
)

// SettlementPolicy defines rules for automatic routing and approval.
type SettlementPolicy struct {
	// AutoApproveLimit: amounts below this (in base currency) auto-approve
	AutoApproveLimit float64 `json:"auto_approve_limit"`
	// ManualReviewLimit: amounts between AutoApprove and this require HITL review
	ManualReviewLimit float64 `json:"manual_review_limit"`
	// AutoRejectLimit: amounts exceeding this are automatically rejected
	AutoRejectLimit float64 `json:"auto_reject_limit"`
	// PreferredPaths ranks settlement paths by preference (direct, correspondent, netting)
	PreferredPaths []SettlementPath `json:"preferred_paths"`
	// NettingPoolThreshold: minimum number of transactions to consider netting
	NettingPoolThreshold int `json:"netting_pool_threshold"`
}

// AuditEntry records one state transition or tool execution.
type AuditEntry struct {
	// ID uniquely identifies this audit log entry
	ID string `json:"id"`
	// SettlementID references the parent SettlementRequest
	SettlementID string `json:"settlement_id"`
	// Agent name that executed the action (fetcher, calculator, router, etc.)
	Agent string `json:"agent"`
	// Action describes what happened (fetch_transactions, calculate_nets, etc.)
	Action string `json:"action"`
	// Input to the agent action (serialized JSON)
	Input json.RawMessage `json:"input"`
	// Output from the agent action (serialized JSON)
	Output json.RawMessage `json:"output"`
	// Approver user ID if this was a HITL action; empty if automatic
	Approver string `json:"approver,omitempty"`
	// Timestamp when the action occurred
	Timestamp time.Time `json:"timestamp"`
	// Error message if the action failed (empty on success)
	Error string `json:"error,omitempty"`
}

// DefaultPolicy returns a sensible default settlement policy.
func DefaultPolicy() SettlementPolicy {
	return SettlementPolicy{
		AutoApproveLimit:     100_000,    // auto-approve up to 100k
		ManualReviewLimit:    1_000_000,  // manual review up to 1M
		AutoRejectLimit:      10_000_000, // reject anything over 10M
		PreferredPaths:       []SettlementPath{PathDirect, PathCorrespondent, PathNettingPool},
		NettingPoolThreshold: 3, // need at least 3 parties to consider netting
	}
}
