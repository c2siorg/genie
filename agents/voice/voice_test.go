package voice

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

func TestVoice_Transcribe(t *testing.T) {
	body, _ := json.Marshal(transcribeRequest{Lang: "hi", Audio: "hello"})
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeTranscribeIn, string(body), nil)
	out, err := New(EchoProvider{}).HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out[0].Content, "(echo)") {
		t.Fatalf("unexpected transcription: %s", out[0].Content)
	}
}

func TestVoice_Synthesise(t *testing.T) {
	body, _ := json.Marshal(synthesiseRequest{Lang: "hi", Text: "namaste"})
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeSynthesiseIn, string(body), nil)
	out, err := New(EchoProvider{}).HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out[0].Content, "namaste") {
		t.Fatalf("unexpected synthesis: %s", out[0].Content)
	}
}
