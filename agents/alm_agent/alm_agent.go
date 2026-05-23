// Package alm_agent computes Asset-Liability Mismatch (ALM) gaps across
// standard repricing buckets. Output is the cumulative gap and a simple
// Net Interest Income (NII) sensitivity to a parallel rate shock.
//
// Buckets follow RBI's ALM master direction:
//   1-7d, 8-14d, 15-30d, 31-90d, 91-180d, 181-365d, 1-3y, 3-5y, >5y.
//
// Cumulative-gap > 15% of total assets in any bucket = breach.
package alm_agent

import (
	"context"
	"encoding/json"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "alm_agent"
	Capability = "compute_alm_gap"
	TypeIn     = "alm_request"
	TypeOut    = "alm_result"
	NextAgent  = "financial_supervisor"

	BreachThreshold = 0.15
)

// BucketAmounts is the assets/liabilities split per bucket (₹ in any
// consistent unit — paise, lakhs, crores).
type BucketAmounts struct {
	Day1to7      float64 `json:"day_1_7"`
	Day8to14     float64 `json:"day_8_14"`
	Day15to30    float64 `json:"day_15_30"`
	Day31to90    float64 `json:"day_31_90"`
	Day91to180   float64 `json:"day_91_180"`
	Day181to365  float64 `json:"day_181_365"`
	Year1to3     float64 `json:"year_1_3"`
	Year3to5     float64 `json:"year_3_5"`
	Year5Plus    float64 `json:"year_5_plus"`
}

// Request is the wire payload.
type Request struct {
	Assets        BucketAmounts `json:"assets"`
	Liabilities   BucketAmounts `json:"liabilities"`
	RateShockBp   float64       `json:"rate_shock_bp"`
	TotalAssetINR float64       `json:"total_assets_for_breach_threshold"`
}

// Gap is one bucket's gap.
type Gap struct {
	Bucket         string  `json:"bucket"`
	GapINR         float64 `json:"gap_rupees"`
	CumulativeINR  float64 `json:"cumulative_rupees"`
	CumulativePct  float64 `json:"cumulative_pct_of_assets"`
	BreachFlag     bool    `json:"breach_flag"`
}

// Result is the wire output.
type Result struct {
	Gaps                 []Gap   `json:"gaps"`
	NIISensitivityINR    float64 `json:"nii_sensitivity_rupees"`
	HasBreach            bool    `json:"has_breach"`
	Note                 string  `json:"note"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "ALM Agent" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	res := a.Compute(req)
	env.Logf("[alm_agent] breach=%v NII shock=%.0f", res.HasBreach, res.NIISensitivityINR)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Compute walks the buckets and returns the gap table + NII shock.
func (a *Agent) Compute(req Request) Result {
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
	gaps := make([]Gap, 0, len(buckets))
	cum := 0.0
	hasBreach := false
	for _, b := range buckets {
		g := b.a - b.l
		cum += g
		pct := 0.0
		if req.TotalAssetINR > 0 {
			pct = cum / req.TotalAssetINR
		}
		breach := pct > BreachThreshold || pct < -BreachThreshold
		if breach {
			hasBreach = true
		}
		gaps = append(gaps, Gap{
			Bucket:        b.name,
			GapINR:        round2(g),
			CumulativeINR: round2(cum),
			CumulativePct: round4(pct * 100),
			BreachFlag:    breach,
		})
	}
	// Crude NII sensitivity: shock × sum of (short-term gaps reprice within
	// the next year). Conventional duration-of-equity calc is heavier; this
	// gives a first-pass directional number.
	shortGap := buckets[0].a + buckets[1].a + buckets[2].a + buckets[3].a +
		buckets[4].a + buckets[5].a -
		(buckets[0].l + buckets[1].l + buckets[2].l + buckets[3].l +
			buckets[4].l + buckets[5].l)
	nii := shortGap * req.RateShockBp / 10_000.0
	return Result{
		Gaps:              gaps,
		NIISensitivityINR: round2(nii),
		HasBreach:         hasBreach,
		Note:              "Cumulative gap > ±15% of total assets in any bucket = breach (RBI ALM master direction).",
	}
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }
