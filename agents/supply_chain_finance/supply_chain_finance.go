// Package supply_chain_finance layers on top of invoice_discounter and
// working_capital. Where those work transaction-by-transaction, SCF needs
// a *view* of the buyer-supplier chain over time: concentration risk,
// payment-cycle stability, and timing of TReDS (Trade Receivables
// Discounting System) auction submissions.
//
// Inspired by Google ADK samples → supply-chain. Tuned for the Indian
// MSME stack: TReDS platforms (RXIL, M1xchange, A.TREDS), GST e-invoicing,
// and the 45-day MSMED Act payment obligation.
package supply_chain_finance

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "supply_chain_finance"
	Capability = "supply_chain_finance"
	TypeIn     = "scf_request"
	TypeOut    = "scf_recommendation"
	NextAgent  = "financial_supervisor"

	// Single-buyer concentration above this triggers a warning.
	concentrationWarnPct = 0.40
	// Days of receivables outstanding beyond which TReDS auction is advised.
	tredsThresholdDays = 45
)

// Invoice is one outstanding receivable.
type Invoice struct {
	InvoiceID    string  `json:"invoice_id"`
	BuyerID      string  `json:"buyer_id"`
	BuyerRating  string  `json:"buyer_rating"` // AAA..D (CRISIL / ICRA scale)
	AmountRupees float64 `json:"amount_rupees"`
	DaysOutstanding int  `json:"days_outstanding"`
	GSTeInvoiced bool    `json:"gst_e_invoiced"` // required for TReDS
}

// Request is the SCF view request.
type Request struct {
	SupplierID string    `json:"supplier_id"`
	Invoices   []Invoice `json:"invoices"`
}

// BuyerSlice is the per-buyer aggregate the recommender uses.
type BuyerSlice struct {
	BuyerID      string  `json:"buyer_id"`
	TotalRupees  float64 `json:"total_rupees"`
	SharePct     float64 `json:"share_pct"`
	WorstDays    int     `json:"worst_days_outstanding"`
	BuyerRating  string  `json:"buyer_rating"`
}

// Recommendation is the structured output.
type Recommendation struct {
	SupplierID       string       `json:"supplier_id"`
	TotalReceivables float64      `json:"total_receivables_rupees"`
	ConcentrationOK  bool         `json:"concentration_ok"`
	WarnReasons      []string     `json:"warn_reasons"`
	BuyerSlices      []BuyerSlice `json:"buyer_slices"`
	TREDSCandidates  []Invoice    `json:"treds_candidates"`
	NextSteps        []string     `json:"next_steps"`
	Disclaimer       string       `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Supply-Chain Finance" }
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
	r := a.Recommend(req)
	env.Logf("[supply_chain_finance] supplier=%s buyers=%d treds=%d",
		r.SupplierID, len(r.BuyerSlices), len(r.TREDSCandidates))
	body, _ := json.Marshal(r)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Recommend runs the pure logic — aggregate, classify, suggest.
func (a *Agent) Recommend(req Request) Recommendation {
	total := 0.0
	byBuyer := map[string]*BuyerSlice{}

	for _, inv := range req.Invoices {
		total += inv.AmountRupees
		s, ok := byBuyer[inv.BuyerID]
		if !ok {
			s = &BuyerSlice{BuyerID: inv.BuyerID, BuyerRating: inv.BuyerRating}
			byBuyer[inv.BuyerID] = s
		}
		s.TotalRupees += inv.AmountRupees
		if inv.DaysOutstanding > s.WorstDays {
			s.WorstDays = inv.DaysOutstanding
		}
	}

	slices := make([]BuyerSlice, 0, len(byBuyer))
	for _, s := range byBuyer {
		if total > 0 {
			s.SharePct = round2((s.TotalRupees / total) * 100)
		}
		slices = append(slices, *s)
	}
	sort.Slice(slices, func(i, j int) bool { return slices[i].TotalRupees > slices[j].TotalRupees })

	warn := []string{}
	concentrationOK := true
	if len(slices) > 0 && total > 0 {
		topShare := slices[0].TotalRupees / total
		if topShare > concentrationWarnPct {
			concentrationOK = false
			warn = append(warn, "Top buyer share above "+pct(concentrationWarnPct)+" — concentration risk")
		}
	}

	// TReDS candidates: e-invoiced AND outstanding > threshold.
	candidates := []Invoice{}
	for _, inv := range req.Invoices {
		if inv.GSTeInvoiced && inv.DaysOutstanding > tredsThresholdDays {
			candidates = append(candidates, inv)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].DaysOutstanding > candidates[j].DaysOutstanding })

	steps := []string{}
	if len(candidates) > 0 {
		steps = append(steps, "Submit aged e-invoiced receivables to a TReDS platform for discounting")
	}
	if !concentrationOK {
		steps = append(steps,
			"Diversify buyer mix or insure top-buyer receivables to manage concentration risk")
	}
	if len(steps) == 0 {
		steps = []string{"Chain looks healthy; no immediate SCF action recommended"}
	}

	return Recommendation{
		SupplierID:       req.SupplierID,
		TotalReceivables: round2(total),
		ConcentrationOK:  concentrationOK,
		WarnReasons:      warn,
		BuyerSlices:      slices,
		TREDSCandidates:  candidates,
		NextSteps:        steps,
		Disclaimer: "Indicative supply-chain view. TReDS auction discount rates depend on buyer " +
			"credit rating and the platform's live auction; final yield is set at clearing.",
	}
}

func pct(p float64) string {
	return ftos(p*100) + "%"
}

func ftos(f float64) string {
	n := int64(f + 0.5)
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
