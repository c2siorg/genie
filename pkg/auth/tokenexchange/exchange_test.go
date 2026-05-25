package tokenexchange

import (
	"context"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

func newIssuer(t *testing.T, ttl time.Duration) *auth.Issuer {
	t.Helper()
	return auth.NewIssuer([]byte("test-secret-for-token-exchange-tests"),
		"genie-api", []string{"genie-api"}, ttl)
}

func TestExchangePreservesUserAddsActor(t *testing.T) {
	issuer := newIssuer(t, time.Hour)
	svc := New(issuer, "genie-api")

	userToken, _, err := issuer.Issue("user-1", "alice@example.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatal(err)
	}

	exchanged, claims, err := svc.Exchange(context.Background(), Request{
		SubjectToken: userToken,
		ActorID:      "kyc_orchestrator",
		Audience:     "mcp://kyc-server",
	})
	if err != nil {
		t.Fatal(err)
	}
	if exchanged == userToken {
		t.Errorf("exchanged token must differ from subject token")
	}
	if claims.Subject != "user-1" {
		t.Errorf("Subject must remain the user; got %q", claims.Subject)
	}
	if claims.Actor == nil || claims.Actor.Subject != "kyc_orchestrator" {
		t.Errorf("Actor must identify the agent; got %+v", claims.Actor)
	}
	if claims.Actor.Issuer != "genie-api" {
		t.Errorf("Actor issuer must record the minting service; got %q", claims.Actor.Issuer)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != "mcp://kyc-server" {
		t.Errorf("Audience must be scoped to the requested target; got %+v", claims.Audience)
	}
}

func TestExchangeRejectsMissingAudience(t *testing.T) {
	issuer := newIssuer(t, time.Hour)
	svc := New(issuer, "genie-api")
	userToken, _, _ := issuer.Issue("u", "u@example.com", []auth.Role{auth.RoleUser})
	if _, _, err := svc.Exchange(context.Background(), Request{
		SubjectToken: userToken, ActorID: "x", Audience: "",
	}); err != ErrAudienceRequired {
		t.Errorf("expected ErrAudienceRequired; got %v", err)
	}
}

func TestExchangeRejectsInvalidSubjectToken(t *testing.T) {
	issuer := newIssuer(t, time.Hour)
	svc := New(issuer, "genie-api")
	_, _, err := svc.Exchange(context.Background(), Request{
		SubjectToken: "not-a-jwt", ActorID: "x", Audience: "a",
	})
	if err == nil {
		t.Errorf("expected error on garbage subject token")
	}
}

func TestExchangeRejectsExpiredSubjectToken(t *testing.T) {
	issuer := newIssuer(t, -time.Minute) // already expired
	svc := New(issuer, "genie-api")
	stale, _, _ := issuer.Issue("u", "u@example.com", []auth.Role{auth.RoleUser})

	_, _, err := svc.Exchange(context.Background(), Request{
		SubjectToken: stale, ActorID: "x", Audience: "a",
	})
	if err == nil {
		t.Errorf("expected refusal on expired subject token")
	}
}

func TestExchangeCachesByTuple(t *testing.T) {
	issuer := newIssuer(t, time.Hour)
	svc := New(issuer, "genie-api")
	userToken, _, _ := issuer.Issue("u-1", "u@example.com", []auth.Role{auth.RoleUser})

	first, _, err := svc.Exchange(context.Background(), Request{
		SubjectToken: userToken, ActorID: "agent-a", Audience: "aud-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	second, _, _ := svc.Exchange(context.Background(), Request{
		SubjectToken: userToken, ActorID: "agent-a", Audience: "aud-1",
	})
	if first != second {
		t.Errorf("same (user, actor, audience) tuple should return cached token")
	}

	// Different audience → fresh token.
	other, _, _ := svc.Exchange(context.Background(), Request{
		SubjectToken: userToken, ActorID: "agent-a", Audience: "aud-2",
	})
	if other == first {
		t.Errorf("different audience must mint a fresh token")
	}

	// Different actor → fresh token.
	other2, _, _ := svc.Exchange(context.Background(), Request{
		SubjectToken: userToken, ActorID: "agent-b", Audience: "aud-1",
	})
	if other2 == first {
		t.Errorf("different actor must mint a fresh token")
	}
}

func TestInvalidateClearsCacheForUser(t *testing.T) {
	// Note: we assert cache state rather than token bytes — within the
	// same second, two mints of the same Claims produce byte-identical
	// JWTs (iat is seconds-resolution). The contract we care about is
	// "the cache is empty after Invalidate", which we observe directly.
	issuer := newIssuer(t, time.Hour)
	svc := New(issuer, "genie-api")
	tokenA, _, _ := issuer.Issue("user-a", "a@example.com", []auth.Role{auth.RoleUser})

	if _, _, err := svc.Exchange(context.Background(), Request{
		SubjectToken: tokenA, ActorID: "agent", Audience: "aud",
	}); err != nil {
		t.Fatal(err)
	}
	if svc.CacheSize() != 1 {
		t.Fatalf("expected 1 cached entry after exchange; got %d", svc.CacheSize())
	}

	svc.Invalidate("user-a")
	if svc.CacheSize() != 0 {
		t.Errorf("Invalidate must clear the user's cache entries; cache size %d", svc.CacheSize())
	}

	// Next call rebuilds the cache.
	if _, _, err := svc.Exchange(context.Background(), Request{
		SubjectToken: tokenA, ActorID: "agent", Audience: "aud",
	}); err != nil {
		t.Fatal(err)
	}
	if svc.CacheSize() != 1 {
		t.Errorf("post-invalidate exchange should repopulate the cache; got %d", svc.CacheSize())
	}
}

func TestNestedActorChain(t *testing.T) {
	// Two-hop scenario: agent runtime exchanges for MCP server, MCP server
	// exchanges again for upstream API. The second exchange should nest
	// the first actor under Actor.Nested.
	issuer := newIssuer(t, time.Hour)
	svc := New(issuer, "genie-api")
	userToken, _, _ := issuer.Issue("user-1", "u@example.com", []auth.Role{auth.RoleUser})

	// Hop 1: agent runtime → MCP server token.
	hop1, _, err := svc.Exchange(context.Background(), Request{
		SubjectToken: userToken,
		ActorID:      "kyc_orchestrator",
		Audience:     "mcp://kyc-server",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Hop 2: MCP server → upstream API token.
	_, claims2, err := svc.Exchange(context.Background(), Request{
		SubjectToken: hop1,
		ActorID:      "kyc-mcp-server",
		Audience:     "https://api.upstream/records",
	})
	if err != nil {
		t.Fatal(err)
	}

	if claims2.Subject != "user-1" {
		t.Errorf("Subject must remain the user across hops; got %q", claims2.Subject)
	}
	if claims2.Actor == nil || claims2.Actor.Subject != "kyc-mcp-server" {
		t.Errorf("Outer Actor must be the second-hop service; got %+v", claims2.Actor)
	}
	if claims2.Actor.Nested == nil || claims2.Actor.Nested.Subject != "kyc_orchestrator" {
		t.Errorf("Nested Actor must record the first-hop service; got %+v", claims2.Actor.Nested)
	}
}
