package erupeecompliance

import (
	"context"
	"testing"
	"time"
)

// The prospective check is what a pre-payment gate needs: CheckVelocity only
// sees already-recorded activity, so a fresh account's first oversized payment
// slips past it. WouldExceed must catch it.
func TestWouldExceed_SingleLargePaymentOnFreshAccount(t *testing.T) {
	vm := NewInMemoryVelocityMonitor() // ₹50k/hr, ₹2L/day, 10 txns/hr
	now := time.Now()

	// CheckVelocity (retrospective) sees nothing yet → allows. Documents the gap.
	if ok, _ := vm.CheckVelocity(context.Background(), "fresh"); !ok {
		t.Fatal("precondition: retrospective check should pass on a fresh account")
	}

	// WouldExceed (prospective) must reject ₹90k against the ₹50k/hr limit.
	exceeded, reason := vm.WouldExceed(context.Background(), "fresh", 9_000_000, now)
	if !exceeded {
		t.Fatalf("₹90k must exceed ₹50k/hr prospectively; reason=%q", reason)
	}

	// And it must NOT have mutated state (it is a pure check). GetRecord returns
	// an error when no record exists.
	if _, err := vm.GetRecord(context.Background(), "fresh"); err == nil {
		t.Fatal("WouldExceed must not record anything")
	}
}

func TestWouldExceed_WithinLimits(t *testing.T) {
	vm := NewInMemoryVelocityMonitor()
	exceeded, _ := vm.WouldExceed(context.Background(), "ok", 1_000_000, time.Now()) // ₹10k
	if exceeded {
		t.Fatal("₹10k must be within the ₹50k/hr limit")
	}
}

func TestCheckAndRecord_AtomicAccumulationBlocks(t *testing.T) {
	vm := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	now := time.Now()

	if ok, _ := vm.CheckAndRecord(ctx, "acc", 4_000_000, now); !ok { // ₹40k
		t.Fatal("first ₹40k should be allowed and recorded")
	}
	if ok, reason := vm.CheckAndRecord(ctx, "acc", 4_000_000, now); ok { // cumulative ₹80k
		t.Fatalf("cumulative ₹80k must exceed ₹50k/hr; got allowed (reason=%q)", reason)
	}

	// The blocked attempt must not have been recorded (still at ₹40k).
	rec, err := vm.GetRecord(ctx, "acc")
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if rec.TotalAmount != 4_000_000 || rec.TransactionCount != 1 {
		t.Fatalf("blocked txn leaked into the record: amount=%d count=%d", rec.TotalAmount, rec.TransactionCount)
	}
}

func TestCheckAndRecord_TransactionCountLimit(t *testing.T) {
	vm := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	now := time.Now()

	// 10 tiny txns are allowed (limit is 10/hr); the 11th must be blocked on count.
	for i := 0; i < 10; i++ {
		if ok, reason := vm.CheckAndRecord(ctx, "many", 100, now); !ok {
			t.Fatalf("txn %d should be allowed: %s", i+1, reason)
		}
	}
	if ok, _ := vm.CheckAndRecord(ctx, "many", 100, now); ok {
		t.Fatal("11th transaction must exceed the 10/hr count limit")
	}
}

func TestCheckAndRecord_WindowResetAllowsAgain(t *testing.T) {
	vm := NewInMemoryVelocityMonitor()
	ctx := context.Background()
	base := time.Now()

	if ok, _ := vm.CheckAndRecord(ctx, "reset", 4_000_000, base); !ok { // ₹40k
		t.Fatal("first ₹40k should be allowed")
	}
	// Same window: another ₹40k blocks.
	if ok, _ := vm.CheckAndRecord(ctx, "reset", 4_000_000, base.Add(time.Minute)); ok {
		t.Fatal("second ₹40k within the hour must block")
	}
	// >1 hour later the hourly window resets, so ₹40k is allowed again.
	if ok, reason := vm.CheckAndRecord(ctx, "reset", 4_000_000, base.Add(2*time.Hour)); !ok {
		t.Fatalf("after the hourly window resets, ₹40k should be allowed: %s", reason)
	}
}

func TestCheckAndRecord_RejectsInvalidInput(t *testing.T) {
	vm := NewInMemoryVelocityMonitor()
	if ok, _ := vm.CheckAndRecord(context.Background(), "", 100, time.Now()); ok {
		t.Fatal("empty account id must be rejected")
	}
	if ok, _ := vm.CheckAndRecord(context.Background(), "a", -1, time.Now()); ok {
		t.Fatal("negative amount must be rejected")
	}
}
