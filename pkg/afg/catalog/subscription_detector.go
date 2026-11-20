package catalog

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// SubscriptionDetectorSpec ports agents/subscription_detector. It scans a
// transaction list for recurring subscription-style debits using two stacked
// signals: periodicity (same merchant debited on >=3 distinct charges with a
// <=7-day standard deviation of day-of-month, i.e. a monthly cadence) and
// amount consistency (>=80% of earlier charges within +/-5% of a stable
// typical amount). A final price jump of >=20% versus the first detection is
// flagged as a "zombie" subscription rather than disqualifying the merchant.
// Output annualises each typical amount (x12), totals them, and attaches an
// audit recommendation. Amounts are in rupees; outputs are informational spend
// analysis per RBI FREE-AI.
func SubscriptionDetectorSpec() afg.Spec {
	return afg.Spec{
		ID:   "subscription_detector",
		Risk: "low",
		Handle: func(input string) (string, error) {
			const (
				minOccurrences  = 3
				maxDayStdDev    = 7.0
				amountTolerance = 0.05 // +/-5%
				zombiePriceJump = 0.20 // >=20% price hike vs the first detection
			)

			// transaction mirrors finance.Transaction's wire fields.
			type transaction struct {
				TransactionID string `json:"transaction_id"`
				AccountID     string `json:"account_id"`
				Date          string `json:"date"` // ISO-8601 YYYY-MM-DD
				AmountCents   int64  `json:"amount_cents"`
				Currency      string `json:"currency"`
				Description   string `json:"description"`
				Merchant      string `json:"merchant,omitempty"`
				Category      string `json:"category,omitempty"`
				Direction     string `json:"direction,omitempty"`
			}
			type analyzerView struct {
				Transactions []transaction `json:"transactions"`
			}
			type subscription struct {
				Merchant            string  `json:"merchant"`
				OccurrenceCount     int     `json:"occurrence_count"`
				TypicalAmountRupees float64 `json:"typical_amount_rupees"`
				LastAmountRupees    float64 `json:"last_amount_rupees"`
				AnnualisedINR       float64 `json:"annualised_rupees"`
				ZombieWarning       string  `json:"zombie_warning,omitempty"`
			}
			type result struct {
				Subscriptions  []subscription `json:"subscriptions"`
				AnnualisedINR  float64        `json:"total_annualised_rupees"`
				Recommendation string         `json:"recommendation"`
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			median := func(xs []float64) float64 {
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

			std := func(xs []float64) float64 {
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

			// parseDay extracts the day-of-month from a YYYY-MM-DD date; the
			// second return reports whether parsing succeeded.
			parseDay := func(date string) (int, bool) {
				if len(date) != 10 || date[4] != '-' || date[7] != '-' {
					return 0, false
				}
				d := int(date[8]-'0')*10 + int(date[9]-'0')
				if date[8] < '0' || date[8] > '9' || date[9] < '0' || date[9] > '9' {
					return 0, false
				}
				if d < 1 || d > 31 {
					return 0, false
				}
				return d, true
			}

			var av analyzerView
			if err := json.Unmarshal([]byte(input), &av); err != nil {
				return "", err
			}

			byMerchant := map[string][]transaction{}
			for _, t := range av.Transactions {
				if t.AmountCents >= 0 || t.Merchant == "" {
					continue
				}
				byMerchant[t.Merchant] = append(byMerchant[t.Merchant], t)
			}

			// Deterministic merchant iteration order for stable output.
			merchants := make([]string, 0, len(byMerchant))
			for m := range byMerchant {
				merchants = append(merchants, m)
			}
			sort.Strings(merchants)

			res := result{}
			for _, merchant := range merchants {
				list := byMerchant[merchant]
				if len(list) < minOccurrences {
					continue
				}
				sort.SliceStable(list, func(i, j int) bool {
					return list[i].Date < list[j].Date
				})
				days := []float64{}
				amounts := []float64{}
				for _, t := range list {
					day, ok := parseDay(t.Date)
					if !ok {
						continue
					}
					days = append(days, float64(day))
					amounts = append(amounts, float64(-t.AmountCents)/100)
				}
				if len(amounts) == 0 {
					continue
				}
				if std(days) > maxDayStdDev {
					continue
				}
				// Establish "typical" from all-but-last charges so a final price
				// jump can be flagged as a zombie subscription instead of
				// disqualifying the whole merchant.
				earlier := amounts[:len(amounts)-1]
				typical := median(earlier)
				if typical == 0 {
					typical = median(amounts)
				}
				if typical == 0 {
					continue
				}
				// >=80% of the earlier charges must be within +/-tolerance of typical.
				consistent := 0
				for _, x := range earlier {
					if math.Abs(x-typical)/typical <= amountTolerance {
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
				if first > 0 && (last-first)/first >= zombiePriceJump {
					zombie = "Price crept up vs first detection — review value or cancel."
				}
				sub := subscription{
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

			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
