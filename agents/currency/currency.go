// Package currency exposes a tiny FX-conversion specialist modeled on the
// google/adk-samples Currency Agent. It uses a static rate table; production
// would call a rate service via a tool.
package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID          = "currency_converter"
	CapConvert  = "convert_currency"
	CapRate     = "fx_rate"
	TypeConvert = "convert_currency"
	TypeRate    = "fx_rate"
)

// Rates are quote-per-base, e.g. rates["USD"]["INR"] = 83.0.
var defaultRates = map[string]map[string]float64{
	"USD": {"INR": 83.0, "EUR": 0.92, "USD": 1.0},
	"INR": {"USD": 1.0 / 83.0, "EUR": 0.011, "INR": 1.0},
	"EUR": {"INR": 90.0, "USD": 1.08, "EUR": 1.0},
}

type ConvertRequest struct {
	AmountMinor int64  `json:"amount_minor"`
	From        string `json:"from"`
	To          string `json:"to"`
}

type ConvertResponse struct {
	From            string  `json:"from"`
	To              string  `json:"to"`
	Rate            float64 `json:"rate"`
	AmountMinorFrom int64   `json:"amount_minor_from"`
	AmountMinorTo   int64   `json:"amount_minor_to"`
}

type Agent struct {
	Rates map[string]map[string]float64
}

func New() *Agent {
	cp := map[string]map[string]float64{}
	for k, v := range defaultRates {
		inner := map[string]float64{}
		for k2, v2 := range v {
			inner[k2] = v2
		}
		cp[k] = inner
	}
	return &Agent{Rates: cp}
}

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Currency Converter" }
func (a *Agent) Capabilities() []string { return []string{CapConvert, CapRate} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	switch msg.Type {
	case TypeConvert:
		var req ConvertRequest
		if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
			return nil, err
		}
		from := strings.ToUpper(req.From)
		to := strings.ToUpper(req.To)
		rate, err := a.lookup(from, to)
		if err != nil {
			return nil, err
		}
		resp := ConvertResponse{
			From:            from,
			To:              to,
			Rate:            rate,
			AmountMinorFrom: req.AmountMinor,
			AmountMinorTo:   int64(float64(req.AmountMinor) * rate),
		}
		body, _ := json.Marshal(resp)
		env.Logf("[currency] %d %s -> %d %s @ %.4f", req.AmountMinor, from, resp.AmountMinorTo, to, rate)
		return []agent.Message{
			agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeConvert, string(body), msg.Metadata),
		}, nil

	case TypeRate:
		var req ConvertRequest
		if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
			return nil, err
		}
		rate, err := a.lookup(strings.ToUpper(req.From), strings.ToUpper(req.To))
		if err != nil {
			return nil, err
		}
		body, _ := json.Marshal(map[string]any{"from": req.From, "to": req.To, "rate": rate})
		return []agent.Message{
			agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeRate, string(body), msg.Metadata),
		}, nil
	}
	return nil, nil
}

func (a *Agent) lookup(from, to string) (float64, error) {
	inner, ok := a.Rates[from]
	if !ok {
		return 0, fmt.Errorf("unsupported base currency %q", from)
	}
	rate, ok := inner[to]
	if !ok {
		return 0, fmt.Errorf("unsupported quote currency %q for base %q", to, from)
	}
	return rate, nil
}
