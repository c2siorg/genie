package catalog

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// AmlMonitorSpec ports agents/aml_monitor. It scans a batch of transactions for
// AML/CFT red flags per FIU-IND guidance and PMLA 2002 and drafts Suspicious
// Transaction Report (STR) entries for a compliance officer to review. The port
// is deterministic: it faithfully reproduces the legacy rule families
// (CTR/LTR threshold breach, structuring within a 7-day window, and adverse-media
// keyword matches) with the same thresholds and JSON field names.
func AmlMonitorSpec() afg.Spec {
	return afg.Spec{
		ID:   "aml_monitor",
		Risk: "high",
		Handle: func(input string) (string, error) {
			const (
				cashCTRThreshold      = 10_00_000_00 // ₹10L in paise — Cash Transaction Report
				wireLTRThreshold      = 50_00_000_00 // ₹50L wire — Large Transaction Report
				structuringWindowDays = 7
				structuringMinCount   = 3
				structuringFloorPct   = 0.80 // each txn ≥80% of CTR threshold but < threshold
			)
			sanctionsKeywords := []string{"sanctioned-entity", "pep-flag", "ofac-listed"}

			type txn struct {
				TransactionID string `json:"transaction_id"`
				AccountID     string `json:"account_id"`
				Date          string `json:"date"`
				AmountCents   int64  `json:"amount_cents"`
				Description   string `json:"description"`
				Merchant      string `json:"merchant,omitempty"`
			}
			type str struct {
				TransactionIDs []string  `json:"transaction_ids"`
				RuleHit        string    `json:"rule_hit"`
				Severity       string    `json:"severity"`
				AmountTotal    float64   `json:"amount_total_rupees"`
				Narrative      string    `json:"narrative"`
				DraftedAt      time.Time `json:"drafted_at"`
			}
			type result struct {
				STRs       []str  `json:"strs"`
				Note       string `json:"note"`
				Disclaimer string `json:"disclaimer"`
			}
			var in struct {
				Transactions []txn `json:"transactions"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return "", err
			}

			absI := func(x int64) int64 {
				if x < 0 {
					return -x
				}
				return x
			}
			isCash := func(t txn) bool {
				d := strings.ToLower(t.Description)
				return strings.Contains(d, "cash") ||
					strings.Contains(d, "atm withdrawal") ||
					strings.Contains(d, "deposit")
			}
			parseDate := func(s string) (time.Time, error) {
				return time.Parse("2006-01-02", s)
			}

			now := time.Now().UTC()
			strs := []str{}

			// Threshold breach — any single txn ≥ CTR/LTR threshold.
			for _, t := range in.Transactions {
				amt := absI(t.AmountCents)
				switch {
				case isCash(t) && amt >= cashCTRThreshold:
					strs = append(strs, str{
						TransactionIDs: []string{t.TransactionID},
						RuleHit:        "ctr_threshold",
						Severity:       "high",
						AmountTotal:    float64(amt) / 100,
						Narrative:      "Single cash transaction ≥₹10L — Cash Transaction Report (CTR) due within 15 days.",
						DraftedAt:      now,
					})
				case amt >= wireLTRThreshold:
					strs = append(strs, str{
						TransactionIDs: []string{t.TransactionID},
						RuleHit:        "wire_threshold",
						Severity:       "medium",
						AmountTotal:    float64(amt) / 100,
						Narrative:      "Wire transfer ≥₹50L — Large Transaction Report (LTR) due.",
						DraftedAt:      now,
					})
				}
			}

			// Structuring — ≥3 cash txns each 80–99% of CTR inside a 7-day window on one account.
			candidates := []txn{}
			for _, t := range in.Transactions {
				if !isCash(t) {
					continue
				}
				amt := absI(t.AmountCents)
				if float64(amt) >= structuringFloorPct*cashCTRThreshold && amt < cashCTRThreshold {
					candidates = append(candidates, t)
				}
			}
			if len(candidates) >= structuringMinCount {
				groups := map[string][]txn{}
				for _, t := range candidates {
					groups[t.AccountID] = append(groups[t.AccountID], t)
				}
				for acct, list := range groups {
					if len(list) < structuringMinCount {
						continue
					}
					first, _ := parseDate(list[0].Date)
					last, _ := parseDate(list[len(list)-1].Date)
					if last.Sub(first).Hours()/24 <= structuringWindowDays {
						ids := []string{}
						var total int64
						for _, t := range list {
							ids = append(ids, t.TransactionID)
							total += absI(t.AmountCents)
						}
						strs = append(strs, str{
							TransactionIDs: ids,
							RuleHit:        "structuring",
							Severity:       "high",
							AmountTotal:    float64(total) / 100,
							Narrative:      "≥3 cash transactions on account " + acct + " each between 80–99% of CTR threshold within a 7-day window — possible structuring.",
							DraftedAt:      now,
						})
					}
				}
			}

			// Adverse media — description/merchant hits a sanctions/PEP keyword.
			for _, t := range in.Transactions {
				needle := strings.ToLower(t.Description + " " + t.Merchant)
				for _, kw := range sanctionsKeywords {
					if strings.Contains(needle, kw) {
						strs = append(strs, str{
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

			out := result{
				STRs:       strs,
				Note:       "Compliance-officer review required before submission to FIU-IND.",
				Disclaimer: "Rule-based first pass. Final STR / CTR filing must be approved by Principal Officer.",
			}
			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
