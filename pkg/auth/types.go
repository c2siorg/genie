// Package auth provides authentication and authorization primitives:
// users, roles, JWT issuance/verification, and password hashing.
//
// The package has no dependency on HTTP, the bus, or governance — it can be
// reused by any transport layer. HTTP middleware lives under pkg/web/mid.
package auth

import "time"

// Role identifies a coarse-grained authorization tier.
//
// Roles are strings (not enums) so they can be extended without recompilation.
// Convention: lower-case, hyphenated, no spaces.
type Role string

const (
	// RoleUser is the default role for new signups.
	RoleUser Role = "user"
	// RoleAdvisor can read aggregated data for accounts they advise.
	RoleAdvisor Role = "advisor"
	// RoleAdmin can manage other users and bypass classification gates.
	RoleAdmin Role = "admin"
)

// User is the persistent identity record.
//
// Email is the login identifier. PasswordHash is a bcrypt hash; never store
// the raw password. Roles are stored as a list to support multi-role users.
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
// The Actor field carries the RFC 8693 dual-identity semantics — when an
// agent runtime exchanges a user's token, the new token's Subject stays
// the user and Actor identifies the agent currently acting on the user's
// behalf. Upstream services can enforce composite policies of the form
// "allow if Subject has permission AND Actor is authorized for this
// operation."
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
type Actor struct {
	Subject string `json:"sub"`           // agent / service identity
	Issuer  string `json:"iss,omitempty"` // who minted the actor identity
	Nested  *Actor `json:"act,omitempty"` // previous actor in the chain
}

// HasRole returns true if the claims include the given role.
func (c Claims) HasRole(r Role) bool {
	for _, x := range c.Roles {
		if x == r {
			return true
		}
	}
	return false
}
