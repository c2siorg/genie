package catalog

import (
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// WorkingCapitalSpec ports agents/working_capital. It forecasts SME cashflow
// from the working-capital cycle: Days Sales Outstanding (DSO) + Days Inventory
// Outstanding (DIO) - Days Payable Outstanding (DPO) = Cash Conversion Cycle
// (CCC). It projects month-by-month operating cashflow and reports the runway
// (months of cash) before the balance turns negative under the current burn
// rate. Deterministic arithmetic, no LLM, no network.
func WorkingCapitalSpec() afg.Spec {
	return afg.Spec{
		ID:   "working_capital_forecaster",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			// Wire payload — mirrors the legacy Request.
			type request struct {
				MonthlyRevenue        float64 `json:"monthly_revenue_rupees"`
				GrossMarginPct        float64 `json:"gross_margin_pct"`
				OperatingCostsMonthly float64 `json:"operating_costs_monthly_rupees"`
				DSO                   int     `json:"dso_days"`
				DIO                   int     `json:"dio_days"`
				DPO                   int     `json:"dpo_days"`
				OpeningCashINR        float64 `json:"opening_cash_rupees"`
				HorizonMonths         int     `json:"horizon_months"`
			}
			// One period of the forecast — mirrors the legacy MonthRow.
			type monthRow struct {
				Month         int     `json:"month"`
				CashCollected float64 `json:"cash_collected_rupees"`
				CashPaid      float64 `json:"cash_paid_rupees"`
				NetCashflow   float64 `json:"net_cashflow_rupees"`
				EndCash       float64 `json:"end_cash_rupees"`
			}
			// Wire output — mirrors the legacy Result.
			type result struct {
				CCC               int        `json:"cash_conversion_cycle_days"`
				RunwayMonths      int        `json:"runway_months"`
				Forecast          []monthRow `json:"forecast"`
				WorkingCapitalGap float64    `json:"working_capital_gap_rupees"`
				Recommendation    string     `json:"recommendation"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			ccc := req.DSO + req.DIO - req.DPO

			// Cash collected this month = revenue from ceil(DSO/30) months ago.
			// Cash paid this month = COGS (current month) shifted by DPO + opex.
			cogs := req.MonthlyRevenue * (1 - req.GrossMarginPct/100)
			cash := req.OpeningCashINR
			rows := make([]monthRow, 0, req.HorizonMonths)
			runway := req.HorizonMonths
			for m := 1; m <= req.HorizonMonths; m++ {
				// Sales originated DSO days ago turn into cash now.
				collected := req.MonthlyRevenue
				if m <= req.DSO/30 {
					collected = 0 // not yet collected for the first DSO/30 months
				}
				// Vendor payment lagged by DPO days.
				paid := cogs
				if m <= req.DPO/30 {
					paid = 0
				}
				paid += req.OperatingCostsMonthly
				net := collected - paid
				cash += net
				rows = append(rows, monthRow{
					Month:         m,
					CashCollected: round2(collected),
					CashPaid:      round2(paid),
					NetCashflow:   round2(net),
					EndCash:       round2(cash),
				})
				if cash < 0 && runway == req.HorizonMonths {
					runway = m - 1
				}
			}

			gap := 0.0
			if len(rows) > 0 && rows[0].EndCash < 0 {
				gap = -rows[0].EndCash
			}

			rec := "Healthy cycle — monitor DSO drift quarterly."
			if runway < 6 {
				rec = "Runway <6 months — accelerate collections (early-payment discount) or extend DPO with vendor financing."
			}
			if ccc > 90 {
				rec = "Cash Conversion Cycle >90 days — consider invoice discounting (TReDS) to free up working capital."
			}

			out := result{
				CCC:               ccc,
				RunwayMonths:      runway,
				Forecast:          rows,
				WorkingCapitalGap: round2(gap),
				Recommendation:    rec,
			}
			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
