// Package afg is Genie's integration layer over the Microsoft Agent Framework for
// Go (github.com/microsoft/agent-framework-go). It is the seed of the in-place
// rewrite: agents become framework agents, but Genie's load-bearing governance gate
// is re-imposed here as agent-framework middleware.
//
// # The single construction door
//
// In the bus architecture the governance composite sat on every hop — one
// unbypassable chokepoint. The framework has no single chokepoint; middleware is
// attached per-agent. So this package preserves the guarantee structurally:
// NewGovernedDeterministic and NewGovernedOllama are the ONLY sanctioned agent
// constructors, and both inject GovMiddleware as the outermost middleware.
// singledoor_test.go fails CI if any framework-importing file outside this package
// calls a raw framework constructor.
package afg

import (
	"context"
	"fmt"
	"iter"
	"os"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
	"github.com/microsoft/agent-framework-go/provider/openaiprovider"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// govMsgKey carries the protocol.Message the governance gate evaluates for a run.
type govMsgKey struct{}

// WithGovMessage attaches the governance-relevant protocol.Message to ctx. The HTTP
// edge / orchestrator populates Type + Metadata (user_roles, classification, region,
// user_id, trace_id) from JWT claims and request context; GovMiddleware reads it.
// When absent, the gate falls back to a minimal user message built from the input.
func WithGovMessage(ctx context.Context, m protocol.Message) context.Context {
	return context.WithValue(ctx, govMsgKey{}, m)
}

func govMessageFrom(ctx context.Context, fallbackContent string) protocol.Message {
	if m, ok := ctx.Value(govMsgKey{}).(protocol.Message); ok {
		if m.Content == "" {
			m.Content = fallbackContent
		}
		if m.CreatedAt.IsZero() {
			m.CreatedAt = time.Now().UTC()
		}
		return m
	}
	return protocol.Message{Role: protocol.RoleUser, Content: fallbackContent, CreatedAt: time.Now().UTC()}
}

// GovMiddleware is Genie's deny-on-first-failure governance gate re-imposed as
// agent-framework middleware. It runs BEFORE the provider and, on deny, returns an
// error-yielding stream WITHOUT calling next — so no model or tool call happens on a
// rejected message. This reconstructs the bus's single chokepoint, per agent.
type GovMiddleware struct {
	Gate governance.Policy
}

// Run implements agent.Middleware.
func (g GovMiddleware) Run(next agent.RunFunc, ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
	pm := govMessageFrom(ctx, messagesText(msgs))
	res, err := g.Gate.Evaluate(ctx, pm)
	if err != nil {
		return errStream(fmt.Errorf("governance evaluation error: %w", err))
	}
	if res.Decision == governance.DecisionDeny {
		return errStream(&DeniedError{Type: pm.Type, Reason: res.Reason})
	}
	return next(ctx, msgs, opts...)
}

// DeniedError is returned (via the response stream) when the governance gate denies
// a message. It is distinct from an agent execution error so fallback routing can
// refuse to rescue a policy rejection — a denied message must not be answered by a
// fallback, only recorded as an incident.
type DeniedError struct {
	Type   string
	Reason string
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("governance denied (type=%q): %s", e.Type, e.Reason)
}

// NewGovernedDeterministic builds a no-LLM, no-network agent (currency, fallbacks,
// offline modes) from a pure RunFunc, with the governance gate as outermost
// middleware. One of the only two sanctioned constructors (see package doc).
func NewGovernedDeterministic(gate governance.Policy, name string, run agent.RunFunc) *agent.Agent {
	return agent.New(
		agent.ProviderConfig{ProviderName: "genie-det:" + name, Run: run},
		agent.Config{
			Name:                name,
			Middlewares:         []agent.Middleware{GovMiddleware{Gate: gate}},
			DisableFuncAutoCall: true, // keep the gate strictly outermost (no auto tool loop under it)
		},
	)
}

// NewGovernedOllama builds an LLM-backed agent on the on-prem Ollama
// OpenAI-compatible endpoint, with the same governance gate. The base URL lives on
// the openai-go client, not the framework config. Cloud (Foundry/Azure) gets a
// sibling constructor in Phase 3. One of the only two sanctioned constructors.
func NewGovernedOllama(gate governance.Policy, name, instructions, model string) *agent.Agent {
	client := openai.NewClient(
		option.WithBaseURL(ollamaBaseURL()),
		option.WithAPIKey(ollamaAPIKey()),
	)
	return openaiprovider.NewChatCompletionsAgent(client, openaiprovider.AgentConfig{
		Config: agent.Config{
			Name:                name,
			Middlewares:         []agent.Middleware{GovMiddleware{Gate: gate}},
			DisableFuncAutoCall: true,
		},
		Instructions: instructions,
		Model:        model,
	})
}

// ResponseText concatenates the text content of a collected response. Robust to
// however Response.String() chooses to format; use this to read an agent's answer.
func ResponseText(resp *agent.Response) string {
	if resp == nil {
		return ""
	}
	return messagesText(resp.Messages)
}

func messagesText(msgs []*message.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		for _, c := range m.Contents {
			if tc, ok := c.(*message.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
	}
	return b.String()
}

func errStream(err error) iter.Seq2[*agent.ResponseUpdate, error] {
	return func(yield func(*agent.ResponseUpdate, error) bool) { yield(nil, err) }
}

func ollamaBaseURL() string {
	if v := os.Getenv("GENIE_AF_OLLAMA_URL"); v != "" {
		return v
	}
	return "http://localhost:11434/v1"
}

func ollamaAPIKey() string {
	if v := os.Getenv("GENIE_AF_OLLAMA_KEY"); v != "" {
		return v
	}
	return "ollama" // any non-empty token; Ollama ignores it
}
