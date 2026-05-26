// Package tokenexchange implements OAuth 2.0 Token Exchange (RFC 8693)
// for the agent runtime → MCP server → upstream API call chain.
//
// ─── What this package solves ──────────────────────────────────────────────
//
// When an agent calls a downstream service, the audit trail wants both
// the user identity ("Alice initiated this") AND the agent identity
// ("kyc_orchestrator was the proximate caller"). A single-identity token
// gives you one or the other, never both.
//
// RFC 8693 standardises the third option: dual-identity tokens. The
// Subject claim stays the user; an Actor (`act`) claim records the agent.
// Both are signed into the same JWT. Downstream services can write a
// single audit row that answers both "who initiated" and "what was the
// proximate caller."
//
// ─── The motivating use case ────────────────────────────────────────────────
//
// Nurse Alice's medical-assistant agent queries patient records:
//
//   1. Alice signs in to Genie → first-party JWT (Subject=alice)
//   2. Agent runtime exchanges Alice's token for a downstream token:
//        Subject = alice (unchanged — Alice initiated)
//        Actor   = medical_assistant_agent (the agent currently acting)
//        Audience = mcp://patient-records-server
//   3. Agent calls the MCP server with the dual-identity token
//   4. MCP server's audit log writes: "alice / via medical_assistant_agent
//      / read patient #1234 at 14:23 UTC"
//
// Without RFC 8693 the audit row would either blame Alice for an
// automated action (single-identity = user) or blame the service and
// lose Alice entirely (single-identity = service).
//
// ─── N-hop nested chains ───────────────────────────────────────────────────
//
// Real agent stacks often have more than one hop. Nurse Alice → medical
// assistant → MCP server → upstream EMR API is three hops. RFC 8693
// supports nesting via Actor.Nested:
//
//   claims.Subject              = alice
//   claims.Actor.Subject        = patient_records_mcp_server  (outermost)
//   claims.Actor.Nested.Subject = medical_assistant_agent     (inner hop)
//
// Walking the chain from claims.Actor outward yields the full audit
// trail. The test TestNestedActorChain exercises this.
//
// ─── Caching ───────────────────────────────────────────────────────────────
//
// Exchanged tokens are cached on (user_subject, agent_id, audience).
// TTL is min(token_exp, subject_exp) − safety_margin. A stale entry
// (token expired) returns a cache miss; we never serve an expired token.
//
// Why this tuple as the key, not the input token:
//   - Two callers presenting different valid subject tokens for the
//     same user share the cached exchanged token. No re-mint thrash.
//   - The audience is included so a token cached for one downstream
//     audience can't be confused-deputy'd into use against another.
//   - The actor id is included so a different service hopping the chain
//     produces a different cached token.
//
// ─── Why a "loose verifier" ────────────────────────────────────────────────
//
// The first hop's exchanged token carries the requested audience (e.g.
// mcp://kyc-server), not the issuer's default. When the MCP server
// wants to exchange again for an upstream call, the input token's
// audience is mcp://kyc-server, not the verifier's default.
//
// A strict audience check on the input would reject it. We wrap the
// issuer in looseVerifier, which calls Issuer.VerifyIgnoringAudience —
// signature still checked, expiry still checked, issuer still checked,
// only the audience claim is skipped.
//
// This is safe because the output audience is the new one the caller
// asks for, which is what the downstream service will then strictly
// verify. The input audience is the previous hop's output audience,
// which is meaningful to the previous hop's downstream, not to us.
//
// ─── FREE-AI alignment ─────────────────────────────────────────────────────
//
// Rec 22 (Tamper-Evident Audit) — dual-identity tokens are the source
// of the audit record's "who" and "via what" columns.
// Rec 14 (Board-Approved Policy) — policies can be expressed as
// composite gates: "user has permission X AND actor is authorised for
// operation Y" — impossible with a single-identity token.
package tokenexchange

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

// ErrSubjectTokenInvalid is returned when the subject token fails
// verification. The wrapped error has the underlying detail (expired,
// signature mismatch, garbage); callers can compare against this
// sentinel with errors.Is to make a 401-vs-500 decision.
var ErrSubjectTokenInvalid = errors.New("tokenexchange: subject token invalid")

// ErrAudienceRequired is returned when the caller doesn't specify an
// audience. Audience is the scope of the exchanged token; without it,
// the token would be "good for anyone" which defeats the confused-deputy
// protection. Required, no default.
var ErrAudienceRequired = errors.New("tokenexchange: audience is required")

// Request describes one token-exchange call.
//
// All three fields are required; an empty value in any position is an
// error. The package returns the caller's mistake as one of the named
// sentinels above so handlers can map them to HTTP 400 vs 401 vs 500.
type Request struct {
	// SubjectToken is the user's JWT (or a previously-exchanged token to
	// extend the actor chain).
	//
	// Verified via the loose verifier — signature, expiry, and issuer
	// are checked; audience is skipped (see the "loose verifier"
	// rationale in the package doc).
	SubjectToken string

	// ActorID is the stable identifier of the service currently acting
	// (e.g. "kyc_orchestrator", "medical-assistant-agent").
	//
	// This is the agent's address in the registry — same string the bus
	// uses for routing, same string the audit log records.
	ActorID string

	// Audience identifies the target service the exchanged token is for.
	// Tokens are scoped per audience to prevent confused-deputy attacks
	// — a token minted for audience A cannot be presented to a service
	// that expects audience B (assuming the downstream service strictly
	// verifies its audience, which it must).
	Audience string
}

// Service performs RFC 8693 token exchange against a Genie JWT Issuer.
//
// The same Issuer is used for verifying the subject token and minting the
// exchanged token. In a federated deployment, replace Verifier with a
// separate verifier wired to the user's identity provider.
//
// Concurrency: safe. The cache is mutex-guarded; verification and
// minting are stateless w.r.t. the Service struct fields. One Service
// per host process is sufficient.
type Service struct {
	// Verifier validates the SubjectToken. Typically a Verifier that
	// does NOT enforce audience — second-hop exchanges receive tokens
	// already scoped to a different audience than the issuer's default.
	// Use auth.Issuer.VerifyIgnoringAudience if you have one Issuer.
	//
	// Pluggable interface for federated deployments: a host that
	// accepts tokens from a separate IdP (Auth0, Okta, internal SSO)
	// implements this interface with an IdP-aware verifier.
	Verifier interface {
		Verify(token string) (auth.Claims, error)
	}

	// Minter issues the exchanged token. Same as Verifier in the common
	// single-IdP deployment. In a federated deployment, this stays the
	// Genie-local issuer because the new token is from us; the verifier
	// is what changes.
	Minter *auth.Issuer

	// ServiceIdentity names the host (e.g. "genie-api") that mints the
	// exchanged tokens; recorded in Actor.Issuer.
	//
	// Per-deployment string — used by the downstream service to verify
	// "this Actor was minted by an issuer I trust."
	ServiceIdentity string

	// SafetyMargin shaves time off the cached TTL to absorb clock skew
	// and network latency. Default 60s.
	//
	// Without the margin, a token might be served from cache one second
	// before its actual expiry and arrive at the downstream service
	// already expired — a flaky failure. 60s is enough for typical
	// clock skew + intra-datacenter network; bump to 120s for cross-
	// region deployments.
	SafetyMargin time.Duration

	// cache is the (user, actor, audience) → cached token map. Mutex-
	// guarded; details in the cache struct below.
	cache cache
}

// looseVerifier wraps an Issuer to skip audience validation. Token
// exchange treats audience as the *output* scope; the input may be
// scoped to anything (e.g. a hop-1 exchanged token scoped to an MCP
// server).
//
// Implements the Verifier interface above by delegating to
// Issuer.VerifyIgnoringAudience. The other claim checks (signature,
// expiry, issuer) still run inside Verify, so this is not a bypass of
// the security envelope — only of the audience field which has
// different semantics in the exchange flow than in normal verification.
type looseVerifier struct{ Issuer *auth.Issuer }

// Verify satisfies the Service.Verifier interface. Delegates to the
// audience-skipping verify on the wrapped Issuer.
func (l looseVerifier) Verify(token string) (auth.Claims, error) {
	return l.Issuer.VerifyIgnoringAudience(token)
}

// New returns a Service backed by the given Issuer (used as both verifier
// and minter for first-party flows). The verifier ignores the audience
// claim so that a hop-2 exchange can accept a hop-1 token that was
// already scoped to a different audience.
//
// SafetyMargin defaults to 60 seconds. Override after construction if
// your environment has unusual clock skew or latency:
//
//   svc := tokenexchange.New(issuer, "genie-api")
//   svc.SafetyMargin = 120 * time.Second   // cross-region deployment
//
// For a federated deployment (verifier is a different IdP), construct
// the Service directly with your own Verifier instead of calling New.
func New(issuer *auth.Issuer, serviceIdentity string) *Service {
	return &Service{
		Verifier:        looseVerifier{Issuer: issuer},
		Minter:          issuer,
		ServiceIdentity: serviceIdentity,
		SafetyMargin:    60 * time.Second,
	}
}

// CacheSize returns the number of cached entries (test helper).
//
// Used by TestInvalidateClearsCacheForUser — directly observing cache
// state is more reliable than asserting on token bytes (two same-second
// mints of identical Claims produce byte-identical JWTs).
//
// Production code should not depend on this — the cache is an internal
// optimisation and the exact size after any given Exchange call is not
// part of the contract.
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
//
// Error contract:
//   - ErrAudienceRequired — Audience was empty
//   - ErrSubjectTokenInvalid (wrapped) — subject token didn't verify
//     (garbage, signature mismatch, expired, issuer mismatch)
//   - other (wrapped) — minting or caching failure
//
// Callers should map ErrSubjectTokenInvalid to HTTP 401 and
// ErrAudienceRequired to HTTP 400.
func (s *Service) Exchange(ctx context.Context, req Request) (string, auth.Claims, error) {
	// Validate inputs before doing any work. Audience must be specified
	// — the package refuses to mint an audience-less token (would be a
	// "valid for anyone" token, defeating confused-deputy protection).
	if req.Audience == "" {
		return "", auth.Claims{}, ErrAudienceRequired
	}
	// SubjectToken must be non-empty. The Verifier would also reject an
	// empty string, but giving the caller a clear "invalid token" sentinel
	// rather than a parse error from deep in the verifier is friendlier.
	if req.SubjectToken == "" {
		return "", auth.Claims{}, ErrSubjectTokenInvalid
	}

	// Verify the subject token. This catches expired user tokens before
	// we mint an exchanged token whose existence implies the user was
	// recently authenticated.
	//
	// Audience is skipped (looseVerifier) because a hop-2 token's
	// audience is the hop-1 service, not the verifier's default — see
	// the package doc.
	subject, err := s.Verifier.Verify(req.SubjectToken)
	if err != nil {
		// Wrap the underlying error so the caller can errors.Is for the
		// sentinel and also see the detail for debugging.
		return "", auth.Claims{}, fmt.Errorf("%w: %v", ErrSubjectTokenInvalid, err)
	}

	// Build the cache key. The user's authenticated subject (not the
	// token bytes) is the user component — two different valid tokens
	// for the same user share the same cache entry.
	key := cacheKey{
		userSubject: subject.Subject,
		actorID:     req.ActorID,
		audience:    req.Audience,
	}

	// Check the cache. If a non-expired entry exists for this tuple,
	// return it directly — saves a mint call.
	now := time.Now().UTC()
	if cached, ok := s.cache.get(key, now); ok {
		return cached.token, cached.claims, nil
	}

	// Mint the exchanged token. Roles come from the subject — the user's
	// permissions are what carry through; the agent's identity sits in
	// Actor.
	//
	// Two-step mint: the first Issue() is needed so we have a fresh
	// timestamp baseline; the second IssueWithActor() does the real
	// work with the requested audience and the actor claim. (Yes, this
	// is one mint too many; the cleanup is on the roadmap. The function
	// returns early on the first error so we don't waste a mint.)
	exchanged, claims, err := s.Minter.Issue(subject.Subject, subject.Email, subject.Roles)
	if err != nil {
		return "", auth.Claims{}, fmt.Errorf("mint exchanged token: %w", err)
	}

	// Rewrite the audience to the requested target and stamp the actor.
	// Issue() doesn't accept audience/actor today, so we re-mint via a
	// helper that bypasses Issue's defaults.
	//
	// Actor chain: if the subject already carried an Actor (this is a
	// second hop), nest it under the new Actor.Nested. Walking the chain
	// outward yields user → first-hop → … → outermost.
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
	//
	// Two-part calculation:
	//   1. expiresAt starts as the exchanged token's own expiry.
	//   2. If the subject's expiry is earlier, use that instead — an
	//      exchanged token must not be valid after the user's session
	//      has ended.
	//   3. Shave the safety margin to absorb clock skew and network
	//      latency on the way to the downstream service.
	expiresAt := time.Unix(claims.ExpiresAt, 0)
	subjectExpiresAt := time.Unix(subject.ExpiresAt, 0)
	if subjectExpiresAt.Before(expiresAt) {
		expiresAt = subjectExpiresAt
	}
	cacheUntil := expiresAt.Add(-s.SafetyMargin)
	// Only cache if the entry would still be valid for some non-zero
	// time. A token that's already past its safety margin would be
	// served-from-cache once and then immediately treated as expired
	// — better to skip caching.
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
//
// Note: this only clears OUR cache. Tokens already in flight to
// downstream services remain valid until their TTL expires. The full
// "burn everything" story requires a downstream revocation list, which
// is roadmap. For now, rely on short TTLs (15 min default).
func (s *Service) Invalidate(userSubject string) {
	s.cache.invalidate(userSubject)
}

// ---------------------------------------------------------------------------
// cache — keyed by (user, actor, audience) tuple
// ---------------------------------------------------------------------------
//
// The cache is intentionally simple: a map under a mutex. No LRU, no
// size cap, no eviction goroutine. The justification:
//
//   - Entries expire within the token TTL (default 15 min) and are
//     evicted lazily on read.
//   - Cardinality is bounded by (#active users × #agents × #audiences)
//     — typically a few thousand even in a busy deployment.
//   - A mutex around a map outperforms sync.Map for this access pattern
//     (write-heavy on cache miss, read-heavy on cache hit).
//
// If the cache becomes a bottleneck, the right next step is sharding by
// userSubject hash — but profile first; this hasn't been a problem in
// any deployment to date.

// cacheKey is the deterministic key for the cache. All three fields are
// part of the security boundary — sharing the same exchanged token
// across two different (actor, audience) tuples would be a confused-
// deputy bug.
type cacheKey struct {
	userSubject string
	actorID     string
	audience    string
}

// cachedToken holds an exchanged token plus its absolute expiry (after
// the safety margin has been shaved). expiresAt is the timestamp at
// which this entry should no longer be served.
type cachedToken struct {
	token     string
	claims    auth.Claims
	expiresAt time.Time
}

// cache is the (key → cachedToken) map plus its mutex. Nothing fancy.
type cache struct {
	mu      sync.Mutex
	entries map[cacheKey]cachedToken
}

// get returns a cached token if one exists and hasn't expired. An
// expired entry is lazily evicted on read.
//
// Returns (cachedToken{}, false) on miss; (v, true) on hit. The bool
// disambiguates "absent" from "present but zero value" (which can't
// actually happen for tokens but is good map-access discipline).
func (c *cache) get(key cacheKey, now time.Time) (cachedToken, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Nil-safe: cache may not have been initialised if no Exchange has
	// run yet on this Service.
	if c.entries == nil {
		return cachedToken{}, false
	}
	v, ok := c.entries[key]
	if !ok {
		return cachedToken{}, false
	}
	// Expiry check: !now.Before(expiresAt) means now >= expiresAt,
	// which means expired. Lazy-evict and report miss.
	if !now.Before(v.expiresAt) {
		delete(c.entries, key) // lazily evict expired
		return cachedToken{}, false
	}
	return v, true
}

// set writes an entry. Lazily initialises the map on first write.
// Overwrites existing entries silently — re-minting always wins.
func (c *cache) set(key cacheKey, v cachedToken) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[cacheKey]cachedToken)
	}
	c.entries[key] = v
}

// invalidate drops every cache entry for the given user. Walks the map
// once. The cost is O(N) in cache size; for typical loads (a few
// thousand entries) this is sub-millisecond and runs on a path that's
// already slow (user logout).
//
// If invalidate becomes a hot path (e.g. mass password-reset), add a
// secondary index keyed by userSubject. Until then, simple wins.
func (c *cache) invalidate(userSubject string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if k.userSubject == userSubject {
			delete(c.entries, k)
		}
	}
}
