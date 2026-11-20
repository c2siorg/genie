package catalog

import (
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// SyntheticIdentitySpec ports agents/synthetic_identity. It scores a KYC
// application for indicators of a "synthetic" identity — a fabricated identity
// assembled from real fragments (e.g. a stolen PAN + a fake address + a thin
// credit file). The port is deterministic — no model — so findings stay
// auditable for RBI / FIU-IND EDD review (FREE-AI Rec 25 explainability). It
// faithfully reproduces the legacy weighted rule stack, thresholds, labels and
// JSON field names:
//
//  1. thin file           — stated age ≥25 with <12 months bureau history (+0.30).
//  2. address velocity     — ≥3 other recent KYCs on the same address (+0.30).
//  3. PAN/Aadhaar mismatch — names share no common token (+0.25).
//  4. PAN structural check — 5th char ≠ surname initial (+0.20).
//  5. throwaway email      — disposable / temp mail domain (+0.15).
//  6. fresh SIM + thin file — SIM <30 days old with <12 months bureau (+0.20).
//
// Score is capped at 1.0; label is high (≥0.7), medium (≥0.4) else low.
func SyntheticIdentitySpec() afg.Spec {
	return afg.Spec{
		ID:   "synthetic_identity_detector",
		Risk: "high",
		Handle: func(input string) (string, error) {
			type application struct {
				StatedAge          int    `json:"stated_age"`
				BureauTenureMonths int    `json:"bureau_tenure_months"`
				NameOnPAN          string `json:"name_on_pan"`
				NameOnAadhaar      string `json:"name_on_aadhaar"`
				PANNumber          string `json:"pan_number"`
				EmailDomain        string `json:"email_domain"`
				PhoneCreatedDays   int    `json:"phone_created_days"`
				AddressID          string `json:"address_id"`
				AddressVelocity    int    `json:"address_velocity"`
			}
			type verdict struct {
				Score      float64  `json:"score_0_1"`
				Label      string   `json:"label"`
				Reasons    []string `json:"reasons"`
				Recommend  string   `json:"recommendation"`
				Disclaimer string   `json:"disclaimer"`
			}

			var app application
			if err := json.Unmarshal([]byte(input), &app); err != nil {
				return "", err
			}

			// Local helpers (kept inside the closure to stay self-contained).
			tokens := func(s string) map[string]bool {
				out := map[string]bool{}
				for _, w := range strings.Fields(strings.ToLower(s)) {
					if len(w) > 1 {
						out[w] = true
					}
				}
				return out
			}
			nameOverlap := func(a, b string) bool {
				at := tokens(a)
				bt := tokens(b)
				for k := range at {
					if bt[k] {
						return true
					}
				}
				return false
			}
			lastToken := func(s string) string {
				f := strings.Fields(strings.TrimSpace(s))
				if len(f) == 0 {
					return ""
				}
				return f[len(f)-1]
			}
			isThrowawayDomain := func(d string) bool {
				bad := []string{"mailinator", "tempmail", "10minutemail", "yopmail", "guerrillamail", "trashmail"}
				d = strings.ToLower(d)
				for _, b := range bad {
					if strings.Contains(d, b) {
						return true
					}
				}
				return false
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

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

			// 3. PAN/Aadhaar name mismatch (token-overlap heuristic).
			if app.NameOnPAN != "" && app.NameOnAadhaar != "" && !nameOverlap(app.NameOnPAN, app.NameOnAadhaar) {
				score += 0.25
				reasons = append(reasons, "PAN and Aadhaar names do not share a common token")
			}

			// 4. PAN structural check — 5th char should match the first letter of the surname.
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

			// 5. Throwaway-domain heuristic.
			if isThrowawayDomain(app.EmailDomain) {
				score += 0.15
				reasons = append(reasons, "Email domain looks like a temp / disposable address")
			}

			// 6. Brand-new SIM (<30 days) combined with thin file.
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

			v := verdict{
				Score:      round2(score),
				Label:      label,
				Reasons:    reasons,
				Recommend:  rec,
				Disclaimer: "Heuristic synthetic-identity score; not a substitute for full CKYCR + V-CIP review.",
			}
			body, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
