// Package sme_loan_workflow orchestrates the SME lending journey from
// application to in-principle sanction, using pkg/workflow as the DAG
// runtime with HITL approval at the disbursal gate.
//
// Inspired by Google ADK samples → small-business-loan-agent. Tuned for
// the Indian SME stack: GST data, CGTMSE collateral-free guarantee,
// Mudra schemes, and bank statement-driven cashflow underwriting.
//
// Steps (in topo order):
//
//	gst_fetch              → pull turnover + filing regularity
//	cashflow_analysis      → consume bank statements via cashflow_underwriter
//	cgtmse_eligibility     → MSME registration + sector + ticket size check
//	indicative_offer       → propose tenor, rate, EMI
//	human_approval (HITL)  → relationship manager sign-off
//	sanction_letter_draft  → emit the document
//
// Each step records an event to the workflow log so the journey is
// auditable end-to-end (FREE-AI Rec 24).
package sme_loan_workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/workflow"
)

const (
	ID         = "sme_loan_workflow"
	Capability = "orchestrate_sme_loan"
	TypeIn     = "sme_loan_request"
	TypeOut    = "sme_loan_offer"
	NextAgent  = "financial_supervisor"

	cgtmseMaxTicketRupees = 50_000_000 // ₹5 cr revised ceiling (2023)
	maxLoanMultipleOfRev  = 0.30       // 30 % of annual turnover
)

// Application is the inbound packet.
type Application struct {
	BorrowerID         string  `json:"borrower_id"`
	UDYAMRegistered    bool    `json:"udyam_registered"`
	Sector             string  `json:"sector"`               // manufacturing | services | trading
	AnnualTurnover     float64 `json:"annual_turnover_rupees"`
	GSTFilingRegular   bool    `json:"gst_filing_regular"`   // last 12 returns on time
	RequestedAmount    float64 `json:"requested_amount_rupees"`
	RequestedTenorMths int     `json:"requested_tenor_months"`
	CashflowScore0to1  float64 `json:"cashflow_score_0_1"`   // from cashflow_underwriter
	CollateralRupees   float64 `json:"collateral_rupees"`    // 0 if collateral-free
}

// Offer is the structured output.
type Offer struct {
	BorrowerID         string   `json:"borrower_id"`
	Decision           string   `json:"decision"` // "approved" | "in_principle" | "rejected"
	OfferedAmount      float64  `json:"offered_amount_rupees"`
	OfferedTenorMths   int      `json:"offered_tenor_months"`
	IndicativeRatePct  float64  `json:"indicative_rate_pct"`
	MonthlyEMIRupees   float64  `json:"monthly_emi_rupees"`
	CGTMSEEligible     bool     `json:"cgtmse_eligible"`
	Rationale          []string `json:"rationale"`
	WorkflowEvents     int      `json:"workflow_event_count"`
	Disclaimer         string   `json:"disclaimer"`
}

type Agent struct{}

func New() *Agent { return &Agent{} }

func (a *Agent) ID() string                 { return ID }
func (a *Agent) Name() string               { return "SME Loan Workflow" }
func (a *Agent) Capabilities() []string     { return []string{Capability} }
func (a *Agent) RiskLevel() agent.RiskClass { return agent.RiskHigh }

func (a *Agent) HandleMessage(ctx context.Context, msg agent.Message, env agent.Environment) ([]agent.Message, error) {
	if msg.Type != TypeIn {
		return nil, nil
	}
	var app Application
	if err := json.Unmarshal([]byte(msg.Content), &app); err != nil {
		return nil, err
	}
	offer, _ := a.Process(ctx, app, true) // auto-approve in synchronous-handler mode
	env.Logf("[sme_loan_workflow] borrower=%s decision=%s amount=%.0f", offer.BorrowerID, offer.Decision, offer.OfferedAmount)
	body, _ := json.Marshal(offer)
	return []agent.Message{
		agent.NewMessage(ID, NextAgent, agent.RoleAgent, TypeOut, string(body), msg.Metadata),
	}, nil
}

// Process builds and runs the DAG. autoApprove=true short-circuits the
// human-approval gate (used in tests and the synchronous HandleMessage path).
// In production callers run with autoApprove=false and call ApproveStep on the
// workflow externally.
func (a *Agent) Process(ctx context.Context, app Application, autoApprove bool) (Offer, error) {
	sink := workflow.NewInMemorySink()
	w := workflow.New(sink)

	state := workflow.State{"app": app}

	w.Add(workflow.Step{ID: "gst_fetch", Run: gstFetch})
	w.Add(workflow.Step{ID: "cashflow_analysis", DependsOn: []string{"gst_fetch"}, Run: cashflowAnalysis})
	w.Add(workflow.Step{ID: "cgtmse_eligibility", DependsOn: []string{"cashflow_analysis"}, Run: cgtmseCheck})
	w.Add(workflow.Step{ID: "indicative_offer", DependsOn: []string{"cgtmse_eligibility"}, Run: indicativeOffer})
	w.Add(workflow.Step{ID: "human_approval", DependsOn: []string{"indicative_offer"}, RequireApproval: true, Run: humanApproval})
	w.Add(workflow.Step{ID: "sanction_letter", DependsOn: []string{"human_approval"}, Run: sanctionLetterDraft})

	// Run in a goroutine so we can either approve mid-flight (autoApprove=true)
	// or bail out promptly when no approval is forthcoming (autoApprove=false).
	type runResult struct {
		err error
	}
	runDone := make(chan runResult, 1)
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	go func() {
		runDone <- runResult{w.Run(runCtx, state)}
	}()

	// Poll for the awaiting_approval event. In auto-approve mode we then
	// approve; in manual mode we simply cancel runCtx so the workflow
	// exits promptly instead of blocking.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		seen := false
		for _, e := range sink.Events() {
			if e.Kind == workflow.EventAwaiting && e.StepID == "human_approval" {
				seen = true
				break
			}
		}
		if seen {
			if autoApprove {
				w.ApproveStep("human_approval")
			} else {
				cancelRun()
			}
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Bound the wait so a never-approved workflow can't hang the caller.
	var res runResult
	select {
	case res = <-runDone:
	case <-time.After(7 * time.Second):
		res = runResult{errors.New("workflow timed out awaiting approval")}
	}
	offer, _ := buildOfferFromState(state, sink, app)
	// Only downgrade to rejected on a hard step failure — context-bounded
	// "no approval" runs stay as in_principle so the caller can still see
	// what the underwriting would have produced.
	if res.err != nil {
		if offer.Decision == "approved" {
			offer.Decision = "rejected"
			offer.Rationale = append(offer.Rationale, "Workflow error: "+res.err.Error())
		}
	}
	return offer, res.err
}

// --- step functions ----------------------------------------------------------

func gstFetch(ctx context.Context, state workflow.State) error {
	app, _ := state["app"].(Application)
	if app.AnnualTurnover <= 0 {
		return errors.New("missing GST turnover data")
	}
	state.Set("gst_turnover", app.AnnualTurnover)
	state.Set("gst_filing_regular", app.GSTFilingRegular)
	return nil
}

func cashflowAnalysis(ctx context.Context, state workflow.State) error {
	app, _ := state["app"].(Application)
	if app.CashflowScore0to1 < 0.30 {
		state.Set("cashflow_pass", false)
		return errors.New("cashflow score below underwriting floor")
	}
	state.Set("cashflow_pass", true)
	state.Set("cashflow_score", app.CashflowScore0to1)
	return nil
}

func cgtmseCheck(ctx context.Context, state workflow.State) error {
	app, _ := state["app"].(Application)
	eligible := app.UDYAMRegistered &&
		app.RequestedAmount <= cgtmseMaxTicketRupees &&
		isCovered(app.Sector)
	state.Set("cgtmse_eligible", eligible)
	return nil
}

func isCovered(sector string) bool {
	switch sector {
	case "manufacturing", "services", "trading":
		return true
	}
	return false
}

func indicativeOffer(ctx context.Context, state workflow.State) error {
	app, _ := state["app"].(Application)
	maxByTurnover := app.AnnualTurnover * maxLoanMultipleOfRev
	offerAmt := app.RequestedAmount
	if offerAmt > maxByTurnover {
		offerAmt = maxByTurnover
	}
	// Indicative rate: 10.5 % floor + cashflow risk premium, minus CGTMSE relief.
	rate := 10.5 + (1.0-app.CashflowScore0to1)*5.0
	if cge, _ := state["cgtmse_eligible"].(bool); cge {
		rate -= 0.5
	}
	state.Set("offered_amount", offerAmt)
	state.Set("offered_rate", rate)
	state.Set("offered_tenor", app.RequestedTenorMths)
	state.Set("monthly_emi", emi(offerAmt, rate, app.RequestedTenorMths))
	return nil
}

func humanApproval(ctx context.Context, state workflow.State) error {
	// nothing to compute; presence of the step proves the gate fired.
	state.Set("human_approved", true)
	return nil
}

func sanctionLetterDraft(ctx context.Context, state workflow.State) error {
	app, _ := state["app"].(Application)
	amt, _ := state["offered_amount"].(float64)
	rate, _ := state["offered_rate"].(float64)
	state.Set("sanction_letter", fmt.Sprintf(
		"Sanction letter for %s — ₹%.0f at %.2f%% (indicative). Final disbursal subject to documentation.",
		app.BorrowerID, amt, rate,
	))
	return nil
}

// --- offer construction ------------------------------------------------------

func buildOfferFromState(state workflow.State, sink *workflow.InMemorySink, app Application) (Offer, error) {
	rationale := []string{}
	decision := "rejected"

	cashflowPass, _ := state["cashflow_pass"].(bool)
	cgtmseEligible, _ := state["cgtmse_eligible"].(bool)
	humanApproved, _ := state["human_approved"].(bool)

	switch {
	case !cashflowPass:
		rationale = append(rationale, "Cashflow score below underwriting threshold")
	case !humanApproved:
		decision = "in_principle"
		rationale = append(rationale, "Indicative offer prepared; awaiting RM approval")
	default:
		decision = "approved"
		rationale = append(rationale, "All checks cleared; sanction letter drafted")
	}
	if cgtmseEligible {
		rationale = append(rationale, "CGTMSE eligible — collateral-free guarantee applicable")
	}

	amt, _ := state["offered_amount"].(float64)
	rate, _ := state["offered_rate"].(float64)
	tenor, _ := state["offered_tenor"].(int)
	emiVal, _ := state["monthly_emi"].(float64)

	return Offer{
		BorrowerID:        app.BorrowerID,
		Decision:          decision,
		OfferedAmount:     round2(amt),
		OfferedTenorMths:  tenor,
		IndicativeRatePct: round2(rate),
		MonthlyEMIRupees:  round2(emiVal),
		CGTMSEEligible:    cgtmseEligible,
		Rationale:         rationale,
		WorkflowEvents:    len(sink.Events()),
		Disclaimer: "Indicative SME loan offer. Final sanction subject to credit committee, " +
			"complete documentation, and CGTMSE registration where applicable.",
	}, nil
}

// emi returns the standard reducing-balance EMI.
func emi(principal, annualRatePct float64, months int) float64 {
	if months <= 0 || principal <= 0 {
		return 0
	}
	r := annualRatePct / 12.0 / 100.0
	if r == 0 {
		return principal / float64(months)
	}
	// p * r * (1+r)^n / ((1+r)^n - 1)
	pow := 1.0
	for i := 0; i < months; i++ {
		pow *= 1 + r
	}
	return principal * r * pow / (pow - 1)
}

func round2(x float64) float64 { return float64(int64(x*100+0.5)) / 100 }
