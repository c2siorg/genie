package catalog

import (
	"encoding/json"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// DeductionsOptimizerSpec ports agents/deductions_optimizer.
//
// It recommends how to fully consume the chapter-VI-A deduction ceilings
// (old regime, FY 2024-25) that an Indian taxpayer still has room for. For each
// section it computes the unused headroom and the marginal tax saved at the
// borrower's slab, then ranks the suggestions by tax saved.
func DeductionsOptimizerSpec() afg.Spec {
	return afg.Spec{
		ID:   "deductions_optimizer",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				limit80C        = 1_50_000.0
				limit80CCD1B    = 50_000.0
				limit80DSelf    = 25_000.0
				limit80DParents = 50_000.0
				limit80TTA      = 10_000.0
			)

			type used struct {
				Sec80C        float64 `json:"sec_80c"`
				Sec80CCD1B    float64 `json:"sec_80ccd_1b"`
				Sec80DSelf    float64 `json:"sec_80d_self"`
				Sec80DParents float64 `json:"sec_80d_parents"`
				Sec80TTA      float64 `json:"sec_80tta"`
			}
			type request struct {
				BorrowerSlabPct  float64 `json:"borrower_slab_pct"`
				Used             used    `json:"used_so_far"`
				ParentsSeniorCit bool    `json:"parents_senior_citizen"`
			}
			type suggestion struct {
				Section     string  `json:"section"`
				HeadroomINR float64 `json:"headroom_rupees"`
				TaxSavedINR float64 `json:"tax_saved_rupees"`
				Instruments string  `json:"sample_instruments"`
			}
			type plan struct {
				Suggestions    []suggestion `json:"suggestions"`
				TotalSavingINR float64      `json:"total_potential_saving_rupees"`
				RegimeNote     string       `json:"regime_note"`
				Disclaimer     string       `json:"disclaimer"`
			}

			max0 := func(x float64) float64 {
				if x < 0 {
					return 0
				}
				return x
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			slab := req.BorrowerSlabPct / 100
			if slab <= 0 {
				slab = 0.3 // default 30% bracket if not supplied
			}
			dheadroomParents := limit80DParents
			if !req.ParentsSeniorCit {
				dheadroomParents = 25_000
			}

			sections := []struct {
				name string
				head float64
				desc string
			}{
				{"80C", max0(limit80C - req.Used.Sec80C),
					"ELSS mutual funds (3-yr lock), PPF, EPF VPF, 5-yr tax-saver FD, term-insurance premium."},
				{"80CCD(1B)", max0(limit80CCD1B - req.Used.Sec80CCD1B),
					"NPS Tier-I additional contribution; over and above 80C."},
				{"80D-self", max0(limit80DSelf - req.Used.Sec80DSelf),
					"Health insurance premium for self / spouse / dependent children."},
				{"80D-parents", max0(dheadroomParents - req.Used.Sec80DParents),
					"Health insurance premium for parents (₹50k cap if senior citizen)."},
				{"80TTA", max0(limit80TTA - req.Used.Sec80TTA),
					"Savings-account interest (declared in ITR; no investment required)."},
			}

			var p plan
			for _, s := range sections {
				if s.head <= 0 {
					continue
				}
				saved := s.head * slab
				p.Suggestions = append(p.Suggestions, suggestion{
					Section:     s.name,
					HeadroomINR: round2(s.head),
					TaxSavedINR: round2(saved),
					Instruments: s.desc,
				})
				p.TotalSavingINR += saved
			}
			sort.SliceStable(p.Suggestions, func(i, j int) bool {
				return p.Suggestions[i].TaxSavedINR > p.Suggestions[j].TaxSavedINR
			})
			p.TotalSavingINR = round2(p.TotalSavingINR)
			p.RegimeNote = "Chapter VI-A deductions apply to the OLD regime only. " +
				"If you have opted-in to the new regime, only the standard ₹75k deduction applies for FY 2024-25."
			p.Disclaimer = "Informational, advisory only per RBI FREE-AI. Each instrument has its own lock-in, taxability and risk profile."

			body, err := json.Marshal(p)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
