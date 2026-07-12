package catalog

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// MuleSpec ports agents/mule. It flags money-mule accounts used by fraud rings
// to layer stolen funds. The port is deterministic — no model — so findings stay
// auditable for RBI / FIU-IND reporting (FREE-AI Rec 25 explainability). It
// faithfully reproduces the three legacy structural rules over a batch of
// transactions (grouped per account_id) with the same thresholds and JSON field
// names:
//
//  1. pass_through      — a credit anchor with ≥90% of it forwarded out as debits
//     within a 24h window.
//  2. fan_in_fan_out    — ≥5 distinct incoming and ≥5 distinct outgoing merchants
//     (counterparties) on one account — hub pattern.
//  3. post_credit_burst — ≥10 debits within 1h of a credit — turnover spike.
//
// Signals are deduplicated by (account, pattern) keeping the highest confidence.
func MuleSpec() afg.Spec {
	return afg.Spec{
		ID:   "mule_detector",
		Risk: "high",
		Handle: func(input string) (string, error) {
			const (
				passThroughWindow = 24 * time.Hour
				passThroughFrac   = 0.9
				fanInThreshold    = 5
				fanOutThreshold   = 5
				velocityWindow    = 1 * time.Hour
				velocityCount     = 10
			)

			type txn struct {
				AccountID   string `json:"account_id"`
				Date        string `json:"date"`
				AmountCents int64  `json:"amount_cents"`
				Merchant    string `json:"merchant,omitempty"`
			}
			type signal struct {
				AccountID  string  `json:"account_id"`
				Pattern    string  `json:"pattern"`
				Severity   string  `json:"severity"`
				Reason     string  `json:"reason"`
				Confidence float64 `json:"confidence"`
			}
			type result struct {
				Signals []signal `json:"signals"`
			}

			var in struct {
				Transactions []txn `json:"transactions"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return "", err
			}

			parseDate := func(s string) (time.Time, error) {
				return time.Parse("2006-01-02", s)
			}

			out := []signal{}
			if len(in.Transactions) > 0 {
				// Group by account.
				byAccount := map[string][]txn{}
				for _, t := range in.Transactions {
					byAccount[t.AccountID] = append(byAccount[t.AccountID], t)
				}

				for acct, list := range byAccount {
					if acct == "" {
						continue
					}
					sort.SliceStable(list, func(i, j int) bool {
						ti, _ := parseDate(list[i].Date)
						tj, _ := parseDate(list[j].Date)
						return ti.Before(tj)
					})

					// Rule 1: pass-through — a credit anchor with ≥frac of it
					// forwarded out as debits inside the window.
					for i, t := range list {
						if t.AmountCents <= 0 {
							continue // need a credit anchor
						}
						credit := t.AmountCents
						ti, _ := parseDate(t.Date)
						windowEnd := ti.Add(passThroughWindow)
						var debits int64
						for j := i + 1; j < len(list); j++ {
							tj, _ := parseDate(list[j].Date)
							if tj.After(windowEnd) {
								break
							}
							if list[j].AmountCents < 0 {
								debits += -list[j].AmountCents
							}
						}
						if credit > 0 && float64(debits)/float64(credit) >= passThroughFrac {
							out = append(out, signal{
								AccountID:  acct,
								Pattern:    "pass_through",
								Severity:   "high",
								Reason:     "≥90% of a recent credit forwarded out within 24h",
								Confidence: 0.92,
							})
						}
					}

					// Rule 2: fan-in fan-out — many distinct counterparties on
					// both sides (Merchant is the counterparty).
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
					if len(inMerchants) >= fanInThreshold && len(outMerchants) >= fanOutThreshold {
						out = append(out, signal{
							AccountID:  acct,
							Pattern:    "fan_in_fan_out",
							Severity:   "high",
							Reason:     "account dispatches to many merchants and receives from many distinct senders — hub pattern",
							Confidence: 0.85,
						})
					}

					// Rule 3: velocity after credit — burst of debits shortly
					// after a credit.
					for i, t := range list {
						if t.AmountCents <= 0 {
							continue
						}
						ti, _ := parseDate(t.Date)
						windowEnd := ti.Add(velocityWindow)
						debits := 0
						for j := i + 1; j < len(list); j++ {
							tj, _ := parseDate(list[j].Date)
							if tj.After(windowEnd) {
								break
							}
							if list[j].AmountCents < 0 {
								debits++
							}
						}
						if debits >= velocityCount {
							out = append(out, signal{
								AccountID:  acct,
								Pattern:    "post_credit_burst",
								Severity:   "high",
								Reason:     "≥10 debits within 1h of a credit — turnover spike",
								Confidence: 0.88,
							})
							break
						}
					}
				}
			}

			// Dedupe by (account, pattern) — keep the highest confidence.
			dedup := map[string]signal{}
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
			signals := make([]signal, 0, len(keys))
			for _, k := range keys {
				signals = append(signals, dedup[k])
			}

			body, err := json.Marshal(result{Signals: signals})
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
