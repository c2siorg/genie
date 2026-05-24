# agents/voice — Streaming ASR/TTS

> **Where:** `agents/voice/streaming.go` · **Lines of code:** ~200 · **Tests:** 6
> **Inspired by:** Google ADK `realtime-conversational-agent`

---

## Overview

The original `voice` agent does request/response transcription on a
complete audio blob — fine for "press to record, release to transcribe"
flows, painful for *conversational* UX where users expect partial
transcripts as they speak.

This file adds:

- **`StreamingVoiceProvider`** — the chunked adapter contract for ASR + TTS
- **`StreamingAgent`** — a bus-side adapter that consumes audio chunks
  and emits partial-transcript / partial-audio messages
- Stable message types and constants for the partial / final stream

The agent is transport-agnostic: pass a `StreamingVoiceProvider`
implementation (Bhashini streaming WebSocket, Whisper-cpp streaming,
Google Cloud Speech-to-Text streaming, Azure Cognitive Services, etc.)
and the bus contract is the same.

---

## Surface

```go
const (
    StreamingID                = "voice_streaming"
    CapVoiceStreaming          = "voice_streaming"

    TypeASRStreamChunkIn       = "voice_asr_chunk"
    TypeASRStreamPartialOut    = "voice_asr_partial"
    TypeASRStreamFinalOut      = "voice_asr_final"

    TypeTTSStreamRequestIn     = "voice_tts_stream"
    TypeTTSStreamAudioChunkOut = "voice_tts_chunk"
)

type Partial struct {
    Text       string
    IsFinal    bool
    Confidence float64 // 0..1
}

type AudioChunk struct {
    Index    int
    AudioB64 string
    IsLast   bool
}

type StreamingVoiceProvider interface {
    Name() string
    StreamingTranscribe(ctx, lang string, chunks <-chan []byte, out chan<- Partial) error
    StreamingSynthesise(ctx, lang, text string, out chan<- AudioChunk) error
}

type StreamingAgent struct {
    Provider   StreamingVoiceProvider
    ChunkQueue int // max in-flight audio chunks (default 16)
}
func NewStreaming(p StreamingVoiceProvider) *StreamingAgent
```

---

## How the streams flow

```
Browser/mobile WebSocket
        │
        │ audio chunks
        ▼
pkg/web/handlers.WSVoice  (host concern, not in this file)
        │
        │ TypeASRStreamChunkIn (one per chunk)
        ▼
Bus → governance → StreamingAgent.HandleMessage
        │
        ▼
provider.StreamingTranscribe (chunk channel in, partial channel out)
        │
        │ Partial (rolling text, IsFinal flag)
        ▼
Emit TypeASRStreamPartialOut (or TypeASRStreamFinalOut on last)
        │
        ▼
pkg/busio.EventTap forwards to the WebSocket as SSE-style events
```

TTS works the other way: a `TypeTTSStreamRequestIn` produces a sequence
of `TypeTTSStreamAudioChunkOut` messages, each carrying base64 audio.

---

## Why the agent uses one-shot collation per HandleMessage

The bus contract is request/response — `HandleMessage` returns
`[]Message`, not a stream. The agent collates the provider's
streaming output into a slice for the bus-message path. The **web layer**
(WebSocket handler in `pkg/web/handlers`) is where the long-lived
channel plumbing lives — keeping it out of the agent keeps the agent
hermetically unit-testable.

For real continuous streaming, the web layer should:

1. Open a WebSocket per session.
2. Stream audio chunks → publish `TypeASRStreamChunkIn` per chunk.
3. Subscribe to `TypeASRStreamPartialOut` / `TypeASRStreamFinalOut`.
4. Push each partial back to the client on the same WebSocket.

The agent handles each chunk as a 1-chunk stream from the provider's
perspective. A future refactor could expose a `RunSession` method that
holds open the provider's channels for the full session — the test
file's `streamingTestEnv` shows the shape.

---

## Provider implementations

The reference repo ships *no* `StreamingVoiceProvider` impl — it's a
host concern, because the right provider depends on the deployment:

- **Bhashini WebSocket** — government's Indic-language ASR
- **Whisper-cpp** — on-device, low-latency, English+IndicTrans
- **Google Cloud Speech-to-Text streaming** — high accuracy, cloud
- **Azure Cognitive Services** — Bhashini alternative for enterprise

Each implementation wraps its native streaming API behind the two
channel-based methods.

---

## Example provider stub (test)

```go
type stubProvider struct{}

func (stubProvider) Name() string { return "stub" }

func (stubProvider) StreamingTranscribe(ctx context.Context, lang string,
    in <-chan []byte, out chan<- Partial) error {
    defer close(out)
    rolling := ""
    for chunk := range in {
        rolling += "[" + string(chunk) + "]"
        out <- Partial{Text: rolling, IsFinal: false, Confidence: 0.5}
    }
    out <- Partial{Text: rolling + " FINAL", IsFinal: true, Confidence: 0.9}
    return nil
}

func (stubProvider) StreamingSynthesise(ctx context.Context, lang, text string,
    out chan<- AudioChunk) error {
    defer close(out)
    for i := 0; i < 3; i++ {
        out <- AudioChunk{Index: i, AudioB64: "AAA", IsLast: i == 2}
    }
    return nil
}
```

---

## FREE-AI alignment

- **Rec 4 (Indigenous Models)** — Bhashini is the canonical Indian
  streaming-ASR provider; the agent's interface is what lets a bank
  swap in Bhashini without changing the bus contract.
- **Rec 18 (Disclosure)** — the transcript output should carry an "AI
  transcription" badge; the agent itself doesn't enforce this, but the
  client should.

---

## Anti-patterns

1. **Holding chunks in `ChunkQueue` larger than memory allows.** Default 16 is fine for typical voice; raise carefully.
2. **Submitting an empty audio chunk.** Treat zero-length as "stream-end signal" only.
3. **Forgetting `IsLast` on the final TTS chunk.** Clients use it to stop the player; without it, the audio plays then waits indefinitely.

---

## Testing

`agents/voice/streaming_test.go` covers:

- ASR single chunk → final partial emitted
- `IsLast=true` flag forces final-type output
- TTS emits multiple chunks with `IsLast` set on the last
- Nil provider → clear error
- RiskClass = Medium

Run:

```bash
go test ./agents/voice/ -v
```

---

## References

- [Bhashini](https://bhashini.gov.in/) — Government of India language-AI platform
- [Whisper streaming](https://github.com/openai/whisper) — OpenAI ASR (Whisper-cpp for on-device)
- [Google Cloud Speech-to-Text streaming](https://cloud.google.com/speech-to-text/docs/streaming-recognize)
- [Genie's existing voice agent](../../agents/voice/voice.go) — the batched original
