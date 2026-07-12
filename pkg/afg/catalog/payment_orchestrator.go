package catalog

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// PaymentOrchestratorSpec ports agents/payment_orchestrator. It routes an
// outbound payment across the NPCI rails — UPI / IMPS / NEFT / RTGS — based on
// amount, urgency, beneficiary status (VPA vs IFSC+account) and time-of-day,
// with a human-in-the-loop (HITL) hold at configurable thresholds. This is the
// bridge to actual money movement: it does not itself submit to a rail, it
// emits a structured Instruction (action = submit | hold_hitl | reject) that a
// host PSP adapter picks up. The routing is pure threshold/keyword/time-window
// arithmetic and so is ported as a deterministic Spec, faithfully mirroring the
// legacy Plan/chooseRail logic and the Request/Instruction wire types. It is
// High risk because it initiates payment instructions: per RBI FREE-AI Rec 8
// (Graded Liability) every payment at or above ₹50k is held for human approval,
// every Instruction carries an AI disclosure (Rec 18), and every policy-deny or
// rail-rejection auto-records an Annexure VI incident payload (Rec 22).
func PaymentOrchestratorSpec() afg.Spec {
	return afg.Spec{
		ID:   "payment_orchestrator",
		Risk: "high",
		Handle: func(input string) (string, error) {
			// NPCI per-rail limits (post-2023 revisions) and the HITL gate.
			const (
				upiPerTxnLimit      = 1_00_000.0 // ₹1 lakh default.
				impsLimit           = 5_00_000.0
				rtgsMinThreshold    = 2_00_000.0 // ₹2L floor for RTGS.
				hitlThresholdRupees = 50_000.0
			)

			type request struct {
				IdempotencyKey       string  `json:"idempotency_key"`
				PayerID              string  `json:"payer_id"`
				PayerAccount         string  `json:"payer_account"`
				BeneficiaryName      string  `json:"beneficiary_name"`
				BeneficiaryVPA       string  `json:"beneficiary_vpa,omitempty"`
				BeneficiaryIFSC      string  `json:"beneficiary_ifsc,omitempty"`
				BeneficiaryAcct      string  `json:"beneficiary_account,omitempty"`
				AmountRupees         float64 `json:"amount_rupees"`
				Currency             string  `json:"currency"`
				Purpose              string  `json:"purpose"`
				Urgency              string  `json:"urgency"`
				IsTrustedBeneficiary bool    `json:"is_trusted_beneficiary"`
			}
			type instruction struct {
				IdempotencyKey  string   `json:"idempotency_key"`
				Action          string   `json:"action"` // "submit" | "hold_hitl" | "reject"
				Rail            string   `json:"rail"`   // "upi" | "imps" | "neft" | "rtgs" | ""
				AmountRupees    float64  `json:"amount_rupees"`
				Reasons         []string `json:"reasons"`
				IncidentPayload string   `json:"incident_payload,omitempty"`
				Disclaimer      string   `json:"disclaimer"`
			}

			stdDisclaimer := func() string {
				return "AI-generated payment instruction. Subject to PSP confirmation, account-balance check, " +
					"and NPCI / RBI rail availability."
			}

			hold := func(req request, reason string) instruction {
				return instruction{
					IdempotencyKey: req.IdempotencyKey,
					Action:         "hold_hitl",
					AmountRupees:   req.AmountRupees,
					Reasons:        []string{reason},
					Disclaimer:     stdDisclaimer(),
				}
			}

			reject := func(req request, reason string) instruction {
				payload, _ := json.Marshal(map[string]any{
					"annexure":     "VI",
					"severity":     "medium",
					"action_taken": "Payment auto-rejected by orchestrator",
					"reason":       reason,
				})
				return instruction{
					IdempotencyKey:  req.IdempotencyKey,
					Action:          "reject",
					AmountRupees:    req.AmountRupees,
					Reasons:         []string{reason},
					IncidentPayload: string(payload),
					Disclaimer:      stdDisclaimer(),
				}
			}

			// chooseRail picks the cheapest/fastest rail that satisfies constraints.
			chooseRail := func(req request) (string, []string) {
				now := time.Now()
				hour := now.Hour()

				// UPI: instant, free, ≤₹1L, requires VPA.
				if req.BeneficiaryVPA != "" && req.AmountRupees <= upiPerTxnLimit {
					return "upi", []string{"Within UPI per-txn limit"}
				}
				// RTGS: instant, ≥₹2L, 7-18 Mon-Sat. Try before IMPS when the window is open.
				if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" &&
					req.AmountRupees >= rtgsMinThreshold && hour >= 7 && hour < 18 && now.Weekday() != time.Sunday {
					return "rtgs", []string{"Amount ≥ ₹2L and within RTGS operating window"}
				}
				// IMPS: instant, 24×7, ≤₹5L, needs IFSC+Account.
				if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" &&
					req.AmountRupees <= impsLimit && strings.ToLower(req.Urgency) != "any" {
					return "imps", []string{"Within IMPS limit and instant credit needed"}
				}
				// NEFT: 24×7 since 2019, batch-settled, no real ceiling.
				if req.BeneficiaryIFSC != "" && req.BeneficiaryAcct != "" {
					return "neft", []string{"Falling back to NEFT batch settlement"}
				}
				return "", nil
			}

			// plan routes the request to a rail, with HITL holds where needed.
			plan := func(req request) instruction {
				if req.Currency != "" && strings.ToUpper(req.Currency) != "INR" {
					return reject(req, "Only INR rails are supported")
				}
				if req.AmountRupees <= 0 {
					return reject(req, "Amount must be positive")
				}
				if req.IdempotencyKey == "" {
					return reject(req, "Missing idempotency key — refusing to risk a duplicate transfer")
				}

				// Untrusted beneficiary + large amount = HITL regardless of rail.
				if !req.IsTrustedBeneficiary && req.AmountRupees >= hitlThresholdRupees {
					return hold(req, "Beneficiary not in trusted list and amount ≥ ₹50k — HITL required")
				}

				rail, reasons := chooseRail(req)
				if rail == "" {
					return reject(req, "No rail satisfies the amount + urgency + IFSC/VPA constraints")
				}

				// All payments at or above the HITL threshold go to human approval.
				if req.AmountRupees >= hitlThresholdRupees {
					return instruction{
						IdempotencyKey: req.IdempotencyKey,
						Action:         "hold_hitl",
						Rail:           rail,
						AmountRupees:   req.AmountRupees,
						Reasons:        append([]string{"Routed via " + rail}, reasons...),
						Disclaimer:     stdDisclaimer(),
					}
				}

				return instruction{
					IdempotencyKey: req.IdempotencyKey,
					Action:         "submit",
					Rail:           rail,
					AmountRupees:   req.AmountRupees,
					Reasons:        append([]string{"Routed via " + rail}, reasons...),
					Disclaimer:     stdDisclaimer(),
				}
			}

			var req request
			if err := json.Unmarshal([]byte(input), &req); err != nil {
				return "", err
			}
			body, err := json.Marshal(plan(req))
			if err != nil {
				return "", err
			}
			return string(body), nil
		},
	}
}
