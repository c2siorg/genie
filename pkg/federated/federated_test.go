package federated

import (
	"math/big"
	"testing"
)

func TestFedAvg_WeightedBySamples(t *testing.T) {
	updates := []Update{
		{WorkerID: "a", Samples: 100, Weights: Weights{1, 0}},
		{WorkerID: "b", Samples: 300, Weights: Weights{0, 1}},
	}
	avg, err := FedAvg(updates)
	if err != nil {
		t.Fatal(err)
	}
	// weighted average: 25% * [1,0] + 75% * [0,1] = [0.25, 0.75]
	if !approxEqual(avg[0], 0.25) || !approxEqual(avg[1], 0.75) {
		t.Fatalf("unexpected avg: %v", avg)
	}
}

func TestFedAvg_DimensionMismatchFails(t *testing.T) {
	updates := []Update{
		{Samples: 1, Weights: Weights{1, 2}},
		{Samples: 1, Weights: Weights{1, 2, 3}},
	}
	if _, err := FedAvg(updates); err == nil {
		t.Fatal("expected dimension error")
	}
}

func TestSecureAggregation_ReconstructsSum(t *testing.T) {
	modulus, _ := new(big.Int).SetString("1000000007", 10) // small prime for test
	// Three workers with values 5, 10, 20. Expected sum = 35.
	values := []int64{5, 10, 20}
	workers := len(values)

	// Each worker splits its (single-dim) value into N shares.
	allShares := make([][]*big.Int, workers)
	for i, v := range values {
		s, err := SplitAdditiveShares(big.NewInt(v), workers, modulus)
		if err != nil {
			t.Fatal(err)
		}
		allShares[i] = s
	}
	// Each worker j publishes sum of shares it received from every peer.
	publicSums := make([]*big.Int, workers)
	for j := 0; j < workers; j++ {
		s := new(big.Int)
		for i := 0; i < workers; i++ {
			s.Add(s, allShares[i][j])
		}
		s.Mod(s, modulus)
		publicSums[j] = s
	}
	agg := NewAggregator(workers, 1, modulus)
	for _, s := range publicSums {
		_, _ = agg.PostShare([]*big.Int{s})
	}
	reveal := agg.Reveal()
	if reveal[0].Cmp(big.NewInt(35)) != 0 {
		t.Fatalf("expected 35, got %s", reveal[0].String())
	}
}

func approxEqual(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
