package catalog

import (
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// DividendPlannerSpec ports agents/dividend_planner: a deterministic Dividend
// Reinvestment Plan (DRIP) simulator that projects the dividend stream and
// reinvested-share growth over N years, net of the recipient's slab + TDS.
func DividendPlannerSpec() afg.Spec {
	return afg.Spec{
		ID:   "dividend_planner",
		Risk: "low",
		Handle: func(input string) (string, error) {
			type request struct {
				Shares               int     `json:"shares"`
				CurrentPrice         float64 `json:"current_price_rupees"`
				DividendPerShare     float64 `json:"dividend_per_share_rupees"`
				DividendGrowthAnnual float64 `json:"dividend_growth_annual"` // decimal
				PriceAppreciationAnn float64 `json:"price_appreciation_annual"`
				HorizonYears         int     `json:"horizon_years"`
				TDSAndSlabPct        float64 `json:"tds_and_slab_pct"`
				Reinvest             bool    `json:"reinvest_dividends"`
			}
			type yearRow struct {
				Year         int     `json:"year"`
				Shares       float64 `json:"shares"`
				DPSGross     float64 `json:"dps_gross"`
				DividendNet  float64 `json:"dividend_net_rupees"`
				PriceEOY     float64 `json:"price_eoy"`
				HoldingValue float64 `json:"holding_value_eoy_rupees"`
			}
			type result struct {
				Schedule       []yearRow `json:"schedule"`
				TotalDividend  float64   `json:"total_dividend_received_net_rupees"`
				TerminalValue  float64   `json:"terminal_holding_value_rupees"`
				YieldOnCostPct float64   `json:"final_yield_on_cost_pct"`
				Disclaimer     string    `json:"disclaimer"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			round4 := func(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			shares := float64(req.Shares)
			price := req.CurrentPrice
			dps := req.DividendPerShare
			costBasis := shares * price
			totalNet := 0.0
			rows := make([]yearRow, 0, req.HorizonYears)
			for y := 1; y <= req.HorizonYears; y++ {
				grossDividend := shares * dps
				net := grossDividend * (1 - req.TDSAndSlabPct/100)
				totalNet += net
				// Apply price appreciation across the year first.
				price *= (1 + req.PriceAppreciationAnn)
				if req.Reinvest && price > 0 {
					shares += net / price
				}
				rows = append(rows, yearRow{
					Year:         y,
					Shares:       round4(shares),
					DPSGross:     round4(dps),
					DividendNet:  round2(net),
					PriceEOY:     round2(price),
					HoldingValue: round2(shares * price),
				})
				dps *= (1 + req.DividendGrowthAnnual)
			}
			terminal := shares * price
			yoc := 0.0
			if costBasis > 0 && len(rows) > 0 {
				yoc = rows[len(rows)-1].DividendNet / costBasis * 100
			}
			res := result{
				Schedule:       rows,
				TotalDividend:  round2(totalNet),
				TerminalValue:  round2(terminal),
				YieldOnCostPct: round2(yoc),
				Disclaimer: "Projection assumes fixed annual growth + price appreciation. " +
					"Indian dividends are taxed at recipient's slab + applicable TDS. Past dividends are not a guarantee of future payouts.",
			}
			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
