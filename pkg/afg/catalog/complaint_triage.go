package catalog

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// ComplaintTriageSpec ports agents/complaint_triage. It classifies a free-text
// customer grievance into one of the RBI Integrated Ombudsman Scheme 2021
// categories using a deterministic first-match keyword pass, gauges severity via
// escalator tokens, scores confidence from the number of keyword matches, flags
// vague complaints for human review, and assembles a structured incident draft
// (Annexure VI of the FREE-AI report) with a suggested remediation action and a
// heuristic ombudsman-eligibility flag. Deterministic by design so the routing
// logic is auditable from source (Rec 25 explainability).
func ComplaintTriageSpec() afg.Spec {
	return afg.Spec{
		ID:   "complaint_triage",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const (
				catDepositAccounts = "deposit_accounts"
				catLoansAdvances   = "loans_and_advances"
				catDigitalBanking  = "digital_banking_upi_imps"
				catCards           = "cards"
				catRemittance      = "remittance"
				catChargesFees     = "charges_and_fees"
				catStaffConduct    = "staff_conduct"
				catMisSelling      = "mis_selling"
				catOther           = "other"
			)

			type request struct {
				UserID         string `json:"user_id"`
				ComplaintText  string `json:"complaint_text"`
				ProductHint    string `json:"product_hint,omitempty"`
				ChannelHint    string `json:"channel_hint,omitempty"`
				OccurredOnDate string `json:"occurred_on_date,omitempty"`
			}
			type incidentDraft struct {
				UserID            string    `json:"user_id"`
				Category          string    `json:"category"`
				Severity          string    `json:"severity"`
				Channel           string    `json:"channel,omitempty"`
				OccurredOn        string    `json:"occurred_on,omitempty"`
				Summary           string    `json:"summary"`
				SuggestedAction   string    `json:"suggested_action"`
				OmbudsmanEligible bool      `json:"ombudsman_eligible"`
				DraftedAt         time.Time `json:"drafted_at"`
			}
			type result struct {
				Category         string        `json:"category"`
				Severity         string        `json:"severity"`
				Confidence       float64       `json:"confidence"`
				Keywords         []string      `json:"keywords_matched"`
				NeedsHumanReview bool          `json:"needs_human_review"`
				Incident         incidentDraft `json:"incident_draft"`
				Disclaimer       string        `json:"disclaimer"`
			}

			// keyword -> category mapping; first match wins, later matches
			// overwrite the category but all matches accumulate for confidence.
			keywordMap := []struct{ needle, cat string }{
				{"upi", catDigitalBanking},
				{"imps", catDigitalBanking},
				{"neft", catDigitalBanking},
				{"rtgs", catDigitalBanking},
				{"net banking", catDigitalBanking},
				{"mobile banking", catDigitalBanking},
				{"phonepe", catDigitalBanking},
				{"gpay", catDigitalBanking},
				{"paytm", catDigitalBanking},
				{"credit card", catCards},
				{"debit card", catCards},
				{"atm", catCards},
				{"loan", catLoansAdvances},
				{"emi", catLoansAdvances},
				{"mortgage", catLoansAdvances},
				{"foreclosure", catLoansAdvances},
				{"mis-sold", catMisSelling},
				{"mis sold", catMisSelling},
				{"hidden charges", catChargesFees},
				{"unauthorised charge", catChargesFees},
				{"unauthorized charge", catChargesFees},
				{"service charge", catChargesFees},
				{"penalty", catChargesFees},
				{"rude", catStaffConduct},
				{"insolent", catStaffConduct},
				{"branch staff", catStaffConduct},
				{"remittance", catRemittance},
				{"international transfer", catRemittance},
				{"savings account", catDepositAccounts},
				{"current account", catDepositAccounts},
				{"fixed deposit", catDepositAccounts},
				{"fd ", catDepositAccounts},
				{"deposit", catDepositAccounts},
			}

			// severity escalators — presence of these tokens bumps severity.
			severityEscalators := map[string]string{
				"fraud":             "high",
				"unauthorised":      "high",
				"unauthorized":      "high",
				"frozen":            "high",
				"missing money":     "high",
				"stolen":            "high",
				"wrongly deducted":  "high",
				"complaint ignored": "high",
				"discrimination":    "high",
				"harass":            "high",
				"deceived":          "high",
				"didn't receive":    "medium",
				"didnt receive":     "medium",
				"refund not issued": "medium",
				"penal interest":    "medium",
				"hidden charge":     "medium",
			}

			rankSeverity := func(s string) int {
				switch s {
				case "low":
					return 1
				case "medium":
					return 2
				case "high":
					return 3
				default:
					return 0
				}
			}

			summarise := func(text string) string {
				t := strings.TrimSpace(text)
				if len(t) <= 240 {
					return t
				}
				return t[:240] + "…"
			}

			suggestAction := func(cat, sev string) string {
				if sev == "high" {
					return "Escalate to grievance officer within 24h; freeze affected account if instructed; obtain customer consent before any AI-suggested remediation."
				}
				switch cat {
				case catDigitalBanking:
					return "Open NPCI dispute via UPI Help; if unresolved in 30 days, escalate to RBI CMS."
				case catLoansAdvances:
					return "Provide loan account statement; review penal interest computation; share grievance officer details."
				case catCards:
					return "Initiate chargeback per scheme rules (Visa/Mastercard/RuPay); hot-list card if fraud suspected."
				case catChargesFees:
					return "Audit the disputed charge against the latest tariff schedule; reverse if non-disclosed."
				case catStaffConduct:
					return "Escalate to branch head with internal note; offer apology + service recovery."
				case catMisSelling:
					return "Pull KYC + product-suitability checklist; consider goodwill remediation; document for board review."
				default:
					return "Acknowledge within 24h; route to L1 customer-care for human review."
				}
			}

			// ombudsmanEligible — non-other categories with high or medium
			// severity are eligible; "other" is never eligible.
			ombudsmanEligible := func(cat, sev string) bool {
				if cat == catOther {
					return false
				}
				if sev == "high" {
					return true
				}
				return sev == "medium"
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			lower := strings.ToLower(req.ComplaintText + " " + req.ProductHint)
			cat := catOther
			matched := []string{}
			for _, kv := range keywordMap {
				if strings.Contains(lower, kv.needle) {
					cat = kv.cat
					matched = append(matched, kv.needle)
				}
			}
			if len(matched) == 0 {
				cat = catOther
			}

			severity := "low"
			for tok, sev := range severityEscalators {
				if strings.Contains(lower, tok) && rankSeverity(sev) > rankSeverity(severity) {
					severity = sev
				}
			}

			confidence := 0.0
			if len(matched) > 0 {
				switch {
				case len(matched) >= 2:
					confidence = 0.9
				default:
					confidence = 0.7
				}
			}
			needsReview := cat == catOther || confidence < 0.7

			when := time.Now().UTC()
			occurred := req.OccurredOnDate
			if occurred == "" {
				occurred = when.Format("2006-01-02")
			}

			res := result{
				Category:         cat,
				Severity:         severity,
				Confidence:       confidence,
				Keywords:         matched,
				NeedsHumanReview: needsReview,
				Incident: incidentDraft{
					UserID:            req.UserID,
					Category:          cat,
					Severity:          severity,
					Channel:           req.ChannelHint,
					OccurredOn:        occurred,
					Summary:           summarise(req.ComplaintText),
					SuggestedAction:   suggestAction(cat, severity),
					OmbudsmanEligible: ombudsmanEligible(cat, severity),
					DraftedAt:         when,
				},
				Disclaimer: "Categories follow RBI Integrated Ombudsman Scheme 2021. " +
					"Eligibility checks here are heuristic; consult RBI CMS portal for binding determination.",
			}

			body, err := json.Marshal(res)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
