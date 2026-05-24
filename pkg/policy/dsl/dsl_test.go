package dsl

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

func msg(content string, meta map[string]any) protocol.Message {
	return protocol.Message{
		ID: "m", From: "u", To: "agent", Role: protocol.RoleUser,
		Type: "finance_question", Content: content, Metadata: meta,
	}
}

func TestEqualsLiteral(t *testing.T) {
	rules, err := Compile([]Rule{{ID: "r", When: `type == "finance_question"`, Decision: "deny", Reason: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), msg("q", nil))
	if res.Decision != governance.DecisionDeny {
		t.Errorf("expected deny, got %s", res.Decision)
	}
}

func TestNotEquals(t *testing.T) {
	rules, _ := Compile([]Rule{{ID: "r", When: `type != "finance_question"`, Decision: "deny"}})
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), msg("q", nil))
	if res.Decision != governance.DecisionAllow {
		t.Errorf("expected allow, got %s", res.Decision)
	}
}

func TestMetadataLookup(t *testing.T) {
	rules, _ := Compile([]Rule{{
		ID:       "offshore_pii",
		When:     `classification == "pii" AND metadata.region != "in"`,
		Decision: "deny",
		Reason:   "PII bound for non-home region",
	}})
	m := msg("x", map[string]any{"classification": "pii", "region": "us"})
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), m)
	if res.Decision != governance.DecisionDeny {
		t.Errorf("expected deny on offshore PII; got %s (%s)", res.Decision, res.Reason)
	}
}

func TestMetadataLookupAllow(t *testing.T) {
	rules, _ := Compile([]Rule{{
		ID:       "offshore_pii",
		When:     `classification == "pii" AND metadata.region != "in"`,
		Decision: "deny",
	}})
	m := msg("x", map[string]any{"classification": "pii", "region": "in"})
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), m)
	if res.Decision != governance.DecisionAllow {
		t.Errorf("expected allow (PII in home region); got %s", res.Decision)
	}
}

func TestOrParens(t *testing.T) {
	rules, _ := Compile([]Rule{{
		ID:       "blocked_types",
		When:     `(type == "raw_dump" OR type == "secret_export") AND metadata.tier != "admin"`,
		Decision: "deny",
	}})
	m := msg("x", map[string]any{"tier": "user"})
	m.Type = "raw_dump"
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), m)
	if res.Decision != governance.DecisionDeny {
		t.Errorf("expected deny on raw_dump for non-admin; got %s", res.Decision)
	}
}

func TestNot(t *testing.T) {
	rules, _ := Compile([]Rule{{
		ID:       "demand_trace",
		When:     `NOT metadata.trace_id == "present"`,
		Decision: "deny",
		Reason:   "missing trace",
	}})
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), msg("x", nil))
	if res.Decision != governance.DecisionDeny {
		t.Errorf("expected deny when trace absent; got %s", res.Decision)
	}
}

func TestContains(t *testing.T) {
	rules, _ := Compile([]Rule{{
		ID:       "block_keyword",
		When:     `content contains "DROP TABLE"`,
		Decision: "deny",
	}})
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), msg("hi; DROP TABLE users;", nil))
	if res.Decision != governance.DecisionDeny {
		t.Errorf("expected deny on SQL injection keyword; got %s", res.Decision)
	}
}

func TestStartsWith(t *testing.T) {
	rules, _ := Compile([]Rule{{
		ID: "r", When: `from startsWith "ext-"`, Decision: "deny",
	}})
	m := msg("x", nil)
	m.From = "ext-thirdparty"
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), m)
	if res.Decision != governance.DecisionDeny {
		t.Errorf("expected deny on external prefix; got %s", res.Decision)
	}
}

func TestNumericCompare(t *testing.T) {
	rules, _ := Compile([]Rule{{
		ID: "r", When: `metadata.amount > 100000`, Decision: "deny",
	}})
	m := msg("x", map[string]any{"amount": 200000})
	res, _ := rules[0].AsPolicy().Evaluate(context.Background(), m)
	if res.Decision != governance.DecisionDeny {
		t.Errorf("expected deny on amount > threshold; got %s", res.Decision)
	}
}

func TestParseError(t *testing.T) {
	_, err := Compile([]Rule{{ID: "r", When: `type ==`, Decision: "deny"}})
	if err == nil {
		t.Errorf("expected parse error on truncated expression")
	}
}

func TestAsPoliciesAdapter(t *testing.T) {
	rules, _ := Compile([]Rule{
		{ID: "r1", When: `type == "x"`, Decision: "deny"},
		{ID: "r2", When: `type == "y"`, Decision: "deny"},
	})
	pols := AsPolicies(rules)
	if len(pols) != 2 {
		t.Errorf("expected 2 wrapped policies, got %d", len(pols))
	}
}
