// streaming.go — incremental ASR/TTS over a streaming transport.
//
// The original voice agent does request/response transcription on a complete
// audio blob. For real conversational UX (think Bhashini interactive),
// users expect partial transcripts to appear word-by-word, and TTS to start
// playing before the model finishes generating.
//
// This file adds:
//   - StreamingVoiceProvider — the chunked adapter contract (ASR + TTS).
//   - StreamingAgent          — a bus-side adapter that consumes audio chunks
//                               and emits partial-transcript / partial-audio
//                               messages.
//
// Inspired by Google ADK samples → realtime-conversational-agent.
package voice

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	StreamingID                = "voice_streaming"
	CapVoiceStreaming          = "voice_streaming"
	TypeASRStreamChunkIn       = "voice_asr_chunk"     // one audio chunk from client
	TypeASRStreamPartialOut    = "voice_asr_partial"   // rolling transcript
	TypeASRStreamFinalOut      = "voice_asr_final"     // turn-end transcript
	TypeTTSStreamRequestIn     = "voice_tts_stream"    // text → stream audio
	TypeTTSStreamAudioChunkOut = "voice_tts_chunk"     // one audio chunk back
)

// Partial is one rolling transcript update from the ASR.
type Partial struct {
	Text       string  `json:"text"`
	IsFinal    bool    `json:"is_final"`
	Confidence float64 `json:"confidence_0_1"`
}

// AudioChunk is one TTS chunk to play.
type AudioChunk struct {
	Index     int    `json:"index"`
	AudioB64  string `json:"audio_b64"`
	IsLast    bool   `json:"is_last"`
}

// StreamingVoiceProvider is the streaming adapter contract.
//
// Implementations: Bhashini streaming WebSocket, Whisper-cpp streaming,
// Google Cloud Speech-to-Text streaming, Azure Cognitive Services, etc.
type StreamingVoiceProvider interface {
	Name() string
	// StreamingTranscribe consumes audio chunks from `chunks` and pushes
	// rolling Partial values to `out`. The implementation closes `out`
	// when the underlying stream ends or ctx is cancelled.
	StreamingTranscribe(ctx context.Context, lang string, chunks <-chan []byte, out chan<- Partial) error
	// StreamingSynthesise emits audio chunks for `text` to `out`. The final
	// chunk has IsLast=true; the implementation closes `out` on completion.
	StreamingSynthesise(ctx context.Context, lang, text string, out chan<- AudioChunk) error
}

// StreamingAgent wraps a StreamingVoiceProvider for use on the message bus.
type StreamingAgent struct {
	Provider   StreamingVoiceProvider
	ChunkQueue int // max in-flight audio chunks (default 16)
}

// NewStreaming constructs the agent.
func NewStreaming(p StreamingVoiceProvider) *StreamingAgent {
	return &StreamingAgent{Provider: p, ChunkQueue: 16}
}

func (a *StreamingAgent) ID() string                 { return StreamingID }
func (a *StreamingAgent) Name() string               { return "Voice ASR/TTS (streaming)" }
func (a *StreamingAgent) Capabilities() []string     { return []string{CapVoiceStreaming} }
func (a *StreamingAgent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

type asrChunkMsg struct {
	Lang     string `json:"lang"`
	AudioB64 string `json:"audio_b64"`
	SessionID string `json:"session_id"`
	IsLast   bool   `json:"is_last"`
}

type ttsStreamReq struct {
	Lang string `json:"lang"`
	Text string `json:"text"`
}

// HandleMessage handles one chunk-at-a-time. The host adapter (WebSocket
// handler in pkg/web) collates a stream of these into the chunk channels
// the provider expects.
//
// For the canonical bus, we run a per-message round-trip: caller sends a
// single chunk, agent returns the partial. The web layer (pkg/web/handlers)
// is where the long-lived WebSocket and chan plumbing lives — keeping it
// out of the agent keeps the agent unit-testable.
func (a *StreamingAgent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if a.Provider == nil {
		return nil, errors.New("voice: no streaming provider configured")
	}
	switch msg.Type {
	case TypeASRStreamChunkIn:
		var req asrChunkMsg
		if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
			return nil, err
		}
		// Wrap the single chunk into a one-shot channel pair so the
		// provider sees a 1-chunk stream. The web layer should call this
		// agent's RunSession for real continuous streaming.
		partial, err := a.oneShotPartial(ctx, req)
		if err != nil {
			return nil, err
		}
		env.Logf("[voice_streaming] partial=%q final=%v conf=%.2f", partial.Text, partial.IsFinal, partial.Confidence)
		body, _ := json.Marshal(partial)
		out := TypeASRStreamPartialOut
		if partial.IsFinal {
			out = TypeASRStreamFinalOut
		}
		return []agent.Message{
			agent.NewMessage(StreamingID, msg.From, agent.RoleAgent, out, string(body), msg.Metadata),
		}, nil
	case TypeTTSStreamRequestIn:
		var req ttsStreamReq
		if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
			return nil, err
		}
		chunks, err := a.collectTTS(ctx, req)
		if err != nil {
			return nil, err
		}
		// Emit one bus message per chunk. The web layer's EventTap will
		// forward each as a separate SSE event.
		out := make([]agent.Message, 0, len(chunks))
		for _, c := range chunks {
			body, _ := json.Marshal(c)
			out = append(out, agent.NewMessage(StreamingID, msg.From, agent.RoleAgent, TypeTTSStreamAudioChunkOut, string(body), msg.Metadata))
		}
		env.Logf("[voice_streaming] tts emitted %d chunks", len(out))
		return out, nil
	}
	return nil, nil
}

// oneShotPartial wraps a single-chunk audio into a 1-chunk stream and
// returns the last partial the provider produces.
func (a *StreamingAgent) oneShotPartial(ctx context.Context, req asrChunkMsg) (Partial, error) {
	in := make(chan []byte, 1)
	in <- []byte(req.AudioB64)
	close(in)

	queue := a.ChunkQueue
	if queue <= 0 {
		queue = 16
	}
	out := make(chan Partial, queue)
	errCh := make(chan error, 1)
	go func() { errCh <- a.Provider.StreamingTranscribe(ctx, req.Lang, in, out) }()

	last := Partial{}
	for p := range out {
		last = p
	}
	if err := <-errCh; err != nil {
		return Partial{}, err
	}
	if req.IsLast {
		last.IsFinal = true
	}
	return last, nil
}

// collectTTS drains the provider's streaming TTS output into a slice for
// the bus-message path. Real streaming flows through the web layer.
func (a *StreamingAgent) collectTTS(ctx context.Context, req ttsStreamReq) ([]AudioChunk, error) {
	queue := a.ChunkQueue
	if queue <= 0 {
		queue = 16
	}
	out := make(chan AudioChunk, queue)
	errCh := make(chan error, 1)
	go func() { errCh <- a.Provider.StreamingSynthesise(ctx, req.Lang, req.Text, out) }()
	chunks := []AudioChunk{}
	for c := range out {
		chunks = append(chunks, c)
	}
	if err := <-errCh; err != nil {
		return nil, err
	}
	return chunks, nil
}
