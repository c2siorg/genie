package opa

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
)

// -------------------------------------------------------------------
// PolicyConfig carries the runtime configuration injected as OPA data.
// The Rego policies reference these values as data.config.*.
// -------------------------------------------------------------------

// PolicyConfig is the structured configuration injected into OPA as
// data.config. It mirrors the fields in pkg/policy.AIPolicy that affect
// message-level decisions.
type PolicyConfig struct {
	// RBAC maps message type → allowed role list. Empty = allow all.
	RBAC map[string][]string `json:"rbac"`
	// HomeRegion is the data-residency home region tag (e.g. "in", "us").
	HomeRegion string `json:"home_region"`
	// AllowCrossBorderForPublic allows public-classified data to leave home.
	AllowCrossBorderForPublic bool `json:"allow_cross_border_for_public"`
	// AdminBypass makes the "admin" role skip RBAC checks.
	AdminBypass bool `json:"admin_bypass"`
	// MaxContentLength is the byte cap on message content. 0 = disabled.
	MaxContentLength int `json:"max_content_length"`
	// BlockPII enables PII pattern scanning.
	BlockPII bool `json:"block_pii"`
	// BlockPromptInjection enables prompt-injection phrase matching.
	BlockPromptInjection bool `json:"block_prompt_injection"`
	// RequiredMetadata maps message type → required metadata key list.
	RequiredMetadata map[string][]string `json:"required_metadata"`
	// AgentRings maps agentID → ring number (0-3). Missing agents = ring 3.
	AgentRings map[string]int `json:"agent_rings"`
	// ExplainabilityAppliesTo lists message types that require a 'rationale'
	// field in their JSON content (RBI FREE-AI Rec 22).
	ExplainabilityAppliesTo []string `json:"explainability_applies_to"`
}

// DefaultPolicyConfig returns sensible defaults matching the reference
// ai-policy.example.yaml.
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		HomeRegion:                "in",
		AllowCrossBorderForPublic: true,
		AdminBypass:               true,
		MaxContentLength:          256 * 1024,
		BlockPII:                  true,
		BlockPromptInjection:      true,
	}
}

// -------------------------------------------------------------------
// Engine
// -------------------------------------------------------------------

const (
	// msgAuthzQuery evaluates the full message-level authz document.
	msgAuthzQuery = "data.genie.authz"
	// httpAuthzQuery evaluates the HTTP-level authz document.
	httpAuthzQuery = "data.genie.http_authz"
)

// Engine implements governance.Policy using OPA.
// Rego policies are compiled once at startup (PreparedEvalQuery) and then
// evaluated concurrently for every message; no locks are required.
type Engine struct {
	msgQuery  rego.PreparedEvalQuery
	httpQuery rego.PreparedEvalQuery
	cfg       PolicyConfig
}

// New compiles the given Rego modules into a prepared query and returns an
// Engine ready to evaluate messages.
//
// modules is a map from a descriptive filename (used in error messages) to
// the Rego source text. Use LoadModules or EmbedModules to build this map.
func New(ctx context.Context, cfg PolicyConfig, modules map[string]string) (*Engine, error) {
	data := map[string]any{"config": cfg}
	store := inmem.NewFromObject(data)

	opts := []func(*rego.Rego){
		rego.Store(store),
		rego.SetRegoVersion(ast.RegoV1), // enable OPA v1 syntax: if, in, contains
	}
	for name, src := range modules {
		opts = append(opts, rego.Module(name, src))
	}

	// Message-level query.
	msgQ, err := rego.New(append(opts, rego.Query(msgAuthzQuery))...).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("opa: prepare message authz query: %w", err)
	}
	// HTTP-level query (optional — HTTP authz module may not be loaded).
	httpQ, err := rego.New(append(opts, rego.Query(httpAuthzQuery))...).PrepareForEval(ctx)
	if err != nil {
		// HTTP authz is optional; don't fail if the module is absent.
		httpQ = rego.PreparedEvalQuery{}
	}

	return &Engine{
		msgQuery:  msgQ,
		httpQuery: httpQ,
		cfg:       cfg,
	}, nil
}

// -------------------------------------------------------------------
// governance.Policy implementation
// -------------------------------------------------------------------

// Evaluate implements governance.Policy.
// It converts msg into an OPA input document, evaluates data.genie.authz,
// and returns a PolicyResult. On any OPA error the policy fails closed
// (DecisionDeny) — governance must not silently pass on infrastructure faults.
func (e *Engine) Evaluate(ctx context.Context, msg protocol.Message) (governance.PolicyResult, error) {
	rs, err := e.msgQuery.Eval(ctx, rego.EvalInput(messageToInput(msg)))
	if err != nil {
		return governance.PolicyResult{
			Decision:    governance.DecisionDeny,
			Reason:      fmt.Sprintf("opa: evaluation error (fail closed): %v", err),
			CheckedAt:   time.Now().UTC(),
			CheckedByID: "opa:genie.authz",
		}, nil // Return nil error; the deny itself carries the explanation.
	}

	allow, reasons := parseResult(rs)
	decision := governance.DecisionAllow
	reason := "opa: allowed"
	if !allow {
		decision = governance.DecisionDeny
		reason = strings.Join(reasons, "; ")
		if reason == "" {
			reason = "opa: denied (no reason provided)"
		}
	}

	return governance.PolicyResult{
		Decision:    decision,
		Reason:      reason,
		CheckedAt:   time.Now().UTC(),
		CheckedByID: "opa:genie.authz",
	}, nil
}

// Config returns the PolicyConfig the engine was built with.
func (e *Engine) Config() PolicyConfig { return e.cfg }

// -------------------------------------------------------------------
// HTTP authz (used by Middleware)
// -------------------------------------------------------------------

// HTTPAuthzInput is the input document for HTTP request-level authorization.
type HTTPAuthzInput struct {
	Method  string            `json:"method"`
	Path    []string          `json:"path"` // e.g. ["v1", "governance", "audit"]
	Headers map[string]string `json:"headers"`
	User    HTTPAuthzUser     `json:"user"`
}

// HTTPAuthzUser is the authenticated principal extracted from the JWT.
type HTTPAuthzUser struct {
	ID       string   `json:"id"`
	Roles    []string `json:"roles"`
	TenantID string   `json:"tenant_id"`
}

// EvaluateHTTP evaluates the HTTP-level authz policy.
// Returns (allowed, denyReason, error).
func (e *Engine) EvaluateHTTP(ctx context.Context, req HTTPAuthzInput) (bool, string, error) {
	rs, err := e.httpQuery.Eval(ctx, rego.EvalInput(req))
	if err != nil {
		return false, fmt.Sprintf("opa http_authz: eval error: %v", err), nil
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return false, "opa http_authz: empty result (fail closed)", nil
	}

	doc, ok := rs[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return false, "opa http_authz: unexpected result shape", nil
	}

	allow, _ := doc["allow"].(bool)
	reason, _ := doc["deny_reason"].(string)
	if allow {
		reason = ""
	} else if reason == "" {
		reason = "opa http_authz: denied"
	}
	return allow, reason, nil
}

// -------------------------------------------------------------------
// Internal helpers
// -------------------------------------------------------------------

// messageToInput converts a protocol.Message to the OPA input document shape.
func messageToInput(msg protocol.Message) map[string]any {
	meta := make(map[string]any, len(msg.Metadata))
	for k, v := range msg.Metadata {
		meta[k] = v
	}
	return map[string]any{
		"message": map[string]any{
			"id":       msg.ID,
			"from":     msg.From,
			"to":       msg.To,
			"role":     string(msg.Role),
			"type":     msg.Type,
			"content":  msg.Content,
			"metadata": meta,
		},
	}
}

// parseResult extracts (allow, denyReasons) from an OPA ResultSet.
// The function is tolerant of different set representations (OPA can return
// sets as []any or map[any]struct{} depending on context).
func parseResult(rs rego.ResultSet) (bool, []string) {
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return false, []string{"opa: empty result set (fail closed)"}
	}

	doc, ok := rs[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return false, []string{"opa: unexpected result type (fail closed)"}
	}

	allow, _ := doc["allow"].(bool)

	var reasons []string
	switch v := doc["deny_reasons"].(type) {
	case []any:
		for _, r := range v {
			if s, ok := r.(string); ok {
				reasons = append(reasons, s)
			}
		}
	case map[any]struct{}:
		for r := range v {
			if s, ok := r.(string); ok {
				reasons = append(reasons, s)
			}
		}
	}
	sort.Strings(reasons) // deterministic output for tests + logs
	return allow, reasons
}

// -------------------------------------------------------------------
// Module loaders
// -------------------------------------------------------------------

// LoadModules reads all *.rego files from dir (recursively) and returns a
// filename→source map suitable for passing to New.
func LoadModules(dir string) (map[string]string, error) {
	modules := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".rego") {
			return nil
		}
		// Skip test files when loading production policies.
		if strings.HasSuffix(path, "_test.rego") {
			return nil
		}
		src, ioErr := os.ReadFile(path)
		if ioErr != nil {
			return fmt.Errorf("LoadModules: %w", ioErr)
		}
		modules[path] = string(src)
		return nil
	})
	return modules, err
}

// EmbedModules reads all *.rego files (excluding _test.rego) from an
// embedded filesystem (go:embed) and returns a filename→source map.
// Usage:
//
//	//go:embed ../../policies/genie/*.rego
//	var policyFS embed.FS
//	modules, err := opa.EmbedModules(policyFS, "policies/genie")
func EmbedModules(fsys embed.FS, dir string) (map[string]string, error) {
	modules := make(map[string]string)
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("EmbedModules: read dir %q: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rego") {
			continue
		}
		if strings.HasSuffix(e.Name(), "_test.rego") {
			continue
		}
		path := dir + "/" + e.Name()
		data, err := fsys.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("EmbedModules: read %q: %w", path, err)
		}
		modules[e.Name()] = string(data)
	}
	return modules, nil
}
