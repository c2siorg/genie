// Package fraud detects suspicious transaction patterns that the pure
// statistical anomaly_detector misses — velocity bursts, geography
// "impossible-travel" jumps, after-hours spikes, and high-risk merchant
// payments. Designed for UPI / card rails where fraud is sequence- and
// time-aware, not just amount-aware.
//
// Risk class is RiskMedium because the agent only flags; downstream
// recommenders + human reviewers decide whether to block or call the
// customer (RBI Rec 17 — human-in-the-loop for high-impact actions).
package fraud

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/finance"
)

const (
	ID         = "fraud_detector"
	Capability = "detect_fraud"
	TypeIn     = "analysis_result"
	TypeOut    = "fraud_signals"
	NextAgent  = "financial_supervisor"
)

// HighRiskCategories are merchants/categories where fraud rings cluster.
// Sourced from RBI / NPCI advisories on UPI fraud trends.
var HighRiskCategories = map[string]bool{
	"crypto":      true,
	"gambling":    true,
	"forex":       true,
	"gift_cards":  true,
	"prepaid_top": true,
}

// Defaults are tuned for retail-banking velocity; override via New(...).
const (
	DefaultBurstWindow      = 5 * time.Minute
	DefaultBurstCount       = 5     // ≥N txns in BurstWindow → flag
	DefaultMinTravelKmph    = 900.0 // commercial-jet ceiling
	DefaultAfterHoursStart  = 0     // hour
	DefaultAfterHoursEnd    = 5
	DefaultAfterHoursAmount = 50_000_00 // ₹50k in paise
)

// Signal is one fraud finding. Severity is "low" / "medium" / "high"
// matching the RBI risk grading so reporters and incident flow downstream
// can route on it directly.
type Signal struct {
	TransactionID string  `json:"transaction_id"`
	Pattern       string  `json:"pattern"`
	Severity      string  `json:"severity"`
	Reason        string  `json:"reason"`
	Confidence    float64 `json:"confidence"` // 0..1
}

// Result is the message payload.
type Result struct {
	Signals []Signal `json:"signals"`
}

// Agent implements agent.Agent.
type Agent struct {
	BurstWindow       time.Duration
	BurstCount        int
	MinTravelKmph     float64
	AfterHoursStart   int
	AfterHoursEnd     int
	AfterHoursAmount  int64
	HighRiskCategorie map[string]bool // shadow of the package var so tests can override
}

// New returns an agent with the package defaults.
func New() *Agent {
	return &Agent{
		BurstWindow:       DefaultBurstWindow,
		BurstCount:        DefaultBurstCount,
		MinTravelKmph:     DefaultMinTravelKmph,
		AfterHoursStart:   DefaultAfterHoursStart,
		AfterHoursEnd:     DefaultAfterHoursEnd,
		AfterHoursAmount:  DefaultAfterHoursAmount,
		HighRiskCategorie: HighRiskCategories,
	}
}

func (a *Agent) ID() string                     { return ID }
func (a *Agent) Name() string                   { return "Fraud Detector" }
func (a *Agent) Capabilities() []string         { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass     { return agent.RiskMedium }

type analyzerView struct {
	Transactions []finance.Transaction `json:"transactions"`
}

// HandleMessage runs the rule set against the incoming analyzer batch.
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var av analyzerView
	if err := json.Unmarshal([]byte(msg.Content), &av); err != nil {
		return nil, err
	}
	signals := a.Detect(av.Transactions)
	env.Logf("[fraud] %d signals across %d txns", len(signals), len(av.Transactions))
	body, _ := json.Marshal(Result{Signals: signals})
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Detect runs the rule set on a sorted-by-time copy of the transactions.
// Exported so tests and other agents can reuse the pure-data path.
func (a *Agent) Detect(txns []finance.Transaction) []Signal {
	if len(txns) == 0 {
		return nil
	}
	// Sort a copy so the caller's slice stays untouched.
	sorted := make([]finance.Transaction, len(txns))
	copy(sorted, txns)
	sort.SliceStable(sorted, func(i, j int) bool {
		ti, _ := parseTxnTime(sorted[i])
		tj, _ := parseTxnTime(sorted[j])
		return ti.Before(tj)
	})

	out := []Signal{}
	out = append(out, a.detectBursts(sorted)...)
	out = append(out, a.detectImpossibleTravel(sorted)...)
	out = append(out, a.detectAfterHoursLargeAmounts(sorted)...)
	out = append(out, a.detectHighRiskCategory(sorted)...)
	return out
}

// detectBursts flags clusters of ≥BurstCount transactions inside a sliding
// BurstWindow. Tags every transaction in the cluster.
func (a *Agent) detectBursts(sorted []finance.Transaction) []Signal {
	signals := []Signal{}
	for i := range sorted {
		ti, ok := parseTxnTime(sorted[i])
		if !ok {
			continue
		}
		j := i
		for j < len(sorted) {
			tj, ok := parseTxnTime(sorted[j])
			if !ok || tj.Sub(ti) > a.BurstWindow {
				break
			}
			j++
		}
		if j-i >= a.BurstCount {
			for k := i; k < j; k++ {
				signals = append(signals, Signal{
					TransactionID: sorted[k].TransactionID,
					Pattern:       "velocity_burst",
					Severity:      "high",
					Reason:        "≥5 transactions within 5 minutes — possible card/UPI compromise",
					Confidence:    0.9,
				})
			}
			return signals // first burst is enough to escalate
		}
	}
	return signals
}

// detectImpossibleTravel scans consecutive transactions for geography jumps
// requiring an implausible speed. Geography lives in finance.Transaction
// nowhere yet — we read it from the description suffix "GEO:lat,lng" which
// the enricher can set when card+location data is available.
func (a *Agent) detectImpossibleTravel(sorted []finance.Transaction) []Signal {
	signals := []Signal{}
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
			if gap > 0 && distKm/gap > a.MinTravelKmph {
				signals = append(signals, Signal{
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
	return signals
}

// detectAfterHoursLargeAmounts flags large debits between 00:00 and 05:00
// local time. The default is ₹50k, configurable.
func (a *Agent) detectAfterHoursLargeAmounts(sorted []finance.Transaction) []Signal {
	signals := []Signal{}
	for _, t := range sorted {
		if t.AmountCents >= 0 {
			continue // credits aren't fraud-suspicious here
		}
		if -t.AmountCents < a.AfterHoursAmount {
			continue
		}
		tt, ok := parseTxnTime(t)
		if !ok {
			continue
		}
		h := tt.Hour()
		if h >= a.AfterHoursStart && h < a.AfterHoursEnd {
			signals = append(signals, Signal{
				TransactionID: t.TransactionID,
				Pattern:       "after_hours_large_debit",
				Severity:      "medium",
				Reason:        "large debit during 00:00–05:00 local time",
				Confidence:    0.6,
			})
		}
	}
	return signals
}

// detectHighRiskCategory flags any debit to a high-risk merchant category.
func (a *Agent) detectHighRiskCategory(sorted []finance.Transaction) []Signal {
	signals := []Signal{}
	for _, t := range sorted {
		if t.AmountCents >= 0 {
			continue
		}
		cat := strings.ToLower(t.Category)
		if a.HighRiskCategorie[cat] {
			signals = append(signals, Signal{
				TransactionID: t.TransactionID,
				Pattern:       "high_risk_category",
				Severity:      "medium",
				Reason:        "debit to high-risk merchant category (" + cat + ")",
				Confidence:    0.7,
			})
		}
	}
	return signals
}

// parseTxnTime tries Description "TIME:HH:MM" suffix first, then falls back
// to ParsedDate at midnight UTC. The enricher attaches TIME:... when the
// upstream rail provides minute-resolution.
func parseTxnTime(t finance.Transaction) (time.Time, bool) {
	if i := strings.Index(t.Description, "TIME:"); i >= 0 {
		tail := t.Description[i+len("TIME:"):]
		if end := strings.Index(tail, " "); end >= 0 {
			tail = tail[:end]
		}
		if when, err := time.Parse("2006-01-02T15:04", t.Date+"T"+tail); err == nil {
			return when, true
		}
	}
	when, err := t.ParsedDate()
	if err != nil {
		return time.Time{}, false
	}
	return when, true
}

// parseGeoSuffix extracts "GEO:lat,lng" out of the description, if present.
func parseGeoSuffix(desc string) (float64, float64, bool) {
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
	var lat, lng float64
	if _, err := fmtScan(parts[0], &lat); err != nil {
		return 0, 0, false
	}
	if _, err := fmtScan(parts[1], &lng); err != nil {
		return 0, 0, false
	}
	return lat, lng, true
}

// haversineKm — great-circle distance in km. We don't need a great deal of
// precision here; the test for impossible_travel uses ≥900 km/h.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// fmtScan is a tiny strconv wrapper kept inline to avoid pulling in fmt.
func fmtScan(s string, out *float64) (int, error) {
	var x float64
	var sign float64 = 1
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	hasDigit := false
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		x = x*10 + float64(s[i]-'0')
		hasDigit = true
	}
	if i < len(s) && s[i] == '.' {
		i++
		div := 1.0
		for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
			div *= 10
			x += float64(s[i]-'0') / div
			hasDigit = true
		}
	}
	if !hasDigit {
		return 0, errEmptyNumber
	}
	*out = sign * x
	return 1, nil
}

// errEmptyNumber lets fmtScan stay zero-alloc.
var errEmptyNumber = &numErr{"empty number"}

type numErr struct{ s string }

func (e *numErr) Error() string { return e.s }
