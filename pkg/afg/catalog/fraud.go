package catalog

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// FraudSpec ports agents/fraud. It scans a batch of transactions for
// suspicious patterns that a pure statistical anomaly detector misses on
// UPI / card rails — velocity bursts, geographic "impossible-travel" jumps,
// after-hours large debits, and high-risk merchant-category payments. The port
// is deterministic: it faithfully reproduces the legacy rule families with the
// same thresholds (≥5 txns in a 5-minute window, >900 km/h implied travel,
// ₹50k debits between 00:00–05:00 local, and the high-risk category set) and
// the same input/output JSON field names. Risk is medium because the agent only
// flags — downstream recommenders and human reviewers decide whether to block
// or call the customer (RBI FREE-AI Rec 17, human-in-the-loop).
func FraudSpec() afg.Spec {
	return afg.Spec{
		ID:   "fraud_detector",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				burstWindow      = 5 * time.Minute
				burstCount       = 5     // ≥N txns in burstWindow → flag
				minTravelKmph    = 900.0 // commercial-jet ceiling
				afterHoursStart  = 0     // hour
				afterHoursEnd    = 5
				afterHoursAmount = 50_000_00 // ₹50k in paise
			)
			highRiskCategories := map[string]bool{
				"crypto":      true,
				"gambling":    true,
				"forex":       true,
				"gift_cards":  true,
				"prepaid_top": true,
			}

			type txn struct {
				TransactionID string `json:"transaction_id"`
				Date          string `json:"date"`
				AmountCents   int64  `json:"amount_cents"`
				Description   string `json:"description"`
				Category      string `json:"category,omitempty"`
			}
			type signal struct {
				TransactionID string  `json:"transaction_id"`
				Pattern       string  `json:"pattern"`
				Severity      string  `json:"severity"`
				Reason        string  `json:"reason"`
				Confidence    float64 `json:"confidence"`
			}
			type result struct {
				Signals    []signal `json:"signals"`
				Disclaimer string   `json:"disclaimer"`
			}

			var in struct {
				Transactions []txn `json:"transactions"`
			}
			if err := json.Unmarshal([]byte(input), &in); err != nil {
				return "", err
			}

			// parseTxnTime tries a Description "TIME:YYYY-MM-DDTHH:MM" style suffix
			// first (enricher attaches it when minute-resolution is available),
			// then falls back to the date at midnight UTC.
			//nolint:unparam // the ok result is consumed by several callers below (velocity, impossible-travel, after-hours)
			parseTxnTime := func(t txn) (time.Time, bool) {
				if i := strings.Index(t.Description, "TIME:"); i >= 0 {
					tail := t.Description[i+len("TIME:"):]
					if end := strings.Index(tail, " "); end >= 0 {
						tail = tail[:end]
					}
					if when, err := time.Parse("2006-01-02T15:04", t.Date+"T"+tail); err == nil {
						return when, true
					}
				}
				when, err := time.Parse("2006-01-02", t.Date)
				if err != nil {
					return time.Time{}, false
				}
				return when, true
			}

			// parseGeoSuffix extracts "GEO:lat,lng" out of the description.
			parseGeoSuffix := func(desc string) (float64, float64, bool) {
				i := strings.Index(desc, "GEO:")
				if i < 0 {
					return 0, 0, false
				}
				tail := desc[i+len("GEO:"):]
				if end := strings.Index(tail, " "); end >= 0 {
					tail = tail[:end]
				}
				parts := strings.Split(tail, ",")
				if len(parts) != 2 {
					return 0, 0, false
				}
				lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
				if err != nil {
					return 0, 0, false
				}
				lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if err != nil {
					return 0, 0, false
				}
				return lat, lng, true
			}

			haversineKm := func(lat1, lng1, lat2, lng2 float64) float64 {
				const R = 6371.0
				rad := math.Pi / 180.0
				dLat := (lat2 - lat1) * rad
				dLng := (lng2 - lng1) * rad
				a := math.Sin(dLat/2)*math.Sin(dLat/2) +
					math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
				c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
				return R * c
			}

			out := result{
				Signals:    []signal{},
				Disclaimer: "Advisory fraud signals only — flags for human review, not an automated block. Downstream reviewers decide any action (RBI FREE-AI Rec 17).",
			}
			if len(in.Transactions) == 0 {
				body, err := json.Marshal(out)
				if err != nil {
					return "", err
				}
				return string(body), nil
			}

			// Sort a copy by time so the caller's ordering stays untouched.
			sorted := make([]txn, len(in.Transactions))
			copy(sorted, in.Transactions)
			sort.SliceStable(sorted, func(i, j int) bool {
				ti, _ := parseTxnTime(sorted[i])
				tj, _ := parseTxnTime(sorted[j])
				return ti.Before(tj)
			})

			signals := []signal{}

			// Velocity burst — ≥burstCount txns inside a sliding burstWindow.
			// Tags every txn in the first cluster found, then stops.
			for i := range sorted {
				ti, ok := parseTxnTime(sorted[i])
				if !ok {
					continue
				}
				j := i
				for j < len(sorted) {
					tj, ok := parseTxnTime(sorted[j])
					if !ok || tj.Sub(ti) > burstWindow {
						break
					}
					j++
				}
				if j-i >= burstCount {
					for k := i; k < j; k++ {
						signals = append(signals, signal{
							TransactionID: sorted[k].TransactionID,
							Pattern:       "velocity_burst",
							Severity:      "high",
							Reason:        "≥5 transactions within 5 minutes — possible card/UPI compromise",
							Confidence:    0.9,
						})
					}
					break // first burst is enough to escalate
				}
			}

			// Impossible travel — implausible speed between consecutive geocoded txns.
			{
				var lastLat, lastLng float64
				var lastTime time.Time
				have := false
				for _, t := range sorted {
					lat, lng, ok := parseGeoSuffix(t.Description)
					if !ok {
						continue
					}
					tt, ok := parseTxnTime(t)
					if !ok {
						continue
					}
					if have {
						distKm := haversineKm(lastLat, lastLng, lat, lng)
						gap := tt.Sub(lastTime).Hours()
						if gap > 0 && distKm/gap > minTravelKmph {
							signals = append(signals, signal{
								TransactionID: t.TransactionID,
								Pattern:       "impossible_travel",
								Severity:      "high",
								Reason:        "implausible distance/time vs previous geocoded transaction",
								Confidence:    0.85,
							})
						}
					}
					lastLat, lastLng, lastTime, have = lat, lng, tt, true
				}
			}

			// After-hours large debit — large debit between 00:00 and 05:00 local.
			for _, t := range sorted {
				if t.AmountCents >= 0 {
					continue // credits aren't fraud-suspicious here
				}
				if -t.AmountCents < afterHoursAmount {
					continue
				}
				tt, ok := parseTxnTime(t)
				if !ok {
					continue
				}
				h := tt.Hour()
				if h >= afterHoursStart && h < afterHoursEnd {
					signals = append(signals, signal{
						TransactionID: t.TransactionID,
						Pattern:       "after_hours_large_debit",
						Severity:      "medium",
						Reason:        "large debit during 00:00–05:00 local time",
						Confidence:    0.6,
					})
				}
			}

			// High-risk category — any debit to a high-risk merchant category.
			for _, t := range sorted {
				if t.AmountCents >= 0 {
					continue
				}
				cat := strings.ToLower(t.Category)
				if highRiskCategories[cat] {
					signals = append(signals, signal{
						TransactionID: t.TransactionID,
						Pattern:       "high_risk_category",
						Severity:      "medium",
						Reason:        "debit to high-risk merchant category (" + cat + ")",
						Confidence:    0.7,
					})
				}
			}

			out.Signals = signals
			body, err := json.Marshal(out)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
