// Package moa_recommender implements Mixture-of-Agents — multiple
// recommender variants vote on each output, the agent emits the
// majority/highest-confidence pick.
//
// Useful when you want robustness without trusting a single model. In
// production each panellist is a different LLM provider or prompt version.
package moa_recommender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

const (
	ID         = "moa_recommender"
	Capability = "moa_recommend"
	TypeIn     = "moa_recommend_request"
	TypeOut    = "moa_recommendation"
	NextAgent  = "financial_supervisor"
)

// Panellist is one model+prompt pair that produces a candidate answer.
type Panellist struct {
	Name     string
	Provider llm.Provider
	Model    string
	System   string
}

// Agent runs all panellists in parallel and votes.
type Agent struct {
	Panel []Panellist
}

// New constructs the MoA agent with the supplied panel.
func New(panel ...Panellist) *Agent { return &Agent{Panel: panel} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Mixture-of-Agents Recommender" }
func (a *Agent) Capabilities() []string { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

// Outcome is the structured response.
type Outcome struct {
	Winner    string           `json:"winner"`     // panellist name
	WinnerText string          `json:"winner_text"`
	Votes     map[string]int   `json:"votes"`      // candidate -> vote count
	Candidates []Candidate     `json:"candidates"`
}

// Candidate is one panellist's answer.
type Candidate struct {
	Panellist string `json:"panellist"`
	Text      string `json:"text"`
	Error     string `json:"error,omitempty"`
}

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	if len(a.Panel) == 0 {
		return nil, errors.New("moa_recommender: empty panel")
	}

	type result struct {
		idx int
		c   Candidate
	}
	out := make(chan result, len(a.Panel))
	for i, p := range a.Panel {
		i, p := i, p
		go func() {
			resp, err := p.Provider.Complete(ctx, llm.CompletionRequest{
				Model: p.Model,
				Messages: []llm.Message{
					{Role: llm.RoleSystem, Content: p.System},
					{Role: llm.RoleUser, Content: msg.Content},
				},
				MaxTokens:   256,
				Temperature: 0.2,
				Residency:   llm.Residency{AllowCrossBorder: true},
			})
			c := Candidate{Panellist: p.Name}
			if err != nil {
				c.Error = err.Error()
			} else {
				c.Text = resp.Text
			}
			out <- result{idx: i, c: c}
		}()
	}
	candidates := make([]Candidate, len(a.Panel))
	for range a.Panel {
		r := <-out
		candidates[r.idx] = r.c
	}

	// Vote: count textually identical outputs (after trimming).
	votes := map[string]int{}
	for _, c := range candidates {
		if c.Error != "" || c.Text == "" {
			continue
		}
		votes[c.Text]++
	}
	winnerText := ""
	winnerName := ""
	best := 0
	for _, c := range candidates {
		if v, ok := votes[c.Text]; ok && v > best {
			best = v
			winnerText = c.Text
			winnerName = c.Panellist
		}
	}
	env.Logf("[moa] %d candidates → winner=%q votes=%d", len(candidates), winnerName, best)

	body, _ := json.Marshal(Outcome{
		Winner: winnerName, WinnerText: winnerText, Votes: votes, Candidates: candidates,
	})
	if winnerText == "" {
		return nil, fmt.Errorf("moa_recommender: all panellists failed")
	}
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}
