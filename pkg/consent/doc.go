// Package consent provides A2A (Agent-to-Agent) consent management and audit logging.
//
// # Overview
//
// The consent package implements a consent registry where agents can grant, revoke, and check
// permissions for other agents to access resources. All operations are thread-safe and
// designed for use in high-concurrency multi-agent environments.
//
// # Core Types
//
// [ConsentRecord] represents a single consent grant: who (userID), what (resourceType),
// at what level (PermissionLevel), until when (expiresAt), and why (reason/grantedBy).
//
// [ConsentMatrix] is the primary interface for managing grants. The [InMemoryConsentMatrix]
// implementation provides a thread-safe, in-memory store suitable for development and testing.
// For production deployments, implement the interface over a durable backend.
//
// [AccessLog] records every authorization decision for audit and compliance. The
// [InMemoryAccessLog] implementation supports querying by userID and time range, as well
// as exporting to JSON or CSV. Like ConsentMatrix, this is designed as an interface to
// support swappable implementations.
//
// [ConsentChecker] is the primary entry point for agents/handlers: a callable function
// that checks consent, logs the decision, and returns (allowed, reasonCode, error).
//
// # Permission Model
//
// Permissions are hierarchical:
//   - [PermRead]: read-only access
//   - [PermReadWrite]: read and write access
//   - [PermAdmin]: all operations including delete and admin actions
//   - [PermNone]: no access (used only internally during revocation checks)
//
// Actions are checked against permissions:
//   - [ActionRead], [ActionWrite], [ActionDelete], [ActionAdmin]
//
// Example:
//
//	record := ConsentRecord{Permissions: PermReadWrite}
//	record.AllowsAction(ActionRead)   // true
//	record.AllowsAction(ActionWrite)  // true
//	record.AllowsAction(ActionDelete) // false
//
// # Usage Pattern
//
// Create a matrix and log, then instantiate a checker:
//
//	matrix := consent.NewInMemoryConsentMatrix()
//	log := consent.NewInMemoryAccessLog()
//	checker := consent.NewConsentChecker(matrix, log)
//
// Agents grant permissions:
//
//	id, err := matrix.Grant(
//	    "agent-b",              // userID
//	    "data-lake",            // resourceType
//	    consent.PermReadWrite,  // permissionLevel
//	    24*time.Hour,           // ttl
//	    "data processing job",  // reason (compliance trace)
//	    "agent-a",              // grantedBy
//	)
//
// Other agents check permissions:
//
//	allowed, reasonCode, err := checker(
//	    "agent-b",
//	    "data-lake",
//	    consent.ActionWrite,
//	    "trace-id-123",         // for correlation
//	)
//	if !allowed {
//	    // Decision was denied and logged with reasonCode
//	    return fmt.Errorf("access denied: %s", reasonCode)
//	}
//	// Access was allowed and logged
//
// Query and audit the log:
//
//	decisions, _ := log.Query("agent-b", since, until)
//	// Check what happened and when
//
//	csvData, _ := log.Export("csv")
//	// Send to compliance/audit system
//
// # Thread Safety
//
// Both [InMemoryConsentMatrix] and [InMemoryAccessLog] are safe for concurrent use
// without external synchronization. All operations acquire appropriate locks (read locks
// for queries, write locks for mutations).
//
// # Expiration
//
// Consents with a TTL automatically expire. Expiration is checked during [ConsentMatrix.Check]
// calls; no background cleanup is needed. A grant with TTL 0 never expires.
//
// # Error Handling
//
// The package defines sentinel errors: [ErrNotFound], [ErrInvalidInput], [ErrInternalError].
// Use [errors.Is] to check for specific error types.
//
// In the [ConsentChecker], infrastructure errors (e.g., matrix.Check failing) are logged
// as "check_error" decisions and returned to the caller. Access denials (no grant, expired,
// insufficient permissions) are logged with specific reasonCodes and the checker returns
// (false, reasonCode, nil).
//
// # Implementation Notes
//
// The in-memory implementations maintain dual indices for fast lookups:
//   - byUserResource: optimized for Grant/Revoke/Check operations (O(1) expected)
//   - byUser / byResourceType: optimized for List operations (O(n) where n = grants for that user/resource)
//
// All indices are kept in sync during mutations. A single RWMutex guards all state.
//
// For very high-concurrency scenarios (millions of grants, thousands of concurrent checkers),
// consider a sharded implementation with per-bucket locks, or a persistent backend with
// indexing support.
package consent
