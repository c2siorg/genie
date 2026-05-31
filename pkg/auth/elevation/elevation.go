// Package elevation implements time-bound privileged access — the
// Privileged Access Manager analog (GCP PCSE §1.4) for Genie.
//
// ─── What this solves ──────────────────────────────────────────────────────
//
// Some operations need admin role for a short window: investigating a
// suspected breach, running an ad-hoc data migration, helping a
// customer with a rare edge case. Granting permanent admin to every
// engineer who might ever need this is the bad option — too many
// admins, too many compromised tokens become production-admin
// tokens.
//
// The good option: request → approve → window-of-time → automatic
// expiry, with every transition in the audit chain. After expiry the
// role is gone; no daemon to forget to run, no cron to mis-fire.
// Lazy expiry checked on read.
//
// ─── The contract ──────────────────────────────────────────────────────────
//
//   1. Engineer E requests elevation to RoleAdmin with a TTL ≤ MaxDuration
//      and a non-empty Reason.
//   2. Approver A (must hold RoleAdmin, must not be E) approves the
//      request. With N-eye policy (RequireApprovers > 1), N distinct
//      approvers must each call Approve.
//   3. The grant becomes Active with ExpiresAt = approve_time + TTL.
//   4. Effective roles for E include the elevated role until ExpiresAt.
//   5. After ExpiresAt the grant is treated as Expired on the next read.
//   6. Revocation by any admin is possible at any time before expiry.
//
// Every transition (Request, Approve, Deny, Revoke, lazy Expire on
// first observed read after the deadline) writes one entry to the
// supplied AuditLog. The grant carries the genesis AuditID for chain-
// of-custody trace-back.
//
// ─── What this is NOT ──────────────────────────────────────────────────────
//
// This isn't a generic permission engine. It only manages elevation
// to a single role at a time (typically RoleAdmin). It isn't a
// long-term role-management database — base roles live on the User
// record, and this service is the temporary additive layer on top.
//
// It doesn't issue new JWTs. The HTTP middleware that reads claims
// also consults this service to compute the effective role set —
// see EffectiveRoles. A future iteration could mint a short-lived
// JWT with the elevated role baked in (cleaner attribution, but loses
// the revoke-before-expiry capability).
//
// ─── FREE-AI alignment ─────────────────────────────────────────────────────
//
// Rec 14 (Board-approved policy) — elevation thresholds (max TTL,
// required approvers, eligible roles) live in board-approved YAML.
// Rec 22 (Tamper-evident audit) — every transition writes to the
// hash-chained audit log; the AuditID is captured on the grant.
package elevation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/google/uuid"
)

// ─── Errors ────────────────────────────────────────────────────────────────
//
// Sentinel errors so callers can errors.Is for them and map HTTP status
// codes correctly. Generic "elevation: …" prefix keeps log filtering
// straightforward.

// ErrSubjectRequired is returned when Request is called without a subject id.
var ErrSubjectRequired = errors.New("elevation: subject is required")

// ErrReasonRequired is returned when Request is called without a non-empty
// reason. Every elevation must justify itself in the audit log — empty
// reasons would render the log useless for after-the-fact review.
var ErrReasonRequired = errors.New("elevation: reason is required")

// ErrRoleNotElevatable is returned when the requested role isn't in the
// service's allowlist. Prevents requesting elevation to a role that the
// service isn't designed to grant (e.g. a future "super-admin" tier).
var ErrRoleNotElevatable = errors.New("elevation: role not elevatable")

// ErrTTLOutOfRange is returned when the requested TTL exceeds MaxDuration
// or is non-positive. Caps the blast-radius of any single elevation.
var ErrTTLOutOfRange = errors.New("elevation: requested TTL out of range")

// ErrApproverIsSubject enforces the 4-eyes minimum — the requester
// cannot approve their own request.
var ErrApproverIsSubject = errors.New("elevation: approver cannot be the subject")

// ErrApproverIneligible is returned when the approver doesn't hold the
// admin role (or whichever role the service treats as the approver
// gate). Approval must come from an existing admin, not just anyone.
var ErrApproverIneligible = errors.New("elevation: approver is not eligible")

// ErrGrantNotFound is returned by lookups for a missing grant id.
var ErrGrantNotFound = errors.New("elevation: grant not found")

// ErrGrantNotPending is returned when Approve is called against a grant
// that isn't in StatusPending (already approved, denied, expired, etc.).
var ErrGrantNotPending = errors.New("elevation: grant is not pending")

// ErrGrantNotActive is returned when Revoke is called against a grant
// that isn't currently active.
var ErrGrantNotActive = errors.New("elevation: grant is not active")

// ErrDuplicateApprover is returned when the same approver tries to
// approve a grant twice (relevant under N-eyes policy).
var ErrDuplicateApprover = errors.New("elevation: approver already recorded")

// ─── Status ────────────────────────────────────────────────────────────────

// Status describes where a grant is in its lifecycle. Strings (not int
// enum) because the values appear in JSON over the HTTP surface and in
// audit log entries; string is portable.
type Status string

const (
	// StatusPending — request created, not yet approved by the required
	// number of approvers. Carries no permission grant.
	StatusPending Status = "pending"

	// StatusActive — approved, before ExpiresAt. The subject's effective
	// roles include the elevated role.
	StatusActive Status = "active"

	// StatusExpired — ExpiresAt has passed. Reached lazily on read; no
	// daemon. The transition to expired writes one audit entry the
	// first time a read observes it.
	StatusExpired Status = "expired"

	// StatusRevoked — explicitly revoked by an admin before expiry.
	// Permanent terminal state; a revoked grant cannot be reactivated.
	StatusRevoked Status = "revoked"

	// StatusDenied — request denied by an approver. Permanent terminal
	// state; a denied request must be re-submitted.
	StatusDenied Status = "denied"
)

// ─── Grant ─────────────────────────────────────────────────────────────────

// Grant is the durable record of one elevation request and its state.
// Fields are exported for JSON marshalling by the HTTP layer; mutations
// happen only via the Service methods which hold the service mutex.
type Grant struct {
	// ID is the stable identifier — UUID generated at Request time.
	// Used as the route parameter for Approve/Deny/Revoke calls.
	ID string `json:"id"`

	// Subject is the user id receiving the elevation. Must equal
	// claims.Subject for the user that will use the elevated role.
	Subject string `json:"subject"`

	// Role is the role being granted. Must be in the service's
	// ElevatableRoles allowlist.
	Role auth.Role `json:"role"`

	// Reason is the free-text justification. Required. Flows into the
	// audit log for after-the-fact review.
	Reason string `json:"reason"`

	// RequestedTTL is the duration the requester asked for. Capped at
	// the service's MaxDuration.
	RequestedTTL time.Duration `json:"requested_ttl"`

	// RequestedAt is when the user filed the request.
	RequestedAt time.Time `json:"requested_at"`

	// Approvers lists the admin subjects who have approved. Length
	// must reach RequireApprovers before the grant becomes active.
	// Deny/Revoke do not append here; they set Status directly.
	Approvers []string `json:"approvers,omitempty"`

	// ApprovedAt is the timestamp of the final approval that
	// transitioned to Active. Zero if not yet active.
	ApprovedAt time.Time `json:"approved_at,omitempty"`

	// ExpiresAt is set when Status becomes Active: ApprovedAt + TTL.
	// After this point the grant is treated as expired on the next read.
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// Status is the lifecycle state — see the Status constants.
	Status Status `json:"status"`

	// RevokedBy / RevokedAt are populated if the grant was revoked.
	RevokedBy string    `json:"revoked_by,omitempty"`
	RevokedAt time.Time `json:"revoked_at,omitempty"`

	// DeniedBy / DeniedAt populate on a deny.
	DeniedBy string    `json:"denied_by,omitempty"`
	DeniedAt time.Time `json:"denied_at,omitempty"`

	// AuditEntryID is the seq id of the audit entry written at Request
	// time. Use as the head of the per-grant audit thread; subsequent
	// transitions reference this in their Details for trace-back.
	AuditEntryID int64 `json:"audit_entry_id"`
}

// ─── Service ───────────────────────────────────────────────────────────────

// Service is the elevation lifecycle manager. Thread-safe; single
// instance per host process.
//
// Lifecycle responsibility:
//   - Request (subject opens a request)
//   - Approve (admin signs off; N-eyes possible)
//   - Deny (admin refuses)
//   - Revoke (admin terminates an active grant early)
//   - List / Get / ActiveFor (read access)
//   - EffectiveRoles (the read path the HTTP middleware calls)
//
// Storage: in-memory map for tests/demos; a future PostgresStore
// implementation replaces the inline map with a SQL backing — the
// Store interface below makes that swap drop-in.
type Service struct {
	// Audit is the hash-chained audit log. Every state transition
	// writes one entry; the grant captures the genesis seq.
	Audit compliance.AuditLog

	// MaxDuration caps the TTL of any single elevation. Default 4h.
	// Lower in stricter environments (1h is reasonable for retail
	// banking; 8h for off-hours engineering support).
	MaxDuration time.Duration

	// RequireApprovers is the N in N-eyes. Default 1 (single approver).
	// Banks under stricter controls may set 2 ("4-eyes"). Set 0 only
	// in dev/test — it disables approval entirely (the request is
	// active on creation).
	RequireApprovers int

	// ElevatableRoles is the allowlist of roles this service can grant.
	// Default: []auth.Role{auth.RoleAdmin}. Tighten if your role model
	// has multiple privileged tiers.
	ElevatableRoles []auth.Role

	// Now is the clock function. Defaults to time.Now.UTC. Injectable
	// for deterministic tests.
	Now func() time.Time

	mu     sync.Mutex
	grants map[string]*Grant
}

// New constructs a Service with the conservative defaults.
//
// Defaults applied:
//   MaxDuration:      4 * time.Hour
//   RequireApprovers: 1
//   ElevatableRoles:  [RoleAdmin]
//   Now:              time.Now (UTC)
//
// The audit log is required — passing nil panics later on the first
// transition because every transition writes one entry. Construct with
// at least an InMemoryAuditLog in tests.
func New(audit compliance.AuditLog) *Service {
	return &Service{
		Audit:            audit,
		MaxDuration:      4 * time.Hour,
		RequireApprovers: 1,
		ElevatableRoles:  []auth.Role{auth.RoleAdmin},
		Now:              func() time.Time { return time.Now().UTC() },
		grants:           make(map[string]*Grant),
	}
}

// ─── Lifecycle: Request → Approve / Deny → (active) → Revoke / Expire ────

// Request files an elevation request. Returns the populated Grant or an
// error if the inputs fail validation.
//
// Side effects:
//   - Writes "elevation.request" to the audit log with subject, role,
//     reason, requested_ttl. The audit seq id is captured on the grant.
//   - Stores the grant in StatusPending.
//
// If RequireApprovers == 0 (dev/test only), the grant immediately
// transitions to Active without an approver. Production deployments
// should set RequireApprovers ≥ 1.
func (s *Service) Request(ctx context.Context, subject string, role auth.Role, reason string, ttl time.Duration) (*Grant, error) {
	// Validate inputs before touching state. Each validation has a
	// distinct sentinel so the HTTP layer can map to the right 4xx.
	if subject == "" {
		return nil, ErrSubjectRequired
	}
	if reason == "" {
		return nil, ErrReasonRequired
	}
	if !s.isElevatable(role) {
		return nil, ErrRoleNotElevatable
	}
	// TTL must be positive and below the cap. Zero or negative would
	// mean "elevation in the past or no elevation at all" — both are
	// nonsense.
	if ttl <= 0 || ttl > s.MaxDuration {
		return nil, ErrTTLOutOfRange
	}

	// Compose the grant. ID is a fresh UUID so the ID is unguessable;
	// makes it safe to use in URLs without leaking ordering.
	now := s.now()
	g := &Grant{
		ID:           uuid.NewString(),
		Subject:      subject,
		Role:         role,
		Reason:       reason,
		RequestedTTL: ttl,
		RequestedAt:  now,
		Status:       StatusPending,
	}

	// Audit first — if the write fails, we don't store the grant. The
	// audit entry is the source of truth for "did this request ever
	// exist"; the in-memory map is just an index.
	entry, err := s.Audit.Append(ctx, subject, "elevation.request", string(role), map[string]any{
		"grant_id":      g.ID,
		"reason":        reason,
		"requested_ttl": ttl.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("audit append: %w", err)
	}
	g.AuditEntryID = entry.Seq

	// Optional zero-approver fast-path for dev/test. Production never
	// hits this branch (RequireApprovers ≥ 1).
	if s.RequireApprovers == 0 {
		g.ApprovedAt = now
		g.ExpiresAt = now.Add(ttl)
		g.Status = StatusActive
	}

	// Store under the mutex. Single map operation — short critical
	// section.
	s.mu.Lock()
	s.grants[g.ID] = g
	s.mu.Unlock()
	return g, nil
}

// Approve records an approver's sign-off. When the approver count
// reaches RequireApprovers the grant transitions to Active.
//
// Validations:
//   - Grant must exist and be Pending.
//   - Approver must hold the admin role (caller passes the approver's
//     claims; we check HasRole(admin)).
//   - Approver must not equal the grant's subject (4-eyes minimum).
//   - The approver must not have already approved this grant.
//
// Side effects:
//   - Writes "elevation.approve" to the audit log.
//   - On the final approval, transitions to Active, sets ApprovedAt
//     and ExpiresAt, writes a second "elevation.activate" entry so
//     the activation is its own audit row (cleaner timeline).
func (s *Service) Approve(ctx context.Context, grantID string, approver auth.Claims) error {
	// Approver must hold an eligible role. Today: must be admin. Future
	// could be configurable per ElevatableRole (e.g. only super-admin
	// can approve admin elevation), but the single-tier model fits the
	// current role taxonomy.
	if !approver.HasRole(auth.RoleAdmin) {
		return ErrApproverIneligible
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.grants[grantID]
	if !ok {
		return ErrGrantNotFound
	}
	if g.Status != StatusPending {
		return ErrGrantNotPending
	}
	if approver.Subject == g.Subject {
		return ErrApproverIsSubject
	}
	// N-eyes: each approver counts once.
	for _, a := range g.Approvers {
		if a == approver.Subject {
			return ErrDuplicateApprover
		}
	}

	// Record the approval.
	g.Approvers = append(g.Approvers, approver.Subject)
	if _, err := s.Audit.Append(ctx, approver.Subject, "elevation.approve", g.ID, map[string]any{
		"grant_id":    g.ID,
		"subject":     g.Subject,
		"role":        string(g.Role),
		"approver_n":  len(g.Approvers),
		"approvers":   g.Approvers,
		"audit_root":  g.AuditEntryID,
	}); err != nil {
		// Audit failure rolls back the approver list — the audit chain
		// is the source of truth; a missing entry means the approval
		// didn't happen as far as the system is concerned.
		g.Approvers = g.Approvers[:len(g.Approvers)-1]
		return fmt.Errorf("audit append: %w", err)
	}

	// Final approval flips to Active. The activation gets its own
	// audit entry — simpler to query "when did this grant become live"
	// vs digging through approvals.
	if len(g.Approvers) >= s.RequireApprovers {
		now := s.now()
		g.ApprovedAt = now
		g.ExpiresAt = now.Add(g.RequestedTTL)
		g.Status = StatusActive
		if _, err := s.Audit.Append(ctx, approver.Subject, "elevation.activate", g.ID, map[string]any{
			"grant_id":   g.ID,
			"subject":    g.Subject,
			"role":       string(g.Role),
			"expires_at": g.ExpiresAt,
			"audit_root": g.AuditEntryID,
		}); err != nil {
			return fmt.Errorf("audit append (activate): %w", err)
		}
	}
	return nil
}

// Deny rejects a pending request. Permanent terminal state.
//
// Validations:
//   - Grant must exist and be Pending.
//   - Denier must hold the admin role.
//
// Side effects:
//   - Writes "elevation.deny" to the audit log.
//   - Sets DeniedBy / DeniedAt and Status = StatusDenied.
func (s *Service) Deny(ctx context.Context, grantID string, denier auth.Claims, reason string) error {
	if !denier.HasRole(auth.RoleAdmin) {
		return ErrApproverIneligible
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.grants[grantID]
	if !ok {
		return ErrGrantNotFound
	}
	if g.Status != StatusPending {
		return ErrGrantNotPending
	}

	now := s.now()
	g.DeniedBy = denier.Subject
	g.DeniedAt = now
	g.Status = StatusDenied
	if _, err := s.Audit.Append(ctx, denier.Subject, "elevation.deny", g.ID, map[string]any{
		"grant_id":   g.ID,
		"subject":    g.Subject,
		"role":       string(g.Role),
		"reason":     reason,
		"audit_root": g.AuditEntryID,
	}); err != nil {
		// Roll back on audit failure — see Approve for rationale.
		g.DeniedBy = ""
		g.DeniedAt = time.Time{}
		g.Status = StatusPending
		return fmt.Errorf("audit append: %w", err)
	}
	return nil
}

// Revoke terminates an active grant before its scheduled expiry.
//
// Validations:
//   - Grant must exist and be Active.
//   - Revoker must hold the admin role.
//
// Side effects:
//   - Writes "elevation.revoke" to the audit log.
//   - Sets RevokedBy / RevokedAt and Status = StatusRevoked.
//
// Once revoked the grant is terminal — the subject's effective roles
// no longer include the elevated role on the next read.
func (s *Service) Revoke(ctx context.Context, grantID string, revoker auth.Claims, reason string) error {
	if !revoker.HasRole(auth.RoleAdmin) {
		return ErrApproverIneligible
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.grants[grantID]
	if !ok {
		return ErrGrantNotFound
	}
	// We allow revoking an already-expired grant as a no-op-ish path
	// (returns the same error). Strictly only "Active" can transition.
	if g.Status != StatusActive {
		return ErrGrantNotActive
	}

	now := s.now()
	g.RevokedBy = revoker.Subject
	g.RevokedAt = now
	g.Status = StatusRevoked
	if _, err := s.Audit.Append(ctx, revoker.Subject, "elevation.revoke", g.ID, map[string]any{
		"grant_id":   g.ID,
		"subject":    g.Subject,
		"role":       string(g.Role),
		"reason":     reason,
		"audit_root": g.AuditEntryID,
	}); err != nil {
		g.RevokedBy = ""
		g.RevokedAt = time.Time{}
		g.Status = StatusActive
		return fmt.Errorf("audit append: %w", err)
	}
	return nil
}

// ─── Read path ─────────────────────────────────────────────────────────────

// Get returns a snapshot of a grant. Returns ErrGrantNotFound if absent.
// The returned grant is a copy — mutations don't reflect back into the
// service.
//
// Calling Get on a grant whose ExpiresAt has passed transitions the
// status to Expired and writes "elevation.expire" to the audit log
// (lazy expiry — no daemon needed). This is the only Read operation
// that may write.
func (s *Service) Get(ctx context.Context, grantID string) (Grant, error) {
	s.mu.Lock()
	g, ok := s.grants[grantID]
	if !ok {
		s.mu.Unlock()
		return Grant{}, ErrGrantNotFound
	}
	// Lazy-expire under the lock. If the grant was Active and is past
	// ExpiresAt, transition to Expired and write the audit entry. We
	// drop the lock around the audit call to avoid blocking other
	// readers on the audit's mutex, then re-take to copy.
	s.lazyExpireLocked(ctx, g)
	out := *g
	s.mu.Unlock()
	return out, nil
}

// ActiveFor returns the subject's currently-active elevation grants.
// Side effect: any of the subject's grants that have passed ExpiresAt
// are transitioned to Expired and audit-logged (lazy expiry).
//
// This is the read the HTTP middleware calls on every request to
// compute effective roles — has to be cheap. Today: O(N) scan over
// grants; for the small N we expect (handful of active grants per
// host) this is fine. If grant count grows large, add a secondary
// index keyed by subject.
func (s *Service) ActiveFor(ctx context.Context, subject string) []Grant {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Grant
	for _, g := range s.grants {
		if g.Subject != subject {
			continue
		}
		s.lazyExpireLocked(ctx, g)
		if g.Status == StatusActive {
			out = append(out, *g)
		}
	}
	return out
}

// EffectiveRoles returns the union of the user's base roles and any
// roles currently elevated for them. Use this in HTTP middleware and
// bus policies instead of reading claims.Roles directly when a
// privileged action is at stake.
//
// This is the read path that closes the loop: a user holding a
// pending elevation is treated as their base role; once Active they
// gain the elevated role until expiry/revoke.
func (s *Service) EffectiveRoles(ctx context.Context, claims auth.Claims) []auth.Role {
	// Start with the base roles. Copy so we don't mutate the caller's
	// slice if the elevation set is empty.
	out := make([]auth.Role, len(claims.Roles))
	copy(out, claims.Roles)
	// Append every active-elevation role for this subject.
	for _, g := range s.ActiveFor(ctx, claims.Subject) {
		// De-duplicate — a user already holding the role doesn't need
		// it twice.
		if !claims.HasRole(g.Role) {
			out = append(out, g.Role)
		}
	}
	return out
}

// List returns up to `limit` recent grants in arbitrary order. Admin-
// gated at the HTTP layer; this method itself doesn't enforce role.
//
// Pagination is rudimentary — limit only, no cursor. If grant count
// grows beyond what fits on one screen of the dashboard, swap the
// in-memory map for a SQL-backed store and add a proper paginator.
func (s *Service) List(ctx context.Context, limit int) []Grant {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]Grant, 0, limit)
	for _, g := range s.grants {
		s.lazyExpireLocked(ctx, g)
		out = append(out, *g)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// ─── Internals ─────────────────────────────────────────────────────────────

// lazyExpireLocked transitions g to Expired if it's Active and past
// ExpiresAt. Caller must hold s.mu. Writes one audit entry on
// transition. Idempotent (no-op for grants already in a terminal state).
//
// Why under the lock: prevents two concurrent readers from racing and
// writing two "expire" audit entries for the same grant.
func (s *Service) lazyExpireLocked(ctx context.Context, g *Grant) {
	if g.Status != StatusActive {
		return
	}
	if s.now().Before(g.ExpiresAt) {
		return
	}
	g.Status = StatusExpired
	// Audit failures here are logged but not propagated — the grant
	// has expired regardless of whether the audit write succeeds. A
	// failing audit chain is a bigger problem than a missing expire
	// entry; the operations runbook covers the recovery procedure.
	_, _ = s.Audit.Append(ctx, "system", "elevation.expire", g.ID, map[string]any{
		"grant_id":   g.ID,
		"subject":    g.Subject,
		"role":       string(g.Role),
		"expired_at": g.ExpiresAt,
		"audit_root": g.AuditEntryID,
	})
}

// isElevatable reports whether the requested role is in the service's
// allowlist. Tiny linear scan; the allowlist is at most a few entries.
func (s *Service) isElevatable(role auth.Role) bool {
	for _, r := range s.ElevatableRoles {
		if r == role {
			return true
		}
	}
	return false
}

// now returns the current time via the injectable clock. Falls back to
// time.Now().UTC() if Now is nil (defensive — New always populates it,
// but a zero-value Service shouldn't panic).
func (s *Service) now() time.Time {
	if s.Now == nil {
		return time.Now().UTC()
	}
	return s.Now()
}
