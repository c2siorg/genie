// Package federated implements FedAvg + a textbook additive secret-sharing
// aggregator. Designed for cross-RE training scenarios the RBI FREE-AI
// report describes in Recommendation 4 (indigenous sector models, trained
// without moving raw data across institutions).
//
// Workflow:
//
//  1. Coordinator initialises a global model (a Weights vector).
//  2. Each round: Coordinator publishes Weights to N workers.
//  3. Workers train locally on their private data and return updated
//     weights + a sample count.
//  4. Coordinator aggregates with FedAvg (weighted by sample count).
//
// Secure aggregation wraps step 3 so the coordinator only ever sees the
// sum across all workers — never any individual update — using additive
// secret-sharing over a large modulus. Not a full Bonawitz et al.
// implementation (no key agreement / dropouts), but the core primitive.
package federated

import (
	"crypto/rand"
	"errors"
	"math/big"
)

// Weights is a flat float64 vector representing a model's parameters.
type Weights []float64

// Update is one worker's contribution to a round.
type Update struct {
	WorkerID string
	Samples  int
	Weights  Weights
}

// FedAvg returns the sample-count-weighted average of the updates.
//
// Each update's weights vector must have the same length as the others;
// otherwise FedAvg returns an error rather than silently truncating.
func FedAvg(updates []Update) (Weights, error) {
	if len(updates) == 0 {
		return nil, errors.New("federated: no updates")
	}
	dim := len(updates[0].Weights)
	for _, u := range updates[1:] {
		if len(u.Weights) != dim {
			return nil, errors.New("federated: dimension mismatch")
		}
	}
	avg := make(Weights, dim)
	var totalSamples float64
	for _, u := range updates {
		totalSamples += float64(u.Samples)
	}
	if totalSamples == 0 {
		return nil, errors.New("federated: total samples is zero")
	}
	for _, u := range updates {
		w := float64(u.Samples) / totalSamples
		for i, x := range u.Weights {
			avg[i] += w * x
		}
	}
	return avg, nil
}

// ---------- secure aggregation ----------

// Aggregator collects additive shares from N workers and reveals only the
// sum once all shares arrive. Each worker splits its update into N shares
// such that share_1 + share_2 + ... + share_N ≡ update (mod P).
//
// Sketch: peer_i sends share_i,j to peer j; each peer publishes the sum of
// shares it received. The Aggregator sums those public sums to get the
// reconstructed total — no peer's individual update is recoverable.
//
// We model the "publish" step as PostShare; production uses peer-to-peer
// + dropouts handling. The arithmetic here is in modular Z_P.
type Aggregator struct {
	Modulus *big.Int
	Dim     int
	Workers int

	received int
	sum      []*big.Int
}

// NewAggregator initialises sums to zero in Z_P^Dim.
func NewAggregator(workers, dim int, modulus *big.Int) *Aggregator {
	sums := make([]*big.Int, dim)
	for i := range sums {
		sums[i] = new(big.Int)
	}
	return &Aggregator{Modulus: modulus, Dim: dim, Workers: workers, sum: sums}
}

// PostShare adds one worker's public sum-of-shares to the aggregator.
// Returns true when all workers have posted.
func (a *Aggregator) PostShare(share []*big.Int) (bool, error) {
	if len(share) != a.Dim {
		return false, errors.New("federated: share dim mismatch")
	}
	for i, s := range share {
		a.sum[i].Add(a.sum[i], s)
		a.sum[i].Mod(a.sum[i], a.Modulus)
	}
	a.received++
	return a.received >= a.Workers, nil
}

// Reveal returns the aggregated sum. Call only after PostShare returns true.
func (a *Aggregator) Reveal() []*big.Int {
	out := make([]*big.Int, a.Dim)
	for i, s := range a.sum {
		out[i] = new(big.Int).Set(s)
	}
	return out
}

// SplitAdditiveShares splits a Z_P value into n shares whose sum mod P
// equals the original. Workers use this on each weight, then route
// share_i,j to peer j.
func SplitAdditiveShares(value *big.Int, n int, modulus *big.Int) ([]*big.Int, error) {
	if n < 2 {
		return nil, errors.New("federated: n must be >= 2")
	}
	shares := make([]*big.Int, n)
	acc := new(big.Int)
	for i := 0; i < n-1; i++ {
		r, err := rand.Int(rand.Reader, modulus)
		if err != nil {
			return nil, err
		}
		shares[i] = r
		acc.Add(acc, r)
	}
	last := new(big.Int).Sub(value, acc)
	last.Mod(last, modulus)
	// Go's Mod can return a negative remainder for negative dividends; fix.
	if last.Sign() < 0 {
		last.Add(last, modulus)
	}
	shares[n-1] = last
	return shares, nil
}
