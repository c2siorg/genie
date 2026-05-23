// Package carbon_estimator computes the carbon footprint (kg-CO₂e) for
// each transaction by mapping merchant category → emission factor. The
// factors are coarse industry averages from public databases (EPA, IEA,
// India BEE) and represent the user's portion of supply-chain emissions
// per rupee spent.
//
// Inputs: finance.Transaction list. Outputs: per-category footprint +
// total + month-over-month trend (last vs prior month) + reduction
// suggestions ranked by impact.
package carbon_estimator

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "carbon_estimator"
	Capability = "estimate_carbon"
	TypeIn     = "analysis_result"
	TypeOut    = "carbon_footprint"
	NextAgent  = "financial_supervisor"
)

// EmissionFactors map category → kg-CO₂e per rupee spent.
// These are illustrative; production deployments should swap in a curated
// dataset (e.g. India-Doconomy / Klima).
var EmissionFactors = map[string]float64{
	"fuel":        0.0024,
	"transport":   0.0006,  // ride hailing, taxis
	"flights":     0.0030,
	"food":        0.0004,
	"meat":        0.0010,
	"groceries":   0.0003,
	"utilities":   0.0014,  // electricity-heavy in India
	"shopping":    0.0005,
	"electronics": 0.0008,
	"rent":        0.0001,  // imputed
	"income":      0.0000,
	"investment":  0.0000,
}

// CategorySummary is one bucket result.
type CategorySummary struct {
	Category   string  `json:"category"`
	SpentINR   float64 `json:"spent_rupees"`
	KgCO2e     float64 `json:"kg_co2e"`
	SharePct   float64 `json:"share_pct"`
}

// Suggestion is one reduction action.
type Suggestion struct {
	Category    string  `json:"category"`
	ActionINR   float64 `json:"action_target_rupees"` // monthly spend cap
	KgSavedYr   float64 `json:"kg_saved_per_year"`
	Action      string  `json:"action"`
}

type analyzerView struct {
	Transactions []finance.Transaction `json:"transactions"`
}

// Result is the wire output.
type Result struct {
	TotalKgCO2e   float64           `json:"total_kg_co2e"`
	ByCategory    []CategorySummary `json:"by_category"`
	Suggestions   []Suggestion      `json:"suggestions"`
	MoMChangePct  float64           `json:"month_over_month_change_pct"`
	Disclaimer    string            `json:"disclaimer"`
}

type Agent struct {
	Factors map[string]float64
}

func New() *Agent { return &Agent{Factors: EmissionFactors} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Carbon Estimator" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskLow }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	res := a.Compute(av.Transactions)
	env.Logf("[carbon_estimator] total %.2f kgCO2e", res.TotalKgCO2e)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute walks the transactions, computes per-category footprint, ranks
// reduction suggestions, and emits a MoM trend.
func (a *Agent) Compute(txns []finance.Transaction) Result {
	byCat := map[string]float64{}      // rupees per category
	monthly := map[string]float64{}    // month → total kgCO2e
	for _, t := range txns {
		if t.AmountCents >= 0 {
			continue
		}
		spent := float64(-t.AmountCents) / 100
		cat := strings.ToLower(t.Category)
		byCat[cat] += spent
		when, err := t.ParsedDate()
		if err != nil {
			continue
		}
		monthly[when.Format("2006-01")] += spent * a.factor(cat)
	}
	total := 0.0
	cats := []CategorySummary{}
	for cat, spent := range byCat {
		kg := spent * a.factor(cat)
		total += kg
		cats = append(cats, CategorySummary{
			Category: cat,
			SpentINR: round2(spent),
			KgCO2e:   round2(kg),
		})
	}
	for i := range cats {
		if total > 0 {
			cats[i].SharePct = round2(cats[i].KgCO2e / total * 100)
		}
	}
	sort.SliceStable(cats, func(i, j int) bool { return cats[i].KgCO2e > cats[j].KgCO2e })

	// Reduction suggestions for the top 3 emitting categories.
	suggestions := []Suggestion{}
	for i, c := range cats {
		if i >= 3 {
			break
		}
		// Suggest a 20 % reduction.
		target := c.SpentINR * 0.80
		saved := (c.SpentINR - target) * a.factor(c.Category) * 12
		suggestions = append(suggestions, Suggestion{
			Category:  c.Category,
			ActionINR: round2(target),
			KgSavedYr: round2(saved),
			Action:    reductionAction(c.Category),
		})
	}

	// MoM trend: pick last two months in the data.
	months := keysSorted(monthly)
	mom := 0.0
	if len(months) >= 2 {
		prev := monthly[months[len(months)-2]]
		last := monthly[months[len(months)-1]]
		if prev > 0 {
			mom = (last - prev) / prev * 100
		}
	}

	return Result{
		TotalKgCO2e:  round2(total),
		ByCategory:   cats,
		Suggestions:  suggestions,
		MoMChangePct: round2(mom),
		Disclaimer:   "Emission factors are coarse industry averages; actual footprint depends on merchant supply chain.",
	}
}

func (a *Agent) factor(cat string) float64 {
	if v, ok := a.Factors[cat]; ok {
		return v
	}
	return 0.0004 // fallback "general goods"
}

func reductionAction(cat string) string {
	switch cat {
	case "fuel":
		return "Combine errands; switch one weekly commute to public transport."
	case "transport":
		return "Choose pool / share over solo ride hailing."
	case "flights":
		return "Replace one short-haul flight with rail; pick direct routings."
	case "meat":
		return "Adopt one meatless day a week."
	case "utilities":
		return "Upgrade to 5-star appliances; switch to LED lighting; consider rooftop solar."
	case "shopping", "electronics":
		return "Buy fewer + higher quality items; pick refurb where possible."
	default:
		return "Audit recurring spend in this category — look for higher-efficiency alternatives."
	}
}

func keysSorted(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
