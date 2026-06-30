package agentgov

import (
	"context"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/microsoft/agent-governance-toolkit/agent-governance-golang/packages/agentmesh"
)

// OrchestratorHooks returns a pair of orchestration hook functions wired to
// the AGT bundle. The returned functions are safe to pass directly to
// orchestration.Hooks{OnPolicyDeny: ..., OnAgentError: ...}.
//
// Both hooks:
//   - Record a trust failure for the agent.
//   - Append a Deny entry to the AGT audit log.
//   - Record an SLO failure event on both objectives.
func (b *Bundle) OrchestratorHooks() (
	onDeny func(ctx context.Context, msg agent.Message, reason string),
	onError func(ctx context.Context, agentID string, msg agent.Message, err error),
) {
	onDeny = func(_ context.Context, msg agent.Message, reason string) {
		agentID := msg.To
		b.Trust.RecordFailure(agentID, 0.1)
		b.Audit.Log(agentID, "policy_deny:"+reason, agentmesh.Deny)
		_ = b.SLO.RecordEvent("agent.availability", false, 0)
		_ = b.SLO.RecordEvent("agent.latency", false, 0)
	}

	onError = func(_ context.Context, agentID string, msg agent.Message, err error) {
		b.Trust.RecordFailure(agentID, 0.1)
		action := "agent_error"
		if err != nil {
			action = "agent_error:" + err.Error()
		}
		_ = msg.ID // silence unused var warning
		b.Audit.Log(agentID, action, agentmesh.Deny)
		_ = b.SLO.RecordEvent("agent.availability", false, 0)
		_ = b.SLO.RecordEvent("agent.latency", false, 0)
	}

	return onDeny, onError
}

// CheckDispatch is the pre-dispatch gate intended for the orchestrator's
// PreDispatch hook. It returns (false, reason) if EITHER:
//   - a global or per-agent kill switch is active, OR
//   - the agent's ring does not grant the "message.handle" capability.
//
// Both checks are fast, lock-free reads against in-memory data. Returning false
// causes the orchestrator to drop the message without calling HandleMessage.
func (b *Bundle) CheckDispatch(agentID string) (allow bool, reason string) {
	// Kill-switch check: global kill switch or per-agent kill switch.
	decision := b.KillSwitches.DecisionFor(agentID, "message.handle")
	if !decision.Allowed {
		reason := ""
		if decision.Event != nil {
			reason = string(decision.Event.Reason)
		}
		return false, "kill-switch active: " + reason
	}

	// Ring enforcement: agent must have the message.handle capability.
	if !b.Rings.CheckAccess(agentID, "message.handle") {
		return false, "ring policy denied message.handle for agent " + agentID
	}

	return true, ""
}

// RecordSuccess records a successful operation for an agent, rewarding trust
// and logging a success SLO event. Call this from within a custom success hook.
func (b *Bundle) RecordSuccess(agentID string, latency time.Duration) {
	b.Trust.RecordSuccess(agentID, 0.05)
	b.Audit.Log(agentID, "agent_success", agentmesh.Allow)
	_ = b.SLO.RecordEvent("agent.availability", true, latency)
	_ = b.SLO.RecordEvent("agent.latency", true, latency)
}

// RecordFailure records a failed operation for an agent: it penalizes trust,
// appends a Deny entry (tagged with reason) to the audit log, and records failed
// SLO events. This mirrors the body of the onError orchestrator hook and exists
// so callers outside the orchestrator hook path — e.g. MessageBridgeAdapter
// wrapping a decomposed agent — can record failures consistently, preserving
// trust scoring and kill-switch enforcement for decomposed agents.
func (b *Bundle) RecordFailure(agentID string, latency time.Duration, reason string) {
	b.Trust.RecordFailure(agentID, 0.1)
	action := "agent_error"
	if reason != "" {
		action = "agent_error:" + reason
	}
	b.Audit.Log(agentID, action, agentmesh.Deny)
	_ = b.SLO.RecordEvent("agent.availability", false, latency)
	_ = b.SLO.RecordEvent("agent.latency", false, latency)
}
