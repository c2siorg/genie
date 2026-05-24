// Package auto_insurance handles motor-insurance touchpoints inside a
// banking app — bancassurance is a major fee-income line for Indian banks,
// and customers expect FNOL (first notice of loss) and roadside-assistance
// dispatch to live alongside their account.
//
// Inspired by Google ADK samples → auto-insurance-agent. Tuned for the
// IRDAI India motor product (OD + TP), no-claim-bonus mechanics, cashless
// network garage routing, and the salvage-vs-total-loss thresholds in the
// Indian motor schedule.
package auto_insurance

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const (
	ID         = "auto_insurance"
	Capability = "service_motor_policy"
	TypeIn     = "motor_request"
	TypeOut    = "motor_response"
	NextAgent  = "financial_supervisor"

	totalLossThresholdPct = 0.75 // repair cost > 75% of IDV → total loss
)

// Request is one motor-insurance ask. Kind drives the branch.
type Request struct {
	Kind                 string  `json:"kind"` // "fnol" | "roadside" | "renewal_quote"
	PolicyNumber         string  `json:"policy_number"`
	VehicleRegNumber     string  `json:"vehicle_reg"`
	IDVRupees            float64 `json:"idv_rupees"`         // current insured declared value
	EstRepairCostRupees  float64 `json:"est_repair_cost_rupees"`
	IncidentType         string  `json:"incident_type"`      // accident | theft | flood | fire | third-party
	LocationLat          float64 `json:"lat"`
	LocationLng          float64 `json:"lng"`
	HoursToExpiry        int     `json:"hours_to_expiry"`    // for renewal quote
	NCBPct               float64 `json:"ncb_pct"`            // 0..50 in steps
	ClaimedThisYear      bool    `json:"claimed_this_year"`
	ZeroDepAddOn         bool    `json:"zero_dep_addon"`     // affects renewal premium
}

// Response is the shaped output.
type Response struct {
	Kind          string   `json:"kind"`
	Action        string   `json:"action"`
	NetworkGarages []string `json:"network_garages,omitempty"`
	TotalLoss     bool     `json:"total_loss_flag,omitempty"`
	SettlementHint float64 `json:"settlement_hint_rupees,omitempty"`
	NewNCBPct     float64  `json:"new_ncb_pct,omitempty"`
	RenewalPremium float64 `json:"renewal_premium_rupees,omitempty"`
	NextSteps     []string `json:"next_steps"`
	Disclaimer    string   `json:"disclaimer"`
}

type Agent struct {
	// Garages is the static cashless-network list keyed by city. In production
	// this is a live API to the insurer; injecting it here keeps unit tests
	// hermetic.
	Garages map[string][]string
}

func New(garages map[string][]string) *Agent {
	if garages == nil {
		garages = map[string][]string{}
	}
	return &Agent{Garages: garages}
}

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "Motor Insurance (Bancassurance)" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskMedium }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var req Request
	if err := json.Unmarshal([]byte(msg.Content), &req); err != nil {
		return nil, err
	}
	r := a.Service(req)
	env.Logf("[auto_insurance] kind=%s action=%s policy=%s", r.Kind, r.Action, req.PolicyNumber)
	body, _ := json.Marshal(r)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Service routes to the right handler and returns a structured response.
func (a *Agent) Service(req Request) Response {
	switch strings.ToLower(req.Kind) {
	case "fnol":
		return a.handleFNOL(req)
	case "roadside":
		return a.handleRoadside(req)
	case "renewal_quote":
		return a.handleRenewalQuote(req)
	}
	return Response{
		Kind:       req.Kind,
		Action:     "unknown",
		NextSteps:  []string{"Unrecognised motor request type"},
		Disclaimer: stdDisclaimer(),
	}
}

func (a *Agent) handleFNOL(req Request) Response {
	totalLoss := req.IDVRupees > 0 && req.EstRepairCostRupees >= req.IDVRupees*totalLossThresholdPct
	action := "register_claim"
	hint := req.EstRepairCostRupees
	steps := []string{
		"Claim registered with insurer; reference number issued via SMS.",
		"Upload photos of damage and FIR (if applicable) within 48 hours.",
	}
	if totalLoss {
		action = "register_claim_total_loss"
		hint = req.IDVRupees
		steps = append(steps, "Estimated repair ≥ 75 % of IDV — total-loss process initiated; settlement at IDV.")
	}
	city := cityFromLatLng(req.LocationLat, req.LocationLng)
	return Response{
		Kind:           "fnol",
		Action:         action,
		NetworkGarages: a.Garages[city],
		TotalLoss:      totalLoss,
		SettlementHint: round2(hint),
		NextSteps:      steps,
		Disclaimer:     stdDisclaimer(),
	}
}

func (a *Agent) handleRoadside(req Request) Response {
	city := cityFromLatLng(req.LocationLat, req.LocationLng)
	return Response{
		Kind:   "roadside",
		Action: "dispatch_partner",
		NetworkGarages: a.Garages[city],
		NextSteps: []string{
			"Roadside partner dispatched; ETA notified by SMS.",
			"Towing up to 50 km to nearest network garage is included.",
		},
		Disclaimer: stdDisclaimer(),
	}
}

func (a *Agent) handleRenewalQuote(req Request) Response {
	// NCB ratchet: clean year bumps NCB to next tier; any claim resets to 0.
	newNCB := req.NCBPct
	if req.ClaimedThisYear {
		newNCB = 0
	} else {
		newNCB = bumpNCB(req.NCBPct)
	}

	// Indicative premium: base = 3 % of IDV, less NCB on OD portion,
	// plus 10 % for zero-dep add-on.
	base := req.IDVRupees * 0.03
	odPortion := base * 0.70
	tpPortion := base * 0.30
	odAfterNCB := odPortion * (1 - newNCB/100.0)
	premium := odAfterNCB + tpPortion
	if req.ZeroDepAddOn {
		premium *= 1.10
	}
	steps := []string{"Indicative quote computed. Confirm to proceed to checkout."}
	if req.HoursToExpiry > 0 && req.HoursToExpiry < 72 {
		steps = append(steps, "Policy expires within 72 hours — driving uninsured is a Motor Vehicles Act offence.")
	}
	return Response{
		Kind:           "renewal_quote",
		Action:         "quote_ready",
		NewNCBPct:      round2(newNCB),
		RenewalPremium: round2(premium),
		NextSteps:      steps,
		Disclaimer:     stdDisclaimer(),
	}
}

// bumpNCB walks the standard Indian NCB ladder: 0 → 20 → 25 → 35 → 45 → 50 → 50.
func bumpNCB(current float64) float64 {
	ladder := []float64{0, 20, 25, 35, 45, 50}
	for _, step := range ladder {
		if current < step {
			return step
		}
	}
	return 50
}

// cityFromLatLng is a placeholder. Production wires a reverse-geocoder.
// We return the empty string when no mapping is known so callers can fall
// back to "nearest serviceable" routing.
func cityFromLatLng(lat, lng float64) string {
	switch {
	case lat > 28.4 && lat < 28.8 && lng > 76.8 && lng < 77.5:
		return "delhi"
	case lat > 18.9 && lat < 19.2 && lng > 72.7 && lng < 73.1:
		return "mumbai"
	case lat > 12.8 && lat < 13.2 && lng > 77.4 && lng < 77.8:
		return "bengaluru"
	}
	return ""
}

func stdDisclaimer() string {
	return "Indicative service action per IRDAI motor product. Final settlement and roadside " +
		"dispatch subject to policy terms, insurer confirmation, and partner availability."
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
