package consent

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestInMemoryConsentMatrixGrant(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant a basic consent
	id, err := m.Grant("user1", "database", PermRead, 24*time.Hour, "testing", "admin")
	if err != nil {
		t.Fatalf("Grant failed: %v", err)
	}
	if id == "" {
		t.Errorf("Grant returned empty ID")
	}

	// Verify it can be retrieved
	records, err := m.List("user1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
	if records[0].Permissions != PermRead {
		t.Errorf("permissions=%v, want PermRead", records[0].Permissions)
	}
}

func TestInMemoryConsentMatrixGrantInvalid(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	tests := []struct {
		name         string
		userID       string
		resourceType string
		perms        PermissionLevel
		wantErr      bool
	}{
		{"empty userID", "", "resource", PermRead, true},
		{"empty resourceType", "user", "", PermRead, true},
		{"PermNone", "user", "resource", PermNone, true},
		{"valid", "user", "resource", PermRead, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.Grant(tt.userID, tt.resourceType, tt.perms, time.Hour, "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("Grant wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestInMemoryConsentMatrixGrantUpdate(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant initial consent
	id1, _ := m.Grant("user1", "db", PermRead, time.Hour, "v1", "admin")

	// Update the same consent
	id2, _ := m.Grant("user1", "db", PermReadWrite, 2*time.Hour, "v2", "admin")

	// IDs should be different (new grant)
	if id1 == id2 {
		t.Errorf("expected different IDs on update, got same: %s", id1)
	}

	// Should only have one record (the new one)
	records, _ := m.List("user1")
	if len(records) != 1 {
		t.Errorf("expected 1 record after update, got %d", len(records))
	}

	// New record should have updated permissions
	if records[0].Permissions != PermReadWrite {
		t.Errorf("permissions=%v, want PermReadWrite", records[0].Permissions)
	}
}

func TestInMemoryConsentMatrixCheck(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant read permission
	m.Grant("user1", "resource1", PermRead, time.Hour, "test", "admin")

	tests := []struct {
		name         string
		userID       string
		resourceType string
		action       ActionType
		wantAllowed  bool
	}{
		{"matching grant, read action", "user1", "resource1", ActionRead, true},
		{"matching grant, write action denied", "user1", "resource1", ActionWrite, false},
		{"missing grant", "user2", "resource1", ActionRead, false},
		{"different resource", "user1", "resource2", ActionRead, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _, err := m.Check(tt.userID, tt.resourceType, tt.action)
			if err != nil {
				t.Fatalf("Check failed: %v", err)
			}
			if allowed != tt.wantAllowed {
				t.Errorf("allowed=%v, want %v", allowed, tt.wantAllowed)
			}
		})
	}
}

func TestInMemoryConsentMatrixCheckExpiration(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant with very short TTL
	m.Grant("user1", "resource", PermRead, 1*time.Millisecond, "test", "admin")

	// Check immediately (should pass)
	allowed, _, _ := m.Check("user1", "resource", ActionRead)
	if !allowed {
		t.Errorf("check immediately after grant: allowed=false, want true")
	}

	// Wait for expiration
	time.Sleep(2 * time.Millisecond)

	// Check after expiration (should fail)
	allowed, _, _ = m.Check("user1", "resource", ActionRead)
	if allowed {
		t.Errorf("check after expiration: allowed=true, want false")
	}
}

func TestInMemoryConsentMatrixRevoke(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant and then revoke
	m.Grant("user1", "resource1", PermRead, time.Hour, "test", "admin")

	err := m.Revoke("user1", "resource1")
	if err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	// Verify it's gone
	records, _ := m.List("user1")
	if len(records) != 0 {
		t.Errorf("expected 0 records after revoke, got %d", len(records))
	}

	// Check should now return false
	allowed, _, _ := m.Check("user1", "resource1", ActionRead)
	if allowed {
		t.Errorf("check after revoke: allowed=true, want false")
	}
}

func TestInMemoryConsentMatrixRevokeNotFound(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	err := m.Revoke("user1", "resource1")
	if err != ErrNotFound {
		t.Errorf("Revoke nonexistent grant: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryConsentMatrixList(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant multiple consents for same user
	m.Grant("user1", "resource1", PermRead, time.Hour, "test1", "admin")
	m.Grant("user1", "resource2", PermReadWrite, 2*time.Hour, "test2", "admin")
	m.Grant("user2", "resource1", PermAdmin, time.Hour, "test3", "admin")

	// List user1
	records, err := m.List("user1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("List user1: got %d records, want 2", len(records))
	}

	// List user2
	records, _ = m.List("user2")
	if len(records) != 1 {
		t.Errorf("List user2: got %d records, want 1", len(records))
	}

	// List nonexistent user
	records, _ = m.List("user3")
	if len(records) != 0 {
		t.Errorf("List nonexistent user: got %d records, want 0", len(records))
	}
}

func TestInMemoryConsentMatrixListByResource(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant multiple consents
	m.Grant("user1", "database", PermRead, time.Hour, "test1", "admin")
	m.Grant("user2", "database", PermReadWrite, time.Hour, "test2", "admin")
	m.Grant("user3", "api", PermAdmin, time.Hour, "test3", "admin")

	// List by resource
	records, err := m.ListByResource("database")
	if err != nil {
		t.Fatalf("ListByResource failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("ListByResource: got %d records, want 2", len(records))
	}

	records, _ = m.ListByResource("api")
	if len(records) != 1 {
		t.Errorf("ListByResource api: got %d records, want 1", len(records))
	}
}

func TestInMemoryConsentMatrixConcurrentAccess(t *testing.T) {
	m := NewInMemoryConsentMatrix()
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	numGoroutines := 100
	operationsPerGoroutine := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := fmt.Sprintf("user%d", id%10)
			resourceType := fmt.Sprintf("resource%d", id%5)

			for j := 0; j < operationsPerGoroutine; j++ {
				// Mix of operations
				switch j % 4 {
				case 0:
					_, err := m.Grant(userID, resourceType, PermRead, time.Hour, "test", "admin")
					if err == nil {
						atomic.AddInt32(&successCount, 1)
					} else {
						atomic.AddInt32(&errorCount, 1)
					}
				case 1:
					allowed, _, err := m.Check(userID, resourceType, ActionRead)
					if err == nil && !allowed {
						atomic.AddInt32(&successCount, 1)
					}
				case 2:
					_, _ = m.List(userID)
					atomic.AddInt32(&successCount, 1)
				case 3:
					_ = m.Revoke(userID, resourceType)
					atomic.AddInt32(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if errorCount > 0 {
		t.Errorf("concurrent operations: %d errors", errorCount)
	}
	if successCount == 0 {
		t.Errorf("concurrent operations: no successes recorded")
	}
}

func TestInMemoryConsentMatrixConcurrentCheckExpiration(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant with short TTL
	m.Grant("user1", "resource", PermRead, 50*time.Millisecond, "test", "admin")

	var wg sync.WaitGroup
	var passedBefore, passedAfter int32

	// Check in parallel
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, _ := m.Check("user1", "resource", ActionRead)
			if allowed {
				atomic.AddInt32(&passedBefore, 1)
			}
		}()
	}

	wg.Wait()

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Check again in parallel
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, _ := m.Check("user1", "resource", ActionRead)
			if allowed {
				atomic.AddInt32(&passedAfter, 1)
			}
		}()
	}

	wg.Wait()

	if passedBefore == 0 {
		t.Errorf("before expiration: expected some passes, got 0")
	}
	if passedAfter > 0 {
		t.Errorf("after expiration: expected 0 passes, got %d", passedAfter)
	}
}

func TestInMemoryConsentMatrixNoExpiration(t *testing.T) {
	m := NewInMemoryConsentMatrix()

	// Grant with 0 TTL (no expiration)
	m.Grant("user1", "resource", PermRead, 0, "test", "admin")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Should still be valid
	allowed, _, _ := m.Check("user1", "resource", ActionRead)
	if !allowed {
		t.Errorf("check with no expiration: allowed=false, want true")
	}
}
