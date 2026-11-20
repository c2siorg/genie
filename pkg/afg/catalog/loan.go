package catalog

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// LoanSpec ports agents/loan, a Small Business Loan-style helper. Given a
// principal, APR, term, and monthly net cashflow, it estimates the EMI using the
// standard amortisation formula, computes the debt-service coverage ratio (DSCR =
// monthly net / EMI), and marks eligibility against a minimum DSCR threshold of
// 1.25. All arithmetic is pure and deterministic. Amounts are in minor units
// (paise/cents).
func LoanSpec() afg.Spec {
	return afg.Spec{
		ID:   "loan_advisor",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const minDSCR = 1.25

			// Request mirrors the legacy wire payload.
			type Request struct {
				PrincipalCents  int64   `json:"principal_cents"`
				APRPct          float64 `json:"apr_pct"`
				TermMonths      int     `json:"term_months"`
				MonthlyNetCents int64   `json:"monthly_net_cents"`
			}
			// Response mirrors the legacy wire output.
			type Response struct {
				EMIInCents int64   `json:"emi_cents"`
				TotalCents int64   `json:"total_cents"`
				DSCR       float64 `json:"dscr"`
				Eligible   bool    `json:"eligible"`
				Reason     string  `json:"reason"`
			}

			computeEMI := func(principalCents int64, aprPct float64, termMonths int) int64 {
				monthlyRate := (aprPct / 100.0) / 12.0
				p := float64(principalCents)
				if monthlyRate == 0 {
					return int64(math.Round(p / float64(termMonths)))
				}
				factor := math.Pow(1+monthlyRate, float64(termMonths))
				emi := p * monthlyRate * factor / (factor - 1)
				return int64(math.Round(emi))
			}

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}
			if req.TermMonths <= 0 || req.PrincipalCents <= 0 || req.APRPct < 0 {
				return "", errors.New("invalid loan parameters")
			}

			emi := computeEMI(req.PrincipalCents, req.APRPct, req.TermMonths)
			resp := Response{
				EMIInCents: emi,
				TotalCents: emi * int64(req.TermMonths),
			}
			if emi > 0 {
				resp.DSCR = float64(req.MonthlyNetCents) / float64(emi)
			}
			resp.Eligible = resp.DSCR >= minDSCR
			if resp.Eligible {
				resp.Reason = "DSCR above threshold"
			} else {
				resp.Reason = "DSCR below threshold; consider lower principal or longer term"
			}

			body, err := json.Marshal(resp)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
