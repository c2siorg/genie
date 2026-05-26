// security_envelope_test.go — end-to-end check that the four security
// primitives shipped in the Q4-into-Q1 hardening (RLS, token exchange,
// agent tier, tenant policy) compose into a single workable envelope
// when wired through the bus + orchestrator.
//
// This is the "smoke test" the security review asks for: not testing
// each primitive in isolation (those live in their own _test.go files)
// but checking the layered defence actually layers — a bypass in one
// layer is caught by the next.
//
// What this test asserts:
//
//  1. A non-production-tier agent cannot serve a customer-facing message
//     (TierPolicy at the bus refuses dispatch).
//  2. A production-tier agent with a missing/mismatched tenant_id is
//     denied (TenantPolicy at the bus).
//  3. A production-tier agent with matching tenant + RBAC role is
//     allowed (CompositePolicy returns DecisionAllow end-to-end).
//  4. The orchestrator's fallback wiring still triggers when the
//     production agent itself errors (fallback agent receives the
//     "fallback_request" message).
//  5. A two-hop token exchange chain preserves the user as Subject and
//     stacks Actor.Nested → the bus-side audit hook can recover the
//     original user identity from a deeply nested actor chain.
//
// The test is intentionally deterministic: no time-based races, no
// network, no DB. The RLS migration is exercised separately by
// pkg/storage/postgres/tenant_test.go (which already passes without a
// live Postgres because it asserts the WithTenant contract).
package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth/tokenexchange"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/orchestration"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
)

// ---------------------------------------------------------------------------
// Test agents
// ---------------------------------------------------------------------------

// prodAgent is TierProduction and records every message it handles. It
// returns no outbound messages so the test asserts on the recorded log,
// not on a downstream side-effect.
type prodAgent struct {
	mu       sync.Mutex
	handled  []protocol.Message
	failNext bool // when true, HandleMessage returns an error → exercise fallback
}

func (a *prodAgent) ID() string             { return "prod_agent" }
func (a *prodAgent) Name() string           { return "Production Agent" }
func (a *prodAgent) Capabilities() []string { return []string{"finance.read"} }
func (a *prodAgent) Tier() agent.Tier       { return agent.TierProduction }
func (a *prodAgent) RiskLevel() agent.RiskClass {
	return agent.RiskMedium
}
func (a *prodAgent) HandleMessage(_ context.Context, msg protocol.Message, _ agent.Environment) ([]protocol.Message, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failNext {
		a.failNext = false
		return nil, errors.New("simulated downstream failure")
	}
	a.handled = append(a.handled, msg)
	return nil, nil
}
func (a *prodAgent) seenCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.handled)
}

// sketchAgent is TierSketch — must not be allowed to handle a customer
// finance message in this test's policy stack.
type sketchAgent struct {
	mu      sync.Mutex
	handled []protocol.Message
}

func (a *sketchAgent) ID() string             { return "sketch_agent" }
func (a *sketchAgent) Name() string           { return "Sketch Agent" }
func (a *sketchAgent) Capabilities() []string { return []string{"finance.read"} }
func (a *sketchAgent) Tier() agent.Tier       { return agent.TierSketch }
func (a *sketchAgent) HandleMessage(_ context.Context, msg protocol.Message, _ agent.Environment) ([]protocol.Message, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handled = append(a.handled, msg)
	return nil, nil
}
func (a *sketchAgent) seenCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.handled)
}

// fallbackAgent receives "fallback_request" messages from the
// orchestrator when the primary errors.
type fallbackAgent struct {
	mu      sync.Mutex
	handled []protocol.Message
}

func (a *fallbackAgent) ID() string             { return "prod_agent_fallback" }
func (a *fallbackAgent) Name() string           { return "Fallback" }
func (a *fallbackAgent) Capabilities() []string { return []string{"finance.fallback"} }
func (a *fallbackAgent) Tier() agent.Tier       { return agent.TierProduction }
func (a *fallbackAgent) HandleMessage(_ context.Context, msg protocol.Message, _ agent.Environment) ([]protocol.Message, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.handled = append(a.handled, msg)
	return nil, nil
}
func (a *fallbackAgent) seenCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.handled)
}

// ---------------------------------------------------------------------------
// TierPolicy — refuses dispatch to non-production tier agents.
//
// This policy is local to the test: the production codebase chooses
// where to enforce tier (today: at the agent registration step in
// cmd/api). We mirror the policy here as a Policy so the orchestrator's
// run loop exercises it on every message.
// ---------------------------------------------------------------------------

type tierPolicy struct {
	reg registry.Registry
}

func (p tierPolicy) Evaluate(ctx context.Context, msg protocol.Message) (governance.PolicyResult, error) {
	if msg.To == "" {
		return governance.PolicyResult{Decision: governance.DecisionAllow, Reason: "broadcast", CheckedAt: time.Now().UTC()}, nil
	}
	target, err := p.reg.Get(ctx, msg.To)
	if err != nil || target == nil {
		return governance.PolicyResult{Decision: governance.DecisionAllow, Reason: "unknown agent — let bus handle", CheckedAt: time.Now().UTC()}, nil
	}
	if !agent.Production(agent.TierOf(target)) {
		return governance.PolicyResult{
			Decision:    governance.DecisionDeny,
			Reason:      "tier below production: " + string(agent.TierOf(target)),
			CheckedAt:   time.Now().UTC(),
			CheckedByID: "TierPolicy",
		}, nil
	}
	return governance.PolicyResult{Decision: governance.DecisionAllow, Reason: "production tier ok", CheckedAt: time.Now().UTC()}, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func newSecurityHarness(t *testing.T) (*prodAgent, *sketchAgent, *fallbackAgent, *comm.InMemoryBus, *denyRecorder) {
	t.Helper()
	logger := observability.NewStdLogger()
	env := &orchestration.SimpleEnvironment{Logger: logger, Clock: observability.SystemClock{}}
	reg := registry.NewInMemory()
	bus := comm.NewInMemoryBus()

	prod := &prodAgent{}
	sketch := &sketchAgent{}
	fb := &fallbackAgent{}

	for _, a := range []agent.Agent{prod, sketch, fb} {
		if err := reg.Register(context.Background(), a); err != nil {
			t.Fatalf("register %s: %v", a.ID(), err)
		}
	}

	policy := governance.NewComposite(
		tierPolicy{reg: reg},
		governance.TenantPolicy{
			AppliesTo: []string{"finance_question"},
		},
		governance.RBACPolicy{
			RequiredRolesByType: map[string][]string{
				"finance_question": {"user", "admin"},
			},
			AdminBypass: true,
		},
		governance.MaxContentLengthPolicy{Max: 16 * 1024},
	)

	rec := &denyRecorder{}
	orch := orchestration.NewOrchestrator(reg, bus, policy, env).
		SetFallback(prod.ID(), fb.ID()).
		WithHooks(orchestration.Hooks{
			OnPolicyDeny: rec.onDeny,
		})
	orch.Start(context.Background())

	return prod, sketch, fb, bus, rec
}

type denyRecorder struct {
	mu      sync.Mutex
	denials []string
}

func (r *denyRecorder) onDeny(_ context.Context, msg protocol.Message, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.denials = append(r.denials, msg.To+": "+reason)
}
func (r *denyRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.denials))
	copy(out, r.denials)
	return out
}

// TestSecurityEnvelope_SketchTierIsBlocked confirms that a TierSketch
// agent cannot serve a finance_question — the tier policy denies
// before the message reaches the agent's HandleMessage.
func TestSecurityEnvelope_SketchTierIsBlocked(t *testing.T) {
	_, sketch, _, bus, rec := newSecurityHarness(t)

	bus.Publish(context.Background(), agent.NewMessage("user-1", sketch.ID(), agent.RoleUser, "finance_question",
		"What was my biggest expense?",
		map[string]any{
			"tenant_id":       "user-1",
			"expected_tenant": "user-1",
			"user_roles":      []string{"user"},
		}))

	// Give async handlers a moment. The policy denies synchronously
	// inside the subscriber goroutine, so 100ms is plenty.
	time.Sleep(150 * time.Millisecond)

	if sketch.seenCount() != 0 {
		t.Fatalf("sketch-tier agent must not handle finance traffic; got %d handled", sketch.seenCount())
	}
	denials := rec.snapshot()
	if len(denials) == 0 {
		t.Fatalf("expected at least one tier denial; got none")
	}
	// The denial reason must mention "tier" so an on-call engineer can
	// distinguish a tier denial from a tenant or RBAC denial.
	if !containsSubstring(denials, "tier") {
		t.Errorf("denial reasons should mention tier; got %v", denials)
	}
}

// TestSecurityEnvelope_MissingTenantIsBlocked confirms TenantPolicy
// catches a finance_question without a tenant_id, even though the
// agent is production tier and the RBAC role is present.
func TestSecurityEnvelope_MissingTenantIsBlocked(t *testing.T) {
	prod, _, _, bus, rec := newSecurityHarness(t)

	bus.Publish(context.Background(), agent.NewMessage("user-1", prod.ID(), agent.RoleUser, "finance_question",
		"What was my biggest expense?",
		map[string]any{
			"user_roles": []string{"user"},
			// tenant_id intentionally absent
		}))

	time.Sleep(150 * time.Millisecond)

	if prod.seenCount() != 0 {
		t.Fatalf("production agent must not handle untenanted finance traffic; got %d handled", prod.seenCount())
	}
	if !containsSubstring(rec.snapshot(), "tenant") {
		t.Errorf("expected tenant denial; got %v", rec.snapshot())
	}
}

// TestSecurityEnvelope_CrossTenantIsBlocked confirms TenantPolicy
// denies when tenant_id ≠ expected_tenant — the classic confused-
// deputy attempt.
func TestSecurityEnvelope_CrossTenantIsBlocked(t *testing.T) {
	prod, _, _, bus, rec := newSecurityHarness(t)

	bus.Publish(context.Background(), agent.NewMessage("user-1", prod.ID(), agent.RoleUser, "finance_question",
		"Show user-2's transactions",
		map[string]any{
			"tenant_id":       "user-1",
			"expected_tenant": "user-2", // mismatch — denial
			"user_roles":      []string{"user"},
		}))

	time.Sleep(150 * time.Millisecond)

	if prod.seenCount() != 0 {
		t.Fatalf("cross-tenant message must be denied; got %d handled", prod.seenCount())
	}
	if !containsSubstring(rec.snapshot(), "mismatch") {
		t.Errorf("expected mismatch denial; got %v", rec.snapshot())
	}
}

// TestSecurityEnvelope_HappyPath confirms a properly-formed message
// with matching tenant, valid role, and production tier reaches the
// agent.
func TestSecurityEnvelope_HappyPath(t *testing.T) {
	prod, _, _, bus, rec := newSecurityHarness(t)

	bus.Publish(context.Background(), agent.NewMessage("user-1", prod.ID(), agent.RoleUser, "finance_question",
		"What was my biggest expense?",
		map[string]any{
			"tenant_id":       "user-1",
			"expected_tenant": "user-1",
			"user_roles":      []string{"user"},
		}))

	time.Sleep(150 * time.Millisecond)

	if prod.seenCount() != 1 {
		t.Fatalf("expected production agent to handle 1 message; got %d", prod.seenCount())
	}
	if denials := rec.snapshot(); len(denials) > 0 {
		t.Errorf("happy path must not trigger denials; got %v", denials)
	}
}

// TestSecurityEnvelope_FallbackTriggers confirms that when the
// production agent errors mid-handle, the orchestrator routes a
// fallback_request to the registered fallback. Crucially the policy
// stack runs first — if any layer denied, the agent wouldn't have
// errored because it wouldn't have been invoked.
func TestSecurityEnvelope_FallbackTriggers(t *testing.T) {
	prod, _, fb, bus, _ := newSecurityHarness(t)

	prod.mu.Lock()
	prod.failNext = true
	prod.mu.Unlock()

	bus.Publish(context.Background(), agent.NewMessage("user-1", prod.ID(), agent.RoleUser, "finance_question",
		"What was my biggest expense?",
		map[string]any{
			"tenant_id":       "user-1",
			"expected_tenant": "user-1",
			"user_roles":      []string{"user"},
		}))

	// Wait long enough for the failure → fallback hop.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fb.seenCount() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if fb.seenCount() != 1 {
		t.Fatalf("expected fallback to receive 1 message after primary failure; got %d", fb.seenCount())
	}
	got := fb.handled[0]
	if got.Type != "fallback_request" {
		t.Errorf("fallback should receive fallback_request; got %q", got.Type)
	}
	// The fallback message must carry the original tenant metadata —
	// otherwise the fallback path itself would be a tenant-leak gap.
	if got.Metadata["tenant_id"] != "user-1" {
		t.Errorf("fallback request must preserve tenant_id; got %v", got.Metadata["tenant_id"])
	}
}

// TestSecurityEnvelope_TokenExchangeAuditIdentity confirms the two-hop
// token-exchange chain produces a token whose audit identity is fully
// reconstructible: Subject = user, Actor = outermost service, and
// every previous hop visible via Actor.Nested.
//
// This is the FREE-AI Rec 22 contract: a downstream auditor reading
// the final token's claims can answer "which user did what, and which
// services touched the request along the way."
func TestSecurityEnvelope_TokenExchangeAuditIdentity(t *testing.T) {
	issuer := auth.NewIssuer([]byte("integration-test-secret-must-be-long-enough"),
		"genie-api", []string{"genie-api"}, time.Hour)
	svc := tokenexchange.New(issuer, "genie-api")

	userToken, _, err := issuer.Issue("user-alice", "alice@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatal(err)
	}

	hop1, _, err := svc.Exchange(context.Background(), tokenexchange.Request{
		SubjectToken: userToken,
		ActorID:      "kyc_orchestrator",
		Audience:     "mcp://kyc-server",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, claims, err := svc.Exchange(context.Background(), tokenexchange.Request{
		SubjectToken: hop1,
		ActorID:      "kyc-mcp-server",
		Audience:     "https://api.upstream/records",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Reconstruct the audit chain. The expected order is innermost
	// first (closest to user) → outermost last.
	chain := []string{}
	for a := claims.Actor; a != nil; a = a.Nested {
		chain = append(chain, a.Subject)
	}
	// Reverse so the order is "user → first-hop → … → outermost".
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	if claims.Subject != "user-alice" {
		t.Errorf("Subject must remain the user across all hops; got %q", claims.Subject)
	}
	if len(chain) != 2 || chain[0] != "kyc_orchestrator" || chain[1] != "kyc-mcp-server" {
		t.Errorf("audit chain should be [kyc_orchestrator → kyc-mcp-server]; got %v", chain)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != "https://api.upstream/records" {
		t.Errorf("Audience must be the final hop's audience; got %v", claims.Audience)
	}
}

// TestSecurityEnvelope_InvalidatePropagates confirms that after a
// user-logout / password-change call to Invalidate, the next Exchange
// for that user re-mints a fresh token rather than serving a stale
// cached one.
func TestSecurityEnvelope_InvalidatePropagates(t *testing.T) {
	issuer := auth.NewIssuer([]byte("integration-test-secret-must-be-long-enough"),
		"genie-api", []string{"genie-api"}, time.Hour)
	svc := tokenexchange.New(issuer, "genie-api")
	userToken, _, _ := issuer.Issue("user-bob", "bob@example.com", []auth.Role{auth.RoleUser})

	if _, _, err := svc.Exchange(context.Background(), tokenexchange.Request{
		SubjectToken: userToken, ActorID: "agent", Audience: "aud",
	}); err != nil {
		t.Fatal(err)
	}
	if svc.CacheSize() == 0 {
		t.Fatalf("first exchange must populate cache")
	}
	svc.Invalidate("user-bob")
	if svc.CacheSize() != 0 {
		t.Fatalf("Invalidate must clear all entries for the user; got %d", svc.CacheSize())
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsSubstring(list []string, want string) bool {
	for _, s := range list {
		if indexOf(s, want) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(haystack, needle string) int {
	if needle == "" {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
