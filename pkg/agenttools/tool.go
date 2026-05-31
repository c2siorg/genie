// Package agenttools provides the tool interface and built-in tool implementations
// for AI agents: file system (lesson 06), shell / code execution (lesson 08),
// and web search (lesson 07).
//
// Every tool follows the same contract: name, description, JSON schema, and an
// Execute method. The agent loop never calls Execute itself — it calls the tool
// by name via the Registry, mirrors the TypeScript executeTool pattern exactly.
package agenttools

import (
	"context"
	"fmt"
)

// ─── Tool interface ────────────────────────────────────────────────────────

// Tool is the core interface every agent tool must satisfy.
//
// Mirrors the TypeScript anatomy:
//
//	{
//	  description: "...",           → Description()
//	  inputSchema: z.object({...}), → Schema()
//	  execute: async (args) => {…}, → Execute(ctx, args)
//	}
type Tool interface {
	// Name is the identifier the LLM uses to call this tool.
	Name() string
	// Description tells the model when to use the tool. Be specific.
	Description() string
	// Schema is the JSON Schema object describing accepted parameters.
	// The LLM uses this to generate valid arguments.
	Schema() map[string]any
	// Execute runs the tool with the given arguments.
	// It should return a string result (or an error string on failure).
	// Errors are returned as strings so the agent can handle them gracefully.
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// ─── ToolDef helper ────────────────────────────────────────────────────────

// ToolDef is a convenience struct for defining tools without a full type.
type ToolDef struct {
	ToolName        string
	ToolDescription string
	ToolSchema      map[string]any
	Fn              func(ctx context.Context, args map[string]any) (string, error)
}

func (t *ToolDef) Name() string                { return t.ToolName }
func (t *ToolDef) Description() string         { return t.ToolDescription }
func (t *ToolDef) Schema() map[string]any      { return t.ToolSchema }
func (t *ToolDef) Execute(ctx context.Context, args map[string]any) (string, error) {
	return t.Fn(ctx, args)
}

// ─── Registry ──────────────────────────────────────────────────────────────

// Registry holds a named set of tools. The agent loop uses it to look up and
// execute tools by name, mirroring executeTool(name, args) in TypeScript.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry. Panics on duplicate name.
func (r *Registry) Register(t Tool) *Registry {
	if _, exists := r.tools[t.Name()]; exists {
		panic(fmt.Sprintf("agenttools: duplicate tool name %q", t.Name()))
	}
	r.tools[t.Name()] = t
	return r
}

// Execute runs the named tool with the given args. Returns an error string
// if the tool is not found (never throws) so the agent can handle it.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("unknown tool: %q", name), nil
	}
	return t.Execute(ctx, args)
}

// Definitions returns the list of tool definitions in OpenAI tool format.
func (r *Registry) Definitions() []map[string]any {
	defs := make([]map[string]any, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  t.Schema(),
			},
		})
	}
	return defs
}

// Names returns all registered tool names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
