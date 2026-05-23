package safety

import "math"

// DemographicParity measures the absolute difference between the positive-
// outcome rate across two subgroups. Closer to 0 = fairer.
//
// pA, pB are counts of positive outcomes; nA, nB are subgroup totals.
type DemographicParity struct {
	GapAbs    float64 `json:"gap_abs"`
	GapRatio  float64 `json:"gap_ratio"` // min/max; closer to 1 = fairer
	RateA     float64 `json:"rate_a"`
	RateB     float64 `json:"rate_b"`
	Acceptable bool   `json:"acceptable"`
}

// ComputeDemographicParity returns the gap stats. Acceptable is true when
// GapAbs <= threshold (defaults to 0.1 if non-positive).
func ComputeDemographicParity(pA, nA, pB, nB int, threshold float64) DemographicParity {
	if threshold <= 0 {
		threshold = 0.1
	}
	d := DemographicParity{}
	if nA > 0 {
		d.RateA = float64(pA) / float64(nA)
	}
	if nB > 0 {
		d.RateB = float64(pB) / float64(nB)
	}
	d.GapAbs = math.Abs(d.RateA - d.RateB)
	if max(d.RateA, d.RateB) > 0 {
		d.GapRatio = min(d.RateA, d.RateB) / max(d.RateA, d.RateB)
	} else {
		d.GapRatio = 1
	}
	d.Acceptable = d.GapAbs <= threshold
	return d
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
