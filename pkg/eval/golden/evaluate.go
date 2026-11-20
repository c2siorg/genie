package golden

import (
	"context"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/judges"
)

// Result is the outcome of evaluating a single golden Case.
type Result struct {
	CaseID          string
	Scored          bool    // false when no judge exists for the domain (excluded from gate)
	Pass            bool    // did the judge's verdict MATCH the case's expected verdict?
	JudgeVerdict    bool    // the verdict the judge actually produced
	ExpectedVerdict string  // "PASS" or "FAIL" the case asserts
	Score           float64 // judge confidence
	Reason          string
}

// EvaluateCase scores one case with the appropriate DETERMINISTIC judge (Track A,
// offline). It builds the judge's typed input from the case's Input map, runs the
// judge, and reports whether the judge's verdict MATCHES what the case asserts
// (ExpectedVerdict). A failure-mode case (ExpectedVerdict=="FAIL") therefore only
// passes the gate when the judge correctly DETECTS the failure — so the gate can
// genuinely fail, unlike the tautological predecessor it replaces.
//
// Domains without a judge yet (merchant, advisor, or an empty domain) return
// Scored=false; callers must EXCLUDE these from the pass-rate gate rather than
// counting them as passes — silence is more honest than a fake green.
func EvaluateCase(ctx context.Context, c Case) Result {
	want := c.ExpectedVerdict()
	res := Result{CaseID: c.ID, ExpectedVerdict: want}

	var verdict judges.Verdict
	switch c.Domain {
	case "settlement":
		verdict, _ = judges.NewMockSettlementJudge().Evaluate(ctx, buildSettlementInput(c))
	case "compliance":
		verdict, _ = judges.NewMockComplianceJudge().Evaluate(ctx, buildComplianceInput(c))
	case "orchestration":
		verdict, _ = judges.NewMockOrchestrationJudge().Evaluate(ctx, buildOrchestrationInput(c))
	case "lineage":
		verdict, _ = judges.NewMockLineageJudge().Evaluate(ctx, buildLineageInput(c))
	case "merchant":
		verdict, _ = judges.NewMockMerchantJudge().Evaluate(ctx, buildMerchantInput(c))
	default:
		// No deterministic judge for this domain yet (advisor + agent_behavior are
		// Track B — real-agent + LLM-judge — not deterministic Track A).
		res.Scored = false
		res.Reason = "no deterministic judge for domain " + strconvQuote(c.Domain)
		return res
	}

	res.Scored = true
	res.JudgeVerdict = verdict.Pass
	res.Score = verdict.Score
	res.Reason = verdict.Reason
	res.Pass = (verdict.Pass == (want == "PASS"))
	return res
}

// --- typed input builders (map[string]any -> *JudgeInput) ---

func buildSettlementInput(c Case) judges.SettlementJudgeInput {
	in := c.Input
	return judges.SettlementJudgeInput{
		OrderID:              getString(in, "order_id"),
		SettlementID:         getString(in, "settlement_id"),
		SettlementAmount:     getInt64(in, "settlement_amount"),
		ExpectedAmount:       getInt64(in, "expected_amount"),
		OrderAmount:          getInt64(in, "order_amount"),
		NettingApplied:       getBool(in, "netting_applied"),
		NettedAmount:         getInt64(in, "netted_amount"),
		CBDCCommitted:        getBool(in, "cbdc_committed"),
		ReconciliationPassed: getBool(in, "reconciliation_passed"),
		AuditTrail:           getString(in, "audit_trail"),
		Metadata:             c.Metadata,
	}
}

func buildComplianceInput(c Case) judges.ComplianceJudgeInput {
	in := c.Input
	return judges.ComplianceJudgeInput{
		CustomerID:             getString(in, "customer_id"),
		OrderID:                getString(in, "order_id"),
		KYCStatus:              getString(in, "kyc_status"),
		VelocityWindowSeconds:  int(getInt64(in, "velocity_window_seconds")),
		VelocityThresholdPaise: getInt64(in, "velocity_threshold_paise"),
		CurrentVelocityPaise:   getInt64(in, "current_velocity_paise"),
		SanctionsListAge:       int(getInt64(in, "sanctions_list_age")),
		MatchingThreshold:      getFloat(in, "matching_threshold"),
		AMLRiskScore:           getFloat(in, "aml_risk_score"),
		VelocityExceeded:       getBool(in, "velocity_exceeded"),
		ConsentProvided:        getBool(in, "consent_provided"),
		AuditTrail:             getString(in, "audit_trail"),
		Metadata:               c.Metadata,
	}
}

func buildOrchestrationInput(c Case) judges.OrchestrationJudgeInput {
	in := c.Input
	return judges.OrchestrationJudgeInput{
		OrderID:             getString(in, "order_id"),
		StepSequence:        getStringSlice(in, "step_sequence"),
		CurrentStep:         getString(in, "current_step"),
		PreviousStep:        getString(in, "previous_step"),
		TransitionValid:     getBool(in, "transition_valid"),
		IdempotencyKey:      getString(in, "idempotency_key"),
		PaymentConfirmed:    getBool(in, "payment_confirmed"),
		SettlementInitiated: getBool(in, "settlement_initiated"),
		WorkflowAuditTrail:  getString(in, "workflow_audit_trail"),
		Metadata:            c.Metadata,
	}
}

func buildLineageInput(c Case) judges.LineageJudgeInput {
	in := c.Input
	return judges.LineageJudgeInput{
		OrderID:                  getString(in, "order_id"),
		HashChainValid:           getBool(in, "hash_chain_valid"),
		TimestampsMonotonic:      getBool(in, "timestamps_monotonic"),
		AllRequiredFieldsPresent: getBool(in, "all_required_fields_present"),
		DecisionTraceability:     getString(in, "decision_traceability"),
		Metadata:                 c.Metadata,
	}
}

func buildMerchantInput(c Case) judges.MerchantJudgeInput {
	in := c.Input
	return judges.MerchantJudgeInput{
		MerchantID:           getString(in, "merchant_id"),
		KYBVerified:          getBool(in, "kyb_verified"),
		BankAccountVerified:  getBool(in, "bank_account_verified"),
		RiskScore:            getFloat(in, "risk_score"),
		SettlementConfigured: getBool(in, "settlement_configured"),
		DisputeSLABreached:   getBool(in, "dispute_sla_breached"),
		Metadata:             c.Metadata,
	}
}
