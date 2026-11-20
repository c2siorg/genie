// Package agentic implements the progressive agent from lessons 01–08.
//
// Lesson 01 — RunAgent: simplest single LLM call (no tools, no loop).
// Lesson 02 — tools wired via agenttools.Registry.
// Lesson 04 — full agent loop: LLM → tool calls → observe → repeat.
// Lesson 07 — context compaction when usage exceeds threshold.
// Lesson 08 — shell/code tools plug in via the Registry.
// Lesson 09 — HITL: Executor.Approver gates each tool call (pkg/hitl).
//
// All LLM calls use the OpenAI-compatible /v1/chat/completions API so the
// same code works with Ollama (local), OpenAI, Anthropic via proxy, etc.
package agentic

import (
	"os"
)

// ─── Config ────────────────────────────────────────────────────────────────

// Config holds the runtime configuration for the agent.
type Config struct {
	// Provider is "ollama", "openai", "anthropic", etc.
	Provider string
	// BaseURL is the API base (default: Ollama at localhost:11434).
	BaseURL string
	// Model is the model name (default: reads GENIE_OLLAMA_CHAT, falls back to "llama3.2:1b").
	Model string
	// APIKey is optional (needed for OpenAI/Anthropic, not Ollama).
	APIKey string
	// MaxSteps caps the agent loop iterations (default 20).
	MaxSteps int
	// SystemPrompt overrides the default system prompt.
	SystemPrompt string
	// ContextThreshold is the fraction of context window that triggers
	// compaction (default 0.80 = 80%).
	ContextThreshold float64
}

// DefaultConfig reads Ollama env vars and returns sensible defaults.
//
// Tool-calling requires a model that supports it. Recommended:
//   - qwen3.5:latest   (default, good all-round tool caller)
//   - granite4.1:3b    (IBM enterprise model, compact and accurate)
//   - llama3.1:8b      (Meta, 8B+ required for reliable tool use)
//   - mistral-nemo     (strong tool calling, medium size)
//
// Avoid llama3.2:1b for tool-heavy workflows — it is too small to reliably
// produce structured tool_calls and tends to put JSON in plain text instead.
func DefaultConfig() Config {
	model := os.Getenv("GENIE_OLLAMA_CHAT")
	if model == "" {
		model = "qwen3.5:latest"
	}
	baseURL := os.Getenv("GENIE_OLLAMA_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return Config{
		Provider:         "ollama",
		BaseURL:          baseURL,
		Model:            model,
		MaxSteps:         20,
		ContextThreshold: 0.80,
		SystemPrompt:     DefaultSystemPrompt,
	}
}

// DefaultSystemPrompt is the minimal system prompt for a general-purpose agent.
// Override via Config.SystemPrompt (exercise 3 from lesson 01).
const DefaultSystemPrompt = `You are a helpful assistant with access to tools.
Use tools when needed to answer the user's request. When you have gathered
enough information, provide a concise final answer.`

// ─── Callbacks ─────────────────────────────────────────────────────────────

// Callbacks lets callers react to agent events without modifying loop logic.
// All fields are optional — nil callbacks are silently skipped.
type Callbacks struct {
	// OnToken is called for each streamed text token.
	OnToken func(token string)
	// OnToolCallStart is called before a tool is executed.
	OnToolCallStart func(toolName string, args map[string]any)
	// OnToolCallEnd is called after a tool returns.
	OnToolCallEnd func(toolName string, result string)
	// OnComplete is called when the agent loop finishes.
	OnComplete func(finalText string)
	// OnTokenUsage is called after each loop iteration with token counts.
	OnTokenUsage func(usage TokenUsage)
}

// TokenUsage holds estimated token consumption for a conversation.
type TokenUsage struct {
	InputTokens   int
	OutputTokens  int
	TotalTokens   int
	ContextWindow int
	Percentage    float64
}

// ─── Message ───────────────────────────────────────────────────────────────

// Message is a single chat turn.
type Message struct {
	Role    string `json:"role"` // "system" | "user" | "assistant" | "tool"
	Content string `json:"content"`
	// ToolCallID is set on tool-result messages.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// Name is the tool name on tool-result messages.
	Name string `json:"name,omitempty"`
}
