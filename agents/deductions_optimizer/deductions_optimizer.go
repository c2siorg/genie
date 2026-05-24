// Package deductions_optimizer recommends how to fully consume the
// chapter-VI-A deduction ceilings (old regime) that an Indian taxpayer
// still has room for.
//
// Sections covered (FY 2024-25):
//
//	80C   ₹1,50,000 — ELSS / PPF / EPF / 5-yr FD / NPS Tier-I / Insurance premium
//	80CCD(1B) ₹50,000  — NPS additional (over and above 80C)
//	80D    ₹25,000  — Self/spouse/children health insurance
//	         +₹50,000 — Parents (senior citizen)
//	80E    no cap   — Education loan interest (up to 8 AY)
//	80TTA  ₹10,000  — Savings-account interest (only old regime, <60yr)
//
// The agent computes the unused ceiling per section and ranks suggestions
// by marginal tax saved at the user's slab.
package deductions_optimizer

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "deductions_optimizer"
	Capability = "optimise_deductions"
	TypeIn     = "deductions_request"
	TypeOut    = "deductions_plan"
	NextAgent  = "financial_supervisor"

	Limit80C        = 1_50_000.0
	Limit80CCD1B    = 50_000.0
	Limit80DSelf    = 25_000.0
	Limit80DParents = 50_000.0
	Limit80TTA      = 10_000.0
)

// Used is what the user has already invested / paid in this FY.
type Used struct {
	Sec80C        float64 `json:"sec_80c"`
	Sec80CCD1B    float64 `json:"sec_80ccd_1b"`
	Sec80DSelf    float64 `json:"sec_80d_self"`
	Sec80DParents float64 `json:"sec_80d_parents"`
	Sec80TTA      float64 `json:"sec_80tta"`
}

// Request is the wire payload.
type Request struct {
	BorrowerSlabPct  float64 `json:"borrower_slab_pct"`   // e.g. 30
	Used             Used    `json:"used_so_far"`
	ParentsSeniorCit bool    `json:"parents_senior_citizen"`
}

// Suggestion is one ranked recommendation.
type Suggestion struct {
	Section       string  `json:"section"`
	HeadroomINR   float64 `json:"headroom_rupees"`
	TaxSavedINR   float64 `json:"tax_saved_rupees"`
	Instruments   string  `json:"sample_instruments"`
}

// Plan is the wire output.
type Plan struct {
	Suggestions      []Suggestion `json:"suggestions"`
	TotalSavingINR   float64      `json:"total_potential_saving_rupees"`
	RegimeNote       string       `json:"regime_note"`
	Disclaimer       string       `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Deductions Optimizer" }
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
	env.Logf("[deductions_optimizer] potential saving %.0f", plan.TotalSavingINR)
	body, _ := json.Marshal(plan)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute walks each section, computes unused headroom, and assigns the
// marginal tax saving at the borrower's slab.
func (a *Agent) Compute(req Request) Plan {
	slab := req.BorrowerSlabPct / 100
	if slab <= 0 {
		slab = 0.3 // default 30% bracket if not supplied
	}
	dheadroomParents := Limit80DParents
	if !req.ParentsSeniorCit {
		dheadroomParents = 25_000
	}
	sections := []struct {
		name string
		head float64
		desc string
	}{
		{"80C", max0(Limit80C - req.Used.Sec80C),
			"ELSS mutual funds (3-yr lock), PPF, EPF VPF, 5-yr tax-saver FD, term-insurance premium."},
		{"80CCD(1B)", max0(Limit80CCD1B - req.Used.Sec80CCD1B),
			"NPS Tier-I additional contribution; over and above 80C."},
		{"80D-self", max0(Limit80DSelf - req.Used.Sec80DSelf),
			"Health insurance premium for self / spouse / dependent children."},
		{"80D-parents", max0(dheadroomParents - req.Used.Sec80DParents),
			"Health insurance premium for parents (₹50k cap if senior citizen)."},
		{"80TTA", max0(Limit80TTA - req.Used.Sec80TTA),
			"Savings-account interest (declared in ITR; no investment required)."},
	}
	var plan Plan
	for _, s := range sections {
		if s.head <= 0 {
			continue
		}
		saved := s.head * slab
		plan.Suggestions = append(plan.Suggestions, Suggestion{
			Section:     s.name,
			HeadroomINR: round2(s.head),
			TaxSavedINR: round2(saved),
			Instruments: s.desc,
		})
		plan.TotalSavingINR += saved
	}
	sort.SliceStable(plan.Suggestions, func(i, j int) bool {
		return plan.Suggestions[i].TaxSavedINR > plan.Suggestions[j].TaxSavedINR
	})
	plan.TotalSavingINR = round2(plan.TotalSavingINR)
	plan.RegimeNote = "Chapter VI-A deductions apply to the OLD regime only. " +
		"If you have opted-in to the new regime, only the standard ₹75k deduction applies for FY 2024-25."
	plan.Disclaimer = "Informational. Each instrument has its own lock-in, taxability and risk profile."
	return plan
}

func max0(x float64) float64 {
	if x < 0 {
		return 0
	}
	return x
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
