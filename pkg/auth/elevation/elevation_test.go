// elevation_test.go — lifecycle contract tests for the elevation Service.
//
// ─── What's pinned here ────────────────────────────────────────────────────
//
// Every error path + every state transition is exercised at least once:
//
//   Request:
//     - Empty subject  → ErrSubjectRequired
//     - Empty reason   → ErrReasonRequired
//     - Non-allowlist role → ErrRoleNotElevatable
//     - TTL ≤ 0 or > MaxDuration → ErrTTLOutOfRange
//     - Happy path → grant stored, audit entry written, status Pending
//   Approve:
//     - Approver not admin → ErrApproverIneligible
//     - Self-approval     → ErrApproverIsSubject
//     - Wrong status      → ErrGrantNotPending
//     - Unknown grant id  → ErrGrantNotFound
//     - Duplicate approver → ErrDuplicateApprover
//     - Single-approver happy path → status Active, ExpiresAt set
//     - 4-eyes (RequireApprovers=2) → pending until 2nd approver
//   Deny:
//     - Non-admin denier → ErrApproverIneligible
//     - Already approved → ErrGrantNotPending
//     - Happy path → status Denied
//   Revoke:
//     - Non-admin revoker → ErrApproverIneligible
//     - Not Active        → ErrGrantNotActive
//     - Happy path → status Revoked
//   Lazy expiry:
//     - Active past ExpiresAt → Expired on next read
//     - Audit entry written exactly once per grant (idempotent)
//   EffectiveRoles:
//     - Base roles preserved
//     - Active grant adds elevated role
//     - Expired grant does NOT add
//     - De-dup if user already holds the role
//   ActiveFor / List:
//     - ActiveFor returns only active grants for the subject
//     - List respects limit
package elevation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
)

// ─── Helpers ───────────────────────────────────────────────────────────────

// newSvc builds a Service against an in-memory audit log with a frozen
// clock pointer. Tests advance the clock by mutating *now.
//
// Returns the service, the audit log (so tests can inspect entries),
// and a pointer to the clock so tests can advance time deterministically.
func newSvc(t *testing.T) (*Service, *compliance.InMemoryAuditLog, *time.Time) {
	t.Helper()
	audit := compliance.NewInMemoryAuditLog()
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	svc := New(audit)
	// Frozen clock — tests advance by reassigning *now via the closure.
	svc.Now = func() time.Time { return now }
	return svc, audit, &now
}

// adminClaims and userClaims help readability — every test needs them.
func adminClaims(sub string) auth.Claims {
	return auth.Claims{Subject: sub, Roles: []auth.Role{auth.RoleAdmin}}
}
func userClaims(sub string) auth.Claims {
	return auth.Claims{Subject: sub, Roles: []auth.Role{auth.RoleUser}}
}

// ─── Request ──────────────────────────────────────────────────────────────

func TestRequest_RejectsEmptySubject(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Request(context.Background(), "", auth.RoleAdmin, "investigate", time.Hour)
	if !errors.Is(err, ErrSubjectRequired) {
		t.Errorf("want ErrSubjectRequired; got %v", err)
	}
}

func TestRequest_RejectsEmptyReason(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Request(context.Background(), "alice", auth.RoleAdmin, "", time.Hour)
	if !errors.Is(err, ErrReasonRequired) {
		t.Errorf("want ErrReasonRequired; got %v", err)
	}
}

// TestRequest_RejectsNonElevatableRole pins the allowlist. RoleUser is
// not in the default ElevatableRoles set; a request for it must fail.
func TestRequest_RejectsNonElevatableRole(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.Request(context.Background(), "alice", auth.RoleUser, "any", time.Hour)
	if !errors.Is(err, ErrRoleNotElevatable) {
		t.Errorf("want ErrRoleNotElevatable; got %v", err)
	}
}

func TestRequest_RejectsTTLOutOfRange(t *testing.T) {
	svc, _, _ := newSvc(t)
	// Zero TTL → reject.
	if _, err := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", 0); !errors.Is(err, ErrTTLOutOfRange) {
		t.Errorf("want ErrTTLOutOfRange for zero TTL; got %v", err)
	}
	// Negative TTL → reject.
	if _, err := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", -time.Hour); !errors.Is(err, ErrTTLOutOfRange) {
		t.Errorf("want ErrTTLOutOfRange for negative TTL; got %v", err)
	}
	// TTL above MaxDuration (default 4h) → reject.
	if _, err := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", 5*time.Hour); !errors.Is(err, ErrTTLOutOfRange) {
		t.Errorf("want ErrTTLOutOfRange for TTL > MaxDuration; got %v", err)
	}
}

// TestRequest_HappyPathWritesAudit is the positive path: a valid
// request creates a Pending grant and emits one audit entry whose Seq
// matches the grant's AuditEntryID.
func TestRequest_HappyPathWritesAudit(t *testing.T) {
	svc, audit, _ := newSvc(t)
	g, err := svc.Request(context.Background(), "alice", auth.RoleAdmin, "investigate-12345", time.Hour)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if g.Status != StatusPending {
		t.Errorf("status should be Pending; got %s", g.Status)
	}
	if g.AuditEntryID == 0 {
		t.Errorf("AuditEntryID should be populated")
	}
	entries, _ := audit.List(context.Background())
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry; got %d", len(entries))
	}
	if entries[0].Action != "elevation.request" {
		t.Errorf("audit action should be elevation.request; got %s", entries[0].Action)
	}
}

// ─── Approve ──────────────────────────────────────────────────────────────

// TestApprove_RejectsNonAdminApprover ensures a regular user cannot
// approve. Security-critical — without this check, any user could
// promote themselves by getting another user to "approve."
func TestApprove_RejectsNonAdminApprover(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	if err := svc.Approve(context.Background(), g.ID, userClaims("bob")); !errors.Is(err, ErrApproverIneligible) {
		t.Errorf("want ErrApproverIneligible; got %v", err)
	}
}

// TestApprove_RejectsSelfApproval enforces the 4-eyes minimum. A user
// cannot approve their own request, even if they hold admin.
func TestApprove_RejectsSelfApproval(t *testing.T) {
	svc, _, _ := newSvc(t)
	// alice is both the subject (in the grant) AND happens to be an admin.
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	if err := svc.Approve(context.Background(), g.ID, adminClaims("alice")); !errors.Is(err, ErrApproverIsSubject) {
		t.Errorf("want ErrApproverIsSubject; got %v", err)
	}
}

func TestApprove_UnknownGrant(t *testing.T) {
	svc, _, _ := newSvc(t)
	if err := svc.Approve(context.Background(), "does-not-exist", adminClaims("bob")); !errors.Is(err, ErrGrantNotFound) {
		t.Errorf("want ErrGrantNotFound; got %v", err)
	}
}

// TestApprove_SingleApproverActivates is the happy path under the
// default RequireApprovers=1: one admin approves and the grant goes
// Active with ExpiresAt = now + TTL.
func TestApprove_SingleApproverActivates(t *testing.T) {
	svc, audit, now := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", 30*time.Minute)
	if err := svc.Approve(context.Background(), g.ID, adminClaims("bob")); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	got, _ := svc.Get(context.Background(), g.ID)
	if got.Status != StatusActive {
		t.Errorf("status should be Active; got %s", got.Status)
	}
	wantExpiry := now.Add(30 * time.Minute)
	if !got.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt mismatch; got %v, want %v", got.ExpiresAt, wantExpiry)
	}
	// Two audit entries: request + activate (approve writes its own
	// entry too — that's three; verify).
	entries, _ := audit.List(context.Background())
	if len(entries) != 3 {
		t.Errorf("want 3 audit entries (request + approve + activate); got %d", len(entries))
	}
}

// TestApprove_NEyesPending verifies the 4-eyes (RequireApprovers=2)
// path: first approval leaves Pending; second approval activates.
func TestApprove_NEyesPending(t *testing.T) {
	svc, _, _ := newSvc(t)
	svc.RequireApprovers = 2
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	if err := svc.Approve(context.Background(), g.ID, adminClaims("bob")); err != nil {
		t.Fatalf("first approval: %v", err)
	}
	got, _ := svc.Get(context.Background(), g.ID)
	if got.Status != StatusPending {
		t.Errorf("after 1 of 2 approvals, status should still be Pending; got %s", got.Status)
	}
	// Second approver activates it.
	if err := svc.Approve(context.Background(), g.ID, adminClaims("carol")); err != nil {
		t.Fatalf("second approval: %v", err)
	}
	got, _ = svc.Get(context.Background(), g.ID)
	if got.Status != StatusActive {
		t.Errorf("after 2 of 2 approvals, status should be Active; got %s", got.Status)
	}
	if len(got.Approvers) != 2 {
		t.Errorf("expected 2 approvers; got %v", got.Approvers)
	}
}

// TestApprove_DuplicateApprover prevents N-eyes bypass via "the same
// admin approves twice and counts as two."
func TestApprove_DuplicateApprover(t *testing.T) {
	svc, _, _ := newSvc(t)
	svc.RequireApprovers = 2
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	if err := svc.Approve(context.Background(), g.ID, adminClaims("bob")); err != nil {
		t.Fatal(err)
	}
	if err := svc.Approve(context.Background(), g.ID, adminClaims("bob")); !errors.Is(err, ErrDuplicateApprover) {
		t.Errorf("want ErrDuplicateApprover on second approval from same admin; got %v", err)
	}
}

// TestApprove_NotPending checks the status guard. An already-active
// grant cannot be approved again.
func TestApprove_NotPending(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	if err := svc.Approve(context.Background(), g.ID, adminClaims("bob")); err != nil {
		t.Fatal(err)
	}
	if err := svc.Approve(context.Background(), g.ID, adminClaims("carol")); !errors.Is(err, ErrGrantNotPending) {
		t.Errorf("want ErrGrantNotPending; got %v", err)
	}
}

// ─── Deny ──────────────────────────────────────────────────────────────────

func TestDeny_RejectsNonAdminDenier(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	if err := svc.Deny(context.Background(), g.ID, userClaims("bob"), "no"); !errors.Is(err, ErrApproverIneligible) {
		t.Errorf("want ErrApproverIneligible; got %v", err)
	}
}

func TestDeny_HappyPath(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	if err := svc.Deny(context.Background(), g.ID, adminClaims("bob"), "insufficient justification"); err != nil {
		t.Fatalf("Deny: %v", err)
	}
	got, _ := svc.Get(context.Background(), g.ID)
	if got.Status != StatusDenied {
		t.Errorf("status should be Denied; got %s", got.Status)
	}
	if got.DeniedBy != "bob" {
		t.Errorf("DeniedBy should be bob; got %s", got.DeniedBy)
	}
}

// TestDeny_NotPending — cannot deny something that's already active.
func TestDeny_NotPending(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	_ = svc.Approve(context.Background(), g.ID, adminClaims("bob"))
	if err := svc.Deny(context.Background(), g.ID, adminClaims("carol"), "too late"); !errors.Is(err, ErrGrantNotPending) {
		t.Errorf("want ErrGrantNotPending; got %v", err)
	}
}

// ─── Revoke ────────────────────────────────────────────────────────────────

func TestRevoke_RejectsNonAdminRevoker(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	_ = svc.Approve(context.Background(), g.ID, adminClaims("bob"))
	if err := svc.Revoke(context.Background(), g.ID, userClaims("eve"), "no"); !errors.Is(err, ErrApproverIneligible) {
		t.Errorf("want ErrApproverIneligible; got %v", err)
	}
}

func TestRevoke_HappyPath(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	_ = svc.Approve(context.Background(), g.ID, adminClaims("bob"))
	if err := svc.Revoke(context.Background(), g.ID, adminClaims("carol"), "task completed"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	got, _ := svc.Get(context.Background(), g.ID)
	if got.Status != StatusRevoked {
		t.Errorf("status should be Revoked; got %s", got.Status)
	}
	if got.RevokedBy != "carol" {
		t.Errorf("RevokedBy should be carol; got %s", got.RevokedBy)
	}
}

// TestRevoke_NotActive — cannot revoke a non-active grant.
func TestRevoke_NotActive(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	// Still Pending.
	if err := svc.Revoke(context.Background(), g.ID, adminClaims("bob"), "x"); !errors.Is(err, ErrGrantNotActive) {
		t.Errorf("want ErrGrantNotActive on Pending grant; got %v", err)
	}
}

// ─── Lazy expiry ──────────────────────────────────────────────────────────

// TestLazyExpiry_TransitionsOnRead — the key invariant of the no-daemon
// design. Once now > ExpiresAt, the next Get/List/ActiveFor call must
// flip Status to Expired and emit "elevation.expire" exactly once.
func TestLazyExpiry_TransitionsOnRead(t *testing.T) {
	svc, audit, now := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", 10*time.Minute)
	_ = svc.Approve(context.Background(), g.ID, adminClaims("bob"))

	// Move past expiry.
	*now = now.Add(11 * time.Minute)

	got, _ := svc.Get(context.Background(), g.ID)
	if got.Status != StatusExpired {
		t.Errorf("status should be Expired after now > ExpiresAt; got %s", got.Status)
	}
	// Inspect audit — exactly one elevation.expire entry.
	entries, _ := audit.List(context.Background())
	expireCount := 0
	for _, e := range entries {
		if e.Action == "elevation.expire" {
			expireCount++
		}
	}
	if expireCount != 1 {
		t.Errorf("expected exactly 1 elevation.expire audit entry; got %d", expireCount)
	}

	// Calling Get again should NOT emit another expire entry (idempotent).
	_, _ = svc.Get(context.Background(), g.ID)
	entries, _ = audit.List(context.Background())
	expireCount = 0
	for _, e := range entries {
		if e.Action == "elevation.expire" {
			expireCount++
		}
	}
	if expireCount != 1 {
		t.Errorf("expire should be idempotent; got %d entries after second read", expireCount)
	}
}

// ─── EffectiveRoles ──────────────────────────────────────────────────────

// TestEffectiveRoles_NoElevation — base roles preserved as-is when the
// subject has no active grants.
func TestEffectiveRoles_NoElevation(t *testing.T) {
	svc, _, _ := newSvc(t)
	roles := svc.EffectiveRoles(context.Background(), userClaims("alice"))
	if len(roles) != 1 || roles[0] != auth.RoleUser {
		t.Errorf("expected [user]; got %v", roles)
	}
}

// TestEffectiveRoles_ActiveGrantUnions — an active grant adds the
// elevated role to the user's effective set.
func TestEffectiveRoles_ActiveGrantUnions(t *testing.T) {
	svc, _, _ := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	_ = svc.Approve(context.Background(), g.ID, adminClaims("bob"))

	roles := svc.EffectiveRoles(context.Background(), userClaims("alice"))
	hasUser, hasAdmin := false, false
	for _, r := range roles {
		if r == auth.RoleUser {
			hasUser = true
		}
		if r == auth.RoleAdmin {
			hasAdmin = true
		}
	}
	if !hasUser || !hasAdmin {
		t.Errorf("expected both user and admin; got %v", roles)
	}
}

// TestEffectiveRoles_ExpiredDoesNotAdd — an expired grant must not
// continue to grant the elevated role.
func TestEffectiveRoles_ExpiredDoesNotAdd(t *testing.T) {
	svc, _, now := newSvc(t)
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", 5*time.Minute)
	_ = svc.Approve(context.Background(), g.ID, adminClaims("bob"))
	*now = now.Add(10 * time.Minute) // past expiry

	roles := svc.EffectiveRoles(context.Background(), userClaims("alice"))
	for _, r := range roles {
		if r == auth.RoleAdmin {
			t.Errorf("admin role should not be effective after expiry; got %v", roles)
		}
	}
}

// TestEffectiveRoles_DeDup — a user already holding the role doesn't
// get it doubled by an active elevation.
func TestEffectiveRoles_DeDup(t *testing.T) {
	svc, _, _ := newSvc(t)
	// alice is already admin and also has an active grant for admin
	// (contrived but possible — alice might be admin via base role AND
	// have requested an elevated session).
	g, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	_ = svc.Approve(context.Background(), g.ID, adminClaims("bob"))

	roles := svc.EffectiveRoles(context.Background(), adminClaims("alice"))
	count := 0
	for _, r := range roles {
		if r == auth.RoleAdmin {
			count++
		}
	}
	if count != 1 {
		t.Errorf("admin role should appear exactly once; got %d in %v", count, roles)
	}
}

// ─── ActiveFor / List ─────────────────────────────────────────────────────

// TestActiveFor_ScopesToSubject — bob's grant doesn't appear in alice's
// active list and vice versa.
func TestActiveFor_ScopesToSubject(t *testing.T) {
	svc, _, _ := newSvc(t)
	gA, _ := svc.Request(context.Background(), "alice", auth.RoleAdmin, "a", time.Hour)
	gB, _ := svc.Request(context.Background(), "bob", auth.RoleAdmin, "b", time.Hour)
	_ = svc.Approve(context.Background(), gA.ID, adminClaims("carol"))
	_ = svc.Approve(context.Background(), gB.ID, adminClaims("carol"))

	aliceActive := svc.ActiveFor(context.Background(), "alice")
	if len(aliceActive) != 1 || aliceActive[0].Subject != "alice" {
		t.Errorf("alice's active list should be just her grant; got %v", aliceActive)
	}
	bobActive := svc.ActiveFor(context.Background(), "bob")
	if len(bobActive) != 1 || bobActive[0].Subject != "bob" {
		t.Errorf("bob's active list should be just his grant; got %v", bobActive)
	}
}

// TestList_RespectsLimit — list pagination cap.
func TestList_RespectsLimit(t *testing.T) {
	svc, _, _ := newSvc(t)
	// Create three grants.
	for i := 0; i < 3; i++ {
		_, _ = svc.Request(context.Background(), "alice", auth.RoleAdmin, "x", time.Hour)
	}
	out := svc.List(context.Background(), 2)
	if len(out) != 2 {
		t.Errorf("List(limit=2) should return 2; got %d", len(out))
	}
}
