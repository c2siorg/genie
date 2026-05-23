// Package synth generates synthetic data with an LLM. Used to expand
// glossaries, augment training fixtures, or seed evaluation sets without
// needing real user data.
//
// Output is intentionally JSON — the Generator validates each sample against
// the supplied schema and discards malformed responses.
package synth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/schema"
)

// Generator produces synthetic data via an LLM.
type Generator struct {
	Provider llm.Provider
	Model    string
	Schema   *schema.Schema
}

// New wraps a Provider with an output schema.
func New(p llm.Provider, model string, sch *schema.Schema) *Generator {
	return &Generator{Provider: p, Model: model, Schema: sch}
}

// Sample asks the LLM to produce one synthetic example matching the schema,
// seeded with the user instruction.
func (g *Generator) Sample(ctx context.Context, instruction string, seed []byte) (json.RawMessage, error) {
	if g.Provider == nil {
		return nil, errors.New("synth: no provider")
	}
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "Generate one synthetic example as a single valid JSON object. No commentary, no markdown fences."},
		{Role: llm.RoleUser, Content: instruction + "\n\nSeed context:\n" + string(seed)},
	}
	resp, err := g.Provider.Complete(ctx, llm.CompletionRequest{
		Model: g.Model, Messages: msgs, MaxTokens: 512, Temperature: 0.7,
		Residency: llm.Residency{AllowCrossBorder: true},
	})
	if err != nil {
		return nil, err
	}
	body := stripFences(resp.Text)
	if g.Schema != nil {
		if err := g.Schema.ValidateJSON([]byte(body)); err != nil {
			return nil, fmt.Errorf("synth: schema validation: %w", err)
		}
	}
	return json.RawMessage(body), nil
}

// stripFences removes ``` … ``` if the model wrapped the JSON.
func stripFences(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
	}
	return strings.TrimSpace(text)
}
