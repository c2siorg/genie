package catalog

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// BulkStatementAnalyzerSpec ports agents/bulk_statement_analyzer. It consolidates
// multiple bank statements (typically an Account Aggregator fetch across several
// accounts) into a single deduplicated cashflow view: inter-account transfers and
// exact duplicates are removed, credits/debits are totalled, debit spend is bucketed
// into heuristic categories, and a monthly average inflow is derived over the
// statement span.
func BulkStatementAnalyzerSpec() afg.Spec {
	return afg.Spec{
		ID:   "bulk_statement_analyzer",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			type txn struct {
				AccountID   string  `json:"account_id"`
				Date        string  `json:"date"` // YYYY-MM-DD
				Description string  `json:"description"`
				Amount      float64 `json:"amount_rupees"`
				Type        string  `json:"type"` // "credit" | "debit"
			}
			type request struct {
				ApplicantID  string `json:"applicant_id"`
				Transactions []txn  `json:"transactions"`
			}
			type summary struct {
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

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			normalise := func(d string) string {
				d = strings.ToLower(strings.TrimSpace(d))
				d = strings.ReplaceAll(d, "  ", " ")
				if len(d) > 24 {
					d = d[:24]
				}
				return d
			}

			dateWithin1Day := func(a, b string) bool {
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

			monthsBetween := func(a, b string) int {
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

			categorise := func(desc string) string {
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

			// dedupe removes inter-account transfers and exact duplicates.
			// Two txns within +/-1 day with the same amount and normalised
			// description are treated as one transfer (opposing types) or a
			// duplicate (same type) and dropped accordingly.
			dedupe := func(txns []txn) []txn {
				type key struct {
					amount    float64
					descToken string
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
						// inter-account transfer if dates within +/-1 day
						if dateWithin1Day(txns[i].Date, txns[j].Date) {
							matched[i], matched[j] = true, true
							break
						}
					}
				}
				out := make([]txn, 0, len(txns))
				for i, t := range txns {
					if !matched[i] {
						out = append(out, t)
					}
				}
				return out
			}

			topNCategories := func(in map[string]float64, n int) map[string]float64 {
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

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

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

			s := summary{
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

			body, err := json.Marshal(s)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
