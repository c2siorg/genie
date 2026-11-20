package adapter

import (
	"context"
	"errors"
	"testing"

	pkgagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// fakeLegacyAgent implements pkg/agent.Agent for bridge tests.
type fakeLegacyAgent struct {
	id        string
	reply     []pkgagent.Message
	err       error
	gotMsg    pkgagent.Message
	gotCalled bool
}

func (f *fakeLegacyAgent) ID() string             { return f.id }
func (f *fakeLegacyAgent) Name() string           { return f.id }
func (f *fakeLegacyAgent) Capabilities() []string { return []string{"message.handle"} }
func (f *fakeLegacyAgent) HandleMessage(_ context.Context, msg pkgagent.Message, _ pkgagent.Environment) ([]pkgagent.Message, error) {
	f.gotCalled = true
	f.gotMsg = msg
	return f.reply, f.err
}

func TestBridge_Execute_CallsHandleMessageAndReturnsMessages(t *testing.T) {
	reply := []pkgagent.Message{{From: "analyzer", To: "supervisor", Content: "done"}}
	inner := &fakeLegacyAgent{id: "analyzer", reply: reply}
	b := NewMessageBridge(inner, nil, "2.1.0")

	if b.Name() != "analyzer" {
		t.Errorf("Name = %q, want analyzer", b.Name())
	}
	if b.Version() != "2.1.0" {
		t.Errorf("Version = %q, want 2.1.0", b.Version())
	}

	out, err := b.Execute(context.Background(), map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !inner.gotCalled {
		t.Fatal("HandleMessage was not called")
	}
	// Map payload should be JSON-encoded into Content, addressed To the agent ID.
	if inner.gotMsg.To != "analyzer" {
		t.Errorf("msg.To = %q, want analyzer", inner.gotMsg.To)
	}
	if inner.gotMsg.Content != `{"k":"v"}` {
		t.Errorf("msg.Content = %q, want JSON-encoded payload", inner.gotMsg.Content)
	}
	msgs, ok := out.([]pkgagent.Message)
	if !ok {
		t.Fatalf("output type %T, want []protocol.Message", out)
	}
	if len(msgs) != 1 || msgs[0].Content != "done" {
		t.Fatalf("unexpected returned messages: %+v", msgs)
	}
}

func TestBridge_Execute_PassesMessageThrough(t *testing.T) {
	inner := &fakeLegacyAgent{id: "supervisor"}
	b := NewMessageBridge(inner, nil, "")

	// A protocol.Message input must pass through unchanged (in-process reuse).
	in := protocol.Message{ID: "m-1", From: "user", To: "supervisor", Content: "how much did I spend?"}
	if _, err := b.Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if inner.gotMsg.Content != "how much did I spend?" {
		t.Errorf("plain-text content not preserved: %q", inner.gotMsg.Content)
	}
	if inner.gotMsg.ID != "m-1" {
		t.Errorf("message identity not preserved: ID=%q", inner.gotMsg.ID)
	}
}

func TestBridge_Execute_StringPayloadStaysPlainText(t *testing.T) {
	inner := &fakeLegacyAgent{id: "educator"}
	b := NewMessageBridge(inner, nil, "")
	if _, err := b.Execute(context.Background(), "what is a mutual fund?"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if inner.gotMsg.Content != "what is a mutual fund?" {
		t.Errorf("string payload should pass through as plain text, got %q", inner.gotMsg.Content)
	}
}

func TestBridge_Execute_ErrorIsWrappedAndHealthDegrades(t *testing.T) {
	inner := &fakeLegacyAgent{id: "analyzer", err: errors.New("boom")}
	b := NewMessageBridge(inner, nil, "")

	if err := b.Health(context.Background()); err != nil {
		t.Fatalf("fresh bridge should be healthy, got %v", err)
	}
	if _, err := b.Execute(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error from failing inner agent")
	}
	if err := b.Health(context.Background()); err == nil {
		t.Fatal("Health should reflect the last failed Execute")
	}
}
