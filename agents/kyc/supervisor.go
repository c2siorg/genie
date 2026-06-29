package kyc

import (
	"context"

	"github.com/c2siorg/genie/pkg/agent"
	"github.com/c2siorg/genie/pkg/compliance"
	"github.com/c2siorg/genie/pkg/kyc"
)

// ID is the canonical agent identifier for the KYC onboarding supervisor
// (matches the platform-wide `ID = "..."` convention enforced by the agent registry).
const (
	ID = "kyc_onboarding_supervisor"
)

// OnboardingAgentSupervisor orchestrates the KYC/Onboarding workflow
// across DocumentProcessor, IdentityVerifier, SanctionsChecker, and OnboardingApprover sub-agents.
type OnboardingAgentSupervisor struct {
	docProc    DocumentProcessor
	idVerifier IdentityVerifier
	sanctions  SanctionsChecker
	approver   OnboardingApprover
	store      kyc.OnboardingStore
	auditLog   compliance.AuditLog
}

// ID returns the supervisor agent's unique identifier.
func (s *OnboardingAgentSupervisor) ID() string {
	return ID
}

// Name returns the supervisor agent's display name.
func (s *OnboardingAgentSupervisor) Name() string {
	// TODO: implement
	panic("not implemented")
}

// Capabilities returns the list of capabilities this agent provides.
func (s *OnboardingAgentSupervisor) Capabilities() []string {
	// TODO: implement
	panic("not implemented")
}

// RiskLevel returns the risk classification of this agent.
func (s *OnboardingAgentSupervisor) RiskLevel() agent.RiskClass {
	// TODO: implement
	panic("not implemented")
}

// HandleMessage implements the agent.Agent interface for message-driven orchestration.
func (s *OnboardingAgentSupervisor) HandleMessage(
	ctx context.Context,
	msg agent.Message,
	env agent.Environment,
) ([]agent.Message, error) {
	// TODO: implement
	panic("not implemented")
}

// Orchestrate executes the full KYC workflow for an onboarding request,
// transitioning through document processing, identity verification, sanctions screening,
// and approval policy evaluation.
func (s *OnboardingAgentSupervisor) Orchestrate(
	ctx context.Context,
	request *kyc.OnboardingRequest,
	env agent.Environment,
) (*kyc.OnboardingRequest, error) {
	// TODO: implement
	panic("not implemented")
}
