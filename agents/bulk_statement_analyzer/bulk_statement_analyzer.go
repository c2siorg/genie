// Package bulk_statement_analyzer aggregates multiple bank statements
// (typically from an Account Aggregator fetch covering several accounts)
// into a single deduplicated cashflow view.
//
// The canonical ingestor→analyzer pipeline handles one statement at a time.
// For SME lending and AA-driven onboarding the applicant submits N statements
// across M accounts; the downstream cashflow_underwriter, working_capital,
// and goal_planner agents need a *single* consolidated view, not N.
//
// Dedup strategy: same (date, amount, normalised_description) within ±1 day
// counts as one transaction. This handles inter-account transfers showing
// up as a debit on Account A and a credit on Account B.
//
// Inspired by Google ADK samples → high-volume-document-analyzer.
package bulk_statement_analyzer

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "bulk_statement_analyzer"
	Capability = "consolidate_statements"
	TypeIn     = "bulk_statement_request"
	TypeOut    = "consolidated_cashflow"
	NextAgent  = "financial_supervisor"
)

// Txn is the canonical inbound transaction.
type Txn struct {
	AccountID   string  `json:"account_id"`
	Date        string  `json:"date"` // YYYY-MM-DD
	Description string  `json:"description"`
	Amount      float64 `json:"amount_rupees"`
	Type        string  `json:"type"` // "credit" | "debit"
}

// Request bundles all statements for one applicant.
type Request struct {
	ApplicantID  string `json:"applicant_id"`
	Transactions []Txn  `json:"transactions"`
}

// Summary is the consolidated output.
type Summary struct {
	ApplicantID    string             `json:"applicant_id"`
	AccountCount   int                `json:"account_count"`
	TxnCountRaw    int                `json:"txn_count_raw"`
	TxnCountDedup  int                `json:"txn_count_dedup"`
	DurationMonths int                `json:"duration_months"`
	TotalCredit    float64            `json:"total_credit_rupees"`
	TotalDebit     float64            `json:"total_debit_rupees"`
	NetCashflow    float64            `json:"net_cashflow_rupees"`
	MonthlyAverage float64            `json:"monthly_avg_inflow_rupees"`
	TopCategories  map[string]float64 `json:"top_debit_categories"`
	Disclaimer     string             `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Bulk Statement Analyzer" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	s := a.Consolidate(req)
	env.Logf("[bulk_statement_analyzer] applicant=%s accounts=%d raw=%d dedup=%d",
		s.ApplicantID, s.AccountCount, s.TxnCountRaw, s.TxnCountDedup)
	body, _ := json.Marshal(s)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Consolidate runs the dedup + aggregation pass. Pure function.
func (a *Agent) Consolidate(req Request) Summary {
	accountSet := map[string]struct{}{}
	for _, t := range req.Transactions {
		accountSet[t.AccountID] = struct{}{}
	}

	deduped := dedupe(req.Transactions)

	var credit, debit float64
	topCat := map[string]float64{}
	dateMin, dateMax := "", ""
	for _, t := range deduped {
		switch t.Type {
		case "credit":
			credit += t.Amount
		case "debit":
			debit += t.Amount
			cat := categorise(t.Description)
			topCat[cat] += t.Amount
		}
		if dateMin == "" || t.Date < dateMin {
			dateMin = t.Date
		}
		if t.Date > dateMax {
			dateMax = t.Date
		}
	}

	months := monthsBetween(dateMin, dateMax)
	monthly := 0.0
	if months > 0 {
		monthly = credit / float64(months)
	}

	return Summary{
		ApplicantID:    req.ApplicantID,
		AccountCount:   len(accountSet),
		TxnCountRaw:    len(req.Transactions),
		TxnCountDedup:  len(deduped),
		DurationMonths: months,
		TotalCredit:    round2(credit),
		TotalDebit:     round2(debit),
		NetCashflow:    round2(credit - debit),
		MonthlyAverage: round2(monthly),
		TopCategories:  topNCategories(topCat, 5),
		Disclaimer: "Consolidated cashflow across multiple statements with inter-account " +
			"transfer dedup. Categorisation is heuristic; verify before underwriting.",
	}
}

// dedupe removes inter-account transfers and exact duplicates.
// Two txns within ±1 day with the same amount and normalised description
// (with opposing types) are treated as one transfer and dropped from both
// sides (so the net effect on cashflow is zero).
func dedupe(txns []Txn) []Txn {
	type key struct {
		amount     float64
		descToken  string
	}
	matched := make([]bool, len(txns))

	for i := 0; i < len(txns); i++ {
		if matched[i] {
			continue
		}
		ki := key{txns[i].Amount, normalise(txns[i].Description)}
		for j := i + 1; j < len(txns); j++ {
			if matched[j] {
				continue
			}
			kj := key{txns[j].Amount, normalise(txns[j].Description)}
			if ki != kj {
				continue
			}
			if txns[i].Type == txns[j].Type {
				// exact duplicate (e.g. same statement loaded twice)
				matched[j] = true
				continue
			}
			// inter-account transfer if dates within ±1 day
			if dateWithin1Day(txns[i].Date, txns[j].Date) {
				matched[i], matched[j] = true, true
				break
			}
		}
	}

	out := make([]Txn, 0, len(txns))
	for i, t := range txns {
		if !matched[i] {
			out = append(out, t)
		}
	}
	return out
}

func normalise(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	d = strings.ReplaceAll(d, "  ", " ")
	if len(d) > 24 {
		d = d[:24]
	}
	return d
}

func dateWithin1Day(a, b string) bool {
	ta, ea := time.Parse("2006-01-02", a)
	tb, eb := time.Parse("2006-01-02", b)
	if ea != nil || eb != nil {
		return a == b
	}
	diff := ta.Sub(tb)
	if diff < 0 {
		diff = -diff
	}
	return diff <= 24*time.Hour
}

func monthsBetween(a, b string) int {
	ta, ea := time.Parse("2006-01-02", a)
	tb, eb := time.Parse("2006-01-02", b)
	if ea != nil || eb != nil {
		return 1
	}
	if tb.Before(ta) {
		ta, tb = tb, ta
	}
	m := int(tb.Month()) - int(ta.Month()) + 12*(tb.Year()-ta.Year()) + 1
	if m < 1 {
		m = 1
	}
	return m
}

func categorise(desc string) string {
	d := strings.ToLower(desc)
	switch {
	case strings.Contains(d, "rent"), strings.Contains(d, "lease"):
		return "housing:rent"
	case strings.Contains(d, "swiggy"), strings.Contains(d, "zomato"):
		return "food:delivery"
	case strings.Contains(d, "uber"), strings.Contains(d, "ola"):
		return "transport"
	case strings.Contains(d, "electric"), strings.Contains(d, "water"), strings.Contains(d, "gas"):
		return "utilities"
	case strings.Contains(d, "amazon"), strings.Contains(d, "flipkart"):
		return "shopping"
	case strings.Contains(d, "netflix"), strings.Contains(d, "spotify"), strings.Contains(d, "prime"):
		return "entertainment"
	default:
		return "other"
	}
}

func topNCategories(in map[string]float64, n int) map[string]float64 {
	type kv struct {
		k string
		v float64
	}
	var s []kv
	for k, v := range in {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].v > s[j].v })
	if len(s) > n {
		s = s[:n]
	}
	out := make(map[string]float64, len(s))
	for _, e := range s {
		out[e.k] = round2(e.v)
	}
	return out
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
