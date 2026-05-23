// Package elo ranks prompt or model versions by pairwise wins using the
// classic Elo formula.
//
// Use case: A/B between prompt versions or LLM providers. Record outcomes
// of head-to-head evaluations (LLM-as-judge, human prefs) and pull the
// current ratings to drive routing.
package elo

import "math"

// K is the maximum rating change per match. 32 matches FIDE for casual
// players; lower to stabilise once you have many matches.
const K = 32

// Ratings is a name -> Elo rating map. Default rating is 1500.
type Ratings map[string]float64

// New returns a fresh Ratings map.
func New() Ratings { return Ratings{} }

// Get returns the rating or 1500 if absent.
func (r Ratings) Get(name string) float64 {
	if v, ok := r[name]; ok {
		return v
	}
	return 1500
}

// Outcome is who won a head-to-head match.
type Outcome int

const (
	OutcomeA   Outcome = iota // A wins
	OutcomeB                  // B wins
	OutcomeTie                // draw / tie
)

// Match updates the ratings based on the outcome.
func (r Ratings) Match(a, b string, outcome Outcome) {
	ra := r.Get(a)
	rb := r.Get(b)
	ea := 1 / (1 + math.Pow(10, (rb-ra)/400))
	eb := 1 - ea
	var sa, sb float64
	switch outcome {
	case OutcomeA:
		sa, sb = 1, 0
	case OutcomeB:
		sa, sb = 0, 1
	case OutcomeTie:
		sa, sb = 0.5, 0.5
	}
	r[a] = ra + K*(sa-ea)
	r[b] = rb + K*(sb-eb)
}

// Leaderboard returns names sorted by rating (desc).
func (r Ratings) Leaderboard() []string {
	type entry struct {
		name   string
		rating float64
	}
	out := make([]entry, 0, len(r))
	for k, v := range r {
		out = append(out, entry{k, v})
	}
	// Insertion sort — small N.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].rating > out[j-1].rating; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	names := make([]string, len(out))
	for i, e := range out {
		names[i] = e.name
	}
	return names
}
