package catalog

import (
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// AlmAgentSpec ports agents/alm_agent. It computes Asset-Liability Mismatch
// (ALM) gaps across the standard RBI repricing buckets, the cumulative gap as a
// percentage of total assets (breach when it exceeds +/-15% in any bucket per
// the RBI ALM master direction), and a first-pass Net Interest Income (NII)
// sensitivity to a parallel rate shock.
func AlmAgentSpec() afg.Spec {
	return afg.Spec{
		ID:   "alm_agent",
		Risk: "high",
		Handle: func(input string) (string, error) {
			const breachThreshold = 0.15

			type bucketAmounts struct {
				Day1to7     float64 `json:"day_1_7"`
				Day8to14    float64 `json:"day_8_14"`
				Day15to30   float64 `json:"day_15_30"`
				Day31to90   float64 `json:"day_31_90"`
				Day91to180  float64 `json:"day_91_180"`
				Day181to365 float64 `json:"day_181_365"`
				Year1to3    float64 `json:"year_1_3"`
				Year3to5    float64 `json:"year_3_5"`
				Year5Plus   float64 `json:"year_5_plus"`
			}
			type request struct {
				Assets        bucketAmounts `json:"assets"`
				Liabilities   bucketAmounts `json:"liabilities"`
				RateShockBp   float64       `json:"rate_shock_bp"`
				TotalAssetINR float64       `json:"total_assets_for_breach_threshold"`
			}
			type gap struct {
				Bucket        string  `json:"bucket"`
				GapINR        float64 `json:"gap_rupees"`
				CumulativeINR float64 `json:"cumulative_rupees"`
				CumulativePct float64 `json:"cumulative_pct_of_assets"`
				BreachFlag    bool    `json:"breach_flag"`
			}
			type result struct {
				Gaps              []gap   `json:"gaps"`
				NIISensitivityINR float64 `json:"nii_sensitivity_rupees"`
				HasBreach         bool    `json:"has_breach"`
				Note              string  `json:"note"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
			round4 := func(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			type pair struct {
				name string
				a, l float64
			}
			buckets := []pair{
				{"1-7d", req.Assets.Day1to7, req.Liabilities.Day1to7},
				{"8-14d", req.Assets.Day8to14, req.Liabilities.Day8to14},
				{"15-30d", req.Assets.Day15to30, req.Liabilities.Day15to30},
				{"31-90d", req.Assets.Day31to90, req.Liabilities.Day31to90},
				{"91-180d", req.Assets.Day91to180, req.Liabilities.Day91to180},
				{"181-365d", req.Assets.Day181to365, req.Liabilities.Day181to365},
				{"1-3y", req.Assets.Year1to3, req.Liabilities.Year1to3},
				{"3-5y", req.Assets.Year3to5, req.Liabilities.Year3to5},
				{">5y", req.Assets.Year5Plus, req.Liabilities.Year5Plus},
			}
			gaps := make([]gap, 0, len(buckets))
			cum := 0.0
			hasBreach := false
			for _, b := range buckets {
				g := b.a - b.l
				cum += g
				pct := 0.0
				if req.TotalAssetINR > 0 {
					pct = cum / req.TotalAssetINR
				}
				breach := pct > breachThreshold || pct < -breachThreshold
				if breach {
					hasBreach = true
				}
				gaps = append(gaps, gap{
					Bucket:        b.name,
					GapINR:        round2(g),
					CumulativeINR: round2(cum),
					CumulativePct: round4(pct * 100),
					BreachFlag:    breach,
				})
			}
			// Crude NII sensitivity: shock x sum of short-term (within one year)
			// repricing gaps. A conventional duration-of-equity calc is heavier;
			// this gives a first-pass directional number.
			shortGap := buckets[0].a + buckets[1].a + buckets[2].a + buckets[3].a +
				buckets[4].a + buckets[5].a -
				(buckets[0].l + buckets[1].l + buckets[2].l + buckets[3].l +
					buckets[4].l + buckets[5].l)
			nii := shortGap * req.RateShockBp / 10_000.0

			res := result{
				Gaps:              gaps,
				NIISensitivityINR: round2(nii),
				HasBreach:         hasBreach,
				Note:              "Cumulative gap > ±15% of total assets in any bucket = breach (RBI ALM master direction).",
			}
			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
