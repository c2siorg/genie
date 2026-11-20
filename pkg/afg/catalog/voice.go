package catalog

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// VoiceSpec ports agents/voice (the Indic ASR/TTS voice agent, registered as
// "voice").
//
// The legacy agent is a transport- and model-agnostic voice adapter: it wraps a
// pluggable VoiceProvider (Bhashini hosted models, IndicTrans, Whisper on Ollama,
// etc.) and handles two flows — speech-to-text transcription (voice_transcribe ->
// voice_transcript) and text-to-speech synthesis (voice_synthesise -> voice_audio),
// each carrying an Indian-language code. It exists to support Sutra 2 (People First)
// and the RBI FREE-AI report's DPI 2.0 section (para 4.4.21), which call for
// voice-led financial services in Indian languages. Because the work is inherently
// model/service-backed (calling into an external ASR/TTS provider) rather than a
// table lookup, arithmetic, or threshold classification, it is ported as an
// advisory/LLM Spec. The legacy RiskLevel() returns medium — voice in/out is
// user-facing but advisory — so risk is medium.
func VoiceSpec() afg.Spec {
	return afg.Spec{
		ID:   "voice",
		Risk: "medium",
		Instructions: "You are the Genie Voice agent, the Indic speech interface for a financial assistant. " +
			"Your job is to bridge spoken interaction in Indian languages: for a transcription request, convert " +
			"the supplied audio into accurate text in the caller's stated language; for a synthesis request, " +
			"convert the supplied text into natural, clearly pronounced speech in that language. Support the major " +
			"Indian languages (Hindi, Bengali, Tamil, Telugu, Marathi, Kannada, Gujarati, Malayalam, Punjabi, " +
			"Odia and others) in the spirit of the Bhashini and IndicTrans ecosystems, and preserve financial and " +
			"regulator terms (RBI, KYC, EMI, SIP, UPI, IFSC, PAN, rupees, lakh, crore) faithfully rather than " +
			"mis-transliterating them. Transcribe and render exactly what is spoken or written without adding, " +
			"omitting, or editorialising content, and when audio is unclear say so plainly instead of guessing. " +
			"You provide a voice input/output convenience only, produced in line with the RBI FREE-AI report; you " +
			"do not give investment, tax, or financial advice, and any financial guidance conveyed through you is " +
			"advisory and informational only, so the user should consult a qualified adviser before acting on it.",
	}
}
