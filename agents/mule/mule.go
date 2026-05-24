// Package mule flags accounts that look like money-mules — used by fraud
// rings to layer stolen funds. The detector works on the standard Genie
// finance graph (pkg/graphrag) and looks for three structural signatures
// the AML/fraud literature considers high-signal:
//
//  1. Pass-through  — money in and back out within a short window, with
//     ≥90 % of the inflow forwarded. The textbook mule signal.
//  2. Fan-in fan-out — many disjoint senders converging on one account
//     that subsequently disperses to many distinct destinations.
//  3. Velocity spike — sudden burst of >N debits within a short window
//     after a recent large credit (turnover ratio).
//
// All three rules are deterministic — no model — so the agent is auditable
// for RBI / FIU-IND reporting (Rec 25 explainability). LLM rationale can
// be layered on top by a downstream reporter if desired.
package mule

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "mule_detector"
	Capability = "detect_mule_account"
	TypeIn     = "analysis_result"
	TypeOut    = "mule_signals"
	NextAgent  = "financial_supervisor"

	// Defaults.
	DefaultPassThroughWindow = 24 * time.Hour
	DefaultPassThroughFrac   = 0.9
	DefaultFanInThreshold    = 5
	DefaultFanOutThreshold   = 5
	DefaultVelocityWindow    = 1 * time.Hour
	DefaultVelocityCount     = 10
)

// Signal is one mule finding. AccountID is the account flagged.
type Signal struct {
	AccountID  string  `json:"account_id"`
	Pattern    string  `json:"pattern"`
	Severity   string  `json:"severity"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// Result is the message payload.
type Result struct {
	Signals []Signal `json:"signals"`
}

// Agent implements agent.Agent.
type Agent struct {
	PassThroughWindow time.Duration
	PassThroughFrac   float64
	FanInThreshold    int
	FanOutThreshold   int
	VelocityWindow    time.Duration
	VelocityCount     int
}

// New returns an agent with package defaults.
func New() *Agent {
	return &Agent{
		PassThroughWindow: DefaultPassThroughWindow,
		PassThroughFrac:   DefaultPassThroughFrac,
		FanInThreshold:    DefaultFanInThreshold,
		FanOutThreshold:   DefaultFanOutThreshold,
		VelocityWindow:    DefaultVelocityWindow,
		VelocityCount:     DefaultVelocityCount,
	}
}

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Mule Account Detector" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

type analyzerView struct {
	Transactions []finance.Transaction `json:"transactions"`
}

// HandleMessage runs Detect against the incoming analyzer batch.
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	signals := a.Detect(av.Transactions)
	env.Logf("[mule] %d signals across %d txns", len(signals), len(av.Transactions))
	body, _ := json.Marshal(Result{Signals: signals})
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Detect runs the three rules over the transaction set, treating each
// AccountID independently. Returns deduplicated signals.
func (a *Agent) Detect(txns []finance.Transaction) []Signal {
	if len(txns) == 0 {
		return nil
	}
	// Group by account.
	byAccount := map[string][]finance.Transaction{}
	for _, t := range txns {
		byAccount[t.AccountID] = append(byAccount[t.AccountID], t)
	}

	out := []Signal{}
	for acct, list := range byAccount {
		if acct == "" {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool {
			ti, _ := list[i].ParsedDate()
			tj, _ := list[j].ParsedDate()
			return ti.Before(tj)
		})
		out = append(out, a.detectPassThrough(acct, list)...)
		out = append(out, a.detectFanInFanOut(acct, list)...)
		out = append(out, a.detectVelocityAfterCredit(acct, list)...)
	}
	// Dedupe by (account, pattern) — keep the highest confidence.
	dedup := map[string]Signal{}
	for _, s := range out {
		k := s.AccountID + "|" + s.Pattern
		if cur, ok := dedup[k]; !ok || s.Confidence > cur.Confidence {
			dedup[k] = s
		}
	}
	keys := make([]string, 0, len(dedup))
	for k := range dedup {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	res := make([]Signal, 0, len(keys))
	for _, k := range keys {
		res = append(res, dedup[k])
	}
	return res
}

// detectPassThrough scans for a large credit closely followed by debits
// totalling ≥ PassThroughFrac of the credit, inside PassThroughWindow.
func (a *Agent) detectPassThrough(acct string, list []finance.Transaction) []Signal {
	var signals []Signal
	for i, t := range list {
		if t.AmountCents <= 0 {
			continue // need a credit anchor
		}
		credit := t.AmountCents
		ti, _ := t.ParsedDate()
		windowEnd := ti.Add(a.PassThroughWindow)

		var debits int64
		for j := i + 1; j < len(list); j++ {
			tj, _ := list[j].ParsedDate()
			if tj.After(windowEnd) {
				break
			}
			if list[j].AmountCents < 0 {
				debits += -list[j].AmountCents
			}
		}
		if credit > 0 && float64(debits)/float64(credit) >= a.PassThroughFrac {
			signals = append(signals, Signal{
				AccountID:  acct,
				Pattern:    "pass_through",
				Severity:   "high",
				Reason:     "≥90% of a recent credit forwarded out within 24h",
				Confidence: 0.92,
			})
		}
	}
	return signals
}

// detectFanInFanOut counts distinct counterparties on both sides. If the
// account has many distinct merchants incoming AND outgoing, it's a hub.
//
// In Genie, finance.Transaction.Merchant is the counterparty. A "credit
// from X" is represented by AmountCents>0 with Merchant=X.
func (a *Agent) detectFanInFanOut(acct string, list []finance.Transaction) []Signal {
	inMerchants := map[string]bool{}
	outMerchants := map[string]bool{}
	for _, t := range list {
		if t.Merchant == "" {
			continue
		}
		if t.AmountCents > 0 {
			inMerchants[t.Merchant] = true
		} else if t.AmountCents < 0 {
			outMerchants[t.Merchant] = true
		}
	}
	if len(inMerchants) >= a.FanInThreshold && len(outMerchants) >= a.FanOutThreshold {
		return []Signal{{
			AccountID:  acct,
			Pattern:    "fan_in_fan_out",
			Severity:   "high",
			Reason:     "account dispatches to many merchants and receives from many distinct senders — hub pattern",
			Confidence: 0.85,
		}}
	}
	return nil
}

// detectVelocityAfterCredit flags VelocityCount or more debits within
// VelocityWindow following a credit — classic burst-out behaviour.
func (a *Agent) detectVelocityAfterCredit(acct string, list []finance.Transaction) []Signal {
	for i, t := range list {
		if t.AmountCents <= 0 {
			continue
		}
		ti, _ := t.ParsedDate()
		windowEnd := ti.Add(a.VelocityWindow)
		debits := 0
		for j := i + 1; j < len(list); j++ {
			tj, _ := list[j].ParsedDate()
			if tj.After(windowEnd) {
				break
			}
			if list[j].AmountCents < 0 {
				debits++
			}
		}
		if debits >= a.VelocityCount {
			return []Signal{{
				AccountID:  acct,
				Pattern:    "post_credit_burst",
				Severity:   "high",
				Reason:     "≥10 debits within 1h of a credit — turnover spike",
				Confidence: 0.88,
			}}
		}
	}
	return nil
}
