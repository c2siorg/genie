package privacy

import (
	"math/rand"
	"testing"
)

func TestTokeniser_DeterministicSameSecret(t *testing.T) {
	tok, err := NewTokeniser([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	a := tok.Tokenise("4111111111111111")
	b := tok.Tokenise("4111111111111111")
	if a != b {
		t.Fatal("same input should yield same token")
	}
	c := tok.Tokenise("4222222222222222")
	if a == c {
		t.Fatal("different inputs should differ")
	}
}

func TestAggregateWithDP_AddsNoise(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	v := AggregateWithDP(rng, 100, 1, 1)
	if v == 100 {
		t.Fatal("expected non-zero noise")
	}
}
