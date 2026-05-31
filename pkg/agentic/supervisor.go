// supervisor.go — Lesson 11: Multi-Agent Supervisor Pattern
//
// A Supervisor routes tasks to the most appropriate sub-agent using a two-step
// LLM call:
//
//  1. Routing step — the supervisor LLM reads each sub-agent's description
//     and returns a JSON routing decision: {"agent": "name", "reason": "..."}
//  2. Delegation step — the chosen sub-agent's Runner executes the task with
//     its own system prompt, tools, and config.
//
// Usage:
//
//	sup := &agentic.Supervisor{
//	    Agents: []agentic.SubAgent{
//	        {Name: "file_agent",  Description: "Reads and writes files",      Registry: fileReg},
//	        {Name: "shell_agent", Description: "Runs shell commands and code", Registry: shellReg},
//	    },
//	    Config: agentic.DefaultConfig(),
//	}
//	text, agentUsed, history, err := sup.Run(ctx, "Read config.json and show me the port", nil)
//
// For multi-agent streaming, set sup.Callbacks.OnToken — the supervisor
// emits a [routing to <agent>] prefix then forwards the sub-agent stream.
package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agenttools"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/hitl"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/memory"
)

// RoutingTimeout is the maximum time the supervisor will wait for the LLM to
// return a routing decision. Kept short because routing is a lightweight call
// (no tools, system prompt only). Override via Supervisor.RoutingTimeout.
const RoutingTimeout = 15 * time.Second

// SubAgent is a named specialist that the Supervisor can delegate to.
type SubAgent struct {
	// Name is the identifier the routing LLM uses to select this agent.
	Name string
	// Description tells the routing LLM what this agent specialises in.
	// Keep it concise and distinctive — the router reads all descriptions at once.
	Description string
	// SystemPrompt overrides the default system prompt for this sub-agent.
	// Leave empty to use the supervisor's Config.SystemPrompt.
	SystemPrompt string
	// Registry contains the tools available to this sub-agent. Nil = no tools.
	Registry *agenttools.Registry
	// Approver gates tool calls for this sub-agent (lesson 09). Nil = auto-approve.
	Approver hitl.Approver
	// Memory wires long-term memory into this sub-agent (lesson 10). Nil = no memory.
	Memory *memory.LongTermMemory
	// UserID scopes memory to a user.
	UserID string
}

// Supervisor orchestrates a pool of sub-agents.
// It uses one LLM call to route the task, then delegates to the chosen agent.
type Supervisor struct {
	// Agents is the pool of available sub-agents.
	Agents []SubAgent
	// Config is used for the routing step and as the default sub-agent config.
	Config Config
	// Callbacks are forwarded to the chosen sub-agent's Runner.
	Callbacks Callbacks
	// FallbackAgent is the agent name to use when routing fails or returns an
	// unknown name. Leave empty to return an error instead.
	FallbackAgent string
	// RoutingTimeout caps the LLM call used to pick a sub-agent.
	// Defaults to RoutingTimeout (15s) when zero.
	RoutingTimeout time.Duration
}

// routingDecision is the JSON the routing LLM returns.
type routingDecision struct {
	Agent  string `json:"agent"`
	Reason string `json:"reason"`
}

// Route asks the LLM to pick the best sub-agent for the task.
// Returns (agentName, reason, error).
func (s *Supervisor) Route(ctx context.Context, task string) (string, string, error) {
	if len(s.Agents) == 0 {
		return "", "", fmt.Errorf("supervisor: no sub-agents registered")
	}
	if len(s.Agents) == 1 {
		// Trivially route to the only agent — no LLM call needed.
		return s.Agents[0].Name, "only agent available", nil
	}

	// Build the routing system prompt.
	var sb strings.Builder
	sb.WriteString("You are a routing supervisor. Select the BEST agent for the user's task.\n\n")
	sb.WriteString("Available agents:\n")
	for _, a := range s.Agents {
		fmt.Fprintf(&sb, "- %s: %s\n", a.Name, a.Description)
	}
	sb.WriteString("\nRespond with ONLY a JSON object — no markdown, no explanation:\n")
	sb.WriteString(`{"agent": "<exact agent name>", "reason": "<one sentence>"}`)

	routeCfg := s.Config
	routeCfg.SystemPrompt = sb.String()
	routeCfg.MaxSteps = 1 // routing never needs tool calls

	// Apply a tight timeout so a slow/hung Ollama call doesn't block forever.
	timeout := s.RoutingTimeout
	if timeout <= 0 {
		timeout = RoutingTimeout
	}
	routeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := RunAgent(routeCtx, task, nil, routeCfg)
	if err != nil {
		return "", "", fmt.Errorf("supervisor: routing LLM: %w", err)
	}

	// Extract JSON from the response — the model may wrap it in text.
	jsonStr := extractJSON(resp)
	var dec routingDecision
	if err := json.Unmarshal([]byte(jsonStr), &dec); err != nil {
		// Graceful degradation: scan response for a known agent name.
		lower := strings.ToLower(resp)
		for _, a := range s.Agents {
			if strings.Contains(lower, strings.ToLower(a.Name)) {
				return a.Name, "matched by name in response", nil
			}
		}
		return "", "", fmt.Errorf("supervisor: parse routing response %q: %w", resp, err)
	}
	if dec.Agent == "" {
		return "", "", fmt.Errorf("supervisor: routing returned empty agent name")
	}
	return dec.Agent, dec.Reason, nil
}

// Run routes the task to the best sub-agent and executes it.
//
// Returns: (responseText, agentNameUsed, updatedHistory, error).
//
// When Callbacks.OnToken is set, the supervisor emits a routing prefix
// followed by the sub-agent's token stream.
func (s *Supervisor) Run(ctx context.Context, task string, history []Message) (string, string, []Message, error) {
	agentName, reason, err := s.Route(ctx, task)
	if err != nil {
		if s.FallbackAgent != "" {
			agentName = s.FallbackAgent
			reason = "routing failed; using fallback"
		} else {
			return "", "", nil, err
		}
	}

	// Find the sub-agent by name.
	chosen, found := s.findAgent(agentName)
	if !found {
		if s.FallbackAgent != "" && agentName != s.FallbackAgent {
			chosen, found = s.findAgent(s.FallbackAgent)
		}
		if !found {
			return "", "", nil, fmt.Errorf("supervisor: unknown agent %q (routing: %s)", agentName, reason)
		}
		agentName = chosen.Name
	}

	// Announce routing decision to caller.
	if s.Callbacks.OnToken != nil {
		s.Callbacks.OnToken(fmt.Sprintf("[→ %s: %s]\n", agentName, reason))
	}

	// Build the sub-agent runner.
	cfg := s.Config
	if chosen.SystemPrompt != "" {
		cfg.SystemPrompt = chosen.SystemPrompt
	}

	runner := &Runner{
		Config:    cfg,
		Registry:  chosen.Registry,
		Approver:  chosen.Approver,
		Memory:    chosen.Memory,
		UserID:    chosen.UserID,
		Callbacks: s.Callbacks,
	}

	text, updatedHistory, err := runner.Run(ctx, task, history)
	return text, agentName, updatedHistory, err
}

// findAgent returns the SubAgent with the given name, or (zero, false).
func (s *Supervisor) findAgent(name string) (SubAgent, bool) {
	for _, a := range s.Agents {
		if a.Name == name {
			return a, true
		}
	}
	return SubAgent{}, false
}

// extractJSON extracts the first {...} block from a string.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
