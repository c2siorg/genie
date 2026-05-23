// Package checklist runs behavioral tests over an agent: canonical inputs,
// expected substrings or schemas in the output.
//
// Inspired by Ribeiro et al.'s CheckList (2020). Different from `tests/` —
// these run in CI *and* as a /v1/checklist endpoint so SREs can fire them
// against a live agent.
package checklist

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

// Case is one behavior test.
type Case struct {
	Name           string
	Input          agent.Message
	WantContains   []string // every substring must appear in some output
	WantNoErrors   bool     // HandleMessage must not return an error
	MinOutputCount int      // minimum number of output messages
}

// Result records one outcome.
type Result struct {
	Name    string
	Passed  bool
	Detail  string
}

// Run executes every case against the agent and returns aggregated results.
func Run(ctx context.Context, a agent.Agent, env agent.Environment, cases []Case) []Result {
	out := make([]Result, 0, len(cases))
	for _, c := range cases {
		r := Result{Name: c.Name, Passed: true}
		got, err := a.HandleMessage(ctx, c.Input, env)
		if err != nil && c.WantNoErrors {
			r.Passed = false
			r.Detail = "unexpected error: " + err.Error()
			out = append(out, r)
			continue
		}
		if c.MinOutputCount > 0 && len(got) < c.MinOutputCount {
			r.Passed = false
			r.Detail = fmt.Sprintf("min outputs: want >=%d, got %d", c.MinOutputCount, len(got))
			out = append(out, r)
			continue
		}
		for _, want := range c.WantContains {
			if !anyContains(got, want) {
				r.Passed = false
				r.Detail = "missing substring: " + want
				break
			}
		}
		out = append(out, r)
	}
	return out
}

func anyContains(msgs []agent.Message, want string) bool {
	for _, m := range msgs {
		if strings.Contains(m.Content, want) {
			return true
		}
	}
	return false
}

// AllPassed returns nil if every result passed; an error otherwise.
func AllPassed(results []Result) error {
	var failures []string
	for _, r := range results {
		if !r.Passed {
			failures = append(failures, r.Name+": "+r.Detail)
		}
	}
	if len(failures) > 0 {
		return errors.New("checklist failures: " + strings.Join(failures, "; "))
	}
	return nil
}
