// Package payment_orchestrator routes outbound payments across UPI / IMPS
// / NEFT / RTGS rails based on amount, urgency, beneficiary status, and
// time-of-day, with HITL approval at configurable thresholds.
//
// This is the *bridge* to actual money movement. Genie's existing agents
// can analyse and recommend; this agent is the one that turns an approved
// recommendation into a payment instruction. PSP integration is pluggable
// — the agent emits a structured Instruction; the host wires it to UPI /
// PSP / IBM-Connect / whatever rails.
//
// Inspired by Google ADK samples → antom-payment. Indianised to NPCI rails.
//
// FREE-AI alignment:
//   - Rec 8 (Graded Liability): every >₹50k payment is RiskHigh → HITL.
//   - Rec 18 (Disclosure): every Instruction carries an AI disclosure.
//   - Rec 22 (Annexure VI): policy-deny or rail-rejection auto-records.
package payment_orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "payment_orchestrator"
	Capability = "orchestrate_payment"
	TypeIn     = "payment_request"
	TypeOut    = "payment_instruction"
	NextAgent  = "financial_supervisor"

	// NPCI per-rail limits (post-2023 revisions).
	upiPerTxnLimit   = 1_00_000.0  // ₹1 lakh default; some categories ₹5L (IPO, AMC).
	impsLimit        = 5_00_000.0
	rtgsMinThreshold = 2_00_000.0  // ₹2L floor for RTGS

	// HITL gate.
	hitlThresholdRupees = 50_000.0
)

// Request is the inbound payment ask.
type Request struct {
	IdempotencyKey   string  `json:"idempotency_key"`
	PayerID          string  `json:"payer_id"`
	PayerAccount     string  `json:"payer_account"`
	BeneficiaryName  string  `json:"beneficiary_name"`
	BeneficiaryVPA   string  `json:"beneficiary_vpa,omitempty"`     // for UPI
	BeneficiaryIFSC  string  `json:"beneficiary_ifsc,omitempty"`    // for IMPS/NEFT/RTGS
	BeneficiaryAcct  string  `json:"beneficiary_account,omitempty"`
	AmountRupees     float64 `json:"amount_rupees"`
	Currency         string  `json:"currency"`                       // INR only for now
	Purpose          string  `json:"purpose"`                        // memo
	Urgency          string  `json:"urgency"`                        // "now" | "today" | "any"
	IsTrustedBeneficiary bool `json:"is_trusted_beneficiary"`         // cooling-off cleared
}

// Instruction is what the orchestrator emits to the bus. The host PSP
// adapter picks it up and submits to the rail.
type Instruction struct {
	IdempotencyKey string   `json:"idempotency_key"`
	Action         string   `json:"action"`        // "submit" | "hold_hitl" | "reject"
	Rail           string   `json:"rail"`          // "upi" | "imps" | "neft" | "rtgs" | ""
	AmountRupees   float64  `json:"amount_rupees"`
	Reasons        []string `json:"reasons"`
	IncidentPayload string  `json:"incident_payload,omitempty"`
	Disclaimer     string   `json:"disclaimer"`
}

type Agent struct {
	// Clock is injectable so tests can fix the time-of-day branch.
	Clock func() time.Time
}

func New() *Agent { return &Agent{Clock: time.Now} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Payment Orchestrator" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	ins := a.Plan(req)
	env.Logf("[payment_orchestrator] key=%s action=%s rail=%s amount=%.0f",
		req.IdempotencyKey, ins.Action, ins.Rail, req.AmountRupees)
	body, _ := json.Marshal(ins)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Plan routes the request to a rail, with HITL holds where needed.
func (a *Agent) Plan(req Request) Instruction {
	if req.Currency != "" && strings.ToUpper(req.Currency) != "INR" {
		return reject(req, "Only INR rails are supported")
	}
	if req.AmountRupees <= 0 {
		return reject(req, "Amount must be positive")
	}
	if req.IdempotencyKey == "" {
		return reject(req, "Missing idempotency key — refusing to risk a duplicate transfer")
	}

	// Untrusted beneficiary + large amount = HITL regardless of rail.
	if !req.IsTrustedBeneficiary && req.AmountRupees >= hitlThresholdRupees {
		return hold(req, "Beneficiary not in trusted list and amount ≥ ₹50k — HITL required")
	}

	rail, reasons := a.chooseRail(req)
	if rail == "" {
		return reject(req, "No rail satisfies the amount + urgency + IFSC/VPA constraints")
	}

	// All payments at or above the HITL threshold go to human approval.
	if req.AmountRupees >= hitlThresholdRupees {
		return Instruction{
			IdempotencyKey: req.IdempotencyKey,
			Action:         "hold_hitl",
			Rail:           rail,
			AmountRupees:   req.AmountRupees,
			Reasons:        append([]string{"Routed via " + rail}, reasons...),
			Disclaimer:     stdDisclaimer(),
		}
	}

	return Instruction{
		IdempotencyKey: req.IdempotencyKey,
		Action:         "submit",
		Rail:           rail,
		AmountRupees:   req.AmountRupees,
		Reasons:        append([]string{"Routed via " + rail}, reasons...),
		Disclaimer:     stdDisclaimer(),
	}
}

// chooseRail picks the cheapest/fastest rail that satisfies constraints.
func (a *Agent) chooseRail(req Request) (string, []string) {
	now := a.Clock()
	hour := now.Hour()

	// UPI: instant, free, ≤₹1L, requires VPA.
	if req.BeneficiaryVPA != "" && req.AmountRupees <= upiPerTxnLimit {
		return "upi", []string{"Within UPI per-txn limit"}
	}
	// RTGS: instant, ≥₹2L, 7-18 Mon-Sat. For high-value transfers this is
	// the cleanest rail when the window is open, so try it before IMPS.
	if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" &&
		req.AmountRupees >= rtgsMinThreshold && hour >= 7 && hour < 18 && now.Weekday() != time.Sunday {
		return "rtgs", []string{"Amount ≥ ₹2L and within RTGS operating window"}
	}
	// IMPS: instant, 24×7, ≤₹5L, needs IFSC+Account. Use when RTGS isn't
	// available (closed window or below ₹2L floor).
	if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" &&
		req.AmountRupees <= impsLimit && strings.ToLower(req.Urgency) != "any" {
		return "imps", []string{"Within IMPS limit and instant credit needed"}
	}
	// NEFT: 24×7 since 2019, batch-settled, no real ceiling.
	if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" {
		return "neft", []string{"Falling back to NEFT batch settlement"}
	}
	return "", nil
}

func hold(req Request, reason string) Instruction {
	return Instruction{
		IdempotencyKey: req.IdempotencyKey,
		Action:         "hold_hitl",
		AmountRupees:   req.AmountRupees,
		Reasons:        []string{reason},
		Disclaimer:     stdDisclaimer(),
	}
}

func reject(req Request, reason string) Instruction {
	payload, _ := json.Marshal(map[string]any{
		"annexure":     "VI",
		"severity":     "medium",
		"action_taken": "Payment auto-rejected by orchestrator",
		"reason":       reason,
	})
	return Instruction{
		IdempotencyKey:  req.IdempotencyKey,
		Action:          "reject",
		AmountRupees:    req.AmountRupees,
		Reasons:         []string{reason},
		IncidentPayload: string(payload),
		Disclaimer:      stdDisclaimer(),
	}
}

func stdDisclaimer() string {
	return "AI-generated payment instruction. Subject to PSP confirmation, account-balance check, " +
		"and NPCI / RBI rail availability."
}
