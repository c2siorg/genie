package catalog

import (
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// AutoInsuranceSpec ports agents/auto_insurance — the motor-insurance
// (bancassurance) touchpoint. It routes FNOL (first notice of loss),
// roadside-assistance dispatch, and renewal-quote requests, applying the
// IRDAI India motor product mechanics: total-loss at repair >= 75% of IDV,
// the standard no-claim-bonus (NCB) ladder, and an indicative OD+TP premium
// with an optional zero-dep add-on. Deterministic port of the legacy logic.
func AutoInsuranceSpec() afg.Spec {
	return afg.Spec{
		ID:   "auto_insurance",
		Risk: "medium",
		Handle: func(input string) (string, error) {
			const totalLossThresholdPct = 0.75 // repair cost > 75% of IDV -> total loss

			// Request is one motor-insurance ask. Kind drives the branch.
			type Request struct {
				Kind                string  `json:"kind"` // "fnol" | "roadside" | "renewal_quote"
				PolicyNumber        string  `json:"policy_number"`
				VehicleRegNumber    string  `json:"vehicle_reg"`
				IDVRupees           float64 `json:"idv_rupees"` // current insured declared value
				EstRepairCostRupees float64 `json:"est_repair_cost_rupees"`
				IncidentType        string  `json:"incident_type"` // accident | theft | flood | fire | third-party
				LocationLat         float64 `json:"lat"`
				LocationLng         float64 `json:"lng"`
				HoursToExpiry       int     `json:"hours_to_expiry"` // for renewal quote
				NCBPct              float64 `json:"ncb_pct"`         // 0..50 in steps
				ClaimedThisYear     bool    `json:"claimed_this_year"`
				ZeroDepAddOn        bool    `json:"zero_dep_addon"` // affects renewal premium
			}

			// Response is the shaped output.
			type Response struct {
				Kind           string   `json:"kind"`
				Action         string   `json:"action"`
				NetworkGarages []string `json:"network_garages,omitempty"`
				TotalLoss      bool     `json:"total_loss_flag,omitempty"`
				SettlementHint float64  `json:"settlement_hint_rupees,omitempty"`
				NewNCBPct      float64  `json:"new_ncb_pct,omitempty"`
				RenewalPremium float64  `json:"renewal_premium_rupees,omitempty"`
				NextSteps      []string `json:"next_steps"`
				Disclaimer     string   `json:"disclaimer"`
			}

			stdDisclaimer := func() string {
				return "Indicative service action per IRDAI motor product. Final settlement and roadside " +
					"dispatch subject to policy terms, insurer confirmation, and partner availability."
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			// bumpNCB walks the standard Indian NCB ladder: 0 -> 20 -> 25 -> 35 -> 45 -> 50 -> 50.
			bumpNCB := func(current float64) float64 {
				ladder := []float64{0, 20, 25, 35, 45, 50}
				for _, step := range ladder {
					if current < step {
						return step
					}
				}
				return 50
			}

			// cityFromLatLng is a placeholder. Production wires a reverse-geocoder.
			// Returns "" when no mapping is known so callers can fall back to
			// "nearest serviceable" routing.
			cityFromLatLng := func(lat, lng float64) string {
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

			// Garages is the static cashless-network list keyed by city. In
			// production this is a live API to the insurer; empty here keeps the
			// deterministic port hermetic (network_garages is then omitted).
			garages := map[string][]string{}

			var req Request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}

			var r Response
			switch strings.ToLower(req.Kind) {
			case "fnol":
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
					steps = append(steps, "Estimated repair >= 75 % of IDV — total-loss process initiated; settlement at IDV.")
				}
				city := cityFromLatLng(req.LocationLat, req.LocationLng)
				r = Response{
					Kind:           "fnol",
					Action:         action,
					NetworkGarages: garages[city],
					TotalLoss:      totalLoss,
					SettlementHint: round2(hint),
					NextSteps:      steps,
					Disclaimer:     stdDisclaimer(),
				}
			case "roadside":
				city := cityFromLatLng(req.LocationLat, req.LocationLng)
				r = Response{
					Kind:           "roadside",
					Action:         "dispatch_partner",
					NetworkGarages: garages[city],
					NextSteps: []string{
						"Roadside partner dispatched; ETA notified by SMS.",
						"Towing up to 50 km to nearest network garage is included.",
					},
					Disclaimer: stdDisclaimer(),
				}
			case "renewal_quote":
				// NCB ratchet: clean year bumps NCB to next tier; any claim resets to 0.
				var newNCB float64
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
				r = Response{
					Kind:           "renewal_quote",
					Action:         "quote_ready",
					NewNCBPct:      round2(newNCB),
					RenewalPremium: round2(premium),
					NextSteps:      steps,
					Disclaimer:     stdDisclaimer(),
				}
			default:
				r = Response{
					Kind:       req.Kind,
					Action:     "unknown",
					NextSteps:  []string{"Unrecognised motor request type"},
					Disclaimer: stdDisclaimer(),
				}
			}

			body, err := json.Marshal(r)
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
