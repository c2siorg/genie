package catalog

import (
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// SmeLoanWorkflowSpec ports agents/sme_loan_workflow. It reproduces the SME
// lending journey — GST turnover fetch, cashflow-score floor check, CGTMSE
// collateral-free-guarantee eligibility, indicative offer (amount capped to 30%
// of annual turnover, rate = 10.5% floor + cashflow risk premium minus a CGTMSE
// relief, reducing-balance EMI), and a relationship-manager approval gate that
// the synchronous handler auto-approves. All arithmetic is pure and
// deterministic. The DAG's human-approval gate short-circuits to approved in
// this synchronous path, exactly as the legacy HandleMessage did with
// autoApprove=true. Amounts are in rupees. Outputs are indicative/advisory per
// RBI FREE-AI; final sanction is subject to the credit committee, complete
// documentation, and CGTMSE registration where applicable.
func SmeLoanWorkflowSpec() afg.Spec {
	return afg.Spec{
		ID:   "sme_loan_workflow",
		Risk: "high",
		Handle: func(input string) (string, error) {
			const (
				cgtmseMaxTicketRupees = 50_000_000 // ₹5 cr revised ceiling (2023)
				maxLoanMultipleOfRev  = 0.30       // 30% of annual turnover
			)

			// Application mirrors the legacy inbound packet.
			type Application struct {
				BorrowerID         string  `json:"borrower_id"`
				UDYAMRegistered    bool    `json:"udyam_registered"`
				Sector             string  `json:"sector"` // manufacturing | services | trading
				AnnualTurnover     float64 `json:"annual_turnover_rupees"`
				GSTFilingRegular   bool    `json:"gst_filing_regular"`
				RequestedAmount    float64 `json:"requested_amount_rupees"`
				RequestedTenorMths int     `json:"requested_tenor_months"`
				CashflowScore0to1  float64 `json:"cashflow_score_0_1"`
				CollateralRupees   float64 `json:"collateral_rupees"`
			}
			// Offer mirrors the legacy structured output.
			type Offer struct {
				BorrowerID        string   `json:"borrower_id"`
				Decision          string   `json:"decision"` // "approved" | "in_principle" | "rejected"
				OfferedAmount     float64  `json:"offered_amount_rupees"`
				OfferedTenorMths  int      `json:"offered_tenor_months"`
				IndicativeRatePct float64  `json:"indicative_rate_pct"`
				MonthlyEMIRupees  float64  `json:"monthly_emi_rupees"`
				CGTMSEEligible    bool     `json:"cgtmse_eligible"`
				Rationale         []string `json:"rationale"`
				WorkflowEvents    int      `json:"workflow_event_count"`
				Disclaimer        string   `json:"disclaimer"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			isCovered := func(sector string) bool {
				switch sector {
				case "manufacturing", "services", "trading":
					return true
				}
				return false
			}

			// emi returns the standard reducing-balance EMI.
			emi := func(principal, annualRatePct float64, months int) float64 {
				if months <= 0 || principal <= 0 {
					return 0
				}
				r := annualRatePct / 12.0 / 100.0
				if r == 0 {
					return principal / float64(months)
				}
				pow := 1.0
				for i := 0; i < months; i++ {
					pow *= 1 + r
				}
				return principal * r * pow / (pow - 1)
			}

			var app Application
			if err := json.Unmarshal([]byte(input), &app); err != nil {
				return "", err
			}

			// Step 1: gst_fetch — requires turnover data.
			gstOK := app.AnnualTurnover > 0

			// Step 2: cashflow_analysis — floor at 0.30.
			cashflowPass := gstOK && app.CashflowScore0to1 >= 0.30

			// Step 3: cgtmse_eligibility.
			cgtmseEligible := app.UDYAMRegistered &&
				app.RequestedAmount <= cgtmseMaxTicketRupees &&
				isCovered(app.Sector)

			// Step 4: indicative_offer (only when the underwriting steps pass).
			var offeredAmt, offeredRate, offeredEMI float64
			if gstOK && cashflowPass {
				offeredAmt = app.RequestedAmount
				if maxByTurnover := app.AnnualTurnover * maxLoanMultipleOfRev; offeredAmt > maxByTurnover {
					offeredAmt = maxByTurnover
				}
				offeredRate = 10.5 + (1.0-app.CashflowScore0to1)*5.0
				if cgtmseEligible {
					offeredRate -= 0.5
				}
				offeredEMI = emi(offeredAmt, offeredRate, app.RequestedTenorMths)
			}

			// Step 5 (human_approval) is auto-approved on the synchronous path,
			// and step 6 (sanction_letter) drafts the letter. Decision mirrors
			// buildOfferFromState with human_approved=true when checks clear.
			rationale := []string{}
			decision := "rejected"
			switch {
			case !cashflowPass:
				rationale = append(rationale, "Cashflow score below underwriting threshold")
			default:
				decision = "approved"
				rationale = append(rationale, "All checks cleared; sanction letter drafted")
			}
			if cgtmseEligible {
				rationale = append(rationale, "CGTMSE eligible — collateral-free guarantee applicable")
			}

			offer := Offer{
				BorrowerID:        app.BorrowerID,
				Decision:          decision,
				OfferedAmount:     round2(offeredAmt),
				OfferedTenorMths:  app.RequestedTenorMths,
				IndicativeRatePct: round2(offeredRate),
				MonthlyEMIRupees:  round2(offeredEMI),
				CGTMSEEligible:    cgtmseEligible,
				Rationale:         rationale,
				WorkflowEvents:    0,
				Disclaimer: "Indicative SME loan offer. Final sanction subject to credit committee, " +
					"complete documentation, and CGTMSE registration where applicable.",
			}

			body, err := json.Marshal(offer)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
