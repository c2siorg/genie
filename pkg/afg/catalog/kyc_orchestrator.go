package catalog

import (
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// KycOrchestratorSpec ports agents/kyc_orchestrator. It sequences the deterministic
// checks that make up a full Indian KYC workflow per the RBI Master Direction on KYC
// (PAN structural validity, Aadhaar offline KYC, PAN/Aadhaar name match, address and
// liveness thresholds, PEP/sanctions screening, geography and occupation risk), then
// scores the packet and routes it to approve / EDD / reject with SDD/standard/EDD
// tiering. The port is deterministic: it faithfully reproduces the legacy decision
// tree, weights, thresholds (EDD ≥0.70, SDD ≤0.30) and JSON field names. A sanctions
// hit is an automatic reject carrying an Annexure VI incident payload.
func KycOrchestratorSpec() afg.Spec {
	return afg.Spec{
		ID:   "kyc_orchestrator",
		Risk: "high",
		Handle: func(input string) (string, error) {
			const (
				scoreEDD = 0.70 // ≥ score → Enhanced Due Diligence
				scoreSDD = 0.30 // ≤ score → Simplified Due Diligence
			)

			type Application struct {
				CustomerID         string  `json:"customer_id"`
				PANNumber          string  `json:"pan_number"`
				NameOnPAN          string  `json:"name_on_pan"`
				AadhaarLast4       string  `json:"aadhaar_last4"`
				AadhaarOfflineKYC  bool    `json:"aadhaar_offline_kyc"`
				NameOnAadhaar      string  `json:"name_on_aadhaar"`
				DigiLockerVerified bool    `json:"digilocker_verified"`
				AddressMatchScore  float64 `json:"address_match_score_0_1"`
				LivenessScore      float64 `json:"liveness_score_0_1"`
				PEPHit             bool    `json:"pep_hit"`
				SanctionsHit       bool    `json:"sanctions_hit"`
				CountryOfResidence string  `json:"country_of_residence"`
				HighRiskCountry    bool    `json:"high_risk_country"`
				OccupationHighRisk bool    `json:"occupation_high_risk"`
			}
			type Verdict struct {
				CustomerID      string   `json:"customer_id"`
				Decision        string   `json:"decision"`
				Tier            string   `json:"tier"`
				RiskScore       float64  `json:"risk_score_0_1"`
				Reasons         []string `json:"reasons"`
				NextSteps       []string `json:"next_steps"`
				IncidentPayload string   `json:"incident_payload,omitempty"`
				Disclaimer      string   `json:"disclaimer"`
			}

			// --- local helpers (closure-scoped, no package-level decls) ---

			tokens := func(s string) map[string]bool {
				out := map[string]bool{}
				for _, w := range strings.Fields(strings.ToLower(s)) {
					if len(w) > 1 {
						out[w] = true
					}
				}
				return out
			}
			nameTokensOverlap := func(a, b string) bool {
				ta := tokens(a)
				tb := tokens(b)
				for k := range ta {
					if tb[k] {
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
			panLooksValid := func(pan, name string) bool {
				if len(pan) != 10 {
					return false
				}
				if strings.ToUpper(string(pan[3])) != "P" {
					return false
				}
				surname := lastToken(name)
				if surname == "" {
					return true // can't check; don't penalise on missing name
				}
				return strings.ToUpper(string(pan[4])) == strings.ToUpper(string(surname[0]))
			}
			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			rejectVerdict := func(app Application, s float64, reason string) Verdict {
				payload, _ := json.Marshal(map[string]any{
					"annexure":     "VI",
					"customer_id":  app.CustomerID,
					"reason":       reason,
					"severity":     "high",
					"action_taken": "Auto-reject; refer to FIU-IND if confirmed.",
				})
				return Verdict{
					CustomerID:      app.CustomerID,
					Decision:        "reject",
					Tier:            "edd",
					RiskScore:       s,
					Reasons:         []string{reason},
					NextSteps:       []string{"File STR with FIU-IND if confirmed sanctioned"},
					IncidentPayload: string(payload),
					Disclaimer:      "Auto-reject triggered by sanctions match; verify list version before final action.",
				}
			}

			decide := func(app Application) Verdict {
				score := 0.0
				reasons := []string{}
				next := []string{}

				// 1. PAN structural validity.
				if !panLooksValid(app.PANNumber, app.NameOnPAN) {
					score += 0.30
					reasons = append(reasons, "PAN failed structural validation (length/4th/5th-char)")
				}

				// 2. Aadhaar offline KYC required.
				if !app.AadhaarOfflineKYC {
					score += 0.20
					reasons = append(reasons, "Aadhaar offline KYC XML missing")
					next = append(next, "Request UIDAI offline KYC XML via Aadhaar e-Aadhaar portal")
				}

				// 3. Name match across PAN / Aadhaar.
				if app.NameOnPAN != "" && app.NameOnAadhaar != "" && !nameTokensOverlap(app.NameOnPAN, app.NameOnAadhaar) {
					score += 0.20
					reasons = append(reasons, "Name on PAN does not share a token with name on Aadhaar")
				}

				// 4. Address validation must meet threshold.
				if app.AddressMatchScore < 0.80 {
					score += 0.10
					reasons = append(reasons, "Address match below 0.80 confidence")
				}

				// 5. Liveness / V-CIP gate.
				if app.LivenessScore > 0 && app.LivenessScore < 0.70 {
					score += 0.15
					reasons = append(reasons, "Liveness below 0.70 — possible spoof attempt")
					next = append(next, "Re-run V-CIP with live agent")
				}

				// 6. PEP / sanctions. Sanctions = automatic reject.
				if app.SanctionsHit {
					return rejectVerdict(app, 1.0, "Sanctions list hit (OFAC / UN / MHA)")
				}
				if app.PEPHit {
					score += 0.35
					reasons = append(reasons, "Politically Exposed Person — Enhanced Due Diligence required")
				}

				// 7. Geography risk.
				if app.HighRiskCountry {
					score += 0.20
					reasons = append(reasons, "Residence in FATF high-risk jurisdiction")
				}

				// 8. Occupation risk.
				if app.OccupationHighRisk {
					score += 0.10
					reasons = append(reasons, "Occupation flagged as high-risk per FATF guidance")
				}

				if score > 1 {
					score = 1
				}

				decision := "approve"
				tier := "standard"
				switch {
				case score >= scoreEDD:
					decision = "edd"
					tier = "edd"
					next = append(next, "Refer to compliance for Enhanced Due Diligence (Master Direction §V)")
				case score <= scoreSDD:
					tier = "sdd"
				}

				return Verdict{
					CustomerID: app.CustomerID,
					Decision:   decision,
					Tier:       tier,
					RiskScore:  round2(score),
					Reasons:    reasons,
					NextSteps:  next,
					Disclaimer: "Deterministic KYC risk score per RBI Master Direction. " +
						"Not a substitute for compliance-officer review on EDD-tier outcomes.",
				}
			}

			var app Application
			if err := json.Unmarshal([]byte(input), &app); err != nil {
				return "", err
			}
			body, err := json.Marshal(decide(app))
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
