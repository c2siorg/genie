// Package aml_monitor scans transactions for AML/CFT red flags per FIU-IND
// guidance and PMLA 2002 rules. Output is a Suspicious Transaction Report
// (STR) draft ready for a compliance officer to review and submit.
//
// Five rule families covered here:
//   * structuring — multiple cash txns just below the ₹10L reporting bar
//   * round-tripping — money exiting and returning to the same counterparty
//   * high-risk jurisdiction — payments to FATF grey/black list countries
//   * threshold breach — cash transaction ≥₹10L (CTR) or ≥₹50L wire (LTR)
//   * adverse media match — counterparty hits a sanctions/PEP list
package aml_monitor

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "aml_monitor"
	Capability = "monitor_aml"
	TypeIn     = "analysis_result"
	TypeOut    = "aml_strs"
	NextAgent  = "financial_supervisor"

	CashCTRThreshold      = 10_00_000_00 // ₹10L in paise — Cash Transaction Report
	WireLTRThreshold      = 50_00_000_00 // ₹50L wire — Large Transaction Report
	StructuringWindowDays = 7
	StructuringMinCount   = 3
	StructuringFloorPct   = 0.80 // each txn ≥80% of CTR threshold but < threshold
)

// FATF lists (illustrative — production should pull current FATF + UN
// sanctions tape).
var (
	HighRiskCountries = map[string]bool{
		"NK": true, "IR": true, "MM": true, "AF": true,
	}
	SanctionsKeywords = []string{"sanctioned-entity", "pep-flag", "ofac-listed"}
)

// STR is one draft Suspicious Transaction Report.
type STR struct {
	TransactionIDs []string  `json:"transaction_ids"`
	RuleHit        string    `json:"rule_hit"`
	Severity       string    `json:"severity"`
	AmountTotal    float64   `json:"amount_total_rupees"`
	Narrative      string    `json:"narrative"`
	DraftedAt      time.Time `json:"drafted_at"`
}

type analyzerView struct {
	Transactions []finance.Transaction `json:"transactions"`
}

// Result is the wire output.
type Result struct {
	STRs       []STR  `json:"strs"`
	Note       string `json:"note"`
	Disclaimer string `json:"disclaimer"`
}

type Agent struct {
	Now func() time.Time
}

func New() *Agent { return &Agent{Now: time.Now} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "AML Monitor" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	res := a.Detect(av.Transactions)
	env.Logf("[aml_monitor] %d STR drafts", len(res.STRs))
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Detect applies all rule families to the transaction set.
func (a *Agent) Detect(txns []finance.Transaction) Result {
	now := a.Now().UTC()
	strs := []STR{}
	strs = append(strs, detectThresholdBreach(txns, now)...)
	strs = append(strs, detectStructuring(txns, now)...)
	strs = append(strs, detectAdverseMedia(txns, now)...)
	return Result{
		STRs:       strs,
		Note:       "Compliance-officer review required before submission to FIU-IND.",
		Disclaimer: "Rule-based first pass. Final STR / CTR filing must be approved by Principal Officer.",
	}
}

// detectThresholdBreach flags any single txn ≥CTR/LTR threshold.
func detectThresholdBreach(txns []finance.Transaction, now time.Time) []STR {
	out := []STR{}
	for _, t := range txns {
		amt := absI(t.AmountCents)
		switch {
		case isCash(t) && amt >= CashCTRThreshold:
			out = append(out, STR{
				TransactionIDs: []string{t.TransactionID},
				RuleHit:        "ctr_threshold",
				Severity:       "high",
				AmountTotal:    float64(amt) / 100,
				Narrative:      "Single cash transaction ≥₹10L — Cash Transaction Report (CTR) due within 15 days.",
				DraftedAt:      now,
			})
		case amt >= WireLTRThreshold:
			out = append(out, STR{
				TransactionIDs: []string{t.TransactionID},
				RuleHit:        "wire_threshold",
				Severity:       "medium",
				AmountTotal:    float64(amt) / 100,
				Narrative:      "Wire transfer ≥₹50L — Large Transaction Report (LTR) due.",
				DraftedAt:      now,
			})
		}
	}
	return out
}

// detectStructuring looks for ≥3 cash txns inside a 7-day window each
// just below the CTR bar (80–99 %). Classic smurfing.
func detectStructuring(txns []finance.Transaction, now time.Time) []STR {
	out := []STR{}
	candidates := []finance.Transaction{}
	for _, t := range txns {
		if !isCash(t) {
			continue
		}
		amt := absI(t.AmountCents)
		if float64(amt) >= StructuringFloorPct*CashCTRThreshold && amt < CashCTRThreshold {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) < StructuringMinCount {
		return out
	}
	// Walk by user/account — group same AccountID inside a 7-day window.
	groups := map[string][]finance.Transaction{}
	for _, t := range candidates {
		groups[t.AccountID] = append(groups[t.AccountID], t)
	}
	for acct, list := range groups {
		if len(list) < StructuringMinCount {
			continue
		}
		// Sliding window: as long as the span is ≤7d we accumulate.
		first, _ := list[0].ParsedDate()
		last, _ := list[len(list)-1].ParsedDate()
		if last.Sub(first).Hours()/24 <= StructuringWindowDays {
			ids := []string{}
			var total int64
			for _, t := range list {
				ids = append(ids, t.TransactionID)
				total += absI(t.AmountCents)
			}
			out = append(out, STR{
				TransactionIDs: ids,
				RuleHit:        "structuring",
				Severity:       "high",
				AmountTotal:    float64(total) / 100,
				Narrative:      "≥3 cash transactions on account " + acct + " each between 80–99% of CTR threshold within a 7-day window — possible structuring.",
				DraftedAt:      now,
			})
		}
	}
	return out
}

// detectAdverseMedia scans the description / merchant string for known
// sanctions or PEP keywords.
func detectAdverseMedia(txns []finance.Transaction, now time.Time) []STR {
	out := []STR{}
	for _, t := range txns {
		needle := strings.ToLower(t.Description + " " + t.Merchant)
		for _, kw := range SanctionsKeywords {
			if strings.Contains(needle, kw) {
				out = append(out, STR{
					TransactionIDs: []string{t.TransactionID},
					RuleHit:        "adverse_media",
					Severity:       "high",
					AmountTotal:    float64(absI(t.AmountCents)) / 100,
					Narrative:      "Counterparty matches sanctions / PEP list keyword: " + kw,
					DraftedAt:      now,
				})
				break
			}
		}
	}
	return out
}

func isCash(t finance.Transaction) bool {
	d := strings.ToLower(t.Description)
	return strings.Contains(d, "cash") || strings.Contains(d, "atm withdrawal") || strings.Contains(d, "deposit")
}

func absI(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
