package catalog

import (
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
)

// All returns every ported request/response specialist spec.
func All() []afg.Spec {
	return []afg.Spec{
		AaFetcherSpec(),
		AdvanceTaxPlannerSpec(),
		AlmAgentSpec(),
		AmlMonitorSpec(),
		AssetAllocatorSpec(),
		AuditorSpec(),
		AutoInsuranceSpec(),
		BulkStatementAnalyzerSpec(),
		CarbonEstimatorSpec(),
		CashflowUnderwriterSpec(),
		ClaimAdjudicatorSpec(),
		ComplaintTriageSpec(),
		CyberGuardianSpec(),
		DebtOptimizerSpec(),
		DeductionsOptimizerSpec(),
		DeepResearchSpec(),
		DividendPlannerSpec(),
		EducatorSpec(),
		EmergencyFundSpec(),
		FraudSpec(),
		GoalPlannerSpec(),
		GoogleTrendsSpec(),
		HealthPreauthSpec(),
		InvoiceDiscounterSpec(),
		InvoiceProcessorSpec(),
		KycOrchestratorSpec(),
		LcrProjectorSpec(),
		LoanSpec(),
		MfScreenerSpec(),
		MpcResearchSpec(),
		MuleSpec(),
		OptionsExplainerSpec(),
		PaymentOrchestratorSpec(),
		PhishingClassifierSpec(),
		PortfolioAdvisorSpec(),
		PrepaymentAdvisorSpec(),
		SipVsLumpsumSpec(),
		SmeLoanWorkflowSpec(),
		SubscriptionDetectorSpec(),
		SupplyChainFinanceSpec(),
		SyntheticIdentitySpec(),
		TaxHarvesterSpec(),
		VarCalculatorSpec(),
		VoiceSpec(),
		WorkingCapitalSpec(),
	}
}

// Registry builds a governed registry of all ported specialists plus the
// hand-ported deterministic agents (currency/rates/tax_estimator/macro).
func Registry(gate governance.Policy) *afg.Registry {
	reg := afg.DefaultRegistry(gate)
	reg.RegisterSpecs(gate, All()...)
	reg.RegisterSpecs(gate, afg.PipelineSpecs()...)
	return reg
}
