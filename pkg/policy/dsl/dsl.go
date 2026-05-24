// Package dsl is a tiny expression DSL for board-approved policies.
//
// The use case: FREE-AI Rec 6 (Adaptive Policies) wants the risk team to be
// able to add a new rule without engineering involvement. Today every rule
// in pkg/governance is a Go struct — the team has to ship code. With this
// DSL they can drop a YAML file in config/policies/ that looks like:
//
//	- id: deny_offshore_pii
//	  when: classification == "pii" AND metadata.region != "in"
//	  decision: deny
//	  reason: "PII bound for non-home region"
//
// and the system loads it at boot.
//
// The DSL deliberately covers a small surface — comparison, boolean
// composition, string `contains`, dotted metadata access. If a rule needs
// more, write a Go-side governance.Policy. The goal is "let the risk team
// own 80 % of rules" not "let the risk team write Turing-complete code."
//
// Inspired by Google ADK samples → policy-as-code. CEL was considered but
// rejected to keep the dependency graph small; if cel-go is added later,
// this package's Evaluator can be swapped for it without changing callers.
package dsl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// Rule is one parsed policy rule.
type Rule struct {
	ID       string `yaml:"id" json:"id"`
	When     string `yaml:"when" json:"when"`         // boolean expression
	Decision string `yaml:"decision" json:"decision"` // "allow" | "deny"
	Reason   string `yaml:"reason" json:"reason"`
}

// Compile parses a rule's `when` expression once at load time.
// Returns an error per rule so the caller can keep going on the rest.
func Compile(rules []Rule) ([]CompiledRule, error) {
	out := make([]CompiledRule, 0, len(rules))
	for _, r := range rules {
		expr, err := parse(r.When)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", r.ID, err)
		}
		out = append(out, CompiledRule{Rule: r, expr: expr})
	}
	return out, nil
}

// CompiledRule is a parsed rule ready to evaluate.
type CompiledRule struct {
	Rule
	expr expr
}

// AsPolicy wraps a compiled rule as a governance.Policy so it can be
// dropped into the existing composite stack.
func (c CompiledRule) AsPolicy() governance.Policy {
	return rulePolicy{c: c}
}

// AsPolicies wraps a slice of compiled rules — typical loader output.
func AsPolicies(rules []CompiledRule) []governance.Policy {
	out := make([]governance.Policy, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.AsPolicy())
	}
	return out
}

type rulePolicy struct{ c CompiledRule }

func (p rulePolicy) Evaluate(_ context.Context, msg protocol.Message) (governance.PolicyResult, error) {
	ok, err := p.c.expr.eval(msg)
	if err != nil {
		return governance.PolicyResult{}, fmt.Errorf("rule %s: %w", p.c.ID, err)
	}
	if !ok {
		return governance.PolicyResult{Decision: governance.DecisionAllow, Reason: "rule not matched", CheckedAt: time.Now().UTC(), CheckedByID: p.c.ID}, nil
	}
	d := governance.DecisionDeny
	if strings.EqualFold(p.c.Decision, "allow") {
		d = governance.DecisionAllow
	}
	reason := p.c.Reason
	if reason == "" {
		reason = "matched rule " + p.c.ID
	}
	return governance.PolicyResult{Decision: d, Reason: reason, CheckedAt: time.Now().UTC(), CheckedByID: p.c.ID}, nil
}

// ----------------------------------------------------------------------------
// expression AST + evaluator
// ----------------------------------------------------------------------------

type expr interface {
	eval(msg protocol.Message) (bool, error)
}

type andExpr struct{ l, r expr }
type orExpr struct{ l, r expr }
type notExpr struct{ e expr }
type cmpExpr struct {
	left, right token
	op          string // == != < > <= >= contains startsWith
}

func (a andExpr) eval(m protocol.Message) (bool, error) {
	lv, err := a.l.eval(m)
	if err != nil || !lv {
		return false, err
	}
	return a.r.eval(m)
}
func (o orExpr) eval(m protocol.Message) (bool, error) {
	lv, err := o.l.eval(m)
	if err != nil {
		return false, err
	}
	if lv {
		return true, nil
	}
	return o.r.eval(m)
}
func (n notExpr) eval(m protocol.Message) (bool, error) {
	v, err := n.e.eval(m)
	return !v, err
}

func (c cmpExpr) eval(m protocol.Message) (bool, error) {
	left, err := c.left.resolve(m)
	if err != nil {
		return false, err
	}
	right, err := c.right.resolve(m)
	if err != nil {
		return false, err
	}
	switch c.op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	case "contains":
		return strings.Contains(left, right), nil
	case "startsWith":
		return strings.HasPrefix(left, right), nil
	case "<", ">", "<=", ">=":
		lf, err1 := strconv.ParseFloat(left, 64)
		rf, err2 := strconv.ParseFloat(right, 64)
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("numeric compare requires numbers, got %q %s %q", left, c.op, right)
		}
		switch c.op {
		case "<":
			return lf < rf, nil
		case ">":
			return lf > rf, nil
		case "<=":
			return lf <= rf, nil
		case ">=":
			return lf >= rf, nil
		}
	}
	return false, fmt.Errorf("unknown operator %q", c.op)
}

// token is either a string literal, a numeric literal, or a field reference.
type token struct {
	kind  string // "literal" | "field"
	value string
}

func (t token) resolve(m protocol.Message) (string, error) {
	if t.kind == "literal" {
		return t.value, nil
	}
	// field reference: classification | type | from | to | role | content | metadata.<key>
	switch {
	case t.value == "classification":
		// no classification on Message in this repo's protocol — read from metadata.
		return metaString(m, "classification"), nil
	case t.value == "type":
		return m.Type, nil
	case t.value == "from":
		return m.From, nil
	case t.value == "to":
		return m.To, nil
	case t.value == "role":
		return string(m.Role), nil
	case t.value == "content":
		return m.Content, nil
	case strings.HasPrefix(t.value, "metadata."):
		key := strings.TrimPrefix(t.value, "metadata.")
		return metaString(m, key), nil
	}
	return "", fmt.Errorf("unknown field %q", t.value)
}

func metaString(m protocol.Message, key string) string {
	if m.Metadata == nil {
		return ""
	}
	v, ok := m.Metadata[key]
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprintf("%v", v)
}

// ----------------------------------------------------------------------------
// parser — recursive descent
//
// Grammar:
//   expr     = orExpr
//   orExpr   = andExpr ("OR" andExpr)*
//   andExpr  = unary ("AND" unary)*
//   unary    = "NOT" unary | primary
//   primary  = "(" expr ")" | cmp
//   cmp      = term op term
//   term     = stringLit | numLit | identifier
//   op       = "==" | "!=" | "<" | ">" | "<=" | ">=" | "contains" | "startsWith"
// ----------------------------------------------------------------------------

func parse(s string) (expr, error) {
	if strings.TrimSpace(s) == "" {
		return nil, errors.New("empty expression")
	}
	p := &parser{toks: tokenise(s)}
	e, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.toks) {
		return nil, fmt.Errorf("unexpected trailing token %q", p.toks[p.pos])
	}
	return e, nil
}

type parser struct {
	toks []string
	pos  int
}

func (p *parser) peek() string {
	if p.pos >= len(p.toks) {
		return ""
	}
	return p.toks[p.pos]
}
func (p *parser) eat() string {
	t := p.peek()
	p.pos++
	return t
}

func (p *parser) parseOr() (expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek(), "OR") {
		p.eat()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = orExpr{l: left, r: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek(), "AND") {
		p.eat()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = andExpr{l: left, r: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (expr, error) {
	if strings.EqualFold(p.peek(), "NOT") {
		p.eat()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return notExpr{e: inner}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (expr, error) {
	if p.peek() == "(" {
		p.eat()
		e, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.eat() != ")" {
			return nil, errors.New("missing closing )")
		}
		return e, nil
	}
	return p.parseCmp()
}

func (p *parser) parseCmp() (expr, error) {
	left := p.eat()
	op := p.eat()
	right := p.eat()
	if left == "" || op == "" || right == "" {
		return nil, fmt.Errorf("incomplete comparison near %q %q %q", left, op, right)
	}
	return cmpExpr{
		left:  classify(left),
		right: classify(right),
		op:    op,
	}, nil
}

// classify turns a raw token into either a literal or a field reference.
// Convention: quoted strings ("..." or '...') are literals; bare identifiers
// are field references; numeric tokens are numeric literals.
func classify(s string) token {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return token{kind: "literal", value: s[1 : len(s)-1]}
		}
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return token{kind: "literal", value: s}
	}
	return token{kind: "field", value: s}
}

// tokenise splits on whitespace while keeping quoted strings and parens intact.
func tokenise(s string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote != 0:
			cur.WriteByte(c)
			if c == inQuote {
				flush()
				inQuote = 0
			}
		case c == '"' || c == '\'':
			flush()
			inQuote = c
			cur.WriteByte(c)
		case c == '(' || c == ')':
			flush()
			out = append(out, string(c))
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}
