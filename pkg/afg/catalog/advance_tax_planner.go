package catalog

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// AdvanceTaxPlannerSpec ports agents/advance_tax_planner. It computes the four
// quarterly advance-tax instalments due in India for individuals (sec 211 IT Act)
// at cumulative 15/45/75/100 percent, surfaces the next upcoming instalment, days
// remaining, and any safe-harbor shortfall warning under sec 234B/234C.
func AdvanceTaxPlannerSpec() afg.Spec {
	return afg.Spec{
		ID:   "advance_tax_planner",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			// Request is the wire payload.
			type Request struct {
				ProjectedAnnualTaxINR float64 `json:"projected_annual_tax_rupees"`
				PaidSoFarINR          float64 `json:"paid_so_far_rupees"`
				AsOfDate              string  `json:"as_of_date"`
			}
			// Instalment is one scheduled advance-tax due.
			type Instalment struct {
				DueDate       string  `json:"due_date"`
				CumulativePct int     `json:"cumulative_pct"`
				CumulativeINR float64 `json:"cumulative_required_rupees"`
				StillDueINR   float64 `json:"still_due_rupees"`
				DaysRemaining int     `json:"days_remaining"`
				ShortfallNote string  `json:"shortfall_note,omitempty"`
			}
			// Plan is the wire output.
			type Plan struct {
				NextInstalment *Instalment  `json:"next_instalment,omitempty"`
				Schedule       []Instalment `json:"schedule"`
				Disclaimer     string       `json:"disclaimer"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			// financialYearOf returns the calendar year that starts the FY (Apr-Mar).
			financialYearOf := func(t time.Time) int {
				if t.Month() < time.April {
					return t.Year() - 1
				}
				return t.Year()
			}

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			now := time.Now()
			if req.AsOfDate != "" {
				if t, err := time.Parse("2006-01-02", req.AsOfDate); err == nil {
					now = t
				}
			}
			year := financialYearOf(now)

			due := []struct {
				date time.Time
				pct  int
			}{
				{time.Date(year, time.June, 15, 0, 0, 0, 0, time.UTC), 15},
				{time.Date(year, time.September, 15, 0, 0, 0, 0, time.UTC), 45},
				{time.Date(year, time.December, 15, 0, 0, 0, 0, time.UTC), 75},
				{time.Date(year+1, time.March, 15, 0, 0, 0, 0, time.UTC), 100},
			}
			plan := Plan{
				Disclaimer: "Quarterly cadence per sec 211. Shortfall ≥10% triggers sec 234B/234C interest @1% per month. Advisory/informational only per RBI FREE-AI; not a substitute for a qualified tax professional.",
			}
			for _, d := range due {
				cumReq := req.ProjectedAnnualTaxINR * float64(d.pct) / 100
				stillDue := cumReq - req.PaidSoFarINR
				if stillDue < 0 {
					stillDue = 0
				}
				days := int(d.date.Sub(now).Hours() / 24)
				note := ""
				if days < 0 && stillDue > 0.10*cumReq {
					note = "Missed deadline with ≥10% shortfall — sec 234C interest applies."
				}
				plan.Schedule = append(plan.Schedule, Instalment{
					DueDate:       d.date.Format("2006-01-02"),
					CumulativePct: d.pct,
					CumulativeINR: round2(cumReq),
					StillDueINR:   round2(stillDue),
					DaysRemaining: days,
					ShortfallNote: note,
				})
			}
			// Pick next upcoming.
			upcoming := []Instalment{}
			for _, i := range plan.Schedule {
				if i.DaysRemaining >= 0 {
					upcoming = append(upcoming, i)
				}
			}
			sort.SliceStable(upcoming, func(i, j int) bool {
				return upcoming[i].DaysRemaining < upcoming[j].DaysRemaining
			})
			if len(upcoming) > 0 {
				next := upcoming[0]
				plan.NextInstalment = &next
			}

			body, err := json.Marshal(plan)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
