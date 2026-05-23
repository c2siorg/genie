package receipt_ocr

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

type testEnv struct{}

func (testEnv) Now() time.Time                  { return time.Unix(0, 0) }
func (testEnv) Logf(format string, args ...any) {}

// fakeVision wraps llm.Mock with the VisionProvider marker so receipt_ocr accepts it.
type fakeVision struct{ *llm.Mock }

func (fakeVision) SupportsVision() bool { return true }

func TestReceiptOCR_ParsesJSON(t *testing.T) {
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{{
		Text: `{
		  "date": "2026-01-05",
		  "description": "Swiggy order",
		  "merchant": "swiggy",
		  "amount_cents": -45000,
		  "currency": "INR",
		  "category": "food:delivery",
		  "direction": "debit"
		}`,
	}}
	a := New(fakeVision{Mock: mock}, "vision-mock")
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeIn, "BASE64IMAGE", map[string]any{
		"mime_type": "image/jpeg",
	})
	out, err := a.HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].To != NextAgent {
		t.Fatalf("unexpected: %+v", out)
	}
	txns, _ := finance.UnmarshalTransactions(out[0].Content)
	if len(txns) != 1 || txns[0].Merchant != "swiggy" || txns[0].AmountCents != -45000 {
		t.Fatalf("unexpected txn: %+v", txns)
	}
}

func TestReceiptOCR_StripsCodeFences(t *testing.T) {
	mock := llm.NewMock()
	mock.Responses = []llm.CompletionResponse{{
		Text: "```json\n{\"date\":\"2026-01-01\",\"description\":\"x\",\"merchant\":\"x\",\"amount_cents\":-100,\"currency\":\"INR\",\"category\":\"uncategorized\",\"direction\":\"debit\"}\n```",
	}}
	a := New(fakeVision{Mock: mock}, "vision-mock")
	msg := agent.NewMessage("user", ID, agent.RoleUser, TypeIn, "BASE64IMAGE", nil)
	out, err := a.HandleMessage(context.Background(), msg, testEnv{})
	if err != nil {
		t.Fatal(err)
	}
	var raw any
	if err := json.Unmarshal([]byte(out[0].Content), &raw); err != nil {
		t.Fatalf("output not valid json: %v", err)
	}
}
