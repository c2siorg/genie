// Package tokenexchange implements OAuth 2.0 Token Exchange (RFC 8693)
// for the agent runtime → MCP server → upstream API call chain.
//
// The motivating use case: when Nurse Alice's medical-assistant agent
// queries patient records, the upstream API should see both *who* (Alice)
// and *what* (the medical-assistant agent) in a single signed token.
// FREE-AI Rec 22 and the broader audit-trail story both depend on this
// dual-identity capability.
//
// Wire flow:
//
//   1. Agent runtime holds the user's first-party JWT (Subject = user).
//   2. Before calling an MCP server, agent calls Exchange() with:
//        - subject_token: the user's JWT
//        - actor_identity: the agent's stable ID and roles
//        - audience: the MCP server's identifier
//   3. Exchange validates the subject token, mints a new JWT with:
//        - Subject:  user (unchanged from the subject token)
//        - Roles:    user's roles (unchanged)
//        - Audience: requested audience
//        - Actor:    { Subject: agent_id, Issuer: this_service }
//   4. Agent calls the MCP server with the new dual-identity token.
//   5. (Optional second hop) MCP server calls upstream with the same
//      token, or exchanges again to extend the actor chain.
//
// Caching: exchanged tokens are cached on (user_subject, agent_id, audience).
// TTL is min(token_exp - safety_margin, configured_max). A stale entry
// (token expired) returns a cache miss; we never serve an expired token.
package tokenexchange

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

// ErrSubjectTokenInvalid is returned when the subject token fails verification.
var ErrSubjectTokenInvalid = errors.New("tokenexchange: subject token invalid")

// ErrAudienceRequired is returned when the caller doesn't specify an audience.
var ErrAudienceRequired = errors.New("tokenexchange: audience is required")

// Request describes one token-exchange call.
type Request struct {
	// SubjectToken is the user's JWT (or a previously-exchanged token to
	// extend the actor chain).
	SubjectToken string
	// ActorID is the stable identifier of the service currently acting
	// (e.g. "kyc_orchestrator", "medical-assistant-agent").
	ActorID string
	// Audience identifies the target service the exchanged token is for.
	// Tokens are scoped per audience to prevent confused-deputy attacks.
	Audience string
}

// Service performs RFC 8693 token exchange against a Genie JWT Issuer.
//
// The same Issuer is used for verifying the subject token and minting the
// exchanged token. In a federated deployment, replace Verifier with a
// separate verifier wired to the user's identity provider.
type Service struct {
	// Verifier validates the SubjectToken. Typically a Verifier that
	// does NOT enforce audience — second-hop exchanges receive tokens
	// already scoped to a different audience than the issuer's default.
	// Use auth.Issuer.VerifyIgnoringAudience if you have one Issuer.
	Verifier interface {
		Verify(token string) (auth.Claims, error)
	}
	// Minter issues the exchanged token. Same as Verifier in the common
	// single-IdP deployment.
	Minter *auth.Issuer
	// ServiceIdentity names the host (e.g. "genie-api") that mints the
	// exchanged tokens; recorded in Actor.Issuer.
	ServiceIdentity string
	// SafetyMargin shaves time off the cached TTL to absorb clock skew
	// and network latency. Default 60s.
	SafetyMargin time.Duration

	cache cache
}

// looseVerifier wraps an Issuer to skip audience validation. Token
// exchange treats audience as the *output* scope; the input may be
// scoped to anything (e.g. a hop-1 exchanged token scoped to an MCP
// server).
type looseVerifier struct{ Issuer *auth.Issuer }

func (l looseVerifier) Verify(token string) (auth.Claims, error) {
	return l.Issuer.VerifyIgnoringAudience(token)
}

// New returns a Service backed by the given Issuer (used as both verifier
// and minter for first-party flows). The verifier ignores the audience
// claim so that a hop-2 exchange can accept a hop-1 token that was
// already scoped to a different audience.
func New(issuer *auth.Issuer, serviceIdentity string) *Service {
	return &Service{
		Verifier:        looseVerifier{Issuer: issuer},
		Minter:          issuer,
		ServiceIdentity: serviceIdentity,
		SafetyMargin:    60 * time.Second,
	}
}

// CacheSize returns the number of cached entries (test helper).
func (s *Service) CacheSize() int {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	return len(s.cache.entries)
}

// Exchange validates the subject token and returns a new token whose
// Subject is the original user, Actor identifies the calling agent, and
// Audience is scoped to the requested target.
//
// Repeated calls with the same (user, actor, audience) tuple return a
// cached token until the cached token's expiration minus the safety margin.
func (s *Service) Exchange(ctx context.Context, req Request) (string, auth.Claims, error) {
	if req.Audience == "" {
		return "", auth.Claims{}, ErrAudienceRequired
	}
	if req.SubjectToken == "" {
		return "", auth.Claims{}, ErrSubjectTokenInvalid
	}

	// Verify the subject token. This catches expired user tokens before
	// we mint an exchanged token whose existence implies the user was
	// recently authenticated.
	subject, err := s.Verifier.Verify(req.SubjectToken)
	if err != nil {
		return "", auth.Claims{}, fmt.Errorf("%w: %v", ErrSubjectTokenInvalid, err)
	}

	key := cacheKey{
		userSubject: subject.Subject,
		actorID:     req.ActorID,
		audience:    req.Audience,
	}

	now := time.Now().UTC()
	if cached, ok := s.cache.get(key, now); ok {
		return cached.token, cached.claims, nil
	}

	// Mint the exchanged token. Roles come from the subject — the user's
	// permissions are what carry through; the agent's identity sits in
	// Actor.
	exchanged, claims, err := s.Minter.Issue(subject.Subject, subject.Email, subject.Roles)
	if err != nil {
		return "", auth.Claims{}, fmt.Errorf("mint exchanged token: %w", err)
	}

	// Rewrite the audience to the requested target and stamp the actor.
	// Issue() doesn't accept audience/actor today, so we re-mint via a
	// helper that bypasses Issue's defaults.
	exchanged, claims, err = s.Minter.IssueWithActor(subject.Subject, subject.Email, subject.Roles, []string{req.Audience}, &auth.Actor{
		Subject: req.ActorID,
		Issuer:  s.ServiceIdentity,
		Nested:  subject.Actor, // chain previous actors if the subject token already carried one
	})
	if err != nil {
		return "", auth.Claims{}, fmt.Errorf("mint exchanged token: %w", err)
	}

	// Cache TTL: the exchanged token's exp minus the safety margin, but
	// never longer than the subject token's remaining lifetime (you can't
	// outlive your own user).
	expiresAt := time.Unix(claims.ExpiresAt, 0)
	subjectExpiresAt := time.Unix(subject.ExpiresAt, 0)
	if subjectExpiresAt.Before(expiresAt) {
		expiresAt = subjectExpiresAt
	}
	cacheUntil := expiresAt.Add(-s.SafetyMargin)
	if cacheUntil.After(now) {
		s.cache.set(key, cachedToken{token: exchanged, claims: claims, expiresAt: cacheUntil})
	}

	return exchanged, claims, nil
}

// Invalidate removes all cached tokens for a given user. Call this on
// user logout or password change so exchanged tokens minted before the
// event cannot be reused after.
//
// This is a precaution — exchanged tokens are already short-lived. But
// if a user changes password to recover from a compromise, every issued
// token for them should be considered burned.
func (s *Service) Invalidate(userSubject string) {
	s.cache.invalidate(userSubject)
}

// ---------------------------------------------------------------------------
// cache — keyed by (user, actor, audience) tuple
// ---------------------------------------------------------------------------

type cacheKey struct {
	userSubject string
	actorID     string
	audience    string
}

type cachedToken struct {
	token     string
	claims    auth.Claims
	expiresAt time.Time
}

type cache struct {
	mu      sync.Mutex
	entries map[cacheKey]cachedToken
}

func (c *cache) get(key cacheKey, now time.Time) (cachedToken, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		return cachedToken{}, false
	}
	v, ok := c.entries[key]
	if !ok {
		return cachedToken{}, false
	}
	if !now.Before(v.expiresAt) {
		delete(c.entries, key) // lazily evict expired
		return cachedToken{}, false
	}
	return v, true
}

func (c *cache) set(key cacheKey, v cachedToken) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[cacheKey]cachedToken)
	}
	c.entries[key] = v
}

func (c *cache) invalidate(userSubject string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if k.userSubject == userSubject {
			delete(c.entries, k)
		}
	}
}
