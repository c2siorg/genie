package voice

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

type streamTestEnv struct{}

func (streamTestEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (streamTestEnv) Logf(format string, args ...any) {}

type stubStreamingProvider struct{}

func (stubStreamingProvider) Name() string { return "stub" }

func (stubStreamingProvider) StreamingTranscribe(ctx context.Context, lang string, in <-chan []byte, out chan<- Partial) error {
	defer close(out)
	rolling := ""
	for chunk := range in {
		rolling += "[" + string(chunk) + "]"
		out <- Partial{Text: rolling, IsFinal: false, Confidence: 0.5}
	}
	out <- Partial{Text: rolling + " FINAL", IsFinal: true, Confidence: 0.9}
	return nil
}

func (stubStreamingProvider) StreamingSynthesise(ctx context.Context, lang, text string, out chan<- AudioChunk) error {
	defer close(out)
	for i := 0; i < 3; i++ {
		out <- AudioChunk{Index: i, AudioB64: "AAA", IsLast: i == 2}
	}
	return nil
}

func TestStreamingASRChunk(t *testing.T) {
	a := NewStreaming(stubStreamingProvider{})
	body, _ := json.Marshal(asrChunkMsg{Lang: "hi-IN", AudioB64: "audio-1", IsLast: false})
	msg := agent.NewMessage("client", StreamingID, agent.RoleUser, TypeASRStreamChunkIn, string(body), nil)
	out, err := a.HandleMessage(context.Background(), msg, streamTestEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Type != TypeASRStreamFinalOut {
		t.Fatalf("expected final partial when stream closes; got %+v", out)
	}
	var p Partial
	_ = json.Unmarshal([]byte(out[0].Content), &p)
	if !p.IsFinal {
		t.Errorf("expected IsFinal=true")
	}
}

func TestStreamingASRIsLastFlagsFinal(t *testing.T) {
	a := NewStreaming(stubStreamingProvider{})
	body, _ := json.Marshal(asrChunkMsg{Lang: "hi-IN", AudioB64: "audio-1", IsLast: true})
	msg := agent.NewMessage("client", StreamingID, agent.RoleUser, TypeASRStreamChunkIn, string(body), nil)
	out, _ := a.HandleMessage(context.Background(), msg, streamTestEnv{})
	if out[0].Type != TypeASRStreamFinalOut {
		t.Errorf("IsLast should force final output type")
	}
}

func TestStreamingTTSEmitsMultipleChunks(t *testing.T) {
	a := NewStreaming(stubStreamingProvider{})
	body, _ := json.Marshal(ttsStreamReq{Lang: "hi-IN", Text: "hello"})
	msg := agent.NewMessage("client", StreamingID, agent.RoleUser, TypeTTSStreamRequestIn, string(body), nil)
	out, err := a.HandleMessage(context.Background(), msg, streamTestEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Errorf("expected 3 audio chunks; got %d", len(out))
	}
	var last AudioChunk
	_ = json.Unmarshal([]byte(out[len(out)-1].Content), &last)
	if !last.IsLast {
		t.Errorf("last chunk must carry IsLast=true")
	}
}

func TestStreamingRejectsNilProvider(t *testing.T) {
	a := &StreamingAgent{}
	msg := agent.NewMessage("c", StreamingID, agent.RoleUser, TypeASRStreamChunkIn, "{}", nil)
	_, err := a.HandleMessage(context.Background(), msg, streamTestEnv{})
	if err == nil {
		t.Errorf("expected error when no provider configured")
	}
}

func TestStreamingRiskClass(t *testing.T) {
	if NewStreaming(stubStreamingProvider{}).RiskLevel() != agent.RiskMedium {
		t.Errorf("streaming voice should be RiskMedium")
	}
}
