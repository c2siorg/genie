package catalog

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// CarbonEstimatorSpec ports agents/carbon_estimator.
//
// It computes the carbon footprint (kg-CO2e) for a batch of transactions by
// mapping each merchant category to a coarse emission factor (kg-CO2e per rupee
// spent, from public EPA/IEA/India-BEE averages). Only debit transactions
// (amount_cents < 0) count as spend. It emits the per-category footprint, the
// total, a month-over-month trend (last vs prior month), and 20% reduction
// suggestions for the top-3 emitting categories. Deterministic table lookup +
// arithmetic; outputs are advisory/informational per RBI FREE-AI.
func CarbonEstimatorSpec() afg.Spec {
	return afg.Spec{
		ID:   "carbon_estimator",
		Risk: "low",
		Handle: func(input string) (string, error) {
			type transaction struct {
				AmountCents int64  `json:"amount_cents"`
				Category    string `json:"category,omitempty"`
				Date        string `json:"date"`
			}
			type analyzerView struct {
				Transactions []transaction `json:"transactions"`
			}
			type categorySummary struct {
				Category string  `json:"category"`
				SpentINR float64 `json:"spent_rupees"`
				KgCO2e   float64 `json:"kg_co2e"`
				SharePct float64 `json:"share_pct"`
			}
			type suggestion struct {
				Category  string  `json:"category"`
				ActionINR float64 `json:"action_target_rupees"`
				KgSavedYr float64 `json:"kg_saved_per_year"`
				Action    string  `json:"action"`
			}
			type result struct {
				TotalKgCO2e  float64           `json:"total_kg_co2e"`
				ByCategory   []categorySummary `json:"by_category"`
				Suggestions  []suggestion      `json:"suggestions"`
				MoMChangePct float64           `json:"month_over_month_change_pct"`
				Disclaimer   string            `json:"disclaimer"`
			}

			// EmissionFactors map category -> kg-CO2e per rupee spent.
			emissionFactors := map[string]float64{
				"fuel":        0.0024,
				"transport":   0.0006, // ride hailing, taxis
				"flights":     0.0030,
				"food":        0.0004,
				"meat":        0.0010,
				"groceries":   0.0003,
				"utilities":   0.0014, // electricity-heavy in India
				"shopping":    0.0005,
				"electronics": 0.0008,
				"rent":        0.0001, // imputed
				"income":      0.0000,
				"investment":  0.0000,
			}

			factor := func(cat string) float64 {
				if v, ok := emissionFactors[cat]; ok {
					return v
				}
				return 0.0004 // fallback "general goods"
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			reductionAction := func(cat string) string {
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

			keysSorted := func(m map[string]float64) []string {
				out := make([]string, 0, len(m))
				for k := range m {
					out = append(out, k)
				}
				sort.Strings(out)
				return out
			}

			var av analyzerView
			if err := json.Unmarshal([]byte(input), &av); err != nil {
				return "", err
			}

			byCat := map[string]float64{}   // rupees per category
			monthly := map[string]float64{} // month -> total kgCO2e
			for _, t := range av.Transactions {
				if t.AmountCents >= 0 {
					continue
				}
				spent := float64(-t.AmountCents) / 100
				cat := strings.ToLower(t.Category)
				byCat[cat] += spent
				when, err := time.Parse("2006-01-02", t.Date)
				if err != nil {
					continue
				}
				monthly[when.Format("2006-01")] += spent * factor(cat)
			}

			total := 0.0
			cats := []categorySummary{}
			for cat, spent := range byCat {
				kg := spent * factor(cat)
				total += kg
				cats = append(cats, categorySummary{
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
			suggestions := []suggestion{}
			for i, c := range cats {
				if i >= 3 {
					break
				}
				// Suggest a 20% reduction.
				target := c.SpentINR * 0.80
				saved := (c.SpentINR - target) * factor(c.Category) * 12
				suggestions = append(suggestions, suggestion{
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

			out := result{
				TotalKgCO2e:  round2(total),
				ByCategory:   cats,
				Suggestions:  suggestions,
				MoMChangePct: round2(mom),
				Disclaimer:   "Emission factors are coarse industry averages; actual footprint depends on merchant supply chain.",
			}

			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
