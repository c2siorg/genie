package agentic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/hitl"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/memory"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/safety"
)

// ─── Wire types (OpenAI-compatible) ───────────────────────────────────────

type wireMessage struct {
	Role       string          `json:"role"`
	Content    any             `json:"content"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolCalls  []wireToolCall  `json:"tool_calls,omitempty"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type wireRequest struct {
	Model    string         `json:"model"`
	Messages []wireMessage  `json:"messages"`
	Tools    []wireTool     `json:"tools,omitempty"`
}

type wireResponse struct {
	Choices []struct {
		Message      wireMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

// ─── Runner ────────────────────────────────────────────────────────────────

// ReflexionConfig enables the Reflexion self-critique loop (lesson 14).
// After the agent produces its final answer, the runner asks the LLM to
// critique it, then refines based on the critique.
//
//	runner := &agentic.Runner{
//	    Reflexion: &agentic.ReflexionConfig{MaxRetries: 1},
//	}
type ReflexionConfig struct {
	// MaxRetries is the number of critique→refine cycles (default 1, max 3).
	MaxRetries int
}

// Runner is the agent. Set fields, then call Run.
//
// Lesson 01: Runner{}.Run(ctx, "Hello") — single LLM call, no tools.
// Lesson 02: Runner{Registry: r}.Run(...) — add tool definitions.
// Lesson 04: same Runner, the loop handles tool calls automatically.
// Lesson 07: Runner{}.Run(...) auto-compacts when context fills up.
// Lesson 09: Runner{Approver: hitl.NewCLIApprover(...)}.Run(...) — HITL.
// Lesson 10: Runner{Memory: store, UserID: "u1"}.Run(...) — persistent memory.
// Lesson 14: Runner{Reflexion: &ReflexionConfig{MaxRetries:1}}.Run(...) — self-critique.
type Runner struct {
	// Config holds model/provider/URL/system prompt settings.
	Config Config
	// Registry provides tool definitions and execution. Nil = no tools (lesson 01).
	Registry *agenttools.Registry
	// Approver gates each tool call (lesson 09 HITL). Nil = auto-approve.
	Approver hitl.Approver
	// Memory injects long-term facts into the system prompt before every LLM
	// call (lesson 10). Pair with agenttools.MemoryTools for read/write access.
	Memory *memory.LongTermMemory
	// UserID scopes memory reads/writes to a specific user.
	UserID string
	// Reflexion enables self-critique and answer refinement after the main
	// agent loop finishes (lesson 14). Nil = disabled.
	Reflexion *ReflexionConfig
	// Safety screens user input (inbound) and model output (outbound).
	// When a message is flagged the agent refuses rather than proceeding.
	// Nil = no safety checks (default). See pkg/safety for detectors.
	Safety *safety.Chain
	// Episodic memory records each turn to a rolling buffer that is
	// summarised by the LLM when it overflows. Inject the snapshot into
	// the system prompt so the agent remembers within a session.
	Episodic  *memory.EpisodicMemory
	SessionID string // scopes episodic memory to a session
	// HTTPClient is reused for all LLM calls.
	HTTPClient *http.Client
	// Callbacks for streaming output and observability.
	Callbacks Callbacks
}

// New returns a Runner with default config (Ollama).
func New() *Runner {
	return &Runner{
		Config:     DefaultConfig(),
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// ─── Lesson 01: RunAgent ───────────────────────────────────────────────────

// RunAgent is the simplest possible agent (lesson 01): one LLM call, no tools,
// no loop. This is the foundation everything else builds on.
//
// Exercise 3 from lesson 01: change r.Config.SystemPrompt to modify behavior.
func RunAgent(ctx context.Context, userMessage string, history []Message, cfg Config) (string, error) {
	r := &Runner{
		Config:     cfg,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
	msgs := r.buildMessages(userMessage, history)
	resp, err := r.callLLM(ctx, messagesToWire(msgs), nil)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("agentic: empty response")
	}
	return contentString(resp.Choices[0].Message.Content), nil
}

// ─── Lesson 04: Run (full agent loop) ─────────────────────────────────────

// Run executes the full agent loop (lesson 04).
//
// Algorithm (matches TypeScript run.ts exactly):
//  1. Build messages (system + history + user).
//  2. Pre-compaction: if context > threshold, summarise history.
//  3. Loop (max MaxSteps):
//     a. Call LLM with messages + tool definitions.
//     b. If finish_reason == "tool_calls": HITL check, execute, append results, continue.
//     c. Else: capture final text, break.
//  4. Return final text + updated message history.
func (r *Runner) Run(ctx context.Context, userMessage string, history []Message) (string, []Message, error) {
	// ── Lesson 15: safety — screen inbound message ─────────────────────────
	if r.Safety != nil {
		v, err := r.Safety.Inspect(ctx, userMessage)
		if err == nil && v.Flagged {
			return fmt.Sprintf("I can't process that request: %s", v.Reason), history, nil
		}
	}

	cfg := r.Config
	client := r.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}

	msgs := r.buildMessages(userMessage, history)

	// ── Lesson 07: pre-compaction ──────────────────────────────────────────
	if cfg.ContextThreshold > 0 && IsOverThreshold(msgs, cfg.ContextThreshold) {
		compact, err := compactMessages(ctx, cfg, msgs, client)
		if err == nil && compact != nil {
			// Replace history with compact seed; keep system prompt.
			var sys []Message
			for _, m := range msgs {
				if m.Role == "system" {
					sys = append(sys, m)
				}
			}
			msgs = append(sys, compact...)
			msgs = append(msgs, Message{Role: "user", Content: userMessage})
		}
	}

	// Build wire messages.
	wire := messagesToWire(msgs)

	// Build tool list (nil when no registry).
	var tools []wireTool
	if r.Registry != nil {
		for _, def := range r.Registry.Definitions() {
			fn, _ := def["function"].(map[string]any)
			var wt wireTool
			wt.Type = "function"
			wt.Function.Name, _ = fn["name"].(string)
			wt.Function.Description, _ = fn["description"].(string)
			wt.Function.Parameters, _ = fn["parameters"].(map[string]any)
			tools = append(tools, wt)
		}
	}

	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 20
	}

	var finalText string

	for step := 0; step < maxSteps; step++ {
		resp, err := r.callLLM(ctx, wire, tools)
		if err != nil {
			return "", nil, fmt.Errorf("step %d: %w", step, err)
		}
		if len(resp.Choices) == 0 {
			return "", nil, fmt.Errorf("step %d: no choices", step)
		}

		choice := resp.Choices[0]
		assistantMsg := choice.Message
		if assistantMsg.Content == nil {
			assistantMsg.Content = ""
		}
		wire = append(wire, assistantMsg)

		// ── Tool calls ───────────────────────────────────────────────────
		if choice.FinishReason == "tool_calls" || len(assistantMsg.ToolCalls) > 0 {
			rejected := false

			for _, tc := range assistantMsg.ToolCalls {
				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

				// ── Lesson 09: HITL ──────────────────────────────────────
				if r.Approver != nil {
					req := hitl.ApprovalRequest{
						ToolName: tc.Function.Name,
						Args:     args,
					}
					approved, approvalErr := r.Approver.RequestApproval(ctx, req)
					if approvalErr != nil {
						return "", nil, fmt.Errorf("hitl: %w", approvalErr)
					}
					if !approved {
						rejected = true
						break
					}
				}

				// Execute tool.
				if r.Callbacks.OnToolCallStart != nil {
					r.Callbacks.OnToolCallStart(tc.Function.Name, args)
				}
				var result string
				if r.Registry != nil {
					result, _ = r.Registry.Execute(ctx, tc.Function.Name, args)
				} else {
					result = fmt.Sprintf("no registry: tool %q not executed", tc.Function.Name)
				}
				if r.Callbacks.OnToolCallEnd != nil {
					r.Callbacks.OnToolCallEnd(tc.Function.Name, result)
				}

				wire = append(wire, wireMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}

			if rejected {
				finalText = "action denied by HITL approval"
				break
			}

			// Emit token usage.
			if r.Callbacks.OnTokenUsage != nil {
				r.Callbacks.OnTokenUsage(estimateWireTokens(wire))
			}
			continue
		}

		// ── No tool calls → final response ───────────────────────────────
		finalText = contentString(assistantMsg.Content)
		if r.Callbacks.OnToken != nil {
			r.Callbacks.OnToken(finalText)
		}
		break
	}

	// ── Safety: screen outbound answer (up to 2 regeneration retries) ─────
	if r.Safety != nil && finalText != "" {
		for attempt := 0; attempt < 2; attempt++ {
			v, err := r.Safety.Inspect(ctx, finalText)
			if err != nil || !v.Flagged {
				break
			}
			// Ask the model to regenerate without the problematic content.
			regenWire := append(wire, wireMessage{
				Role:    "user",
				Content: "Your previous response was flagged. Please provide a safe, policy-compliant answer.",
			})
			resp, err := r.callLLM(ctx, regenWire, nil)
			if err != nil || len(resp.Choices) == 0 {
				finalText = "I'm unable to provide a response to that request."
				break
			}
			finalText = contentString(resp.Choices[0].Message.Content)
		}
	}

	// ── Episodic memory: record this turn ───────────────────────────────────
	if r.Episodic != nil && r.SessionID != "" {
		_ = r.Episodic.Append(ctx, r.SessionID, "user", userMessage)
		if finalText != "" {
			_ = r.Episodic.Append(ctx, r.SessionID, "assistant", finalText)
		}
	}

	// ── Lesson 14: Reflexion self-critique ────────────────────────────────
	// After the main agent loop produces its answer, run up to MaxRetries
	// critique→refine cycles to improve quality.
	if r.Reflexion != nil && finalText != "" {
		retries := r.Reflexion.MaxRetries
		if retries <= 0 {
			retries = 1
		}
		if retries > 3 {
			retries = 3
		}
		for i := 0; i < retries; i++ {
			refined, err := r.reflexionCycle(ctx, userMessage, finalText)
			if err != nil {
				break // non-fatal — keep the last good answer
			}
			if refined == finalText || refined == "" {
				break // no improvement
			}
			if r.Callbacks.OnToken != nil {
				r.Callbacks.OnToken("\n[reflexion refined answer]\n")
			}
			finalText = refined
		}
	}

	if r.Callbacks.OnComplete != nil {
		r.Callbacks.OnComplete(finalText)
	}

	// Convert wire back to Messages for caller.
	return finalText, wireToMessages(wire), nil
}

// ─── Lesson 15 (streaming): RunStream ────────────────────────────────────────

// RunStream executes the agent loop with streaming delivery.
// Tokens arrive on the returned channel as they are generated; the error
// channel receives at most one value (nil on success) when the stream ends.
//
//	tokens, errs := runner.RunStream(ctx, "Hello", nil)
//	for t := range tokens { fmt.Print(t) }
//	if err := <-errs; err != nil { log.Fatal(err) }
func (r *Runner) RunStream(ctx context.Context, userMessage string, history []Message) (<-chan string, <-chan error) {
	tokens := make(chan string, 64)
	errs := make(chan error, 1)

	go func() {
		defer close(tokens)
		defer close(errs)

		// Safety: screen inbound.
		if r.Safety != nil {
			if v, err := r.Safety.Inspect(ctx, userMessage); err == nil && v.Flagged {
				tokens <- fmt.Sprintf("I can't process that request: %s", v.Reason)
				errs <- nil
				return
			}
		}

		cfg := r.Config
		client := r.HTTPClient
		if client == nil {
			client = &http.Client{Timeout: 120 * time.Second}
		}

		msgs := r.buildMessages(userMessage, history)
		wire := messagesToWire(msgs)

		var tools []wireTool
		if r.Registry != nil {
			for _, def := range r.Registry.Definitions() {
				fn, _ := def["function"].(map[string]any)
				var wt wireTool
				wt.Type = "function"
				wt.Function.Name, _ = fn["name"].(string)
				wt.Function.Description, _ = fn["description"].(string)
				wt.Function.Parameters, _ = fn["parameters"].(map[string]any)
				tools = append(tools, wt)
			}
		}

		maxSteps := cfg.MaxSteps
		if maxSteps <= 0 {
			maxSteps = 20
		}

		var finalText strings.Builder

		for step := 0; step < maxSteps; step++ {
			body := wireRequest{Model: cfg.Model, Messages: wire, Tools: tools}
			// Add stream: true.
			type streamRequest struct {
				wireRequest
				Stream bool `json:"stream"`
			}
			sr := streamRequest{wireRequest: body, Stream: true}
			raw, _ := json.Marshal(sr)

			endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions"
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
			if err != nil {
				errs <- err
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if cfg.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			}

			resp, err := client.Do(req)
			if err != nil {
				errs <- fmt.Errorf("stream step %d: %w", step, err)
				return
			}

			// Parse SSE stream.
			var (
				stepContent strings.Builder
				toolCalls   []wireToolCall
				finishReason string
			)
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content   string        `json:"content"`
							ToolCalls []wireToolCall `json:"tool_calls"`
						} `json:"delta"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
				}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}
				if len(chunk.Choices) == 0 {
					continue
				}
				c := chunk.Choices[0]
				if c.FinishReason != "" {
					finishReason = c.FinishReason
				}
				if c.Delta.Content != "" {
					stepContent.WriteString(c.Delta.Content)
					finalText.WriteString(c.Delta.Content)
					select {
					case tokens <- c.Delta.Content:
					case <-ctx.Done():
						resp.Body.Close()
						errs <- ctx.Err()
						return
					}
				}
				toolCalls = append(toolCalls, c.Delta.ToolCalls...)
			}
			resp.Body.Close()

			if finishReason == "tool_calls" || len(toolCalls) > 0 {
				// Append assistant message + execute tools (non-streaming).
				assistantMsg := wireMessage{
					Role:      "assistant",
					Content:   stepContent.String(),
					ToolCalls: toolCalls,
				}
				wire = append(wire, assistantMsg)
				for _, tc := range toolCalls {
					var args map[string]any
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
					result := fmt.Sprintf("no registry: tool %q not executed", tc.Function.Name)
					if r.Registry != nil {
						result, _ = r.Registry.Execute(ctx, tc.Function.Name, args)
					}
					wire = append(wire, wireMessage{
						Role:       "tool",
						Content:    result,
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
					})
				}
				continue
			}
			break // no tool calls → done
		}

		// Episodic memory.
		if r.Episodic != nil && r.SessionID != "" {
			_ = r.Episodic.Append(ctx, r.SessionID, "user", userMessage)
			if finalText.Len() > 0 {
				_ = r.Episodic.Append(ctx, r.SessionID, "assistant", finalText.String())
			}
		}

		if r.Callbacks.OnComplete != nil {
			r.Callbacks.OnComplete(finalText.String())
		}
		errs <- nil
	}()

	return tokens, errs
}

// reflexionCycle runs one critique→refine pass.
// It uses two cheap LLM calls (no tools) and returns the refined answer.
func (r *Runner) reflexionCycle(ctx context.Context, userMessage, currentAnswer string) (string, error) {
	critiqueSP := "You are a critical reviewer. Evaluate the assistant's answer for accuracy, completeness, and clarity. Be concise — one paragraph. If the answer is already excellent, say exactly: 'No improvement needed.'"

	// Step 1: critique.
	critiqueWire := []wireMessage{
		{Role: "system", Content: critiqueSP},
		{Role: "user", Content: fmt.Sprintf("Task: %s\n\nAnswer:\n%s", userMessage, currentAnswer)},
	}
	critiqueResp, err := r.callLLM(ctx, critiqueWire, nil)
	if err != nil {
		return "", err
	}
	if len(critiqueResp.Choices) == 0 {
		return "", nil
	}
	critique := contentString(critiqueResp.Choices[0].Message.Content)
	if strings.Contains(strings.ToLower(critique), "no improvement needed") {
		return currentAnswer, nil // signal: already good
	}

	// Step 2: refine based on critique.
	refineWire := []wireMessage{
		{Role: "system", Content: r.Config.SystemPrompt},
		{Role: "user", Content: userMessage},
		{Role: "assistant", Content: currentAnswer},
		{Role: "user", Content: "Please revise your answer based on this critique:\n\n" + critique},
	}
	refineResp, err := r.callLLM(ctx, refineWire, nil)
	if err != nil {
		return "", err
	}
	if len(refineResp.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(contentString(refineResp.Choices[0].Message.Content)), nil
}

// ─── HTTP helper ───────────────────────────────────────────────────────────

func (r *Runner) callLLM(ctx context.Context, msgs []wireMessage, tools []wireTool) (wireResponse, error) {
	body := wireRequest{
		Model:    r.Config.Model,
		Messages: msgs,
		Tools:    tools,
	}
	raw, _ := json.Marshal(body)

	endpoint := strings.TrimRight(r.Config.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return wireResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.Config.APIKey)
	}

	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return wireResponse{}, fmt.Errorf("LLM call: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode/100 != 2 {
		return wireResponse{}, fmt.Errorf("LLM http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out wireResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return wireResponse{}, fmt.Errorf("LLM decode: %w", err)
	}
	return out, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func (r *Runner) buildMessages(userMessage string, history []Message) []Message {
	sp := r.Config.SystemPrompt
	if sp == "" {
		sp = DefaultSystemPrompt
	}
	// ── Lesson 10: seed long-term memory into system prompt ─────────────────
	if r.Memory != nil && r.UserID != "" {
		if summary := agenttools.FactsSummary(r.Memory, r.UserID); summary != "" {
			sp += summary
		}
	}

	// ── Episodic memory: prepend session history snapshot ───────────────────
	if r.Episodic != nil && r.SessionID != "" {
		episodicSummary, recent := r.Episodic.Snapshot(r.SessionID)
		if episodicSummary != "" || len(recent) > 0 {
			var eb strings.Builder
			eb.WriteString("\n\n## Conversation history (this session):\n")
			if episodicSummary != "" {
				eb.WriteString("Summary of earlier turns: ")
				eb.WriteString(episodicSummary)
				eb.WriteString("\n\n")
			}
			for _, ep := range recent {
				fmt.Fprintf(&eb, "%s: %s\n", ep.Role, ep.Content)
			}
			sp += eb.String()
		}
	}
	msgs := []Message{{Role: "system", Content: sp}}
	msgs = append(msgs, history...)
	msgs = append(msgs, Message{Role: "user", Content: userMessage})
	return msgs
}

func messagesToWire(msgs []Message) []wireMessage {
	wire := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		wire[i] = wireMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
	}
	return wire
}

func wireToMessages(wire []wireMessage) []Message {
	msgs := make([]Message, len(wire))
	for i, w := range wire {
		msgs[i] = Message{
			Role:       w.Role,
			Content:    contentString(w.Content),
			ToolCallID: w.ToolCallID,
			Name:       w.Name,
		}
	}
	return msgs
}

func contentString(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func estimateWireTokens(wire []wireMessage) TokenUsage {
	msgs := wireToMessages(wire)
	return EstimateTokens(msgs)
}
