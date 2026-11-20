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

// det adapts a plain string->string handler into a governed-agent RunFunc. Most
// deterministic specialists are one-liners on top of it — the batch-port template.
func det(handle func(input string) (string, error)) agent.RunFunc {
	return func(ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
		return func(yield func(*agent.ResponseUpdate, error) bool) {
			out, err := handle(messagesText(msgs))
			if err != nil {
				yield(nil, err)
				return
			}
			yield(&agent.ResponseUpdate{
				Role:     message.RoleAssistant,
				Contents: message.Contents{&message.TextContent{Text: out}},
			}, nil)
		}
	}
}

// --- rates: faithful port of the legacy currency agent's fx_rate branch ---

const RatesName = "fx_rates"

func NewRatesAgent(gate governance.Policy) *agent.Agent {
	return NewGovernedDeterministic(gate, RatesName, det(func(in string) (string, error) {
		var req CurrencyRequest
		if err := json.Unmarshal([]byte(in), &req); err != nil {
			return "", fmt.Errorf("rates: bad request: %w", err)
		}
		from, to := strings.ToUpper(req.From), strings.ToUpper(req.To)
		rate, err := currencyLookup(from, to)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"from": from, "to": to, "rate": rate})
		return string(b), nil
	}))
}

// --- tax_estimator: representative deterministic new-regime slab estimate ---
// (Simplified: no cess/rebate/surcharge — a faithful full port is mechanical.)

const TaxEstimatorName = "tax_estimator"

type TaxRequest struct {
	IncomeMinor int64 `json:"income_minor"` // annual income in paise
}

type TaxResponse struct {
	IncomeMinor int64  `json:"income_minor"`
	TaxMinor    int64  `json:"tax_minor"`
	Regime      string `json:"regime"`
}

func estimateTaxMinor(incomeMinor int64) int64 {
	inc := float64(incomeMinor) / 100.0 // rupees
	slabs := []struct {
		upto, rate float64
	}{
		{300000, 0.00}, {600000, 0.05}, {900000, 0.10}, {1200000, 0.15}, {1500000, 0.20},
	}
	var tax, prev float64
	for _, s := range slabs {
		if inc > s.upto {
			tax += (s.upto - prev) * s.rate
			prev = s.upto
			continue
		}
		tax += (inc - prev) * s.rate
		return int64(tax * 100)
	}
	tax += (inc - prev) * 0.30 // above 15L
	return int64(tax * 100)
}

func NewTaxEstimatorAgent(gate governance.Policy) *agent.Agent {
	return NewGovernedDeterministic(gate, TaxEstimatorName, det(func(in string) (string, error) {
		var req TaxRequest
		if err := json.Unmarshal([]byte(in), &req); err != nil {
			return "", fmt.Errorf("tax: bad request: %w", err)
		}
		b, _ := json.Marshal(TaxResponse{IncomeMinor: req.IncomeMinor, TaxMinor: estimateTaxMinor(req.IncomeMinor), Regime: "new"})
		return string(b), nil
	}))
}

// --- macro: canned macro-indicator snapshot (representative) ---

const MacroName = "macro_indicators"

func NewMacroAgent(gate governance.Policy) *agent.Agent {
	return NewGovernedDeterministic(gate, MacroName, det(func(_ string) (string, error) {
		b, _ := json.Marshal(map[string]any{
			"repo_rate_pct": 6.5, "crr_pct": 4.5, "cpi_inflation_pct": 5.1, "region": "in",
		})
		return string(b), nil
	}))
}

// DefaultRegistry builds and registers the currently-ported specialists, all
// governed by the same gate. It is the seed of the framework port's live
// inventory; remaining legacy agents are added here as they are ported (mechanical,
// one entry each). Provider/risk mirror the legacy declarations.
func DefaultRegistry(gate governance.Policy) *Registry {
	reg := NewRegistry()
	reg.Register(AgentInfo{ID: CurrencyName, Provider: ProviderDeterministic, Risk: "low"}, NewCurrencyAgent(gate))
	reg.Register(AgentInfo{ID: RatesName, Provider: ProviderDeterministic, Risk: "low"}, NewRatesAgent(gate))
	reg.Register(AgentInfo{ID: TaxEstimatorName, Provider: ProviderDeterministic, Risk: "medium"}, NewTaxEstimatorAgent(gate))
	reg.Register(AgentInfo{ID: MacroName, Provider: ProviderDeterministic, Risk: "low"}, NewMacroAgent(gate))
	return reg
}
