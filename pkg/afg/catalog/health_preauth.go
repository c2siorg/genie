package catalog

import (
	"encoding/json"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// HealthPreauthSpec ports agents/health_preauth — the cashless-claim
// pre-authorisation engine for the Indian IRDAI health product. Given the
// network hospital's inbound pre-auth packet and the insurer-supplied policy
// rulebook (data, not code), it decides — deterministically — how much of an
// anticipated bill is approvable before admission: PPN-network gating,
// permanent-exclusion match, pre-existing-disease (PED) and specific-condition
// waiting-period denial, room-rent and ICU-rent proportionate deductions (the
// famous Indian-health-claim gotcha), procedure-package cap, co-pay, and the
// sum-insured cap. Bills >= ₹5L are routed to a medical officer (HITL).
// Deterministic port of the legacy pure rule engine; faithfully mirrors the
// legacy input/output JSON.
func HealthPreauthSpec() afg.Spec {
	return afg.Spec{
		ID:   "health_preauth",
		Risk: "high",
		Handle: func(input string) (string, error) {
			// Plan is the insurer-supplied health-policy rulebook.
			type Plan struct {
				ProductCode            string             `json:"product_code"`
				SumInsuredRupees       float64            `json:"sum_insured_rupees"`
				RoomRentSubLimitRupees float64            `json:"room_rent_sublimit_rupees"`
				ICURentSubLimitRupees  float64            `json:"icu_rent_sublimit_rupees"`
				CoPayPct               float64            `json:"copay_pct"`               // 0..1
				PEDWaitingMonths       int                `json:"ped_waiting_months"`      // pre-existing disease
				SpecificWaitingMonths  int                `json:"specific_waiting_months"` // hernia, cataract etc.
				ExcludedProcedures     []string           `json:"excluded_procedures"`
				ProcedurePackageRupees map[string]float64 `json:"procedure_package_rupees"` // capped payouts
			}

			// Request is the inbound pre-auth packet from the network hospital.
			type Request struct {
				PreauthID            string  `json:"preauth_id"`
				PolicyCode           string  `json:"policy_code"`
				Patient              string  `json:"patient"`
				HospitalCode         string  `json:"hospital_code"`
				NetworkPPN           bool    `json:"network_ppn"`
				Procedure            string  `json:"procedure"`
				IsPED                bool    `json:"is_pre_existing"`
				IsSpecificWaiting    bool    `json:"is_specific_waiting"`
				PolicyMonthsAtAdmit  int     `json:"policy_months_at_admit"`
				EstimatedBillRupees  float64 `json:"estimated_bill_rupees"`
				RoomRentPerDayRupees float64 `json:"room_rent_per_day_rupees"`
				ICURentPerDayRupees  float64 `json:"icu_rent_per_day_rupees"`
				LengthOfStayDays     int     `json:"length_of_stay_days"`
				ICUDays              int     `json:"icu_days"`
			}

			// Decision is the structured output.
			type Decision struct {
				PreauthID        string   `json:"preauth_id"`
				Action           string   `json:"action"` // "approve_full" | "approve_partial" | "approve_with_deduction" | "deny" | "hitl"
				ApprovedRupees   float64  `json:"approved_rupees"`
				DeductionsRupees float64  `json:"deductions_rupees"`
				DeductionReasons []string `json:"deduction_reasons"`
				Reasons          []string `json:"reasons"`
				Disclaimer       string   `json:"disclaimer"`
			}

			stdDisclaimer := func() string {
				return "Indicative pre-authorisation decision. IRDAI mandates a final cashless decision " +
					"within 1 hour of complete documentation; figures shown are estimates and may revise " +
					"on final bill review."
			}

			round2 := func(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }

			marshal := func(d Decision) (string, error) {
				body, err := json.Marshal(d)
				if err != nil {
					return "", err
				}
				return string(body), nil
			}

			deny := func(id, reason string) Decision {
				return Decision{
					PreauthID:  id,
					Action:     "deny",
					Reasons:    []string{reason},
					Disclaimer: stdDisclaimer(),
				}
			}

			var bundle struct {
				Request Request `json:"request"`
				Plan    Plan    `json:"plan"`
			}
			if err := json.Unmarshal([]byte(input), &bundle); err != nil {
				return "", err
			}
			r, p := bundle.Request, bundle.Plan

			// 1. Network gate.
			if !r.NetworkPPN {
				return marshal(deny(r.PreauthID, "Hospital not in insurer's PPN network — claim must be processed as reimbursement"))
			}

			// 2. Exclusions.
			procLower := strings.ToLower(r.Procedure)
			for _, ex := range p.ExcludedProcedures {
				if procLower == strings.ToLower(ex) {
					return marshal(deny(r.PreauthID, "Procedure listed under permanent exclusions"))
				}
			}

			// 3. PED waiting.
			if r.IsPED && r.PolicyMonthsAtAdmit < p.PEDWaitingMonths {
				return marshal(deny(r.PreauthID, "Pre-existing-disease waiting period not yet served"))
			}

			// 4. Specific waiting.
			if r.IsSpecificWaiting && r.PolicyMonthsAtAdmit < p.SpecificWaitingMonths {
				return marshal(deny(r.PreauthID, "Specific-condition waiting period not yet served"))
			}

			// 5. Room-rent proportionate deduction (the famous Indian-health-claim gotcha).
			deductions := 0.0
			reasons := []string{}
			if p.RoomRentSubLimitRupees > 0 && r.RoomRentPerDayRupees > p.RoomRentSubLimitRupees {
				// proportionate deduction = (chosen - allowed) / chosen × associated charges
				// associated charges approximated as 60 % of the bill — empirical industry rule.
				excessRoom := (r.RoomRentPerDayRupees - p.RoomRentSubLimitRupees) * float64(r.LengthOfStayDays-r.ICUDays)
				proportional := r.EstimatedBillRupees * 0.60 *
					((r.RoomRentPerDayRupees - p.RoomRentSubLimitRupees) / r.RoomRentPerDayRupees)
				deductions += excessRoom + proportional
				reasons = append(reasons, "Room-rent above sub-limit triggered proportionate deduction on associated charges")
			}
			if p.ICURentSubLimitRupees > 0 && r.ICURentPerDayRupees > p.ICURentSubLimitRupees && r.ICUDays > 0 {
				excessICU := (r.ICURentPerDayRupees - p.ICURentSubLimitRupees) * float64(r.ICUDays)
				deductions += excessICU
				reasons = append(reasons, "ICU rent above sub-limit deducted")
			}

			// 6. Procedure package cap.
			if capRupees, ok := p.ProcedurePackageRupees[procLower]; ok && capRupees > 0 {
				gross := r.EstimatedBillRupees - deductions
				if gross > capRupees {
					deductions += gross - capRupees
					reasons = append(reasons, "Procedure package cap applied")
				}
			}

			// 7. Co-pay.
			gross := r.EstimatedBillRupees - deductions
			if p.CoPayPct > 0 {
				coPay := gross * p.CoPayPct
				deductions += coPay
				reasons = append(reasons, "Co-pay applied")
			}

			// 8. Sum-insured cap.
			approved := r.EstimatedBillRupees - deductions
			if approved < 0 {
				approved = 0
			}
			if p.SumInsuredRupees > 0 && approved > p.SumInsuredRupees {
				over := approved - p.SumInsuredRupees
				approved = p.SumInsuredRupees
				deductions += over
				reasons = append(reasons, "Capped by sum insured")
			}

			action := "approve_full"
			switch {
			case approved == 0:
				action = "deny"
			case approved < r.EstimatedBillRupees && len(reasons) > 0:
				action = "approve_with_deduction"
			case approved < r.EstimatedBillRupees:
				action = "approve_partial"
			}
			// Large claims always go to HITL (medical officer review).
			if r.EstimatedBillRupees >= 500_000 {
				action = "hitl"
				reasons = append(reasons, "Bill ≥ ₹5L — routed to medical officer for review")
			}

			return marshal(Decision{
				PreauthID:        r.PreauthID,
				Action:           action,
				ApprovedRupees:   round2(approved),
				DeductionsRupees: round2(deductions),
				DeductionReasons: reasons,
				Reasons:          []string{"Standard cashless pre-authorisation flow"},
				Disclaimer:       stdDisclaimer(),
			})
		},
	}
}
