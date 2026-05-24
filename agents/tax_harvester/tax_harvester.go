// Package tax_harvester identifies Indian equity holdings whose unrealised
// losses can be sold to offset realised gains in the same financial year.
//
// India-specific tax mechanics applied here (FY 2024-25 onwards):
//
//   - Equity STCG (held < 12 months): taxed at 20 % (raised from 15 % in
//     the July 2024 budget).
//   - Equity LTCG (held ≥ 12 months): taxed at 12.5 % above ₹1.25L exempt
//     per FY (raised from 10 %/₹1L).
//   - Losses set off: STCL offsets both STCG and LTCG; LTCL only LTCG.
//     Carry-forward up to 8 AYs.
//   - No formal wash-sale rule, but buying back the *same* security within
//     30 days is flagged as cosmetic — the agent surfaces it so the user
//     can take legal advice (Sutra 4 Fairness, Rec 18 disclosure).
//
// The agent is deterministic; no LLM is consulted. Outputs are designed
// to feed the recommender for plain-language explanation.
package tax_harvester

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "tax_harvester"
	Capability = "harvest_tax_losses"
	TypeIn     = "tax_harvest_request"
	TypeOut    = "tax_harvest_plan"
	NextAgent  = "financial_supervisor"

	// FY 2024-25 onwards.
	STCGRate         = 0.20
	LTCGRate         = 0.125
	LTCGExemptRupees = 1_25_000.0
	LTCGHoldingDays  = 365
	WashSaleDays     = 30
)

// Holding is one equity line item in the user's portfolio.
type Holding struct {
	Symbol         string  `json:"symbol"`
	Quantity       float64 `json:"quantity"`
	CostBasisRupee float64 `json:"cost_basis_rupees"` // total, not per-unit
	CurrentPrice   float64 `json:"current_price_rupees"`
	PurchaseDate   string  `json:"purchase_date"` // YYYY-MM-DD
}

// RealisedGain represents booked gains so far this FY, used to size the
// harvesting opportunity.
type RealisedGain struct {
	ShortTermRupees float64 `json:"short_term_rupees"`
	LongTermRupees  float64 `json:"long_term_rupees"`
}

// Request is the input payload.
type Request struct {
	Holdings  []Holding    `json:"holdings"`
	Realised  RealisedGain `json:"realised_gains"`
	AsOfDate  string       `json:"as_of_date"`  // YYYY-MM-DD; defaults to today
	HorizonFY string       `json:"horizon_fy"`  // e.g. "FY2024-25"; cosmetic
}

// Opportunity is one harvesting suggestion.
type Opportunity struct {
	Symbol            string  `json:"symbol"`
	Quantity          float64 `json:"quantity"`
	HoldingDays       int     `json:"holding_days"`
	GainCategory      string  `json:"gain_category"` // "STCL" | "LTCL"
	UnrealisedLossINR float64 `json:"unrealised_loss_rupees"`
	TaxSavedINR       float64 `json:"tax_saved_rupees"`
	Rationale         string  `json:"rationale"`
	WashSaleWarning   string  `json:"wash_sale_warning,omitempty"`
}

// Plan is the message payload.
type Plan struct {
	TotalTaxSavedINR  float64       `json:"total_tax_saved_rupees"`
	Opportunities     []Opportunity `json:"opportunities"`
	UnusedSTCLBudget  float64       `json:"unused_stcl_offset_rupees"`
	UnusedLTCLBudget  float64       `json:"unused_ltcl_offset_rupees"`
	Disclaimer        string        `json:"disclaimer"`
}

// Agent implements agent.Agent.
type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Tax-Loss Harvester (India)" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

// HandleMessage runs Compute and emits the plan.
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	plan := a.Compute(req)
	env.Logf("[tax_harvester] %d opportunities saving ₹%.0f", len(plan.Opportunities), plan.TotalTaxSavedINR)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute applies the harvesting rules to the request.
func (a *Agent) Compute(req Request) Plan {
	asOf := today()
	if req.AsOfDate != "" {
		if t, err := time.Parse("2006-01-02", req.AsOfDate); err == nil {
			asOf = t
		}
	}

	// Initial offset budget: STCL can wipe STCG + (LTCG above exemption).
	// LTCL can wipe LTCG above exemption only.
	taxableLTCG := max0(req.Realised.LongTermRupees - LTCGExemptRupees)
	stclBudget := req.Realised.ShortTermRupees + taxableLTCG
	ltclBudget := taxableLTCG

	candidates := []Opportunity{}
	for _, h := range req.Holdings {
		loss := h.CostBasisRupee - h.Quantity*h.CurrentPrice
		if loss <= 0 {
			continue // only losers harvestable
		}
		days := holdingDays(h.PurchaseDate, asOf)
		cat := "STCL"
		if days >= LTCGHoldingDays {
			cat = "LTCL"
		}
		candidates = append(candidates, Opportunity{
			Symbol:            h.Symbol,
			Quantity:          h.Quantity,
			HoldingDays:       days,
			GainCategory:      cat,
			UnrealisedLossINR: loss,
		})
	}

	// Rank by loss size — biggest first soaks the most budget.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].UnrealisedLossINR > candidates[j].UnrealisedLossINR
	})

	var plan Plan
	for i, c := range candidates {
		var used, saved float64
		switch c.GainCategory {
		case "STCL":
			used = minF(c.UnrealisedLossINR, stclBudget)
			stclBudget -= used
			// STCL first soaks STCG (taxed @ 20%), then LTCG (@ 12.5%).
			stOffset := minF(used, req.Realised.ShortTermRupees)
			req.Realised.ShortTermRupees -= stOffset
			ltOffset := used - stOffset
			saved = stOffset*STCGRate + ltOffset*LTCGRate
			c.Rationale = "Short-term loss; offsets STCG @20% before LTCG @12.5%."
		case "LTCL":
			used = minF(c.UnrealisedLossINR, ltclBudget)
			ltclBudget -= used
			saved = used * LTCGRate
			c.Rationale = "Long-term loss; offsets LTCG above ₹1.25L exemption @12.5%."
		}
		if used <= 0 {
			continue
		}
		c.TaxSavedINR = round2(saved)
		c.UnrealisedLossINR = round2(c.UnrealisedLossINR)
		// Cosmetic wash-sale check: if the user re-buys the same symbol
		// within 30 days, surface the warning. Genie only sees the
		// holdings snapshot; the recommender will combine with the
		// transactions list to confirm.
		c.WashSaleWarning = "If you re-buy " + c.Symbol +
			" within 30 days the harvested loss may be challenged. India has no formal wash-sale rule but the IT Dept can disallow cosmetic transactions."
		plan.Opportunities = append(plan.Opportunities, c)
		plan.TotalTaxSavedINR += saved
		_ = i // keep ranked order
	}

	plan.TotalTaxSavedINR = round2(plan.TotalTaxSavedINR)
	plan.UnusedSTCLBudget = round2(stclBudget)
	plan.UnusedLTCLBudget = round2(ltclBudget)
	plan.Disclaimer = "Informational; not tax advice. Consult a chartered accountant. " +
		"Slabs and rates per FY 2024-25 Union Budget."
	return plan
}

func today() time.Time {
	return time.Now().UTC()
}

func holdingDays(purchase string, asOf time.Time) int {
	p, err := time.Parse("2006-01-02", purchase)
	if err != nil {
		return 0
	}
	return int(asOf.Sub(p).Hours() / 24)
}

func max0(x float64) float64 {
	if x < 0 {
		return 0
	}
	return x
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func round2(x float64) float64 {
	return float64(int64(x*100+0.5)) / 100
}
