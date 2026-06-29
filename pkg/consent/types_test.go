package consent

import (
	"testing"
	"time"
)

func TestConsentRecordIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{
			name:      "zero expiration (never expires)",
			expiresAt: time.Time{},
			now:       time.Now(),
			want:      false,
		},
		{
			name:      "expires in the future",
			expiresAt: time.Now().Add(1 * time.Hour),
			now:       time.Now(),
			want:      false,
		},
		{
			name:      "expired in the past",
			expiresAt: time.Now().Add(-1 * time.Hour),
			now:       time.Now(),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &ConsentRecord{ExpiresAt: tt.expiresAt}
			if got := cr.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsentRecordAllowsAction(t *testing.T) {
	tests := []struct {
		name        string
		permissions PermissionLevel
		action      ActionType
		want        bool
	}{
		{
			name:        "admin allows read",
			permissions: PermAdmin,
			action:      ActionRead,
			want:        true,
		},
		{
			name:        "admin allows write",
			permissions: PermAdmin,
			action:      ActionWrite,
			want:        true,
		},
		{
			name:        "admin allows delete",
			permissions: PermAdmin,
			action:      ActionDelete,
			want:        true,
		},
		{
			name:        "admin allows admin",
			permissions: PermAdmin,
			action:      ActionAdmin,
			want:        true,
		},
		{
			name:        "read allows read",
			permissions: PermRead,
			action:      ActionRead,
			want:        true,
		},
		{
			name:        "read denies write",
			permissions: PermRead,
			action:      ActionWrite,
			want:        false,
		},
		{
			name:        "read_write allows read",
			permissions: PermReadWrite,
			action:      ActionRead,
			want:        true,
		},
		{
			name:        "read_write allows write",
			permissions: PermReadWrite,
			action:      ActionWrite,
			want:        true,
		},
		{
			name:        "read_write denies delete",
			permissions: PermReadWrite,
			action:      ActionDelete,
			want:        false,
		},
		{
			name:        "none denies everything",
			permissions: PermNone,
			action:      ActionRead,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &ConsentRecord{Permissions: tt.permissions}
			if got := cr.AllowsAction(tt.action); got != tt.want {
				t.Errorf("AllowsAction(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestNewConsentChecker(t *testing.T) {
	matrix := NewInMemoryConsentMatrix()
	log := NewInMemoryAccessLog()
	checker := NewConsentChecker(matrix, log)

	// Grant permission
	_, err := matrix.Grant("agent-a", "database", PermReadWrite, 24*time.Hour, "test", "admin")
	if err != nil {
		t.Fatalf("Grant failed: %v", err)
	}

	// Check with valid action
	allowed, reasonCode, err := checker("agent-a", "database", ActionRead, "trace-123")
	if err != nil {
		t.Fatalf("checker returned error: %v", err)
	}
	if !allowed {
		t.Errorf("check returned allowed=false, want true")
	}
	if reasonCode != "" {
		t.Errorf("check returned reasonCode=%q, want empty", reasonCode)
	}

	// Verify decision was logged
	decisions, err := log.Query("agent-a", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(decisions) != 1 {
		t.Errorf("expected 1 decision in log, got %d", len(decisions))
	}
	if decisions[0].Result != "allowed" {
		t.Errorf("logged decision result=%q, want allowed", decisions[0].Result)
	}

	// Check without permission
	allowed, reasonCode, err = checker("agent-b", "database", ActionRead, "trace-124")
	if err != nil {
		t.Fatalf("checker returned error: %v", err)
	}
	if allowed {
		t.Errorf("check returned allowed=true, want false")
	}
	if reasonCode != "no_grant" {
		t.Errorf("check returned reasonCode=%q, want no_grant", reasonCode)
	}

	// Verify denial was logged
	decisions, _ = log.Query("agent-b", time.Time{}, time.Time{})
	if len(decisions) != 1 {
		t.Errorf("expected 1 decision for agent-b, got %d", len(decisions))
	}
	if decisions[0].Result != "denied" {
		t.Errorf("logged decision result=%q, want denied", decisions[0].Result)
	}
}
