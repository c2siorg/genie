package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

// fakeClock is an injectable, manually-advanced clock for deterministic
// cooldown tests.
type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time          { return f.t }
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func fail(context.Context) error { return errBoom }
func ok(context.Context) error   { return nil }

func TestCircuit_OpensAfterThreshold(t *testing.T) {
	c := New(3, time.Minute)

	// First two failures stay closed.
	for i := 0; i < 2; i++ {
		if err := c.Do(context.Background(), fail); err != errBoom {
			t.Fatalf("call %d: got %v, want errBoom", i, err)
		}
	}
	if c.State() != StateClosed {
		t.Fatalf("after 2 failures state=%v, want closed", c.State())
	}

	// Third failure trips the breaker.
	if err := c.Do(context.Background(), fail); err != errBoom {
		t.Fatalf("3rd call: got %v, want errBoom", err)
	}
	if c.State() != StateOpen {
		t.Fatalf("after 3 failures state=%v, want open", c.State())
	}
}

func TestCircuit_OpenShortCircuitsWithoutCallingFn(t *testing.T) {
	c := New(1, time.Minute)
	// One failure opens it (threshold 1).
	_ = c.Do(context.Background(), fail)
	if c.State() != StateOpen {
		t.Fatalf("state=%v, want open", c.State())
	}

	called := false
	err := c.Do(context.Background(), func(context.Context) error {
		called = true
		return nil
	})
	if err != ErrOpen {
		t.Fatalf("got %v, want ErrOpen", err)
	}
	if called {
		t.Fatal("fn was called while breaker open — should have short-circuited")
	}
}

func TestCircuit_HalfOpenThenCloseOnSuccess(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := New(1, 30*time.Second)
	c.now = clk.now

	// Trip it.
	_ = c.Do(context.Background(), fail)
	if !c.IsOpen() {
		t.Fatal("expected open immediately after trip")
	}

	// Before cooldown: still open, short-circuits.
	clk.advance(10 * time.Second)
	if err := c.Do(context.Background(), ok); err != ErrOpen {
		t.Fatalf("before cooldown got %v, want ErrOpen", err)
	}

	// After cooldown: probe allowed, success closes the breaker.
	clk.advance(30 * time.Second)
	if c.IsOpen() {
		t.Fatal("after cooldown IsOpen should be false (probe allowed)")
	}
	if err := c.Do(context.Background(), ok); err != nil {
		t.Fatalf("probe got %v, want nil", err)
	}
	if c.State() != StateClosed {
		t.Fatalf("after successful probe state=%v, want closed", c.State())
	}
}

func TestCircuit_HalfOpenReopensOnFailure(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := New(1, 30*time.Second)
	c.now = clk.now

	_ = c.Do(context.Background(), fail) // open
	clk.advance(31 * time.Second)        // cooldown elapsed → probe allowed

	// Failed probe must re-open and refresh the cooldown window.
	if err := c.Do(context.Background(), fail); err != errBoom {
		t.Fatalf("probe got %v, want errBoom", err)
	}
	if c.State() != StateOpen {
		t.Fatalf("after failed probe state=%v, want open", c.State())
	}
	// Immediately after re-open, calls short-circuit again.
	if err := c.Do(context.Background(), ok); err != ErrOpen {
		t.Fatalf("got %v, want ErrOpen after re-open", err)
	}
}

func TestCircuit_SuccessResetsFailureCount(t *testing.T) {
	c := New(3, time.Minute)
	_ = c.Do(context.Background(), fail) // failures=1
	_ = c.Do(context.Background(), fail) // failures=2
	_ = c.Do(context.Background(), ok)   // reset → failures=0

	// Two more failures must NOT open it (count was reset).
	_ = c.Do(context.Background(), fail)
	_ = c.Do(context.Background(), fail)
	if c.State() != StateClosed {
		t.Fatalf("state=%v, want closed (success should have reset count)", c.State())
	}
}

func TestCircuit_DefaultsApplied(t *testing.T) {
	c := New(0, 0) // invalid → defaults
	if c.threshold != 5 {
		t.Fatalf("threshold=%d, want default 5", c.threshold)
	}
	if c.cooldown != 30*time.Second {
		t.Fatalf("cooldown=%v, want default 30s", c.cooldown)
	}
}
