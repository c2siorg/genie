package drift

import "testing"

func TestKL_ZeroForSameDist(t *testing.T) {
	d := FromCounts(map[string]int{"a": 5, "b": 5})
	if kl := KL(d, d); kl > 1e-6 {
		t.Fatalf("KL of same dist should be ~0, got %f", kl)
	}
}

func TestKL_PositiveForDifferent(t *testing.T) {
	a := FromCounts(map[string]int{"a": 9, "b": 1})
	b := FromCounts(map[string]int{"a": 1, "b": 9})
	if kl := KL(a, b); kl <= 0.1 {
		t.Fatalf("expected sizeable drift, got %f", kl)
	}
}
