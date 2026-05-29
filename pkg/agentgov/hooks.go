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

// RecordSuccess records a successful operation for an agent, rewarding trust
// and logging a success SLO event. Call this from within a custom success hook.
func (b *Bundle) RecordSuccess(agentID string, latency time.Duration) {
	b.Trust.RecordSuccess(agentID, 0.05)
	b.Audit.Log(agentID, "agent_success", agentmesh.Allow)
	_ = b.SLO.RecordEvent("agent.availability", true, latency)
	_ = b.SLO.RecordEvent("agent.latency", true, latency)
}
