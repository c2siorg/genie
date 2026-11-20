package afg

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
)

// CurrencyName is the framework agent's name; it matches the legacy agent's ID
// (agents/currency.ID) so the Phase-5 parity diff lines up.
const CurrencyName = "currency_converter"

// currencyRates mirrors agents/currency.defaultRates. The table + lookup are
// reproduced here (not imported) on purpose: the legacy bus-era currency agent
// stays byte-for-byte untouched as the Phase-5 parity oracle on the
// legacy/bus-architecture branch. De-duplication happens after cutover.
var currencyRates = map[string]map[string]float64{
	"USD": {"INR": 83.0, "EUR": 0.92, "USD": 1.0},
	"INR": {"USD": 1.0 / 83.0, "EUR": 0.011, "INR": 1.0},
	"EUR": {"INR": 90.0, "USD": 1.08, "EUR": 1.0},
}

// CurrencyRequest / CurrencyResponse mirror the legacy agent's wire types.
type CurrencyRequest struct {
	AmountMinor int64  `json:"amount_minor"`
	From        string `json:"from"`
	To          string `json:"to"`
}

type CurrencyResponse struct {
	From            string  `json:"from"`
	To              string  `json:"to"`
	Rate            float64 `json:"rate"`
	AmountMinorFrom int64   `json:"amount_minor_from"`
	AmountMinorTo   int64   `json:"amount_minor_to"`
}

func currencyLookup(from, to string) (float64, error) {
	inner, ok := currencyRates[from]
	if !ok {
		return 0, fmt.Errorf("unsupported base currency %q", from)
	}
	rate, ok := inner[to]
	if !ok {
		return 0, fmt.Errorf("unsupported quote currency %q for base %q", to, from)
	}
	return rate, nil
}

// CurrencyRun is the deterministic RunFunc: the legacy HandleMessage's convert
// branch, reshaped to parse the input text, convert, and yield a JSON response.
func CurrencyRun() agent.RunFunc {
	return func(ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
		return func(yield func(*agent.ResponseUpdate, error) bool) {
			var req CurrencyRequest
			if err := json.Unmarshal([]byte(messagesText(msgs)), &req); err != nil {
				yield(nil, fmt.Errorf("currency: bad request: %w", err))
				return
			}
			from, to := strings.ToUpper(req.From), strings.ToUpper(req.To)
			rate, err := currencyLookup(from, to)
			if err != nil {
				yield(nil, err)
				return
			}
			body, _ := json.Marshal(CurrencyResponse{
				From:            from,
				To:              to,
				Rate:            rate,
				AmountMinorFrom: req.AmountMinor,
				AmountMinorTo:   int64(float64(req.AmountMinor) * rate),
			})
			yield(&agent.ResponseUpdate{
				Role:     message.RoleAssistant,
				Contents: message.Contents{&message.TextContent{Text: string(body)}},
			}, nil)
		}
	}
}

// NewCurrencyAgent builds the governed, framework-native currency agent through the
// single construction door.
func NewCurrencyAgent(gate governance.Policy) *agent.Agent {
	return NewGovernedDeterministic(gate, CurrencyName, CurrencyRun())
}
