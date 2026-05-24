// Package subscription_detector finds recurring subscription-style debits
// in a transaction list. Two signals stacked:
//
//  1. Periodicity — same merchant debited on ≥3 distinct months with
//     ≤7-day standard deviation of day-of-month (monthly cadence).
//  2. Amount consistency — same merchant within ±5 % of a stable amount.
//
// The agent flags zombie subscriptions (price crept up, usage dropped) so
// the user can act. Output feeds a recommender that can draft cancellation
// scripts.
package subscription_detector

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "subscription_detector"
	Capability = "detect_subscriptions"
	TypeIn     = "analysis_result"
	TypeOut    = "subscriptions"
	NextAgent  = "financial_supervisor"

	MinOccurrences  = 3
	MaxDayStdDev    = 7.0
	AmountTolerance = 0.05 // ±5%
	ZombiePriceJump = 0.20 // ≥20% price hike vs the first detection
)

// Subscription is one recurring charge.
type Subscription struct {
	Merchant            string  `json:"merchant"`
	OccurrenceCount     int     `json:"occurrence_count"`
	TypicalAmountRupees float64 `json:"typical_amount_rupees"`
	LastAmountRupees    float64 `json:"last_amount_rupees"`
	AnnualisedINR       float64 `json:"annualised_rupees"`
	ZombieWarning       string  `json:"zombie_warning,omitempty"`
}

type analyzerView struct {
	Transactions []finance.Transaction `json:"transactions"`
}

// Result is the wire output.
type Result struct {
	Subscriptions  []Subscription `json:"subscriptions"`
	AnnualisedINR  float64        `json:"total_annualised_rupees"`
	Recommendation string         `json:"recommendation"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Subscription Detector" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskLow }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	res := a.Detect(av.Transactions)
	env.Logf("[subscription_detector] %d subs, ₹%.0f / yr", len(res.Subscriptions), res.AnnualisedINR)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Detect groups debits by merchant and applies the two rules.
func (a *Agent) Detect(txns []finance.Transaction) Result {
	byMerchant := map[string][]finance.Transaction{}
	for _, t := range txns {
		if t.AmountCents >= 0 || t.Merchant == "" {
			continue
		}
		byMerchant[t.Merchant] = append(byMerchant[t.Merchant], t)
	}
	res := Result{}
	for merchant, list := range byMerchant {
		if len(list) < MinOccurrences {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool {
			ti, _ := list[i].ParsedDate()
			tj, _ := list[j].ParsedDate()
			return ti.Before(tj)
		})
		days := []float64{}
		amounts := []float64{}
		for _, t := range list {
			when, err := t.ParsedDate()
			if err != nil {
				continue
			}
			days = append(days, float64(when.Day()))
			amounts = append(amounts, float64(-t.AmountCents)/100)
		}
		if std(days) > MaxDayStdDev {
			continue
		}
		// Establish "typical" from all-but-last charges so a final price jump
		// can be flagged as a zombie subscription instead of disqualifying
		// the whole merchant.
		earlier := amounts[:len(amounts)-1]
		typical := median(earlier)
		if typical == 0 {
			typical = median(amounts)
		}
		if typical == 0 {
			continue
		}
		// ≥80 % of the earlier charges must be within ±tolerance of typical.
		consistent := 0
		for _, x := range earlier {
			if math.Abs(x-typical)/typical <= AmountTolerance {
				consistent++
			}
		}
		// One-off escape: if there's only one "earlier" charge, accept it.
		if len(earlier) > 1 && float64(consistent)/float64(len(earlier)) < 0.8 {
			continue
		}
		last := amounts[len(amounts)-1]
		first := amounts[0]
		zombie := ""
		if first > 0 && (last-first)/first >= ZombiePriceJump {
			zombie = "Price crept up vs first detection — review value or cancel."
		}
		sub := Subscription{
			Merchant:            merchant,
			OccurrenceCount:     len(list),
			TypicalAmountRupees: round2(typical),
			LastAmountRupees:    round2(last),
			AnnualisedINR:       round2(typical * 12),
			ZombieWarning:       zombie,
		}
		res.Subscriptions = append(res.Subscriptions, sub)
		res.AnnualisedINR += sub.AnnualisedINR
	}
	sort.SliceStable(res.Subscriptions, func(i, j int) bool {
		return res.Subscriptions[i].AnnualisedINR > res.Subscriptions[j].AnnualisedINR
	})
	res.AnnualisedINR = round2(res.AnnualisedINR)
	if len(res.Subscriptions) > 0 {
		res.Recommendation = "Audit each subscription against actual usage in the last 30 days; cancel unused ones via the merchant app or your card issuer's recurring-mandate dashboard."
	}
	return res
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := make([]float64, len(xs))
	copy(cp, xs)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func std(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var sumSq float64
	for _, x := range xs {
		d := x - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(xs)))
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
