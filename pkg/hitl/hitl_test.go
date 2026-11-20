package hitl_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/hitl"
)

// ─── Sentinels ─────────────────────────────────────────────────────────────

func TestAllow_AlwaysApproves(t *testing.T) {
	ok, err := hitl.Allow.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "delete_file"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("Allow should always return true")
	}
}

func TestDeny_AlwaysDenies(t *testing.T) {
	ok, err := hitl.Deny.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "read_file"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("Deny should always return false")
	}
}

// ─── CLIApprover ───────────────────────────────────────────────────────────

func TestCLIApprover_Approve(t *testing.T) {
	in := strings.NewReader("y\n")
	out := &strings.Builder{}
	a := hitl.NewCLIApprover(in, out)

	ok, err := a.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "send_email", Args: map[string]any{"to": "test@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected approval for 'y' input")
	}
	if !strings.Contains(out.String(), "send_email") {
		t.Error("prompt should mention the tool name")
	}
}

func TestCLIApprover_Deny_Empty(t *testing.T) {
	in := strings.NewReader("\n") // empty = deny
	a := hitl.NewCLIApprover(in, &strings.Builder{})
	ok, err := a.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "delete_file"})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("empty input should deny")
	}
}

func TestCLIApprover_Deny_No(t *testing.T) {
	in := strings.NewReader("no\n")
	a := hitl.NewCLIApprover(in, &strings.Builder{})
	ok, _ := a.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "shell_exec"})
	if ok {
		t.Error("'no' should deny")
	}
}

func TestCLIApprover_ContextCancelled(t *testing.T) {
	// Create a reader that never produces data (simulate waiting).
	pr, _ := newBlockingPipe()
	a := hitl.NewCLIApprover(pr, &strings.Builder{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	ok, err := a.RequestApproval(ctx, hitl.ApprovalRequest{ToolName: "write_file"})
	if ok {
		t.Error("cancelled context should deny")
	}
	if err == nil {
		t.Error("expected a context error")
	}
}

// ─── PolicyApprover ────────────────────────────────────────────────────────

func TestPolicyApprover_AllowRule(t *testing.T) {
	pa := hitl.NewPolicyApprover(hitl.Deny, // inner = deny, so any escalation fails
		hitl.Rule{ToolPattern: "read_*", Action: hitl.RuleAllow},
	)
	ok, err := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "read_file"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("read_* should auto-approve")
	}
}

func TestPolicyApprover_DenyRule(t *testing.T) {
	pa := hitl.NewPolicyApprover(hitl.Allow, // inner = allow, so any escalation would pass
		hitl.Rule{ToolPattern: "delete_*", Action: hitl.RuleDeny},
	)
	ok, _ := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "delete_user"})
	if ok {
		t.Error("delete_* should auto-deny")
	}
}

func TestPolicyApprover_AskHumanRule_Approved(t *testing.T) {
	pa := hitl.NewPolicyApprover(hitl.Allow, // human says yes
		hitl.Rule{ToolPattern: "shell_*", Action: hitl.RuleAskHuman},
	)
	ok, err := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "shell_exec"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("shell_exec with Allow inner should be approved")
	}
}

func TestPolicyApprover_AskHumanRule_Denied(t *testing.T) {
	pa := hitl.NewPolicyApprover(hitl.Deny, // human says no
		hitl.Rule{ToolPattern: "shell_*", Action: hitl.RuleAskHuman},
	)
	ok, _ := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "shell_exec"})
	if ok {
		t.Error("shell_exec with Deny inner should be denied")
	}
}

func TestPolicyApprover_NoMatchEscalates(t *testing.T) {
	// No rule matches "unknown_tool" → escalates to inner (Allow).
	pa := hitl.NewPolicyApprover(hitl.Allow,
		hitl.Rule{ToolPattern: "read_*", Action: hitl.RuleAllow},
	)
	ok, _ := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "unknown_tool"})
	if !ok {
		t.Error("no matching rule should escalate to inner (Allow)")
	}
}

func TestPolicyApprover_ArgPattern(t *testing.T) {
	// Deny shell commands containing 'rm'.
	pa := hitl.NewPolicyApprover(hitl.Allow,
		hitl.Rule{
			ToolPattern: "shell_command",
			ArgPatterns: map[string]string{"cmd": `\brm\b`},
			Action:      hitl.RuleDeny,
		},
		hitl.Rule{ToolPattern: "shell_command", Action: hitl.RuleAllow},
	)

	// rm should be denied
	ok, _ := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{
		ToolName: "shell_command",
		Args:     map[string]any{"cmd": "rm -rf /tmp/data"},
	})
	if ok {
		t.Error("shell_command with rm should be denied")
	}

	// ls should be allowed
	ok, _ = pa.RequestApproval(context.Background(), hitl.ApprovalRequest{
		ToolName: "shell_command",
		Args:     map[string]any{"cmd": "ls /tmp"},
	})
	if !ok {
		t.Error("shell_command with ls should be allowed")
	}
}

func TestDefaultRules_ReadApproved(t *testing.T) {
	pa := hitl.NewPolicyApprover(hitl.Deny, hitl.DefaultRules()...)
	ok, _ := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "read_file"})
	if !ok {
		t.Error("read_file should be auto-approved by DefaultRules")
	}
}

func TestDefaultRules_DeleteAskHuman(t *testing.T) {
	// With Deny as inner, AskHuman → Deny.
	pa := hitl.NewPolicyApprover(hitl.Deny, hitl.DefaultRules()...)
	ok, _ := pa.RequestApproval(context.Background(), hitl.ApprovalRequest{ToolName: "delete_record"})
	if ok {
		t.Error("delete_* should escalate (and inner=Deny → denied)")
	}
}

// ─── InMemoryStore ─────────────────────────────────────────────────────────

func TestStore_CreateAndDecide(t *testing.T) {
	store := hitl.NewInMemoryStore()
	req := hitl.ApprovalRequest{ID: "req-1", ToolName: "send_email"}
	ch, err := store.Create(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.List()) != 1 {
		t.Errorf("expected 1 pending, got %d", len(store.List()))
	}

	go func() {
		_ = store.Decide(hitl.ApprovalDecision{RequestID: "req-1", Approved: true})
	}()

	select {
	case d := <-ch:
		if !d.Approved {
			t.Error("expected approved=true")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decision")
	}
	if len(store.List()) != 0 {
		t.Error("store should be empty after decision")
	}
}

func TestStore_DecideUnknown(t *testing.T) {
	store := hitl.NewInMemoryStore()
	err := store.Decide(hitl.ApprovalDecision{RequestID: "not-exist"})
	if err == nil {
		t.Error("expected error for unknown request ID")
	}
}

func TestStore_DuplicateID(t *testing.T) {
	store := hitl.NewInMemoryStore()
	req := hitl.ApprovalRequest{ID: "dup"}
	_, _ = store.Create(req)
	_, err := store.Create(req)
	if err == nil {
		t.Error("expected error on duplicate ID")
	}
}

func TestStore_Cancel(t *testing.T) {
	store := hitl.NewInMemoryStore()
	req := hitl.ApprovalRequest{ID: "cancel-me", ToolName: "write_file"}
	_, _ = store.Create(req)
	store.Cancel("cancel-me")
	if len(store.List()) != 0 {
		t.Error("cancelled request should be removed from store")
	}
}

// ─── AsyncApprover ─────────────────────────────────────────────────────────

func TestAsyncApprover_Approve(t *testing.T) {
	store := hitl.NewInMemoryStore()
	a := hitl.NewAsyncApprover(store)

	req := hitl.ApprovalRequest{ID: "async-1", ToolName: "send_report"}

	// Deliver the decision from a background goroutine.
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = store.Decide(hitl.ApprovalDecision{RequestID: "async-1", Approved: true})
	}()

	ok, err := a.RequestApproval(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected approval")
	}
}

func TestAsyncApprover_Deny(t *testing.T) {
	store := hitl.NewInMemoryStore()
	a := hitl.NewAsyncApprover(store)

	req := hitl.ApprovalRequest{ID: "async-2", ToolName: "delete_database"}
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = store.Decide(hitl.ApprovalDecision{RequestID: "async-2", Approved: false, Reason: "too risky"})
	}()

	ok, _ := a.RequestApproval(context.Background(), req)
	if ok {
		t.Error("expected denial")
	}
}

func TestAsyncApprover_ContextTimeout(t *testing.T) {
	store := hitl.NewInMemoryStore()
	a := hitl.NewAsyncApprover(store)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	req := hitl.ApprovalRequest{ID: "async-timeout", ToolName: "long_op"}
	// No one calls Decide → should timeout.
	ok, err := a.RequestApproval(ctx, req)
	if ok {
		t.Error("timed-out request should be denied")
	}
	if err == nil {
		t.Error("expected a context deadline error")
	}
}

// ─── Notifier ─────────────────────────────────────────────────────────────

func TestAsyncApprover_Notifier(t *testing.T) {
	store := hitl.NewInMemoryStore()
	a := hitl.NewAsyncApprover(store)

	notified := make(chan string, 1)
	a.AddNotifier(hitl.NotifierFunc(func(ctx context.Context, req hitl.ApprovalRequest) error {
		notified <- req.ToolName
		return nil
	}))

	req := hitl.ApprovalRequest{ID: "notify-1", ToolName: "send_sms"}
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = store.Decide(hitl.ApprovalDecision{RequestID: "notify-1", Approved: true})
	}()
	_, _ = a.RequestApproval(context.Background(), req)

	select {
	case tool := <-notified:
		if tool != "send_sms" {
			t.Errorf("notifier got %q, want send_sms", tool)
		}
	default:
		t.Error("notifier was not called")
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// newBlockingPipe returns a reader that blocks indefinitely (no data written).
func newBlockingPipe() (*blockingReader, func()) {
	br := &blockingReader{ch: make(chan struct{})}
	return br, func() { close(br.ch) }
}

type blockingReader struct{ ch chan struct{} }

func (b *blockingReader) Read(p []byte) (int, error) {
	<-b.ch
	return 0, context.Canceled
}
