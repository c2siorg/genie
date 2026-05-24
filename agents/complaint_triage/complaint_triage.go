// Package complaint_triage classifies a free-text customer grievance into
// one of the RBI Banking Ombudsman scheme categories, gauges severity,
// and drafts the structured incident record that pkg/incidents persists
// (per Annexure VI of the FREE-AI report).
//
// The classifier is deterministic keyword-based with an optional LLM
// fallback for ambiguous text. Two reasons for the keyword default:
//
//  1. Audit. A regulator can verify the routing logic from the source
//     code alone (Rec 25 explainability).
//  2. Cost / latency. Most complaints are routine and don't need a model.
//
// When the keyword pass produces no high-confidence match the agent
// returns a "needs_human_review" outcome; the supervisor can then route
// to LLM-as-a-judge (agents/auditor) or to a human queue.
package complaint_triage

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "complaint_triage"
	Capability = "triage_complaint"
	TypeIn     = "complaint"
	TypeOut    = "complaint_triage"
	NextAgent  = "financial_supervisor"
)

// Category mirrors the RBI Integrated Ombudsman Scheme 2021 grounds.
// Kept as string constants so wire payloads stay stable across releases.
const (
	CatDepositAccounts = "deposit_accounts"
	CatLoansAdvances   = "loans_and_advances"
	CatDigitalBanking  = "digital_banking_upi_imps"
	CatCards           = "cards"
	CatRemittance      = "remittance"
	CatChargesFees     = "charges_and_fees"
	CatStaffConduct    = "staff_conduct"
	CatMisSelling      = "mis_selling"
	CatOther           = "other"
)

var (
	// keyword → category mapping. The first match wins. Lowercase patterns;
	// the input is lower-cased before matching.
	keywordMap = []struct {
		needle, cat string
	}{
		{"upi", CatDigitalBanking},
		{"imps", CatDigitalBanking},
		{"neft", CatDigitalBanking},
		{"rtgs", CatDigitalBanking},
		{"net banking", CatDigitalBanking},
		{"mobile banking", CatDigitalBanking},
		{"phonepe", CatDigitalBanking},
		{"gpay", CatDigitalBanking},
		{"paytm", CatDigitalBanking},
		{"credit card", CatCards},
		{"debit card", CatCards},
		{"atm", CatCards},
		{"loan", CatLoansAdvances},
		{"emi", CatLoansAdvances},
		{"mortgage", CatLoansAdvances},
		{"foreclosure", CatLoansAdvances},
		{"mis-sold", CatMisSelling},
		{"mis sold", CatMisSelling},
		{"hidden charges", CatChargesFees},
		{"unauthorised charge", CatChargesFees},
		{"unauthorized charge", CatChargesFees},
		{"service charge", CatChargesFees},
		{"penalty", CatChargesFees},
		{"rude", CatStaffConduct},
		{"insolent", CatStaffConduct},
		{"branch staff", CatStaffConduct},
		{"remittance", CatRemittance},
		{"international transfer", CatRemittance},
		{"savings account", CatDepositAccounts},
		{"current account", CatDepositAccounts},
		{"fixed deposit", CatDepositAccounts},
		{"fd ", CatDepositAccounts},
		{"deposit", CatDepositAccounts},
	}

	// severity escalators — presence of these tokens bumps severity.
	severityEscalators = map[string]string{
		"fraud":              "high",
		"unauthorised":       "high",
		"unauthorized":       "high",
		"frozen":             "high",
		"missing money":      "high",
		"stolen":             "high",
		"wrongly deducted":   "high",
		"complaint ignored":  "high",
		"discrimination":     "high",
		"harass":             "high",
		"deceived":           "high",
		"didn't receive":     "medium",
		"didnt receive":      "medium",
		"refund not issued":  "medium",
		"penal interest":     "medium",
		"hidden charge":      "medium",
	}
)

// Request is the wire payload.
type Request struct {
	UserID         string `json:"user_id"`
	ComplaintText  string `json:"complaint_text"`
	ProductHint    string `json:"product_hint,omitempty"`    // optional: "credit card", "loan", etc.
	ChannelHint    string `json:"channel_hint,omitempty"`    // "branch" | "app" | "web" | "ivr"
	OccurredOnDate string `json:"occurred_on_date,omitempty"`// YYYY-MM-DD
}

// IncidentDraft mirrors the shape pkg/incidents will persist if the
// supervisor approves the routing. Annexure VI of the FREE-AI report.
type IncidentDraft struct {
	UserID           string    `json:"user_id"`
	Category         string    `json:"category"`
	Severity         string    `json:"severity"`
	Channel          string    `json:"channel,omitempty"`
	OccurredOn       string    `json:"occurred_on,omitempty"`
	Summary          string    `json:"summary"`
	SuggestedAction  string    `json:"suggested_action"`
	OmbudsmanEligible bool     `json:"ombudsman_eligible"`
	DraftedAt        time.Time `json:"drafted_at"`
}

// Result is the wire output. NeedsHumanReview is true when the keyword
// pass found no confident category match.
type Result struct {
	Category         string        `json:"category"`
	Severity         string        `json:"severity"`
	Confidence       float64       `json:"confidence"`
	Keywords         []string      `json:"keywords_matched"`
	NeedsHumanReview bool          `json:"needs_human_review"`
	Incident         IncidentDraft `json:"incident_draft"`
	Disclaimer       string        `json:"disclaimer"`
}

// Agent implements agent.Agent.
type Agent struct {
	Now func() time.Time // injectable for tests
}

// New returns a triage agent with time.Now wired in.
func New() *Agent { return &Agent{Now: time.Now} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Complaint Triage" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

// HandleMessage runs Classify and emits the structured triage.
func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	res := a.Classify(req)
	env.Logf("[complaint_triage] cat=%s sev=%s review=%v", res.Category, res.Severity, res.NeedsHumanReview)
	body, _ := json.Marshal(res)
	return []agent.Message{
		agent.NewMessage(ID, msg.From, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Classify runs the keyword pass and assembles the IncidentDraft.
func (a *Agent) Classify(req Request) Result {
	lower := strings.ToLower(req.ComplaintText + " " + req.ProductHint)
	cat := CatOther
	matched := []string{}
	for _, kv := range keywordMap {
		if strings.Contains(lower, kv.needle) {
			cat = kv.cat
			matched = append(matched, kv.needle)
		}
	}
	if len(matched) == 0 {
		cat = CatOther
	}
	severity := "low"
	for tok, sev := range severityEscalators {
		if strings.Contains(lower, tok) && rankSeverity(sev) > rankSeverity(severity) {
			severity = sev
		}
	}
	confidence := 0.0
	if len(matched) > 0 {
		// Crude: matches saturate quickly; 1 match → 0.7, ≥2 → 0.9.
		switch {
		case len(matched) >= 2:
			confidence = 0.9
		default:
			confidence = 0.7
		}
	}
	needsReview := cat == CatOther || confidence < 0.7

	when := a.Now().UTC()
	occurred := req.OccurredOnDate
	if occurred == "" {
		occurred = when.Format("2006-01-02")
	}

	res := Result{
		Category:         cat,
		Severity:         severity,
		Confidence:       confidence,
		Keywords:         matched,
		NeedsHumanReview: needsReview,
		Incident: IncidentDraft{
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
	return res
}

func suggestAction(cat, sev string) string {
	if sev == "high" {
		return "Escalate to grievance officer within 24h; freeze affected account if instructed; obtain customer consent before any AI-suggested remediation."
	}
	switch cat {
	case CatDigitalBanking:
		return "Open NPCI dispute via UPI Help; if unresolved in 30 days, escalate to RBI CMS."
	case CatLoansAdvances:
		return "Provide loan account statement; review penal interest computation; share grievance officer details."
	case CatCards:
		return "Initiate chargeback per scheme rules (Visa/Mastercard/RuPay); hot-list card if fraud suspected."
	case CatChargesFees:
		return "Audit the disputed charge against the latest tariff schedule; reverse if non-disclosed."
	case CatStaffConduct:
		return "Escalate to branch head with internal note; offer apology + service recovery."
	case CatMisSelling:
		return "Pull KYC + product-suitability checklist; consider goodwill remediation; document for board review."
	default:
		return "Acknowledge within 24h; route to L1 customer-care for human review."
	}
}

// ombudsmanEligible — high-severity + non-other categories always eligible.
// Per the scheme, the customer must first complain to the bank and wait 30
// days; we assume the triage record itself becomes the bank's "received"
// event.
func ombudsmanEligible(cat, sev string) bool {
	if cat == CatOther {
		return false
	}
	if sev == "high" {
		return true
	}
	// Medium severity + categorised → eligible after 30 days lapses.
	return sev == "medium"
}

func rankSeverity(s string) int {
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

func summarise(text string) string {
	t := strings.TrimSpace(text)
	if len(t) <= 240 {
		return t
	}
	return t[:240] + "…"
}
