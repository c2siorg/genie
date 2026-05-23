package mid

import "testing"

func TestRateLimit_TokenBucket(t *testing.T) {
	rl := NewRateLimit(2, 0) // 2 tokens, no refill
	ok, _ := rl.Allow("a")
	if !ok {
		t.Fatal("first request should be allowed")
	}
	ok, _ = rl.Allow("a")
	if !ok {
		t.Fatal("second request should be allowed")
	}
	ok, _ = rl.Allow("a")
	if ok {
		t.Fatal("third request should be rate-limited")
	}

	// Different key has its own bucket.
	ok, _ = rl.Allow("b")
	if !ok {
		t.Fatal("different key should have its own bucket")
	}
}
