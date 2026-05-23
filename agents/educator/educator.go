// Package educator answers "explain finance concept X" style requests.
// Modeled on the ADK Financial Advisor sample. Backed by a static glossary;
// swap with an LLM call in production.
package educator

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

const (
	ID         = "financial_educator"
	CapExplain = "explain_finance"
	CapEducate = "educate"
	TypeIn     = "explain_finance"
	TypeOut    = "finance_explanation"
)

var glossary = map[string]string{
	"sip":             "A Systematic Investment Plan invests a fixed amount in a mutual fund on a recurring schedule to average cost over time.",
	"emi":             "Equated Monthly Installment: fixed monthly payment toward a loan covering both principal and interest.",
	"compound interest": "Interest earned on both the principal and previously accrued interest, leading to exponential growth.",
	"emergency fund":  "Liquid savings (typically 3-6 months of expenses) reserved for unexpected income or expense shocks.",
	"asset allocation": "How investments are split between asset classes such as equity, debt, real estate, and cash.",
	"ppf":             "Public Provident Fund: a government-backed long-term savings instrument with tax benefits.",
	"nps":             "National Pension System: a market-linked, defined-contribution pension scheme.",
	"index fund":      "A mutual fund that passively tracks a market index such as the Nifty 50 or S&P 500.",
}

type Agent struct {
	Glossary map[string]string
	// Index is the optional RAG index. When set, the educator appends source
	// citations to its answer. Leave nil to fall back to glossary-only.
	Index *rag.Index
}

// Answer is the structured payload the educator now emits when the RAG
// index is configured. Without RAG it still ships a plain-text answer for
// backwards compatibility.
type Answer struct {
	Text      string             `json:"text"`
	Citations []rag.ScoredChunk  `json:"citations,omitempty"`
}

func New() *Agent {
	cp := make(map[string]string, len(glossary))
	for k, v := range glossary {
		cp[k] = v
	}
	return &Agent{Glossary: cp}
}

// WithRAG attaches a rag.Index so answers carry citations.
func (a *Agent) WithRAG(idx *rag.Index) *Agent { a.Index = idx; return a }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Financial Educator" }
func (a *Agent) Capabilities() []string { return []string{CapExplain, CapEducate} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	key := strings.ToLower(strings.TrimSpace(msg.Content))
	text, ok := a.Glossary[key]
	if !ok {
		text = "No glossary entry yet. Try one of: " + joinKeys(a.Glossary) + "."
	}
	env.Logf("[educator] explained %q", key)

	// Backwards-compatible path: no RAG index → plain text payload.
	if a.Index == nil {
		return []agent.Message{
			agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, text, msg.Metadata),
		}, nil
	}

	// RAG path: attach top citations so the consumer can verify the claim.
	cites, err := a.Index.Search(ctx, msg.Content, 3)
	if err != nil {
		// Fail soft: still return the answer, just without citations.
		env.Logf("[educator] rag search failed: %v", err)
	}
	ans := Answer{Text: text, Citations: cites}
	body, err := json.Marshal(ans)
	if err != nil {
		return nil, err
	}
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

func joinKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}
