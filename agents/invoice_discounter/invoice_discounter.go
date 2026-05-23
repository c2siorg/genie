// Package invoice_discounter ranks outstanding invoices by the cost of
// factoring them on TReDS (RBI's Trade Receivables Discounting System).
// Each invoice yields a "discount cost" = face × annualised_factor_rate ×
// (days_to_maturity / 365). The agent picks the cheapest combination
// that brings the user to a target working-capital infusion.
package invoice_discounter

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "invoice_discounter"
	Capability = "rank_invoice_discounting"
	TypeIn     = "invoice_discount_request"
	TypeOut    = "invoice_discount_plan"
	NextAgent  = "financial_supervisor"
)

// Invoice is one receivable.
type Invoice struct {
	ID                 string  `json:"id"`
	Counterparty       string  `json:"counterparty"`
	CounterpartyRating string  `json:"counterparty_rating"` // "AAA", "AA", ...
	FaceValueRupees    float64 `json:"face_value_rupees"`
	IssuedOn           string  `json:"issued_on"`
	DueOn              string  `json:"due_on"`
}

// Request is the wire payload.
type Request struct {
	Invoices         []Invoice `json:"invoices"`
	TargetCashINR    float64   `json:"target_cash_rupees"`
	ReferenceAsOf    string    `json:"as_of_date"`
	AnnualisedRateBp float64   `json:"annualised_rate_bp"` // 950 = 9.5%
}

// Choice is one ranked recommendation.
type Choice struct {
	InvoiceID         string  `json:"invoice_id"`
	NetCashINR        float64 `json:"net_cash_rupees"`
	DiscountCostINR   float64 `json:"discount_cost_rupees"`
	EffectiveAPR      float64 `json:"effective_apr_pct"`
	DaysToMaturity    int     `json:"days_to_maturity"`
	Rating            string  `json:"rating"`
}

// Plan is the wire output.
type Plan struct {
	Selected           []Choice `json:"selected"`
	TotalNetCashINR    float64  `json:"total_net_cash_rupees"`
	TotalDiscountINR   float64  `json:"total_discount_cost_rupees"`
	UnfundedGapINR     float64  `json:"unfunded_gap_rupees"`
	Note               string   `json:"note"`
}

type Agent struct {
	Now func() time.Time
}

func New() *Agent { return &Agent{Now: time.Now} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Invoice Discounter" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	plan := a.Plan(req)
	env.Logf("[invoice_discounter] selected=%d net=%.0f", len(plan.Selected), plan.TotalNetCashINR)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Plan ranks invoices and greedily selects cheapest until target met.
func (a *Agent) Plan(req Request) Plan {
	now := a.Now()
	if req.ReferenceAsOf != "" {
		if t, err := time.Parse("2006-01-02", req.ReferenceAsOf); err == nil {
			now = t
		}
	}
	rate := req.AnnualisedRateBp / 10_000.0
	candidates := []Choice{}
	for _, inv := range req.Invoices {
		due, err := time.Parse("2006-01-02", inv.DueOn)
		if err != nil {
			continue
		}
		days := int(due.Sub(now).Hours() / 24)
		if days <= 0 {
			continue // past due — not discountable here
		}
		discount := inv.FaceValueRupees * rate * float64(days) / 365.0
		discount = applyRatingPremium(discount, inv.CounterpartyRating)
		netCash := inv.FaceValueRupees - discount
		apr := 0.0
		if netCash > 0 && days > 0 {
			apr = (discount / netCash) * (365.0 / float64(days)) * 100
		}
		candidates = append(candidates, Choice{
			InvoiceID: inv.ID, NetCashINR: round2(netCash),
			DiscountCostINR: round2(discount), EffectiveAPR: round2(apr),
			DaysToMaturity: days, Rating: inv.CounterpartyRating,
		})
	}
	// Cheapest first (lowest effective APR).
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].EffectiveAPR < candidates[j].EffectiveAPR
	})

	plan := Plan{Note: "Greedy selection; for binding decision combine with bank's TReDS auction outcome."}
	for _, c := range candidates {
		if plan.TotalNetCashINR >= req.TargetCashINR {
			break
		}
		plan.Selected = append(plan.Selected, c)
		plan.TotalNetCashINR += c.NetCashINR
		plan.TotalDiscountINR += c.DiscountCostINR
	}
	plan.TotalNetCashINR = round2(plan.TotalNetCashINR)
	plan.TotalDiscountINR = round2(plan.TotalDiscountINR)
	if plan.TotalNetCashINR < req.TargetCashINR {
		plan.UnfundedGapINR = round2(req.TargetCashINR - plan.TotalNetCashINR)
	}
	return plan
}

// applyRatingPremium adds a credit-risk premium based on counterparty rating.
func applyRatingPremium(base float64, rating string) float64 {
	switch rating {
	case "AAA":
		return base
	case "AA":
		return base * 1.10
	case "A":
		return base * 1.25
	case "BBB":
		return base * 1.50
	default:
		return base * 2.0
	}
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
