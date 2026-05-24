// Package advance_tax_planner computes the four quarterly advance-tax
// instalments due in India for individuals (sec 211 IT Act):
//
//	15-Jun  15 % of total tax
//	15-Sep  45 % (cumulative)
//	15-Dec  75 % (cumulative)
//	15-Mar 100 %
//
// If shortfall ≥10 % at any deadline, interest under sec 234B/234C
// accrues at 1 % per month. The agent surfaces the next instalment
// amount, days remaining, and any safe-harbor shortfall warning.
package advance_tax_planner

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "advance_tax_planner"
	Capability = "plan_advance_tax"
	TypeIn     = "advance_tax_request"
	TypeOut    = "advance_tax_plan"
	NextAgent  = "financial_supervisor"
)

// Request is the wire payload.
type Request struct {
	ProjectedAnnualTaxINR float64 `json:"projected_annual_tax_rupees"`
	PaidSoFarINR          float64 `json:"paid_so_far_rupees"`
	AsOfDate              string  `json:"as_of_date"`
}

// Instalment is one scheduled advance-tax due.
type Instalment struct {
	DueDate         string  `json:"due_date"`
	CumulativePct   int     `json:"cumulative_pct"`
	CumulativeINR   float64 `json:"cumulative_required_rupees"`
	StillDueINR     float64 `json:"still_due_rupees"`
	DaysRemaining   int     `json:"days_remaining"`
	ShortfallNote   string  `json:"shortfall_note,omitempty"`
}

// Plan is the wire output.
type Plan struct {
	NextInstalment *Instalment  `json:"next_instalment,omitempty"`
	Schedule       []Instalment `json:"schedule"`
	Disclaimer     string       `json:"disclaimer"`
}

type Agent struct {
	Now func() time.Time
}

func New() *Agent { return &Agent{Now: time.Now} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Advance Tax Planner" }
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
	plan := a.Compute(req)
	env.Logf("[advance_tax_planner] next due rupees=%v", plan.NextInstalment)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute builds the four instalments and identifies the next-upcoming one.
func (a *Agent) Compute(req Request) Plan {
	now := a.Now()
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
		Disclaimer: "Quarterly cadence per sec 211. Shortfall ≥10% triggers sec 234B/234C interest @1% per month.",
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
	return plan
}

// financialYearOf returns the calendar year that starts the FY (Apr-Mar).
func financialYearOf(t time.Time) int {
	if t.Month() < time.April {
		return t.Year() - 1
	}
	return t.Year()
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
