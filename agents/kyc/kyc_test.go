package kyc

import (
	"testing"

	"github.com/c2siorg/genie/pkg/agent"
)

// Compile-time guarantees that the concrete sub-agents satisfy the sub-agent
// interfaces the supervisor depends on. These assertions are the real safety
// net for the interface/implementation alignment in this package — if a method
// signature drifts, the build fails here rather than at a distant call site.
var (
	_ DocumentProcessor = (*DocumentProcessorAgent)(nil)
	_ IdentityVerifier  = (*IdentityVerifierAgent)(nil)
	_ SanctionsChecker  = (*SanctionsCheckerAgent)(nil)
)

// The supervisor must satisfy the platform agent.Agent interface.
var _ agent.Agent = (*OnboardingAgentSupervisor)(nil)

// TestSupervisorID verifies the supervisor reports the canonical agent ID,
// matching the package-level ID constant the registry convention requires.
func TestSupervisorID(t *testing.T) {
	if ID != "kyc_onboarding_supervisor" {
		t.Fatalf("unexpected agent ID constant: got %q", ID)
	}
	s := &OnboardingAgentSupervisor{}
	if got := s.ID(); got != ID {
		t.Errorf("OnboardingAgentSupervisor.ID() = %q, want %q", got, ID)
	}
}
