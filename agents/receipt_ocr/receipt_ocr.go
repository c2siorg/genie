// Package receipt_ocr converts a photographed receipt into a Genie
// canonical Transaction via a vision-capable LLM. Massive Indian fintech
// use case — UPI / cash receipts → ledger.
//
// Input message:
//
//	Type: "receipt_ocr_request"
//	Content: base64 of the image bytes
//	Metadata["mime_type"]: "image/jpeg"|"image/png"
//
// Output message: a finance.Transaction JSON published to the normalizer.
package receipt_ocr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

const (
	ID         = "receipt_ocr"
	Capability = "ocr_receipt"
	TypeIn     = "receipt_ocr_request"
	TypeOut    = "raw_transactions"
	NextAgent  = "normalizer"
)

// Agent uses a vision LLM to read a receipt image.
type Agent struct {
	Vision llm.VisionProvider
	Model  string
}

// New constructs the agent.
func New(vision llm.VisionProvider, model string) *Agent {
	return &Agent{Vision: vision, Model: model}
}

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Receipt OCR" }
func (a *Agent) Capabilities() []string { return []string{Capability} }

// RiskLevel — image-derived transactions enter the ledger; Medium per RBI
// FREE-AI Rec 14 (downstream agents still validate + the customer reviews).
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

const ocrPrompt = `You are reading a transaction receipt photo from India.
Extract a single transaction as JSON with this exact schema. Do not add commentary.

{
  "date": "YYYY-MM-DD",
  "description": "<merchant name + line item>",
  "merchant": "<lowercase merchant slug>",
  "amount_cents": <integer, NEGATIVE for debits, POSITIVE for credits>,
  "currency": "<ISO 4217, default INR>",
  "category": "<food:dining | food:delivery | transport:ride | shopping:ecom | utilities:* | uncategorized>",
  "direction": "debit" | "credit"
}`

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	if a.Vision == nil || !a.Vision.SupportsVision() {
		return nil, errors.New("receipt_ocr: vision provider not configured")
	}
	mime, _ := msg.Metadata["mime_type"].(string)
	if mime == "" {
		mime = "image/jpeg"
	}
	resp, err := a.Vision.Complete(ctx, llm.CompletionRequest{
		Model: a.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: ocrPrompt},
			{
				Role:    llm.RoleUser,
				Content: "Extract the transaction from this receipt.",
				Images:  []llm.ImagePart{{Base64: msg.Content, MimeType: mime}},
			},
		},
		MaxTokens: 256, Temperature: 0,
		Residency: llm.Residency{AllowCrossBorder: true},
	})
	if err != nil {
		return nil, fmt.Errorf("receipt_ocr: vision call: %w", err)
	}

	body := stripJSONFences(resp.Text)
	var txn finance.Transaction
	if err := json.Unmarshal([]byte(body), &txn); err != nil {
		return nil, fmt.Errorf("receipt_ocr: parse: %w (body=%q)", err, body)
	}
	if txn.Currency == "" {
		txn.Currency = "INR"
	}
	if txn.Direction == "" && txn.AmountCents < 0 {
		txn.Direction = finance.DirectionDebit
	}

	payload, err := finance.MarshalTransactions([]finance.Transaction{txn})
	if err != nil {
		return nil, err
	}
	env.Logf("[receipt_ocr] extracted txn merchant=%s amount=%d", txn.Merchant, txn.AmountCents)

	md := cloneMetadata(msg.Metadata)
	md[protocol.MetaKeyClassification] = string(protocol.ClassPII)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, payload, md),
	}, nil
}

func stripJSONFences(text string) string {
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

func cloneMetadata(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
