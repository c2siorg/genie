// Package auth provides authentication and authorization primitives:
// users, roles, JWT issuance/verification, and password hashing.
//
// ─── Scope of the package ──────────────────────────────────────────────────
//
// This package has no dependency on HTTP, the bus, or governance — it
// can be reused by any transport layer. HTTP middleware lives under
// pkg/web/mid; that layer adapts these types to net/http handlers.
//
// Why the separation: keeping auth pure of HTTP means the same types
// flow over the in-memory bus, over MCP, over CLI tools, and over
// HTTP without translation layers. Claims is the canonical "who is
// asking" type across every entry point.
//
// ─── What's in this file ───────────────────────────────────────────────────
//
//   - Role: coarse authorisation tier (user / advisor / admin)
//   - User: persistent identity record
//   - Claims: JWT payload (with the RFC 8693 Actor field)
//   - Actor: dual-identity claim for token-exchange flows
//   - HasRole: convenience predicate
//
// The HS256 JWT encode/decode lives in jwt.go; password hashing in
// password.go. Multiple authentication paths (password, JWT, passkey,
// OAuth) all land in this same Claims type — downstream code never has
// to branch on "how did the user prove themselves."
package auth

import "time"

// Role identifies a coarse-grained authorization tier.
//
// Roles are strings (not enums) so they can be extended without
// recompilation — a new role can be introduced via a database migration
// and a config change rather than a release. Convention: lower-case,
// hyphenated, no spaces.
//
// Fine-grained authorisation (per-tenant, per-resource) is the job of
// the governance policy stack, not roles. Roles answer "what kind of
// user is this?"; policies answer "is this user allowed to do this
// specific thing with this specific resource?"
type Role string

// Role constants — keep stable, these appear in JWTs and audit logs.
const (
	// RoleUser is the default role for new signups. Covers every
	// customer-facing capability: ask questions, upload documents,
	// view their own audit log.
	RoleUser Role = "user"

	// RoleAdvisor can read aggregated data for accounts they advise.
	// Used by relationship managers in private banking deployments.
	// Aggregated only — never sees raw transactions for a customer
	// who didn't authorise the advisor relationship.
	RoleAdvisor Role = "advisor"

	// RoleAdmin can manage other users and bypass classification gates.
	// Required for: the audit reader, the AI inventory endpoint, the
	// incident reader, the AIBOM endpoint. The HTTP router gates each
	// of those with RequireRole(RoleAdmin).
	//
	// Admin does NOT automatically bypass tenant isolation —
	// TenantPolicy.AdminBypass is opt-in per policy instance, and the
	// customer-facing routes don't enable it. The admin role is also
	// the gate for WithAdminContext on the DB side.
	RoleAdmin Role = "admin"
)

// User is the persistent identity record.
//
// Email is the login identifier (lower-cased on signup, unique across
// the table). PasswordHash is a bcrypt hash; never store the raw
// password. Roles are stored as a list to support multi-role users —
// a single user can be both "user" and "advisor" without needing a
// second account.
//
// The PasswordHash JSON tag is "-" so it never marshals out — any
// handler that returns a User to a client (signup, profile read) is
// safe from leaking the hash. Defence in depth: even if a future
// handler is mis-written, the hash can't ride along.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name,omitempty"`
	Roles        []Role    `json:"roles"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Claims is the JWT payload Genie issues.
//
// ─── The Actor field ──────────────────────────────────────────────────────
//
// The Actor field carries the RFC 8693 dual-identity semantics — when an
// agent runtime exchanges a user's token, the new token's Subject stays
// the user and Actor identifies the agent currently acting on the user's
// behalf. Upstream services can enforce composite policies of the form
// "allow if Subject has permission AND Actor is authorized for this
// operation."
//
// For first-party tokens (user logs in, gets a token directly), Actor is
// nil. The omitempty JSON tag keeps the field out of the wire format
// when it's not in use, so the JWT stays small for the common case.
//
// ─── Claim field reference ────────────────────────────────────────────────
//
//   Subject   — the authenticated user's stable id (UUID)
//   Email     — convenience, used by audit logs and UI labels
//   Roles     — authorisation tier list (one or more)
//   IssuedAt  — Unix seconds, UTC
//   ExpiresAt — Unix seconds, UTC; verifier rejects past this
//   Issuer    — the minting service (e.g. "genie-api"); used for
//                cross-issuer routing in federated deployments
//   Audience  — list of accepted audiences; verifier requires at least
//                one match if it has a non-empty audience list
//   Actor     — RFC 8693 `act` claim (see above)
//
// Wire format is JSON with the JWT-standard short field names (sub,
// iat, exp, iss, aud, act). Genie does not invent new claim names —
// stays interoperable with any RFC-compliant verifier.
type Claims struct {
	Subject   string   `json:"sub"`
	Email     string   `json:"email"`
	Roles     []Role   `json:"roles"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
	Issuer    string   `json:"iss,omitempty"`
	Audience  []string `json:"aud,omitempty"`
	// Actor is the RFC 8693 `act` claim. Set when this token was issued
	// via a token-exchange flow. Empty for first-party user tokens.
	Actor *Actor `json:"act,omitempty"`
}

// Actor identifies the service currently acting on the Subject's behalf
// (RFC 8693 § 4.1). For nested actor chains (an MCP server that itself
// exchanges the token for an upstream call), Nested holds the previous
// actor and the chain extends.
//
// ─── Chain semantics ──────────────────────────────────────────────────────
//
// Walking from Actor outward (Actor.Nested.Nested...) gives the
// innermost-first sequence: the first service to act, then the next,
// and so on. The outermost Actor (claims.Actor itself) is the most
// recent service to mint the token — the "proximate caller."
//
// Example for a three-hop chain (user → orchestrator → MCP server →
// upstream API):
//
//   claims.Subject                            = user-alice
//   claims.Actor.Subject                      = mcp_server_or_api  (outermost)
//   claims.Actor.Nested.Subject               = kyc_orchestrator   (inner)
//   claims.Actor.Nested.Nested                = nil (chain head)
//
// To list the chain user-first → outermost-last, collect [Subject],
// then walk Actor → Actor.Nested → ... and reverse.
//
// ─── Field semantics ──────────────────────────────────────────────────────
//
//   Subject — the service / agent identity. Same string the registry
//              uses; same string the audit log records as the actor.
//   Issuer  — who minted this actor identity. The downstream service
//              uses this to verify the actor was minted by a trusted
//              authority (cross-issuer trust check).
//   Nested  — the previous actor in the chain. nil for a first-hop
//              exchange; non-nil for second-and-later hops.
type Actor struct {
	Subject string `json:"sub"`           // agent / service identity
	Issuer  string `json:"iss,omitempty"` // who minted the actor identity
	Nested  *Actor `json:"act,omitempty"` // previous actor in the chain
}

// HasRole returns true if the claims include the given role.
//
// Linear scan, but the list is short (typically 1-3 roles) so a map
// would be overkill. The compiler inlines this in most call sites.
//
// Used by the HTTP middleware (RequireRole) and the bus governance
// (RBACPolicy) to gate access on role membership. Centralising the
// check in one method means a future change to role semantics
// (case-insensitive comparison, hierarchical roles) is a one-file
// change.
func (c Claims) HasRole(r Role) bool {
	for _, x := range c.Roles {
		if x == r {
			return true
		}
	}
	return false
}
