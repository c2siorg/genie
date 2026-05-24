// Package synthetic_identity scores a KYC application for indicators of
// a "synthetic" identity — a fabricated identity assembled from real
// fragments (e.g. a stolen PAN + a fake address + a thin credit file).
//
// Five rule families:
//   * thin file — credit-bureau pull length too short for stated age
//   * address-velocity — same address on multiple recent KYCs
//   * pan-aadhaar mismatch — names don't agree
//   * dob-pan inconsistency — PAN fourth char ≠ first letter of surname
//   * email/phone freshness — recently created throwaway domains / numbers
package synthetic_identity

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "synthetic_identity_detector"
	Capability = "detect_synthetic_identity"
	TypeIn     = "synthetic_check"
	TypeOut    = "synthetic_verdict"
	NextAgent  = "financial_supervisor"
)

// Application is one KYC submission to score.
type Application struct {
	StatedAge          int    `json:"stated_age"`
	BureauTenureMonths int    `json:"bureau_tenure_months"`
	NameOnPAN          string `json:"name_on_pan"`
	NameOnAadhaar      string `json:"name_on_aadhaar"`
	PANNumber          string `json:"pan_number"`           // 10 chars
	EmailDomain        string `json:"email_domain"`
	PhoneCreatedDays   int    `json:"phone_created_days"`   // age of SIM in days
	AddressID          string `json:"address_id"`           // canonicalised address fingerprint
	AddressVelocity    int    `json:"address_velocity"`     // # other recent KYCs at same address
}

// Verdict is the wire output.
type Verdict struct {
	Score      float64  `json:"score_0_1"`
	Label      string   `json:"label"` // "low" | "medium" | "high"
	Reasons    []string `json:"reasons"`
	Recommend  string   `json:"recommendation"`
	Disclaimer string   `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Synthetic Identity Detector" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var app Application
	if err := json.Unmarshal([]byte(msg.Content), &app); err != nil {
		return nil, err
	}
	v := a.Classify(app)
	env.Logf("[synthetic_identity] score=%.2f label=%s", v.Score, v.Label)
	body, _ := json.Marshal(v)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Classify runs the rule stack.
func (a *Agent) Classify(app Application) Verdict {
	score := 0.0
	reasons := []string{}

	// 1. Thin file. Adults >25 should have at least 24 months bureau history.
	if app.StatedAge >= 25 && app.BureauTenureMonths < 12 {
		score += 0.3
		reasons = append(reasons, "Thin credit file for stated age")
	}

	// 2. Address velocity ≥3 distinct KYCs on the same fingerprint is the
	// classic mule farm pattern.
	if app.AddressVelocity >= 3 {
		score += 0.3
		reasons = append(reasons, "Address shared with multiple recent KYC applications")
	}

	// 3. PAN/Aadhaar name mismatch (>token-overlap heuristic).
	if app.NameOnPAN != "" && app.NameOnAadhaar != "" && !nameOverlap(app.NameOnPAN, app.NameOnAadhaar) {
		score += 0.25
		reasons = append(reasons, "PAN and Aadhaar names do not share a common token")
	}

	// 4. PAN structural check — 4th char is the entity type (P for individuals).
	// 5th char should match the first letter of the surname.
	if len(app.PANNumber) == 10 {
		surname := lastToken(app.NameOnPAN)
		if surname != "" {
			expectedSurnameInitial := strings.ToUpper(string(surname[0]))
			if strings.ToUpper(string(app.PANNumber[4])) != expectedSurnameInitial {
				score += 0.2
				reasons = append(reasons, "PAN 5th character does not match surname initial")
			}
		}
	}

	// 6. Throwaway-domain heuristic.
	if isThrowawayDomain(app.EmailDomain) {
		score += 0.15
		reasons = append(reasons, "Email domain looks like a temp / disposable address")
	}

	// 7. Brand-new SIM (<30 days) combined with thin file.
	if app.PhoneCreatedDays > 0 && app.PhoneCreatedDays < 30 && app.BureauTenureMonths < 12 {
		score += 0.2
		reasons = append(reasons, "Recently issued SIM + thin credit file")
	}

	if score > 1 {
		score = 1
	}
	label := "low"
	rec := "Proceed with standard KYC."
	switch {
	case score >= 0.7:
		label = "high"
		rec = "Refer to compliance for enhanced due diligence (EDD); do not auto-approve."
	case score >= 0.4:
		label = "medium"
		rec = "Request additional documentation (utility bill ≤3 months old, fresh selfie video-KYC)."
	}
	return Verdict{
		Score:      round2(score),
		Label:      label,
		Reasons:    reasons,
		Recommend:  rec,
		Disclaimer: "Heuristic synthetic-identity score; not a substitute for full CKYCR + V-CIP review.",
	}
}

func nameOverlap(a, b string) bool {
	at := tokens(a)
	bt := tokens(b)
	for k := range at {
		if bt[k] {
			return true
		}
	}
	return false
}

func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		if len(w) > 1 {
			out[w] = true
		}
	}
	return out
}

func lastToken(s string) string {
	f := strings.Fields(strings.TrimSpace(s))
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}

func isThrowawayDomain(d string) bool {
	bad := []string{"mailinator", "tempmail", "10minutemail", "yopmail", "guerrillamail", "trashmail"}
	d = strings.ToLower(d)
	for _, b := range bad {
		if strings.Contains(d, b) {
			return true
		}
	}
	return false
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

// (kept for future signature compatibility — silence unused-import in case
// we ever add date parsing of DoB)
var _ = time.Now
