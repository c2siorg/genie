package catalog

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
)

// CobSpec is the coordination_of_benefits agent: a deterministic
// Coordination-of-Benefits (COB) engine for a patient insured under two
// overlapping health plans. It decides which plan pays first, applies the
// secondary plan's non-duplication rule, honours per-plan deductible/OOP-max
// accumulation across claims, and (for family mode) searches the primary
// ordering that minimises total out-of-pocket. All money is integer paise and
// computed here — never by a model. Standard US-style COB applied in INR.
//
// This is a genie-native reimplementation of the standard COB domain rules
// (deductible → coinsurance → OOP cap; secondary pays
// max(0, min(secondary_as_primary − primary, patient_residual))). Advisory /
// informational per RBI FREE-AI — not a benefits determination; the insurer's
// adjudication governs.
func CobSpec() afg.Spec {
	return afg.Spec{
		ID:     "coordination_of_benefits",
		Risk:   "medium",
		Handle: cobHandle,
	}
}

type cobPlan struct {
	DeductiblePaise int64   `json:"deductible_paise"`
	CoinsuranceRate float64 `json:"coinsurance_rate"` // patient's share of the post-deductible amount
	OOPMaxPaise     int64   `json:"oop_max_paise"`    // 0 = no cap
}

type cobClaim struct {
	ClaimID     string `json:"claim_id"`
	Patient     string `json:"patient"`
	BilledPaise int64  `json:"billed_paise"`
	OwnPlanID   string `json:"own_plan_id"`   // plan where the patient is the member (primary, standard order)
	OtherPlanID string `json:"other_plan_id"` // plan where the patient is a dependent (secondary, standard order)
}

type cobRequest struct {
	Mode   string             `json:"mode"` // "individual" (default) | "family"
	Plans  map[string]cobPlan `json:"plans"`
	Claims []cobClaim         `json:"claims"`
}

type cobPayment struct {
	PlanID                 string `json:"plan_id"`
	Role                   string `json:"role"` // primary|secondary
	PaysPaise              int64  `json:"pays_paise"`
	DeductibleAppliedPaise int64  `json:"deductible_applied_paise"`
	CoinsurancePaise       int64  `json:"coinsurance_paise"`
}

type cobClaimResult struct {
	ClaimID         string       `json:"claim_id"`
	Patient         string       `json:"patient"`
	BilledPaise     int64        `json:"billed_paise"`
	PrimaryPlanID   string       `json:"primary_plan_id"`
	SecondaryPlanID string       `json:"secondary_plan_id"`
	Payments        []cobPayment `json:"payments"`
	PatientOOPPaise int64        `json:"patient_oop_paise"`
}

type cobResult struct {
	Mode                   string           `json:"mode"`
	Claims                 []cobClaimResult `json:"claims"`
	TotalPatientOOPPaise   int64            `json:"total_patient_oop_paise"`
	TotalPatientOOPRupees  string           `json:"total_patient_oop_rupees"`
	StandardOOPPaise       int64            `json:"standard_oop_paise"`
	SavingsVsStandardPaise int64            `json:"savings_vs_standard_paise"`
	Recommendation         string           `json:"recommendation"`
	Disclaimer             string           `json:"disclaimer"`
}

type cobAcc struct {
	deductibleMet int64
	oopSpent      int64
}

func cobScopeKey(planID, patient, mode string) string {
	if mode == "family" {
		return planID + "\x00*family*"
	}
	return planID + "\x00" + patient
}

// cobAdjudicate adjudicates `billed` against plan `p` as if it were primary,
// given prior accumulated spend `acc`. Mirrors DuCO's engine.adjudicate.
func cobAdjudicate(billed int64, p cobPlan, acc cobAcc) (planPays, patientShare, dedApplied, coins int64) {
	remainingDeductible := p.DeductiblePaise - acc.deductibleMet
	if remainingDeductible < 0 {
		remainingDeductible = 0
	}
	dedApplied = billed
	if remainingDeductible < dedApplied {
		dedApplied = remainingDeductible
	}
	afterDeductible := billed - dedApplied
	coins = int64(math.RoundToEven(p.CoinsuranceRate * float64(afterDeductible))) // half-to-even, matches DuCO
	patientShare = dedApplied + coins
	if p.OOPMaxPaise > 0 {
		remainingOOP := p.OOPMaxPaise - acc.oopSpent
		if remainingOOP < 0 {
			remainingOOP = 0
		}
		if patientShare > remainingOOP {
			patientShare = remainingOOP
		}
	}
	planPays = billed - patientShare
	return planPays, patientShare, dedApplied, coins
}

// cobRunScenario adjudicates every claim with a given per-claim primary choice
// (ownPrimary[i] true = the patient's own plan is primary), threading per-scope
// accumulators, and returns the per-claim results and the total patient OOP.
func cobRunScenario(req cobRequest, ownPrimary []bool) ([]cobClaimResult, int64, error) {
	accs := map[string]cobAcc{}
	results := make([]cobClaimResult, 0, len(req.Claims))
	var total int64
	for i, c := range req.Claims {
		primaryID, secondaryID := c.OwnPlanID, c.OtherPlanID
		if !ownPrimary[i] {
			primaryID, secondaryID = c.OtherPlanID, c.OwnPlanID
		}
		primary, ok := req.Plans[primaryID]
		if !ok {
			return nil, 0, fmt.Errorf("cob: claim %d references unknown plan %q", i, primaryID)
		}
		secondary, ok := req.Plans[secondaryID]
		if !ok {
			return nil, 0, fmt.Errorf("cob: claim %d references unknown plan %q", i, secondaryID)
		}
		pKey := cobScopeKey(primaryID, c.Patient, req.Mode)
		sKey := cobScopeKey(secondaryID, c.Patient, req.Mode)

		pPays, pShare, pDed, pCoins := cobAdjudicate(c.BilledPaise, primary, accs[pKey])
		sPays0, _, sDed, sCoins := cobAdjudicate(c.BilledPaise, secondary, accs[sKey])

		// Secondary non-duplication: it pays only the gap between its own
		// as-primary payout and the primary's, capped at the patient's residual.
		secondaryPays := sPays0 - pPays
		if secondaryPays < 0 {
			secondaryPays = 0
		}
		if secondaryPays > pShare {
			secondaryPays = pShare
		}
		patientOOP := c.BilledPaise - pPays - secondaryPays

		// Accumulate deductible rollover + OOP.
		pAcc := accs[pKey]
		pAcc.deductibleMet += pDed
		pAcc.oopSpent += pShare
		accs[pKey] = pAcc
		sAcc := accs[sKey]
		sAcc.deductibleMet += sDed
		sAcc.oopSpent += patientOOP
		accs[sKey] = sAcc

		results = append(results, cobClaimResult{
			ClaimID: c.ClaimID, Patient: c.Patient, BilledPaise: c.BilledPaise,
			PrimaryPlanID: primaryID, SecondaryPlanID: secondaryID,
			Payments: []cobPayment{
				{PlanID: primaryID, Role: "primary", PaysPaise: pPays, DeductibleAppliedPaise: pDed, CoinsurancePaise: pCoins},
				{PlanID: secondaryID, Role: "secondary", PaysPaise: secondaryPays, DeductibleAppliedPaise: sDed, CoinsurancePaise: sCoins},
			},
			PatientOOPPaise: patientOOP,
		})
		total += patientOOP
	}
	return results, total, nil
}

func cobHandle(input string) (string, error) {
	var req cobRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("cob: bad input: %w", err)
	}
	if len(req.Claims) == 0 {
		return "", fmt.Errorf("cob: no claims")
	}
	n := len(req.Claims)

	// Standard ordering: each patient's own plan is primary.
	std := make([]bool, n)
	for i := range std {
		std[i] = true
	}
	stdResults, stdOOP, err := cobRunScenario(req, std)
	if err != nil {
		return "", err
	}

	bestResults, bestOOP := stdResults, stdOOP
	// Search the min-OOP primary ordering. Bounded to keep it deterministic and
	// cheap; beyond the cap we keep the standard ordering (and say so).
	optimized := false
	if n <= 16 {
		for mask := 0; mask < (1 << n); mask++ {
			order := make([]bool, n)
			for i := 0; i < n; i++ {
				order[i] = mask&(1<<i) != 0
			}
			r, oop, rerr := cobRunScenario(req, order)
			if rerr != nil {
				return "", rerr
			}
			if oop < bestOOP {
				bestOOP, bestResults = oop, r
			}
		}
		optimized = true
	}

	savings := stdOOP - bestOOP
	var rec string
	switch {
	case !optimized:
		rec = fmt.Sprintf("Too many claims (%d) to search all primary orderings; reporting the standard order (each patient's own plan primary).", n)
	case savings > 0:
		rec = fmt.Sprintf("Reordering which plan is primary lowers total out-of-pocket from %s to %s (saves %s), mainly via shared-deductible accumulation.", cobRupees(stdOOP), cobRupees(bestOOP), cobRupees(savings))
	default:
		rec = fmt.Sprintf("Total out-of-pocket is %s regardless of primary ordering; the standard order (each patient's own plan primary) is recommended.", cobRupees(bestOOP))
	}

	out := cobResult{
		Mode:                   modeOrDefault(req.Mode),
		Claims:                 bestResults,
		TotalPatientOOPPaise:   bestOOP,
		TotalPatientOOPRupees:  cobRupees(bestOOP),
		StandardOOPPaise:       stdOOP,
		SavingsVsStandardPaise: savings,
		Recommendation:         rec,
		Disclaimer:             "AI-generated, informational only; not a benefits determination. The insurer's adjudication and your policy terms govern.",
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func modeOrDefault(m string) string {
	if m == "family" {
		return "family"
	}
	return "individual"
}

func cobRupees(paise int64) string {
	sign := ""
	if paise < 0 {
		sign, paise = "-", -paise
	}
	return fmt.Sprintf("%s₹%d.%02d", sign, paise/100, paise%100)
}
