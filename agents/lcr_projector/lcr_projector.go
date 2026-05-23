// Package lcr_projector computes the Liquidity Coverage Ratio (LCR) for
// a bank balance sheet under Basel III. LCR = HQLA / Net Cash Outflows
// over a 30-day stress window. RBI requires ≥100 % for SCBs.
//
// This agent applies the standard run-off factors (RBI master direction
// on LCR, 2014 + amendments) to each liability bucket and the standard
// haircuts to HQLA categories.
package lcr_projector

import (
	"context"
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "lcr_projector"
	Capability = "project_lcr"
	TypeIn     = "lcr_request"
	TypeOut    = "lcr_result"
	NextAgent  = "financial_supervisor"
)

// HQLA buckets.
type HQLA struct {
	Level1INR         float64 `json:"level1_cash_govsec_rupees"`
	Level2AINR        float64 `json:"level2a_corp_bonds_aa_plus_rupees"`
	Level2BINR        float64 `json:"level2b_lower_rupees"`
}

// Outflows buckets.
type Outflows struct {
	StableRetailINR        float64 `json:"stable_retail_deposits"`
	LessStableRetailINR    float64 `json:"less_stable_retail_deposits"`
	OperationalCorpINR     float64 `json:"operational_corporate_deposits"`
	NonOperationalCorpINR  float64 `json:"non_operational_corporate_deposits"`
	UnsecuredWholesaleINR  float64 `json:"unsecured_wholesale_funding"`
	UndrawnCommitments     float64 `json:"undrawn_credit_commitments"`
}

// Inflows buckets.
type Inflows struct {
	ContractualRetailINR   float64 `json:"contractual_retail_inflows"`
	ContractualWholesale   float64 `json:"contractual_wholesale_inflows"`
	OtherInflowsINR        float64 `json:"other_inflows"`
}

// Request is the wire payload.
type Request struct {
	HQLA     HQLA     `json:"hqla"`
	Outflows Outflows `json:"outflows"`
	Inflows  Inflows  `json:"inflows"`
}

// Result is the wire output.
type Result struct {
	TotalHQLA       float64 `json:"total_hqla_after_haircut_rupees"`
	TotalOutflow    float64 `json:"total_outflow_rupees"`
	TotalInflow     float64 `json:"total_inflow_capped_rupees"`
	NetCashOutflow  float64 `json:"net_cash_outflow_rupees"`
	LCRPct          float64 `json:"lcr_pct"`
	Compliant       bool    `json:"compliant_with_100pct"`
	Note            string  `json:"note"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "LCR Projector" }
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
	res := a.Compute(req)
	env.Logf("[lcr_projector] LCR=%.0f%% compliant=%v", res.LCRPct, res.Compliant)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute applies haircuts and run-off factors.
func (a *Agent) Compute(req Request) Result {
	// HQLA after haircuts. Level 2A=15% haircut, 2B=50%. Level 2 total
	// capped at 40% of total HQLA; Level 2B at 15%.
	l1 := req.HQLA.Level1INR
	l2a := req.HQLA.Level2AINR * 0.85
	l2b := req.HQLA.Level2BINR * 0.50
	// Cap Level 2B at 15% of total.
	total := l1 + l2a + l2b
	if l2b > 0.15*total {
		l2b = 0.15 * total
	}
	// Cap Level 2 (2A+2B) at 40% of total.
	if l2a+l2b > 0.40*total {
		excess := (l2a + l2b) - 0.40*total
		if l2b > 0 {
			cut := min(excess, l2b)
			l2b -= cut
			excess -= cut
		}
		if excess > 0 {
			l2a -= excess
		}
	}
	hqla := l1 + l2a + l2b

	// Outflow run-off factors (RBI LCR master direction).
	out := req.Outflows.StableRetailINR*0.05 +
		req.Outflows.LessStableRetailINR*0.10 +
		req.Outflows.OperationalCorpINR*0.25 +
		req.Outflows.NonOperationalCorpINR*0.40 +
		req.Outflows.UnsecuredWholesaleINR*0.75 +
		req.Outflows.UndrawnCommitments*0.10

	// Inflows: cap at 75% of outflows.
	rawIn := req.Inflows.ContractualRetailINR*0.50 +
		req.Inflows.ContractualWholesale*0.50 +
		req.Inflows.OtherInflowsINR*0.20
	cappedIn := rawIn
	if cappedIn > 0.75*out {
		cappedIn = 0.75 * out
	}
	netOut := out - cappedIn
	lcr := 0.0
	if netOut > 0 {
		lcr = hqla / netOut * 100
	} else {
		lcr = 999
	}
	return Result{
		TotalHQLA:      round2(hqla),
		TotalOutflow:   round2(out),
		TotalInflow:    round2(cappedIn),
		NetCashOutflow: round2(netOut),
		LCRPct:         round2(lcr),
		Compliant:      lcr >= 100,
		Note:           "Applies RBI LCR master direction run-off factors. Live LCR must reconcile against bank's NSF group.",
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
