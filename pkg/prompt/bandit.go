package prompt

import (
	"math/rand"
	"sync"
)

// Bandit is a Thompson-sampling multi-armed bandit over prompt versions.
// Each "arm" is a version id; pulling an arm runs that prompt; recording a
// reward (0..1) tightens the posterior. Over time the bandit naturally
// routes traffic to the highest-reward prompt without manual A/B cuts.
//
// We model rewards as Bernoulli — every record() either succeeded or
// didn't. Use 0.5 to mean "ambiguous" if you need a middle ground.
type Bandit struct {
	mu     sync.Mutex
	arms   map[string]*armStats
	rng    *rand.Rand
	choose []string
}

type armStats struct {
	alpha, beta float64
}

// NewBandit constructs an empty bandit with a seeded RNG (use 0 for time-based).
func NewBandit(seed int64) *Bandit {
	src := rand.NewSource(seed)
	if seed == 0 {
		src = rand.NewSource(int64(1)) // deterministic by default for tests
	}
	return &Bandit{arms: map[string]*armStats{}, rng: rand.New(src)}
}

// Register adds an arm with a Beta(1,1) prior (uniform). Idempotent.
func (b *Bandit) Register(version string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.arms[version]; ok {
		return
	}
	b.arms[version] = &armStats{alpha: 1, beta: 1}
	b.choose = append(b.choose, version)
}

// Choose samples a version using Thompson sampling: draw θ ~ Beta(α, β)
// for each arm, return the arm with the highest θ.
func (b *Bandit) Choose() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.choose) == 0 {
		return ""
	}
	var best string
	bestTheta := -1.0
	for _, v := range b.choose {
		a := b.arms[v]
		theta := sampleBeta(b.rng, a.alpha, a.beta)
		if theta > bestTheta {
			bestTheta = theta
			best = v
		}
	}
	return best
}

// Record updates the posterior for a version. Reward 1 = success, 0 = fail.
func (b *Bandit) Record(version string, reward float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.arms[version]
	if !ok {
		return
	}
	if reward < 0 {
		reward = 0
	}
	if reward > 1 {
		reward = 1
	}
	a.alpha += reward
	a.beta += 1 - reward
}

// Stats returns a snapshot of the current posteriors.
func (b *Bandit) Stats() map[string]struct{ Alpha, Beta float64 } {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := map[string]struct{ Alpha, Beta float64 }{}
	for k, v := range b.arms {
		out[k] = struct{ Alpha, Beta float64 }{v.alpha, v.beta}
	}
	return out
}

// sampleBeta draws from Beta(alpha, beta) via the gamma-ratio trick.
// math/rand has Gamma via rejection; rand has NormFloat but not Gamma.
// For Bernoulli rewards with small (alpha, beta) we can use a simple
// Marsaglia-Tsang sampler implemented here.
func sampleBeta(rng *rand.Rand, alpha, beta float64) float64 {
	x := sampleGamma(rng, alpha)
	y := sampleGamma(rng, beta)
	if x+y == 0 {
		return 0
	}
	return x / (x + y)
}

// sampleGamma uses the Marsaglia & Tsang method (2000).
func sampleGamma(rng *rand.Rand, shape float64) float64 {
	if shape < 1 {
		u := rng.Float64()
		return sampleGamma(rng, shape+1) * pow(u, 1/shape)
	}
	d := shape - 1.0/3.0
	c := 1.0 / sqrt(9*d)
	for {
		x := rng.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.0331*x*x*x*x {
			return d * v
		}
		if log(u) < 0.5*x*x+d*(1-v+log(v)) {
			return d * v
		}
	}
}

// Tiny math wrappers — kept inline so this file doesn't need a math import
// at every line. (We do import math via aliases below.)
func pow(a, b float64) float64 { return mathPow(a, b) }
func sqrt(x float64) float64   { return mathSqrt(x) }
func log(x float64) float64    { return mathLog(x) }
