// Package loan is a tiny ADK Small Business Loan-style helper. Given a
// monthly net cashflow, principal, term, and APR, it estimates EMI and the
// debt-service coverage ratio (DSCR).
package loan

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID            = "loan_advisor"
	CapEligible   = "loan_eligibility"
	CapSimulate   = "simulate_loan"
	TypeSimulate  = "simulate_loan"
	TypeOut       = "loan_simulation"
)

type Request struct {
	PrincipalCents    int64   `json:"principal_cents"`
	APRPct            float64 `json:"apr_pct"`
	TermMonths        int     `json:"term_months"`
	MonthlyNetCents   int64   `json:"monthly_net_cents"`
}

type Response struct {
	EMIInCents int64   `json:"emi_cents"`
	TotalCents int64   `json:"total_cents"`
	DSCR       float64 `json:"dscr"`
	Eligible   bool    `json:"eligible"`
	Reason     string  `json:"reason"`
}

type Agent struct {
	MinDSCR float64
}

func New() *Agent { return &Agent{MinDSCR: 1.25} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Loan Advisor" }
func (a *Agent) Capabilities() []string { return []string{CapEligible, CapSimulate} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeSimulate {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	if req.TermMonths <= 0 || req.PrincipalCents <= 0 || req.APRPct < 0 {
		return nil, errors.New("invalid loan parameters")
	}
	emi := computeEMI(req.PrincipalCents, req.APRPct, req.TermMonths)
	resp := Response{
		EMIInCents: emi,
		TotalCents: emi * int64(req.TermMonths),
	}
	if emi > 0 {
		resp.DSCR = float64(req.MonthlyNetCents) / float64(emi)
	}
	resp.Eligible = resp.DSCR >= a.MinDSCR
	if resp.Eligible {
		resp.Reason = "DSCR above threshold"
	} else {
		resp.Reason = "DSCR below threshold; consider lower principal or longer term"
	}
	env.Logf("[loan] EMI=%d DSCR=%.2f eligible=%v", resp.EMIInCents, resp.DSCR, resp.Eligible)
	body, _ := json.Marshal(resp)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

func computeEMI(principalCents int64, aprPct float64, termMonths int) int64 {
	monthlyRate := (aprPct / 100.0) / 12.0
	p := float64(principalCents)
	if monthlyRate == 0 {
		return int64(math.Round(p / float64(termMonths)))
	}
	factor := math.Pow(1+monthlyRate, float64(termMonths))
	emi := p * monthlyRate * factor / (factor - 1)
	return int64(math.Round(emi))
}
