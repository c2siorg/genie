// Package drift measures output distribution drift between a baseline and
// a recent window using KL divergence over discrete buckets.
//
// Use it to detect when an agent's outputs are shifting — e.g. the
// recommender starts proposing different categories week over week.
package drift

import "math"

// Distribution is a discrete probability distribution keyed by bucket.
type Distribution map[string]float64

// Normalize ensures the distribution sums to 1.
func Normalize(d Distribution) Distribution {
	var sum float64
	for _, v := range d {
		sum += v
	}
	if sum == 0 {
		return d
	}
	out := make(Distribution, len(d))
	for k, v := range d {
		out[k] = v / sum
	}
	return out
}

// FromCounts builds a Distribution from raw counts.
func FromCounts(counts map[string]int) Distribution {
	d := make(Distribution, len(counts))
	for k, v := range counts {
		d[k] = float64(v)
	}
	return Normalize(d)
}

// KL returns the KL divergence from baseline to recent.
// Returns 0 when the distributions match; positive otherwise.
// Smoothing avoids the math.Log(0) explosion.
func KL(baseline, recent Distribution) float64 {
	const eps = 1e-9
	b := Normalize(baseline)
	r := Normalize(recent)
	keys := map[string]struct{}{}
	for k := range b {
		keys[k] = struct{}{}
	}
	for k := range r {
		keys[k] = struct{}{}
	}
	var kl float64
	for k := range keys {
		p := r[k] + eps
		q := b[k] + eps
		kl += p * math.Log(p/q)
	}
	return kl
}
