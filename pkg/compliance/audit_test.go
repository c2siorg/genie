package compliance

import (
	"context"
	"testing"
)

func TestAudit_ChainAndVerify(t *testing.T) {
	a := NewInMemoryAuditLog()
	_, _ = a.Append(context.Background(), "u-1", "consent.grant", "portfolio", nil)
	_, _ = a.Append(context.Background(), "u-1", "ask", "doc-1", map[string]any{"trace": "tr-1"})
	if err := a.Verify(context.Background()); err != nil {
		t.Fatalf("expected verify ok, got %v", err)
	}
	// Tamper.
	entries, _ := a.List(context.Background())
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// We can't directly mutate the in-memory entries because List returns a
	// copy; instead append using a fake "Append" by reaching into the slice.
	// Use a separate test that mutates the internal copy.
}

func TestAudit_DetectsTamper(t *testing.T) {
	a := NewInMemoryAuditLog()
	_, _ = a.Append(context.Background(), "u-1", "x", "", nil)
	_, _ = a.Append(context.Background(), "u-1", "y", "", nil)

	// Reach in and flip an action.
	a.mu.Lock()
	a.entries[0].Action = "MALICIOUSLY_CHANGED"
	a.mu.Unlock()

	if err := a.Verify(context.Background()); err == nil {
		t.Fatal("expected verify to detect tamper")
	}
}

func TestConsentLedger_GrantRevoke(t *testing.T) {
	l := NewInMemoryLedger()
	if got, _ := l.HasActive(context.Background(), "u", CategoryPortfolio); got {
		t.Fatal("should not be active before grant")
	}
	_, _ = l.Grant(context.Background(), "u", CategoryPortfolio, "show")
	if got, _ := l.HasActive(context.Background(), "u", CategoryPortfolio); !got {
		t.Fatal("should be active after grant")
	}
	_ = l.Revoke(context.Background(), "u", CategoryPortfolio)
	if got, _ := l.HasActive(context.Background(), "u", CategoryPortfolio); got {
		t.Fatal("should not be active after revoke")
	}
}
