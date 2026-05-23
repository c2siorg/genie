// Package auditor implements the ADK LLM Auditor role for Genie.
//
// It is a broadcast subscriber that records every agent message into an
// eval.Store, computes lightweight quality signals (empty content, missing
// required metadata, suspicious content length), and flags issues.
//
// Wire it via bus.Subscribe("", auditor.NewHandler(store)) or register the
// agent variant if you want it to live in the registry.
package auditor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/constitution"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

const (
	ID      = "llm_auditor"
	CapAudit = "audit_messages"
	TypeIn   = "audit_request"
	TypeOut  = "audit_result"
)

// Agent is the registry form (so capability lookups find it). When using the
// broadcast subscription, you only need NewHandler.
//
// LLM-as-judge mode is engaged when both LLM and Constitution are set.
type Agent struct {
	Store        eval.Store
	LLM          llm.Provider
	Constitution *constitution.Constitution
	Model        string
}

func New(store eval.Store) *Agent { return &Agent{Store: store} }

// WithJudge enables LLM-as-judge scoring against the supplied constitution.
func (a *Agent) WithJudge(p llm.Provider, c *constitution.Constitution, model string) *Agent {
	a.LLM = p
	a.Constitution = c
	a.Model = model
	return a
}

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "LLM Auditor" }
func (a *Agent) Capabilities() []string { return []string{CapAudit} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	flags := audit(msg)
	body := strings.Join(flags, "; ")
	if body == "" {
		body = "ok"
	}

	metrics := map[string]float64{}
	// LLM-as-judge: score the candidate output against the constitution.
	if a.LLM != nil && a.Constitution != nil {
		if v, err := a.Constitution.Critique(ctx, a.LLM, a.Model, msg.Content); err == nil {
			metrics["constitution_score"] = float64(v.Score)
			if v.Score < 6 {
				flags = append(flags, "constitution_score_low: "+v.Reasoning)
				body = strings.Join(flags, "; ")
			}
		}
	}

	_ = a.Store.Save(eval.InteractionRecord{
		ID:        msg.ID,
		Scenario:  "audit",
		Success:   len(flags) == 0,
		Metrics:   metrics,
		Metadata:  msg.Metadata,
		StartedAt: time.Now().UTC(),
		EndedAt:   time.Now().UTC(),
	})
	env.Logf("[auditor] %s -> %s", msg.ID, body)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleEvaluator, TypeOut, body, msg.Metadata),
	}, nil
}

// NewHandler returns a broadcast-style handler that records every message that
// crosses the bus. Hand it to bus.Subscribe("", ...).
func NewHandler(store eval.Store) comm.Handler {
	return func(ctx context.Context, msg protocol.Message) {
		flags := audit(msg)
		_ = store.Save(eval.InteractionRecord{
			ID:       fmt.Sprintf("audit-%s", msg.ID),
			Scenario: "bus.broadcast",
			Success:  len(flags) == 0,
			Metadata: map[string]any{
				"from":  msg.From,
				"to":    msg.To,
				"type":  msg.Type,
				"flags": strings.Join(flags, ","),
			},
			StartedAt: msg.CreatedAt,
			EndedAt:   time.Now().UTC(),
		})
	}
}

// audit returns a slice of human-readable issue strings for the given message.
// Empty slice means clean.
func audit(msg protocol.Message) []string {
	var flags []string
	if strings.TrimSpace(msg.Content) == "" {
		flags = append(flags, "empty_content")
	}
	if msg.From == "" || msg.To == "" {
		flags = append(flags, "missing_address")
	}
	if len(msg.Content) > 16*1024 {
		flags = append(flags, "oversized_content")
	}
	return flags
}
