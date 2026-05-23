package prompt

import "testing"

func TestBandit_PrefersHigherRewardArm(t *testing.T) {
	b := NewBandit(7)
	b.Register("v1")
	b.Register("v2")
	// Hammer v1 with successes, v2 with failures.
	for i := 0; i < 50; i++ {
		b.Record("v1", 1)
		b.Record("v2", 0)
	}
	// After many rounds, Choose should overwhelmingly favour v1.
	v1Hits := 0
	for i := 0; i < 200; i++ {
		if b.Choose() == "v1" {
			v1Hits++
		}
	}
	if v1Hits < 180 {
		t.Fatalf("expected bandit to converge on v1; got %d/200 hits", v1Hits)
	}
}
