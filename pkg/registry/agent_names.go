package registry

// AllLegacyAgents is the canonical list of agents in cmd/api/main.go that can be
// served as remote HTTP services (via HTTPRegistryAgent + a LegacyHTTPServer).
//
// Capabilities mirror pkg/agentgov/setup.go capabilitiesFor() EXACTLY, so the
// RingEnforcer (when enforcement is switched on — it is currently passive) sees
// the same capability set for a proxied agent as for the original in-process
// agent. Keep these in sync with the ring maps in agentgov/setup.go.
//
// Deliberately EXCLUDED (these stay in-process — see EVAL/decoupling notes):
//   - auditor                — bus broadcast subscriber; must see ALL messages,
//     so it cannot be a targeted HTTP agent.
//   - portfolio_advisor_fallback, recommender_fallback — ~10-line safety stubs,
//     not business logic.
//
// Special prerequisites (migratable, but not naively — flagged for later waves):
//   - supervisor       — holds per-trace fan-out state (needs Redis or 1 replica)
//   - portfolio_advisor — Postgres token lookup + KMS (needs PgBouncer/KMS)
//   - educator         — active RAG (needs Ollama connectivity in its pod)
var AllLegacyAgents = []AgentDef{
	// Ring 0 (admin, wildcard capability).
	{ID: "supervisor", Name: "supervisor", Capabilities: capRing0},

	// Ring 1 (standard: message.handle + tool.call + llm.complete + document.read + bus.publish).
	{ID: "analyzer", Name: "analyzer", Capabilities: capRing1},
	{ID: "recommender", Name: "recommender", Capabilities: capRing1},
	{ID: "reporter", Name: "reporter", Capabilities: capRing1},
	{ID: "forecaster", Name: "forecaster", Capabilities: capRing1},
	{ID: "educator", Name: "educator", Capabilities: capRing1},
	{ID: "portfolio_advisor", Name: "portfolio_advisor", Capabilities: capRing1},
	{ID: "deep_research", Name: "deep_research", Capabilities: capRing1},

	// Ring 2 (restricted: message.handle + tool.call + bus.publish).
	{ID: "ingestor", Name: "ingestor", Capabilities: capRing2},
	{ID: "normalizer", Name: "normalizer", Capabilities: capRing2},
	{ID: "enricher", Name: "enricher", Capabilities: capRing2},
	{ID: "currency", Name: "currency", Capabilities: capRing2},
	{ID: "macro", Name: "macro", Capabilities: capRing2},
	{ID: "rates", Name: "rates", Capabilities: capRing2},
	{ID: "loan", Name: "loan", Capabilities: capRing2},
	{ID: "tax_estimator", Name: "tax_estimator", Capabilities: capRing2},
	{ID: "kyc_orchestrator", Name: "kyc_orchestrator", Capabilities: capRing2},
	{ID: "claim_adjudicator", Name: "claim_adjudicator", Capabilities: capRing2},
	{ID: "sme_loan_workflow", Name: "sme_loan_workflow", Capabilities: capRing2},
	{ID: "invoice_processor", Name: "invoice_processor", Capabilities: capRing2},
	{ID: "bulk_statement_analyzer", Name: "bulk_statement_analyzer", Capabilities: capRing2},
	{ID: "anomaly", Name: "anomaly", Capabilities: capRing2},
	{ID: "cyber_guardian", Name: "cyber_guardian", Capabilities: capRing2},
	{ID: "payment_orchestrator", Name: "payment_orchestrator", Capabilities: capRing2},

	// Ring 3 (sandboxed: message.handle only).
	{ID: "aa_fetcher", Name: "aa_fetcher", Capabilities: capRing3},
	{ID: "voice", Name: "voice", Capabilities: capRing3},
	{ID: "mpc_research", Name: "mpc_research", Capabilities: capRing3},
	{ID: "google_trends", Name: "google_trends", Capabilities: capRing3},
	{ID: "auto_insurance", Name: "auto_insurance", Capabilities: capRing3},
	{ID: "health_preauth", Name: "health_preauth", Capabilities: capRing3},
	{ID: "supply_chain_finance", Name: "supply_chain_finance", Capabilities: capRing3},
}

// Ring capability sets — mirror agentgov/setup.go capabilitiesFor().
var (
	capRing0 = []string{"*"}
	capRing1 = []string{"message.handle", "tool.call", "llm.complete", "document.read", "bus.publish"}
	capRing2 = []string{"message.handle", "tool.call", "bus.publish"}
	capRing3 = []string{"message.handle"}
)

// InProcessOnly lists the agent IDs that must NOT be replaced by an
// HTTPRegistryAgent (they stay in-process). Documented here so the cmd/api
// wiring loop can assert it never proxies one of them.
var InProcessOnly = map[string]bool{
	"auditor":                    true, // bus broadcast subscriber
	"portfolio_advisor_fallback": true, // safety stub
	"recommender_fallback":       true, // safety stub
}
