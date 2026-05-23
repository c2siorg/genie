// Package llm is the model-provider abstraction Genie agents call when they
// need natural-language reasoning. Concrete providers (Anthropic, Gemini,
// OpenAI, local Ollama) implement Provider and Genie agents take a
// Provider in their constructor.
//
// Decoupling LLM access from agents means tests can pass a Mock and
// production can pick a provider based on residency requirements (e.g. a
// local provider when the data classification is PII).
package llm

import (
	"context"
	"errors"
	"sync"
)

// Role for a chat message. Mirrors the OpenAI / Anthropic shape.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is the unit of conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	// Optional: name of the tool that produced this message (Role=tool).
	ToolName string `json:"tool_name,omitempty"`
}

// ToolDefinition is a JSON-schema-described function the model is allowed to call.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ToolCall is the model's request to invoke a tool.
type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// CompletionRequest is the unified request shape.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolDefinition
	MaxTokens   int
	Temperature float64
	// Residency constraints — providers MUST honour or refuse.
	Residency Residency
}

// Residency is the data-locality envelope around an LLM call.
type Residency struct {
	// Region is e.g. "in" (India), "us", "eu", or "on-prem".
	Region string
	// AllowCrossBorder is false by default; when false providers in different
	// regions must refuse the call.
	AllowCrossBorder bool
	// Classification of the payload — providers may downgrade or refuse based
	// on this (e.g. refuse "pii" if the provider is outside the required region).
	Classification string
}

// CompletionResponse is the unified result.
type CompletionResponse struct {
	Text       string
	ToolCalls  []ToolCall
	FinishedAt string
	Provider   string
	Model      string
	// Usage helps with cost telemetry and quota enforcement.
	Usage Usage
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// ErrResidencyDenied is returned by providers that refuse a call because the
// residency envelope says no.
var ErrResidencyDenied = errors.New("residency policy denied this provider")

// Provider is the model-agnostic interface.
type Provider interface {
	// Name returns a stable identifier ("anthropic", "gemini", "mock") used for
	// telemetry, audit, and provider-registry lookup.
	Name() string
	// Region returns the provider's hosting region tag (e.g. "us", "in", "on-prem").
	Region() string
	// Complete runs one inference call.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// Mock is a deterministic Provider for tests. Set Responses to enqueue
// canned answers; Complete consumes them in order and returns the last
// response on repeat to keep tests stable when fan-out happens.
type Mock struct {
	NameVal   string
	RegionVal string

	mu        sync.Mutex
	Responses []CompletionResponse
	Calls     []CompletionRequest
}

// NewMock builds a Mock provider with default region "on-prem".
func NewMock() *Mock { return &Mock{NameVal: "mock", RegionVal: "on-prem"} }

func (m *Mock) Name() string   { return m.NameVal }
func (m *Mock) Region() string { return m.RegionVal }

func (m *Mock) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, req)
	if !req.Residency.AllowCrossBorder && req.Residency.Region != "" && req.Residency.Region != m.RegionVal {
		return CompletionResponse{}, ErrResidencyDenied
	}
	if len(m.Responses) == 0 {
		return CompletionResponse{Text: "(mock empty response)", Provider: m.NameVal, Model: req.Model}, nil
	}
	resp := m.Responses[0]
	if len(m.Responses) > 1 {
		m.Responses = m.Responses[1:]
	}
	resp.Provider = m.NameVal
	resp.Model = req.Model
	return resp, nil
}
