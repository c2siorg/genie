package hitl

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
)

// ─── Rule ──────────────────────────────────────────────────────────────────

// RuleAction is the outcome of a matching Rule.
type RuleAction int

const (
	// RuleAllow auto-approves without human involvement.
	RuleAllow RuleAction = iota
	// RuleDeny auto-denies without human involvement.
	RuleDeny
	// RuleAskHuman escalates to the inner Approver.
	RuleAskHuman
)

// Rule defines a single approval policy rule. Rules are evaluated in order;
// the first match wins. A rule matches when:
//  1. ToolPattern matches the tool name (glob syntax, e.g. "delete_*"), AND
//  2. all ArgPatterns match their respective argument values (regex).
type Rule struct {
	// ToolPattern is a glob pattern for the tool name ("*" matches all).
	ToolPattern string
	// ArgPatterns maps argument keys to regex patterns.
	// All supplied patterns must match for the rule to fire.
	// An empty map matches any args.
	ArgPatterns map[string]string
	// Action is what happens when this rule matches.
	Action RuleAction
	// Reason is attached to the ApprovalDecision for audit purposes.
	Reason string

	// compiled regexps — populated lazily on first use.
	compiled map[string]*regexp.Regexp
}

// matches returns true when the rule applies to the given tool call.
func (r *Rule) matches(toolName string, args map[string]any) bool {
	ok, _ := filepath.Match(r.ToolPattern, toolName)
	if !ok {
		return false
	}
	if len(r.ArgPatterns) == 0 {
		return true
	}
	// compile once
	if r.compiled == nil {
		r.compiled = make(map[string]*regexp.Regexp, len(r.ArgPatterns))
		for k, pat := range r.ArgPatterns {
			if re, err := regexp.Compile(pat); err == nil {
				r.compiled[k] = re
			}
		}
	}
	for k, re := range r.compiled {
		val, ok := args[k]
		if !ok {
			return false
		}
		if !re.MatchString(fmt.Sprintf("%v", val)) {
			return false
		}
	}
	return true
}

// ─── PolicyApprover ────────────────────────────────────────────────────────

// PolicyApprover evaluates a list of Rules before deciding whether to
// auto-approve, auto-deny, or escalate to a human via an inner Approver.
//
// Example — safe reads auto-approve, risky shell commands ask a human,
// and deletions are blocked entirely:
//
//	pa := hitl.NewPolicyApprover(hitl.NewCLIApprover(nil, nil),
//	    hitl.Rule{ToolPattern: "read_*",        Action: hitl.RuleAllow},
//	    hitl.Rule{ToolPattern: "shell_command",  Action: hitl.RuleAskHuman},
//	    hitl.Rule{ToolPattern: "delete_*",       Action: hitl.RuleDeny},
//	    hitl.Rule{ToolPattern: "*",              Action: hitl.RuleAllow},
//	)
type PolicyApprover struct {
	inner Approver
	rules []Rule
}

// NewPolicyApprover creates a PolicyApprover. inner is called when a rule
// returns RuleAskHuman. If no rule matches, the request is escalated to inner.
func NewPolicyApprover(inner Approver, rules ...Rule) *PolicyApprover {
	return &PolicyApprover{inner: inner, rules: rules}
}

// RequestApproval evaluates rules in order and returns a decision.
// The first matching rule wins; no match escalates to the inner Approver.
func (p *PolicyApprover) RequestApproval(ctx context.Context, req ApprovalRequest) (bool, error) {
	for i := range p.rules {
		r := &p.rules[i]
		if !r.matches(req.ToolName, req.Args) {
			continue
		}
		switch r.Action {
		case RuleAllow:
			return true, nil
		case RuleDeny:
			return false, nil
		case RuleAskHuman:
			return p.inner.RequestApproval(ctx, req)
		}
	}
	// No rule matched — escalate to human (fail-safe).
	return p.inner.RequestApproval(ctx, req)
}

// ─── Default rule sets ─────────────────────────────────────────────────────

// DefaultRules returns an opinionated rule set for general agent usage:
//   - read_* / list_* / get_* / search_*       → auto-approve (read-only)
//   - write_* / create_* / update_* / send_* / post_* → ask human
//   - delete_* / remove_* / drop_*             → ask human (destructive)
//   - shell_* / exec_*                          → ask human (arbitrary exec)
//   - everything else                           → ask human (fail-safe)
func DefaultRules() []Rule {
	return []Rule{
		{ToolPattern: "read_*",   Action: RuleAllow,    Reason: "read-only: auto-approved"},
		{ToolPattern: "list_*",   Action: RuleAllow,    Reason: "read-only: auto-approved"},
		{ToolPattern: "get_*",    Action: RuleAllow,    Reason: "read-only: auto-approved"},
		{ToolPattern: "search_*", Action: RuleAllow,    Reason: "read-only: auto-approved"},
		{ToolPattern: "delete_*", Action: RuleAskHuman, Reason: "destructive: requires approval"},
		{ToolPattern: "remove_*", Action: RuleAskHuman, Reason: "destructive: requires approval"},
		{ToolPattern: "drop_*",   Action: RuleAskHuman, Reason: "destructive: requires approval"},
		{ToolPattern: "shell_*",  Action: RuleAskHuman, Reason: "shell execution: requires approval"},
		{ToolPattern: "exec_*",   Action: RuleAskHuman, Reason: "exec: requires approval"},
		{ToolPattern: "write_*",  Action: RuleAskHuman, Reason: "write: requires approval"},
		{ToolPattern: "send_*",   Action: RuleAskHuman, Reason: "send: requires approval"},
		{ToolPattern: "post_*",   Action: RuleAskHuman, Reason: "post: requires approval"},
		{ToolPattern: "*",        Action: RuleAskHuman, Reason: "unknown tool: requires approval"},
	}
}
