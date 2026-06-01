// Package consent provides A2A (Agent-to-Agent) consent management.
//
// It implements a consent registry where agents can grant, revoke, and check
// permissions for other agents to access resources. All operations are
// thread-safe and suitable for concurrent use in multi-agent environments.
//
// The consent model is simple and flexible:
//   - ConsentRecord captures a grant of permission (who, what, permissions level, expiry)
//   - ConsentMatrix manages grants with Check, Grant, Revoke, List operations
//   - AccessLog records every authorization decision for audit/compliance
//   - ConsentChecker ties them together as a callable decision function
//
// Usage:
//
//	matrix := consent.NewInMemoryConsentMatrix()
//	log := consent.NewInMemoryAccessLog()
//	checker := consent.NewConsentChecker(matrix, log)
//
//	// Grant permission
//	matrix.Grant("agent-a", "database", consent.PermReadWrite, 24*time.Hour, "data processing", "admin")
//
//	// Check and log the decision
//	allowed, reason, _ := checker(ctx, "agent-a", "database", "write")
package consent

import (
	"fmt"
	"time"
)

// PermissionLevel defines the permission tier granted in a consent record.
type PermissionLevel string

const (
	// PermNone means no permission (default, used only for revocation).
	PermNone PermissionLevel = "none"

	// PermRead allows read-only access to the resource.
	PermRead PermissionLevel = "read"

	// PermReadWrite allows both read and write access.
	PermReadWrite PermissionLevel = "read_write"

	// PermAdmin allows administrative access (highest privilege).
	PermAdmin PermissionLevel = "admin"
)

// ActionType defines the kind of action being checked against a resource.
type ActionType string

const (
	ActionRead   ActionType = "read"
	ActionWrite  ActionType = "write"
	ActionDelete ActionType = "delete"
	ActionAdmin  ActionType = "admin"
)

// ConsentRecord represents a single consent grant in the registry.
//
// A record captures the fact that grantedBy authorized userID to access
// resourceType at the given permission level until expiresAt.
type ConsentRecord struct {
	// ID is a unique identifier for this consent grant (UUIDv4).
	ID string `json:"id"`

	// UserID is the agent or principal being granted permission.
	UserID string `json:"user_id"`

	// ResourceType identifies the resource being accessed (e.g. "database", "api", "file-store").
	ResourceType string `json:"resource_type"`

	// Permissions is the level of access (read, read_write, admin).
	Permissions PermissionLevel `json:"permissions"`

	// ExpiresAt is the time after which this grant is no longer valid.
	// If zero, the grant never expires.
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// Reason explains why this consent was granted (compliance trace).
	Reason string `json:"reason"`

	// GrantedBy identifies the principal who authorized this grant.
	GrantedBy string `json:"granted_by"`

	// CreatedAt records when the grant was issued.
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired returns true if the consent record has passed its expiration time.
func (cr *ConsentRecord) IsExpired() bool {
	if cr.ExpiresAt.IsZero() {
		return false // No expiration
	}
	return time.Now().UTC().After(cr.ExpiresAt)
}

// AllowsAction returns true if this record's permission level covers the given action.
func (cr *ConsentRecord) AllowsAction(action ActionType) bool {
	switch cr.Permissions {
	case PermAdmin:
		return true // Admin allows everything
	case PermReadWrite:
		return action == ActionRead || action == ActionWrite
	case PermRead:
		return action == ActionRead
	default:
		return false
	}
}

// ConsentMatrix is the primary interface for managing consent grants.
//
// All implementations must be thread-safe and support concurrent access.
type ConsentMatrix interface {
	// Grant records a new consent or updates an existing one.
	// Returns the ID of the granted consent record.
	Grant(userID, resourceType string, perms PermissionLevel, ttl time.Duration, reason, grantedBy string) (string, error)

	// Revoke removes a consent grant by userID + resourceType.
	// Returns ErrNotFound if no matching grant exists.
	Revoke(userID, resourceType string) error

	// Check evaluates whether a grant exists, is not expired, and covers the action.
	// Returns (allowed, permissionLevel, error).
	Check(userID, resourceType string, action ActionType) (bool, PermissionLevel, error)

	// List returns all consent records for a given userID.
	// Results include expired records; the caller should filter if needed.
	List(userID string) ([]ConsentRecord, error)

	// ListByResource returns all consent records for a given resourceType.
	ListByResource(resourceType string) ([]ConsentRecord, error)
}

// AccessDecision captures the result of a single authorization check.
type AccessDecision struct {
	// Timestamp is when the decision was made (UTC).
	Timestamp time.Time `json:"timestamp"`

	// UserID is the agent making the request.
	UserID string `json:"user_id"`

	// ResourceType is what was being accessed.
	ResourceType string `json:"resource_type"`

	// Action is what was being attempted (read, write, etc).
	Action ActionType `json:"action"`

	// Result is "allowed" or "denied".
	Result string `json:"result"`

	// ReasonCode explains the decision (e.g. "no_grant", "expired", "insufficient_perms").
	ReasonCode string `json:"reason_code"`

	// TraceID is a correlation ID for linking back to the request.
	TraceID string `json:"trace_id"`
}

// AccessLog is the interface for recording authorization decisions.
type AccessLog interface {
	// Log records a single decision.
	Log(decision AccessDecision) error

	// Query returns decisions matching the given filters.
	// Set since/until to zero for no time filtering.
	// Leave userID empty to match all users.
	Query(userID string, since, until time.Time) ([]AccessDecision, error)

	// Export returns all logged decisions in a structured format.
	// format can be "json" or "csv". Returns serialized bytes.
	Export(format string) ([]byte, error)
}

// ConsentChecker is a callable function that checks consent and logs the decision.
//
// It is the primary interface for agents/handlers to use when authorizing
// an A2A request.
//
// Returns (allowed, reasonCode, error).
// If error is non-nil, it indicates an infrastructure issue (not an authz denial).
type ConsentChecker func(userID, resourceType string, action ActionType, traceID string) (bool, string, error)

// NewConsentChecker creates a ConsentChecker that uses the given matrix and log.
func NewConsentChecker(matrix ConsentMatrix, log AccessLog) ConsentChecker {
	return func(userID, resourceType string, action ActionType, traceID string) (bool, string, error) {
		// Check the matrix
		allowed, _, err := matrix.Check(userID, resourceType, action)
		if err != nil {
			// Log infrastructure errors as denied with error reason
			decision := AccessDecision{
				Timestamp:    time.Now().UTC(),
				UserID:       userID,
				ResourceType: resourceType,
				Action:       action,
				Result:       "denied",
				ReasonCode:   "check_error",
				TraceID:      traceID,
			}
			_ = log.Log(decision) // Best effort; don't fail the overall check
			return false, "check_error", err
		}

		// Record the decision
		result := "denied"
		reasonCode := "no_grant"
		if allowed {
			result = "allowed"
			reasonCode = ""
		}

		decision := AccessDecision{
			Timestamp:    time.Now().UTC(),
			UserID:       userID,
			ResourceType: resourceType,
			Action:       action,
			Result:       result,
			ReasonCode:   reasonCode,
			TraceID:      traceID,
		}
		_ = log.Log(decision) // Best effort; don't fail the overall check

		return allowed, reasonCode, nil
	}
}

// Error types for consent operations.
var (
	ErrNotFound      = fmt.Errorf("consent: not found")
	ErrAlreadyExists = fmt.Errorf("consent: already exists")
	ErrInvalidInput  = fmt.Errorf("consent: invalid input")
	ErrInternalError = fmt.Errorf("consent: internal error")
)
