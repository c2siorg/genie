// Package anomaly flags transactions whose amount exceeds a z-score threshold
// against the mean+stddev of their category.
package anomaly

import (
	"context"
	"encoding/json"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "anomaly_detector"
	CapDetect  = "detect_anomaly"
	TypeIn     = "analysis_result"
	TypeOut    = "anomalies"
	NextAgent  = "financial_supervisor"

	// Z-score above which a transaction is flagged.
	DefaultZThreshold = 2.0
)

type analyzerView struct {
	Transactions []finance.Transaction `json:"transactions"`
}

type Anomaly struct {
	TransactionID string  `json:"transaction_id"`
	Category      string  `json:"category"`
	AmountCents   int64   `json:"amount_cents"`
	Z             float64 `json:"z_score"`
	Reason        string  `json:"reason"`
}

type Result struct {
	Anomalies []Anomaly `json:"anomalies"`
}

type Agent struct {
	ZThreshold float64
}

func New() *Agent { return &Agent{ZThreshold: DefaultZThreshold} }

func (a *Agent) ID() string             { return ID }
func (a *Agent) Name() string           { return "Anomaly Detector" }
func (a *Agent) Capabilities() []string { return []string{CapDetect} }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	res := Result{Anomalies: detect(av.Transactions, a.ZThreshold)}
	env.Logf("[anomaly] flagged %d", len(res.Anomalies))
	body, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

func detect(txns []finance.Transaction, z float64) []Anomaly {
	if len(txns) < 2 {
		return nil
	}
	groups := map[string][]int64{}
	for _, t := range txns {
		if t.AmountCents >= 0 {
			continue
		}
		groups[t.Category] = append(groups[t.Category], -t.AmountCents)
	}
	stats := map[string]struct{ mean, std float64 }{}
	for cat, amounts := range groups {
		if len(amounts) < 2 {
			continue
		}
		var sum, sumSq float64
		for _, a := range amounts {
			sum += float64(a)
		}
		mean := sum / float64(len(amounts))
		for _, a := range amounts {
			diff := float64(a) - mean
			sumSq += diff * diff
		}
		std := math.Sqrt(sumSq / float64(len(amounts)))
		stats[cat] = struct{ mean, std float64 }{mean, std}
	}
	out := []Anomaly{}
	for _, t := range txns {
		if t.AmountCents >= 0 {
			continue
		}
		s, ok := stats[t.Category]
		if !ok || s.std == 0 {
			continue
		}
		zv := (float64(-t.AmountCents) - s.mean) / s.std
		if zv >= z {
			out = append(out, Anomaly{
				TransactionID: t.TransactionID,
				Category:      t.Category,
				AmountCents:   -t.AmountCents,
				Z:             zv,
				Reason:        "amount > mean + threshold*stddev for category",
			})
		}
	}
	return out
}
