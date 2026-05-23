// Package voice exposes an Indic ASR/TTS agent. The agent is transport- and
// model-agnostic — it talks to whatever VoiceProvider you wire in (Bhashini
// hosted models, IndicTrans, Whisper on Ollama, etc.).
//
// Sutra 2 (People First) and the FREE-AI report's DPI 2.0 section (para
// 4.4.21) ask for voice-led financial services in Indian languages; this is
// the minimum plumbing.
package voice

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID                 = "voice"
	CapVoice           = "voice_io"
	TypeTranscribeIn   = "voice_transcribe"
	TypeTranscribeOut  = "voice_transcript"
	TypeSynthesiseIn   = "voice_synthesise"
	TypeSynthesiseOut  = "voice_audio"
)

// VoiceProvider is the adapter contract. Implementations call into Bhashini,
// Whisper, or any other ASR/TTS service.
type VoiceProvider interface {
	Name() string
	// Transcribe runs speech-to-text. AudioBase64 is base64-encoded WAV/MP3.
	Transcribe(ctx context.Context, lang, audioBase64 string) (string, error)
	// Synthesise runs text-to-speech and returns base64-encoded audio.
	Synthesise(ctx context.Context, lang, text string) (string, error)
}

// Agent is the bus-side adapter.
type Agent struct {
	Provider VoiceProvider
}

// New constructs the agent.
func New(p VoiceProvider) *Agent { return &Agent{Provider: p} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Voice ASR/TTS" }
func (a *Agent) Capabilities() []string { return []string{CapVoice} }

// RiskLevel — voice in/out is medium risk: user-facing but advisory.
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

type transcribeRequest struct {
	Lang  string `json:"lang"`
	Audio string `json:"audio_b64"`
}

type synthesiseRequest struct {
	Lang string `json:"lang"`
	Text string `json:"text"`
}

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if a.Provider == nil {
		return nil, errors.New("voice: no provider configured")
	}
	switch msg.Type {
	case TypeTranscribeIn:
		var req transcribeRequest
		if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
			return nil, err
		}
		text, err := a.Provider.Transcribe(ctx, req.Lang, req.Audio)
		if err != nil {
			return nil, err
		}
		env.Logf("[voice] transcribed %d chars (lang=%s)", len(text), req.Lang)
		body, _ := json.Marshal(map[string]any{"lang": req.Lang, "text": text})
		return []agent.Message{
			agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeTranscribeOut, string(body), msg.Metadata),
		}, nil
	case TypeSynthesiseIn:
		var req synthesiseRequest
		if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
			return nil, err
		}
		audio, err := a.Provider.Synthesise(ctx, req.Lang, req.Text)
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(map[string]any{"lang": req.Lang, "audio_b64": audio})
		return []agent.Message{
			agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeSynthesiseOut, string(body), msg.Metadata),
		}, nil
	}
	return nil, nil
}

// EchoProvider is a deterministic stub for tests/demo. Transcribe returns
// the supplied text appended with "(echo)" and Synthesise returns the input.
type EchoProvider struct{}

func (EchoProvider) Name() string { return "echo" }
func (EchoProvider) Transcribe(_ context.Context, _ string, audio string) (string, error) {
	return audio + " (echo)", nil
}
func (EchoProvider) Synthesise(_ context.Context, _ string, text string) (string, error) {
	return text, nil
}
