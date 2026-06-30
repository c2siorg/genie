// This file defines the deterministic, fail-closed compliance gate that the
// workflow orchestrator evaluates BEFORE any money moves. It is the control
// that makes "compliance is enforced in the money path" true rather than a
// claim: a denied order never reaches payment.
//
// Design rule (non-negotiable for a regulated money path): the gate MUST fail
// closed. A nil/invalid order, an uninitialized dependency, or any internal
// uncertainty resolves to Allowed=false. An LLM is never permitted to produce
// the allow/block bit — only a deterministic check is authoritative here.
//
// License: MIT (see root LICENSE file)
package commerce

import (
	"context"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/erupeecompliance"
)

// GateDecision is the outcome of a pre-payment compliance evaluation.
type GateDecision struct {
	// Allowed is true only when the order may proceed to payment.
	Allowed bool
	// Decision is the machine-readable verdict: "allow" | "review" | "block".
	Decision string
	// Reason is a human-readable explanation, suitable for the audit trail.
	Reason string
}

// ComplianceGate is the deterministic pre-payment control. Implementations MUST
// fail closed: any error, missing dependency, or uncertainty returns
// Allowed=false. The orchestrator records every evaluation to the audit trail.
type ComplianceGate interface {
	Evaluate(ctx context.Context, order *Order) GateDecision
}

// velocityChecker is the narrow slice of the velocity monitor the gate needs.
// Keeping it local avoids coupling the gate to the concrete monitor type and
// keeps the dependency testable with a fake.
type velocityChecker interface {
	CheckAndRecord(ctx context.Context, accountID string, amount int64, now time.Time) (bool, string)
}

// VelocityComplianceGate enforces deterministic per-customer velocity limits
// (RBI-aligned defaults: ₹50k/hour, ₹2L/day, 10 txns/hour). It is fail-closed
// by construction.
type VelocityComplianceGate struct {
	vm  velocityChecker
	now func() time.Time
}

// Compile-time assertion that the gate satisfies the interface.
var _ ComplianceGate = (*VelocityComplianceGate)(nil)

// NewVelocityComplianceGate builds a gate backed by an in-memory velocity
// monitor with default limits. Production wiring should construct one of these
// (or supply a shared monitor via NewVelocityComplianceGateWithMonitor) so the
// money path fails closed on velocity breaches.
func NewVelocityComplianceGate() *VelocityComplianceGate {
	return &VelocityComplianceGate{
		vm:  erupeecompliance.NewInMemoryVelocityMonitor(),
		now: time.Now,
	}
}

// NewVelocityComplianceGateWithMonitor injects a velocity monitor and clock.
// Use this to share velocity state across components or to drive a
// deterministic clock in tests. A nil clock defaults to time.Now.
func NewVelocityComplianceGateWithMonitor(vm velocityChecker, now func() time.Time) *VelocityComplianceGate {
	if now == nil {
		now = time.Now
	}
	return &VelocityComplianceGate{vm: vm, now: now}
}

// Evaluate applies the deterministic velocity control. It fails closed on any
// invalid input or missing dependency, and records the customer's transaction
// against the velocity window only when the order is allowed.
func (g *VelocityComplianceGate) Evaluate(ctx context.Context, order *Order) GateDecision {
	if g == nil || g.vm == nil {
		return GateDecision{Allowed: false, Decision: "block", Reason: "compliance gate not initialized (fail-closed)"}
	}
	if order == nil {
		return GateDecision{Allowed: false, Decision: "block", Reason: "nil order (fail-closed)"}
	}
	if order.CustomerID == "" {
		return GateDecision{Allowed: false, Decision: "block", Reason: "missing customer id (fail-closed)"}
	}
	if order.TotalPaise <= 0 {
		return GateDecision{Allowed: false, Decision: "block", Reason: "non-positive order amount (fail-closed)"}
	}

	allowed, reason := g.vm.CheckAndRecord(ctx, order.CustomerID, order.TotalPaise, g.now())
	if !allowed {
		return GateDecision{Allowed: false, Decision: "block", Reason: reason}
	}
	return GateDecision{Allowed: true, Decision: "allow", Reason: reason}
}
