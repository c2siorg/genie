package commerce

import (
	"context"
	"testing"
	"time"
)

// fakeVelocity is a controllable velocityChecker for gate unit tests.
type fakeVelocity struct {
	allow  bool
	reason string
	calls  int
}

func (f *fakeVelocity) CheckAndRecord(_ context.Context, _ string, _ int64, _ time.Time) (bool, string) {
	f.calls++
	return f.allow, f.reason
}

func TestVelocityComplianceGate_AllowsWithinLimits(t *testing.T) {
	g := NewVelocityComplianceGateWithMonitor(&fakeVelocity{allow: true, reason: "ok"}, time.Now)
	d := g.Evaluate(context.Background(), &Order{CustomerID: "c1", TotalPaise: 100_000})
	if !d.Allowed || d.Decision != "allow" {
		t.Fatalf("want allow, got %+v", d)
	}
}

func TestVelocityComplianceGate_BlocksWhenMonitorDenies(t *testing.T) {
	fv := &fakeVelocity{allow: false, reason: "Hourly amount limit exceeded"}
	g := NewVelocityComplianceGateWithMonitor(fv, time.Now)
	d := g.Evaluate(context.Background(), &Order{CustomerID: "c1", TotalPaise: 9_000_000})
	if d.Allowed || d.Decision != "block" {
		t.Fatalf("want block, got %+v", d)
	}
	if d.Reason != fv.reason {
		t.Fatalf("reason not propagated: got %q", d.Reason)
	}
}

// The gate must FAIL CLOSED on every invalid input and never even consult the
// monitor — a regulated money path may not fail open.
func TestVelocityComplianceGate_FailsClosed(t *testing.T) {
	cases := map[string]*Order{
		"nil order":        nil,
		"missing customer": {CustomerID: "", TotalPaise: 100},
		"zero amount":      {CustomerID: "c1", TotalPaise: 0},
		"negative amount":  {CustomerID: "c1", TotalPaise: -5},
	}
	for name, order := range cases {
		t.Run(name, func(t *testing.T) {
			fv := &fakeVelocity{allow: true, reason: "ok"} // monitor WOULD allow
			g := NewVelocityComplianceGateWithMonitor(fv, time.Now)
			d := g.Evaluate(context.Background(), order)
			if d.Allowed {
				t.Fatalf("%s: gate failed OPEN", name)
			}
			if fv.calls != 0 {
				t.Fatalf("%s: monitor consulted on invalid input (should short-circuit closed)", name)
			}
		})
	}
}

// A gate with no monitor must also fail closed rather than panic or allow.
func TestVelocityComplianceGate_NilMonitorFailsClosed(t *testing.T) {
	g := &VelocityComplianceGate{vm: nil, now: time.Now}
	d := g.Evaluate(context.Background(), &Order{CustomerID: "c1", TotalPaise: 100})
	if d.Allowed {
		t.Fatal("nil monitor must fail closed")
	}
}

// End-to-end against the real velocity monitor: a single ₹90k order is blocked,
// a ₹1k order from the same customer is then allowed and accumulates, and once
// accumulation crosses ₹50k/hour the next order is blocked.
func TestVelocityComplianceGate_RealMonitor(t *testing.T) {
	g := NewVelocityComplianceGate()
	ctx := context.Background()

	big := g.Evaluate(ctx, &Order{CustomerID: "whale", TotalPaise: 9_000_000}) // ₹90k
	if big.Allowed {
		t.Fatal("₹90k single order must be blocked by ₹50k/hr limit")
	}

	// Fresh customer: ₹40k allowed, then another ₹40k pushes past ₹50k/hr → block.
	first := g.Evaluate(ctx, &Order{CustomerID: "steady", TotalPaise: 4_000_000})
	if !first.Allowed {
		t.Fatalf("first ₹40k should be allowed: %+v", first)
	}
	second := g.Evaluate(ctx, &Order{CustomerID: "steady", TotalPaise: 4_000_000})
	if second.Allowed {
		t.Fatal("second ₹40k (cumulative ₹80k) must exceed the ₹50k/hr limit")
	}
}
