// Package eval defines failure mode taxonomy, evaluation rubrics, and assessment matrices
// for comprehensive scenario-based testing of the Genie multi-agent platform.
//
// Failure modes are organized across 5 domains:
//   - Settlement (SE): financial ledger, netting, batching, finality
//   - Compliance (CO): AML, KYC, velocity, consent, sanctions screening
//   - Orchestration (OR): workflow state machine, agent communication, ordering
//   - Merchant/Customer (MC): onboarding, disputes, identity, fraud
//   - Agent Behavior (AB): hallucination, consistency, prompt injection, alignment
//   - Lineage/Audit (LA): transaction tracking, immutability, proof chains
//
// Each failure mode includes:
//   - ID: FM-DOMAIN-NNN (e.g., FM-SE-001)
//   - Name: human-readable title
//   - Category: operational, security, compliance, availability
//   - Severity: critical, high, medium, low
//   - RBIConcern: RBI regulatory framework mapping
//   - ExampleScenario: concrete instance when this might occur
//   - LinkedRubrics: evaluation methods to assess this failure
//
// Usage:
//   fm := failure_modes.AllFailureModes["FM-SE-001"]
//   rubric := rubrics.AllRubrics[fm.LinkedRubrics[0]]
//   assessment := rubric.Evaluate(testResult)
package eval

// FailureModeSeverity represents the operational/compliance impact level
type FailureModeSeverity string

const (
	SeverityCritical FailureModeSeverity = "critical"
	SeverityHigh     FailureModeSeverity = "high"
	SeverityMedium   FailureModeSeverity = "medium"
	SeverityLow      FailureModeSeverity = "low"
)

// FailureModeCategory classifies the type of failure
type FailureModeCategory string

const (
	CategoryOperational FailureModeCategory = "operational"
	CategorySecurity    FailureModeCategory = "security"
	CategoryCompliance  FailureModeCategory = "compliance"
	CategoryAvailability FailureModeCategory = "availability"
)

// FailureModeDomain buckets related failure modes
type FailureModeDomain string

const (
	DomainSettlement   FailureModeDomain = "settlement"
	DomainCompliance   FailureModeDomain = "compliance"
	DomainOrchestration FailureModeDomain = "orchestration"
	DomainMerchantCustomer FailureModeDomain = "merchant_customer"
	DomainAgentBehavior FailureModeDomain = "agent_behavior"
	DomainLineageAudit FailureModeDomain = "lineage_audit"
)

// FailureMode describes a specific system failure scenario with diagnostic context
type FailureMode struct {
	// ID unique identifier (FM-DOMAIN-NNN)
	ID string `json:"id"`
	// Name human-readable failure mode title
	Name string `json:"name"`
	// Domain which system area this affects
	Domain FailureModeDomain `json:"domain"`
	// Category operational/security/compliance/availability
	Category FailureModeCategory `json:"category"`
	// Description detailed explanation of what goes wrong
	Description string `json:"description"`
	// Severity critical/high/medium/low
	Severity FailureModeSeverity `json:"severity"`
	// RBIConcern which RBI regulatory concern this maps to
	RBIConcern string `json:"rbi_concern"`
	// ExampleScenario concrete instance when this occurs
	ExampleScenario string `json:"example_scenario"`
	// RootCausePatterns what system conditions trigger this
	RootCausePatterns []string `json:"root_cause_patterns"`
	// SystemImpact how this affects downstream components
	SystemImpact string `json:"system_impact"`
	// LinkedRubrics which evaluation rubrics assess this failure
	LinkedRubrics []string `json:"linked_rubrics"`
	// RecoveryPath how to detect and recover from this failure
	RecoveryPath string `json:"recovery_path"`
	// PreventionControl what control prevents this failure
	PreventionControl string `json:"prevention_control"`
}

// AllFailureModes registry of all 38 failure modes across 5 domains
var AllFailureModes = map[string]*FailureMode{
	// === SETTLEMENT (SE) domain: 8 modes ===
	// Settlement ledger, finality, batching, netting

	"FM-SE-001": {
		ID:      "FM-SE-001",
		Name:    "Double-Spend in CBDC Ledger",
		Domain:  DomainSettlement,
		Category: CategorySecurity,
		Description: "Customer can spend the same CBDC token twice due to missing idempotency checks " +
			"or race conditions in ledger commit. Results in negative merchant balance or unreconciled entries.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-PS-001: Payment System Integrity (CBDC finality)",
		ExampleScenario: "Order 1 and Order 2 initiated simultaneously by same customer; both commit with same balance without mutual exclusion.",
		RootCausePatterns: []string{
			"missing ledger transaction lock",
			"concurrent payment confirmation",
			"no idempotency key validation",
		},
		SystemImpact: "Ledger balance corruption, merchant settlement incorrect, regulatory breach",
		LinkedRubrics: []string{"RB-SE-001", "RB-SE-002", "RB-SE-008"},
		RecoveryPath:  "Detect via reconciliation.IsReconciled() returning false; trigger manual ledger reversal and customer re-payment",
		PreventionControl: "Implement ledger transaction locks, idempotency key validation, pre-commit balance verification",
	},

	"FM-SE-002": {
		ID:      "FM-SE-002",
		Name:    "Settlement Batch Mismatch",
		Domain:  DomainSettlement,
		Category: CategoryOperational,
		Description: "Settlement consolidation includes wrong merchants or amounts, causing batch total to differ " +
			"from sum of orders. Ledger records batch but individual order lineage shows different amounts.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-FS-002: Financial Stability (settlement accuracy)",
		ExampleScenario: "ConsolidateSettlementBatch() groups 3 orders (100, 200, 300 paise) but ledger records 500 paise total.",
		RootCausePatterns: []string{
			"incorrect merchant grouping logic",
			"amount rounding error in batch sum",
			"orders added/removed during consolidation window",
		},
		SystemImpact: "Merchant reconciliation fails, settlement history audit breaks, financial discrepancy",
		LinkedRubrics: []string{"RB-SE-003", "RB-SE-004", "RB-LA-001"},
		RecoveryPath:  "Reconciliation check VerifyOrderSettlement() should catch; halt settlement, re-validate batch composition",
		PreventionControl: "Validate batch sum against individual orders pre-commit, use deterministic sorting for merchant grouping",
	},

	"FM-SE-003": {
		ID:      "FM-SE-003",
		Name:    "Netting Calculation Overflow",
		Domain:  DomainSettlement,
		Category: CategoryOperational,
		Description: "Multi-merchant netting calculation overflows or underflows integer bounds, causing " +
			"negative merchant positions or settlement amount truncation.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-ACC-001: Accounting Accuracy (settlement math)",
		ExampleScenario: "Three merchants settle with large bilateral transfers; netting sum exceeds int64 max, stored as negative value",
		RootCausePatterns: []string{
			"no overflow check in netting sum",
			"unsigned/signed integer mismatch",
			"accumulator not big.Int",
		},
		SystemImpact: "Merchant settlements incorrect, accounting ledger imbalance, regulatory reporting wrong",
		LinkedRubrics: []string{"RB-SE-005", "RB-SE-006"},
		RecoveryPath:  "Detect in ConvertToCBDCPositions() via range check; fall back to serial settlement (no netting)",
		PreventionControl: "Use big.Int for netting accumulators, validate sum before final commit, test with large amounts",
	},

	"FM-SE-004": {
		ID:      "FM-SE-004",
		Name:    "Settlement Finality Violation",
		Domain:  DomainSettlement,
		Category: CategoryCompliance,
		Description: "Order marked as settled but ledger transaction not yet committed (t+1 latency not respected). " +
			"Subsequent queries show conflicting settlement status.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-PS-002: Payment Finality (settlement time guarantees)",
		ExampleScenario: "Order workflow transitions to Fulfilled before CBDC ledger WaitSettlementCompletion() returns success",
		RootCausePatterns: []string{
			"order status updated before ledger confirmation",
			"async ledger commit without blocking",
			"no settlement finality verification",
		},
		SystemImpact: "Order lineage shows settled but ledger shows pending; reconciliation detects contradiction",
		LinkedRubrics: []string{"RB-OR-003", "RB-LA-002"},
		RecoveryPath:  "Lineage tracker detects mismatch via VerifyOrderSettlement(); rollback order to StatusPaid state",
		PreventionControl: "Enforce blocking ledger.CommitTransaction() before updating order status to Fulfilled",
	},

	"FM-SE-005": {
		ID:      "FM-SE-005",
		Name:    "Orphaned Settlement Records",
		Domain:  DomainSettlement,
		Category: CategoryOperational,
		Description: "Settlement record created in ledger but order workflow does not complete (e.g., network partition). " +
			"Merchant payment stuck in limbo, customer not charged.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-OP-001: Operational Risk (incomplete settlement)",
		ExampleScenario: "Settlement initiated, ledger.CommitTransaction() succeeds, but workflow crashes before lineage.Record() called",
		RootCausePatterns: []string{
			"settlement committed but workflow not updated",
			"no compensating transaction on partial failure",
			"missing settlement acknowledgment handshake",
		},
		SystemImpact: "Merchant balance increased but order not marked fulfilled; settlement-order mismatch in reconciliation",
		LinkedRubrics: []string{"RB-LA-003", "RB-OR-004"},
		RecoveryPath:  "Reconciliation query finds settlement without matching order; create manual order correction entry",
		PreventionControl: "Use two-phase commit pattern: prepare settlement, record in lineage, then finalize order status",
	},

	"FM-SE-006": {
		ID:      "FM-SE-006",
		Name:    "Currency Conversion Error in Settlement",
		Domain:  DomainSettlement,
		Category: CategoryOperational,
		Description: "FX conversion applied with stale rate or rounding error, causing merchant to receive " +
			"different amount than order value in foreign currency settlement.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-FX-001: Foreign Exchange Risk (conversion accuracy)",
		ExampleScenario: "Order in EUR settled to INR merchant; FX rate outdated by >5min or rounding loses paise",
		RootCausePatterns: []string{
			"FX rate cache not invalidated",
			"rounding to nearest rupee instead of paise",
			"no FX rate validation before conversion",
		},
		SystemImpact: "Merchant receives incorrect amount, customer complaint, potential regulatory fine",
		LinkedRubrics: []string{"RB-SE-007"},
		RecoveryPath:  "Detect via lineage tracking FX rate metadata; calculate adjustment, issue credit note",
		PreventionControl: "Enforce FX rate freshness check (max 1min old), use bankers rounding to paise, audit trail FX rates used",
	},

	"FM-SE-007": {
		ID:      "FM-SE-007",
		Name:    "Batch Settlement Timeout",
		Domain:  DomainSettlement,
		Category: CategoryAvailability,
		Description: "Settlement batch initiated but CBDC ledger does not respond within SLA, leaving orders " +
			"in inconsistent state (settlement initiated but not confirmed).",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-AVL-001: Availability (settlement SLA)",
		ExampleScenario: "ConsolidateSettlementBatch() times out at t+30min; orders remain StatusPending, customer refund timeout triggers",
		RootCausePatterns: []string{
			"CBDC ledger unavailable",
			"long network latency",
			"batch processing queue overloaded",
			"no settlement timeout retry logic",
		},
		SystemImpact: "Orders stuck in settlement limbo, merchants not paid, customers refunded late, SLA breach",
		LinkedRubrics: []string{"RB-OR-005", "RB-OR-006"},
		RecoveryPath:  "Monitor settlement timeout; trigger escalation to settlement agent to retry with exponential backoff",
		PreventionControl: "Implement settlement timeout SLA (e.g., 5min), automatic retry with circuit breaker, HITL escalation after 3 retries",
	},

	"FM-SE-008": {
		ID:      "FM-SE-008",
		Name:    "Reconciliation Data Loss",
		Domain:  DomainSettlement,
		Category: CategoryCompliance,
		Description: "Settlement record lost or corrupted in storage (database, cache, ledger snapshot), causing " +
			"reconciliation to report missing entries and imbalance.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-AUD-001: Audit Trail Integrity (immutable records)",
		ExampleScenario: "Ledger snapshot taken but settlement batch record not included due to timing issue; reconciliation.IsReconciled() returns false",
		RootCausePatterns: []string{
			"database transaction rolled back unexpectedly",
			"ledger snapshot race condition",
			"cache eviction without persistent backup",
		},
		SystemImpact: "Cannot prove settlement occurred, audit trail broken, regulatory inspection failure",
		LinkedRubrics: []string{"RB-LA-004", "RB-LA-005"},
		RecoveryPath:  "Detect via reconciliation mismatch; query backup ledger copy, restore from transaction log replay",
		PreventionControl: "Implement dual-write to persistent ledger and backup store, transaction log replication, periodic audit snapshots",
	},

	// === COMPLIANCE (CO) domain: 8 modes ===
	// AML, KYC, velocity, consent, sanctions

	"FM-CO-001": {
		ID:      "FM-CO-001",
		Name:    "AML Velocity Breach (Undetected)",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Customer makes rapid transactions exceeding velocity limits (e.g., 5 orders >₹50k in 1 hour) " +
			"but compliance check is bypassed or returns false negative. Order settles without AML block.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-AML-001: Anti-Money Laundering (velocity monitoring)",
		ExampleScenario: "Customer submits 5 orders of ₹51k each in 45 minutes; velocity checker cached outdated results, orders approved",
		RootCausePatterns: []string{
			"velocity cache not invalidated on order creation",
			"timestamp comparison off-by-one in window calculation",
			"concurrent order submission race condition",
		},
		SystemImpact: "Potential money laundering proceeds through system, regulatory breach, customer sanction evasion",
		LinkedRubrics: []string{"RB-CO-001", "RB-CO-002"},
		RecoveryPath:  "Post-settlement compliance audit detects pattern; flag orders for HITL review, reverse settlements, file regulatory report",
		PreventionControl: "Enforce real-time velocity check before payment initiation, use distributed counter for per-customer tracking, alert on threshold approach",
	},

	"FM-CO-002": {
		ID:      "FM-CO-002",
		Name:    "Sanctions Screening False Negative",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Customer or merchant is on sanctions list (FATF, OFAC, RBI) but screening lookup returns false " +
			"due to name matching threshold too high, cache corruption, or lookup service failure.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-SANC-001: Sanctions Compliance (FATF/OFAC blacklist)",
		ExampleScenario: "Merchant 'Ahmed Khan Ltd' matches sanctions list entry 'Ahmed Khan' but name distance metric >0.85, screening passes",
		RootCausePatterns: []string{
			"fuzzy match threshold misconfigured",
			"sanctions list update stale (>24h old)",
			"whitelist override applied incorrectly",
		},
		SystemImpact: "Transacts with sanctioned entity, regulatory fine, license suspension risk",
		LinkedRubrics: []string{"RB-CO-003", "RB-CO-004"},
		RecoveryPath:  "Regulatory audit or external notification triggers screening re-check; flag settlement for reversal, investigate prior transactions",
		PreventionControl: "Use conservative threshold (>0.95), update sanctions lists daily, verify merchant against multiple FATF lists, log all screening results",
	},

	"FM-CO-003": {
		ID:      "FM-CO-003",
		Name:    "KYC Verification Bypass",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Customer or merchant onboards without completing KYC (e.g., document upload skipped, " +
			"verification pending). Orders placed and settled before KYC completes.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-KYC-001: Know Your Customer (verification requirements)",
		ExampleScenario: "Merchant submits onboarding, gets temporary account ID, creates orders before KYC completion flag is checked in handler",
		RootCausePatterns: []string{
			"KYC status check missing from order creation handler",
			"temporary account flag not enforced in payment workflow",
			"KYC deadline extension not validated",
		},
		SystemImpact: "Unverified customer/merchant settles funds, regulatory breach, compliance investigation",
		LinkedRubrics: []string{"RB-CO-005", "RB-MC-001"},
		RecoveryPath:  "Risk team detects unverified transactions in audit; place merchant account on hold, reverse recent settlements until KYC complete",
		PreventionControl: "Enforce KYC status check at order creation, payment initiation, and settlement phases; block orders for unverified accounts",
	},

	"FM-CO-004": {
		ID:      "FM-CO-004",
		Name:    "Data Consent Missing (A2A/Data Sharing)",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Order uses customer data (e.g., account linking, payment history) without explicit consent " +
			"recorded in consent registry. Data sharing downstream without permission trail.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-CONS-001: Consent Management (Account Aggregator)",
		ExampleScenario: "Payment settlement queries customer bank balance (via Account Aggregator) without consent.Verify() check",
		RootCausePatterns: []string{
			"consent check bypassed in settlement flow",
			"consent record not created before data access",
			"consent expiry not validated",
		},
		SystemImpact: "Regulatory violation (data privacy), customer complaint, potential fine for data misuse",
		LinkedRubrics: []string{"RB-CO-006", "RB-LA-006"},
		RecoveryPath:  "Audit detects data access without consent; notify customer, obtain retroactive consent or reverse transaction",
		PreventionControl: "Record consent before any data access, validate consent expiry, audit all data access against consent log",
	},

	"FM-CO-005": {
		ID:      "FM-CO-005",
		Name:    "Regulatory Reporting False Data",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Settlement data aggregated for regulatory reporting (RBI, FATF) but includes errors: " +
			"wrong amounts, missing transactions, duplicate entries. Filed report inaccurate.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-REP-001: Regulatory Reporting (accuracy and timeliness)",
		ExampleScenario: "Daily settlement report includes order that was reversed, causing reported turnover >actual; RBI notice",
		RootCausePatterns: []string{
			"reversed orders not excluded from report",
			"amount rounding error in aggregation",
			"report generation time window off-by-one",
		},
		SystemImpact: "Regulatory investigation, compliance penalty, audit dispute",
		LinkedRubrics: []string{"RB-CO-007", "RB-LA-007"},
		RecoveryPath:  "Detect via RBI inquiry; file amended report, demonstrate correction in audit trail",
		PreventionControl: "Validate report data against settlement ledger, exclude reversed/cancelled orders, test reporting with known data set",
	},

	"FM-CO-006": {
		ID:      "FM-CO-006",
		Name:    "Customer Risk Scoring Stale",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Customer risk score used for compliance decision but cached score is outdated (e.g., recent PEP " +
			"designation not reflected). Order approved for high-risk customer.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-KYC-002: Risk-Based Approach (customer risk classification)",
		ExampleScenario: "Customer was clean 24h ago; now designated PEP but cached score still 'low-risk', order approved",
		RootCausePatterns: []string{
			"risk score cache TTL too long (>24h)",
			"no cache invalidation on external data update",
			"PEP list update not triggering score recalculation",
		},
		SystemImpact: "High-risk customer transactions approved, regulatory oversight failure, potential sanctions evasion",
		LinkedRubrics: []string{"RB-CO-008"},
		RecoveryPath:  "Compliance audit detects stale risk score; recalculate for all orders by customer in past 24h, escalate high-risk ones",
		PreventionControl: "Reduce risk score cache TTL to 4h, subscribe to external PEP/sanctions updates, trigger immediate recalculation on alert",
	},

	"FM-CO-007": {
		ID:      "FM-CO-007",
		Name:    "Compliance Decision Audit Trail Missing",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Compliance check executed (AML, KYC, sanctions) but decision reason and evidence not recorded " +
			"in audit trail. Later review cannot determine why order was approved/blocked.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AUD-002: Audit Trail (compliance decisions)",
		ExampleScenario: "Order approved after velocity check but check parameters, thresholds, and cached results not logged",
		RootCausePatterns: []string{
			"compliance decision not logged to lineage",
			"check parameters stripped before audit record",
			"evidence (matching scores, list entries) not captured",
		},
		SystemImpact: "Cannot prove regulatory compliance, inspection failure, potential violation if decision was wrong",
		LinkedRubrics: []string{"RB-LA-008"},
		RecoveryPath:  "Enhance compliance logging; retroactively document decisions where possible from system state",
		PreventionControl: "Log compliance decision with full context: check type, thresholds, evidence, customer score, pass/fail, timestamp",
	},

	"FM-CO-008": {
		ID:      "FM-CO-008",
		Name:    "Customer Document Verification Fraud",
		Domain:  DomainCompliance,
		Category: CategorySecurity,
		Description: "KYC document (PAN, Aadhaar) verified but fraudulent (deepfake, altered, reused from another customer). " +
			"Merchant onboards with fake identity.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-KYC-003: Document Verification (authenticity)",
		ExampleScenario: "Customer uploads AI-generated PAN image; liveness check passes; account created with fake identity",
		RootCausePatterns: []string{
			"document verification using simple image comparison",
			"no liveness check for identity docs",
			"duplicate document check not enforced",
		},
		SystemImpact: "Merchant account used for fraud, money laundering, regulatory breach",
		LinkedRubrics: []string{"RB-CO-009", "RB-MC-002"},
		RecoveryPath:  "Fraud detection pattern identifies duplicate KYC docs; account quarantine, investigation, potential law enforcement",
		PreventionControl: "Use certified KYC providers (e-KYC via UIDAI), liveness checks, blockchain verification of documents, duplicate PAN screening",
	},

	// === ORCHESTRATION (OR) domain: 8 modes ===
	// Workflow state machine, agent communication, ordering

	"FM-OR-001": {
		ID:      "FM-OR-001",
		Name:    "Invalid State Transition",
		Domain:  DomainOrchestration,
		Category: CategoryOperational,
		Description: "Order workflow transitions from invalid state (e.g., Cancelled -> Fulfilled, or skips required step). " +
			"State machine logic not enforced; order lineage shows impossible sequence.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-OP-002: Operational Control (process integrity)",
		ExampleScenario: "Order in StatusCancelled transitioned to StatusFulfilled, skipping payment step entirely",
		RootCausePatterns: []string{
			"no validation of current state before transition",
			"external order status update not using state machine",
			"concurrent state modification without lock",
		},
		SystemImpact: "Order lineage invalid, reconciliation detects impossible sequence, settlement integrity questioned",
		LinkedRubrics: []string{"RB-OR-001", "RB-LA-002"},
		RecoveryPath:  "Lineage tracker detects invalid transition, flags order for manual review, rolls back to last valid state",
		PreventionControl: "Enforce state machine in DefaultWorkflowOrchestrator.ExecuteWorkflow(), use strict enum validation, log all transitions",
	},

	"FM-OR-002": {
		ID:      "FM-OR-002",
		Name:    "Agent Communication Timeout",
		Domain:  DomainOrchestration,
		Category: CategoryAvailability,
		Description: "Payment Agent or Settlement Agent does not respond within timeout window. Workflow hangs " +
			"waiting for confirmation, order stuck indefinitely.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-AVL-002: System Availability (agent responsiveness)",
		ExampleScenario: "PaymentAgentStub.InitiatePayment() blocks for >30s; workflow timeout not triggered, customer refund waits",
		RootCausePatterns: []string{
			"no timeout set on agent RPC call",
			"synchronous agent call in critical path",
			"agent service overloaded or crashed",
		},
		SystemImpact: "Order processing stalled, customer refund delay, potential refund timeout trigger, settlement missed",
		LinkedRubrics: []string{"RB-OR-002", "RB-OR-005"},
		RecoveryPath:  "Monitor workflow timeout; escalate to agent health check, trigger automatic retry with circuit breaker, HITL escalation",
		PreventionControl: "Set explicit timeout on agent calls (e.g., 30s), implement automatic retry logic, use async patterns (task queue)",
	},

	"FM-OR-003": {
		ID:      "FM-OR-003",
		Name:    "Workflow Step Out-of-Order Execution",
		Domain:  DomainOrchestration,
		Category: CategoryOperational,
		Description: "Workflow steps executed in wrong order (e.g., settlement initiated before payment confirmed, " +
			"fulfillment before settlement). Order state corrupted.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-OP-003: Process Control (step sequencing)",
		ExampleScenario: "Settlement initiated while payment still pending; settlement fails because payment not confirmed in ledger",
		RootCausePatterns: []string{
			"no dependency check between workflow steps",
			"concurrent step execution without blocking",
			"retry logic re-executing in wrong order",
		},
		SystemImpact: "Settlement fails due to invalid preconditions, order requires manual intervention, audit trail shows bad sequence",
		LinkedRubrics: []string{"RB-OR-003", "RB-OR-004"},
		RecoveryPath:  "Lineage tracker detects out-of-order steps; rewind to last valid checkpoint and re-execute in correct order",
		PreventionControl: "Enforce strict step sequencing in ExecuteWorkflow(), block step execution until prior step completes, log step entry/exit",
	},

	"FM-OR-004": {
		ID:      "FM-OR-004",
		Name:    "Duplicate Payment Agent Call",
		Domain:  DomainOrchestration,
		Category: CategoryOperational,
		Description: "Payment agent call initiated twice for same order (e.g., retry logic re-triggers before first response received). " +
			"Customer charged twice.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-PS-003: Payment Idempotency (prevent double-charge)",
		ExampleScenario: "Retry timer fires while first payment still pending; second InitiatePayment() called with same orderID",
		RootCausePatterns: []string{
			"no idempotency key on payment request",
			"retry logic triggered before first attempt completes",
			"agent response cache not checked before retry",
		},
		SystemImpact: "Customer charged twice, merchant receives double settlement, complaint and chargeback",
		LinkedRubrics: []string{"RB-OR-004", "RB-SE-001"},
		RecoveryPath:  "Reconciliation detects duplicate settlement; identify duplicate, issue reversal to customer, adjust merchant balance",
		PreventionControl: "Implement idempotency key (orderID) on payment request, check payment status before retrying, use deduplication cache",
	},

	"FM-OR-005": {
		ID:      "FM-OR-005",
		Name:    "Orchestrator Crash During Workflow",
		Domain:  DomainOrchestration,
		Category: CategoryAvailability,
		Description: "Orchestrator process crashes mid-workflow (e.g., during settlement). Order state not persisted; " +
			"on restart, unclear if settlement completed or rolled back.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-AVL-003: System Resilience (crash recovery)",
		ExampleScenario: "Orchestrator initiates settlement, ledger confirms, but orchestrator crashes before lineage.Record() and order status update",
		RootCausePatterns: []string{
			"workflow state not persisted to durable store",
			"no recovery checkpoint on orchestrator start",
			"two-phase commit not used",
		},
		SystemImpact: "Settlement state ambiguous, order-ledger mismatch on restart, requires manual reconciliation",
		LinkedRubrics: []string{"RB-OR-006", "RB-LA-009"},
		RecoveryPath:  "On restart, query ledger for settlement confirmation; if found, complete order status update; if not, rollback and refund",
		PreventionControl: "Persist workflow state before each critical step, implement recovery handler on startup, use two-phase commit",
	},

	"FM-OR-006": {
		ID:      "FM-OR-006",
		Name:    "Race Condition in Workflow Lock",
		Domain:  DomainOrchestration,
		Category: CategoryOperational,
		Description: "Two concurrent workflows attempt to process same order simultaneously (e.g., webhook callback + polling). " +
			"Both proceed as if they have exclusive lock; state corrupted.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-OP-004: Data Integrity (concurrent access control)",
		ExampleScenario: "Payment webhook callback and polling timer both fire; both call ExecuteWorkflow(orderID), both update state without mutual exclusion",
		RootCausePatterns: []string{
			"no mutex lock in ExecuteWorkflow()",
			"distributed lock (e.g., Redis) not implemented",
			"webhook + polling race window",
		},
		SystemImpact: "Order state corrupted, settlement applied twice or conflicted, audit trail shows race damage",
		LinkedRubrics: []string{"RB-OR-007", "RB-SE-001"},
		RecoveryPath:  "Detect via reconciliation mismatch; manually inspect state, undo corrupted transitions, replay in correct order",
		PreventionControl: "Use mutex lock (o.mu) in ExecuteWorkflow(), implement distributed lock for multi-instance scenarios, use optimistic locking",
	},

	"FM-OR-007": {
		ID:      "FM-OR-007",
		Name:    "Missing Error Handler in Workflow",
		Domain:  DomainOrchestration,
		Category: CategoryOperational,
		Description: "Workflow encounters error (payment declined, settlement timeout) but error is ignored or not propagated. " +
			"Order proceeds as if success.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-OP-005: Operational Monitoring (error tracking)",
		ExampleScenario: "PaymentAgentStub returns error; ExecuteWorkflow doesn't check error, assumes success, initiates settlement anyway",
		RootCausePatterns: []string{
			"missing error check on agent response",
			"_ = ignoring error in handler",
			"error swallowed in goroutine without propagation",
		},
		SystemImpact: "Order proceeds without payment/settlement, customer not charged, revenue lost, settlement mismatch",
		LinkedRubrics: []string{"RB-OR-008"},
		RecoveryPath:  "Audit finds settlement without payment confirmation, flag order for reversal and refund",
		PreventionControl: "Check all error returns, log errors with context, propagate errors to caller, use structured error types",
	},

	"FM-OR-008": {
		ID:      "FM-OR-008",
		Name:    "Workflow Rollback Incomplete",
		Domain:  DomainOrchestration,
		Category: CategoryOperational,
		Description: "Payment succeeds but settlement fails. Rollback initiated but only partially completes (e.g., " +
			"ledger reversal succeeds but order status not rolled back). Inconsistent state.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-OP-006: Failure Recovery (rollback completeness)",
		ExampleScenario: "Settlement fails; ledger refund issued but order status not reset to StatusPaid; audit shows inconsistency",
		RootCausePatterns: []string{
			"rollback logic not transactional",
			"rollback itself fails without cascading undo",
			"missing compensating transaction",
		},
		SystemImpact: "Order state inconsistent with ledger, reconciliation fails, manual HITL required to resolve",
		LinkedRubrics: []string{"RB-OR-009"},
		RecoveryPath:  "Detect via reconciliation.IsReconciled(); apply compensating transaction to match order and ledger state",
		PreventionControl: "Implement transactional rollback with all-or-nothing semantics, log rollback steps, verify rollback success",
	},

	// === MERCHANT/CUSTOMER (MC) domain: 8 modes ===
	// Onboarding, disputes, identity, fraud

	"FM-MC-001": {
		ID:      "FM-MC-001",
		Name:    "Merchant Account Impersonation",
		Domain:  DomainMerchantCustomer,
		Category: CategorySecurity,
		Description: "Third party registers merchant account using stolen merchant identity documents or name spoofing. " +
			"Fraudster settles orders and withdraws funds.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-KYC-004: Merchant Identity (verification strength)",
		ExampleScenario: "Attacker uploads merchant PAN/GSTIN belonging to legitimate business; account approved, orders settled to fraudster bank account",
		RootCausePatterns: []string{
			"weak merchant document verification",
			"no business registration validation",
			"bank account ownership not confirmed",
		},
		SystemImpact: "Fraudster controls merchant account, legitimate merchant loses reputation, regulatory investigation",
		LinkedRubrics: []string{"RB-MC-001", "RB-CO-003"},
		RecoveryPath:  "Merchant reports fraud; suspend account, quarantine settlements, file police report, identify and block attacker",
		PreventionControl: "Verify merchant identity with government registries (GST, MCA), confirm bank account ownership via micro-deposit, use certified KYUA",
	},

	"FM-MC-002": {
		ID:      "FM-MC-002",
		Name:    "Customer Card Number Reuse (Multiple Accounts)",
		Domain:  DomainMerchantCustomer,
		Category: CategorySecurity,
		Description: "Same card linked to multiple customer accounts, allowing fraud (e.g., velocity bypass, chargebacks). " +
			"Account linking rule not enforced.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-FRAUD-001: Fraud Detection (duplicate payment instruments)",
		ExampleScenario: "Card ending in 1234 linked to Customer A and Customer B; both exceed velocity limits in aggregate but checked independently",
		RootCausePatterns: []string{
			"duplicate card check only per customer",
			"no aggregate velocity across linked accounts",
			"card linkage table not consulted",
		},
		SystemImpact: "Fraud activity across multiple accounts, velocity bypass, chargebacks, customer account takeover",
		LinkedRubrics: []string{"RB-MC-002", "RB-CO-001"},
		RecoveryPath:  "Fraud detection identifies card reuse; freeze linked accounts, investigate transaction pattern, contact legitimate account holders",
		PreventionControl: "Implement card-to-account graph, check all transactions by all accounts linked to card, enforce shared velocity limits",
	},

	"FM-MC-003": {
		ID:      "FM-MC-003",
		Name:    "Merchant Dispute Not Routed (Lost)",
		Domain:  DomainMerchantCustomer,
		Category: CategoryOperational,
		Description: "Customer disputes charge or merchant initiates refund dispute. Dispute message not routed to " +
			"dispute agent or loses message in queue. Dispute never resolved.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-DISP-001: Dispute Resolution (timely processing)",
		ExampleScenario: "Customer raises dispute in UI; message queued but never picked up by dispute agent due to queue crash",
		RootCausePatterns: []string{
			"message queue failure",
			"dispute handler crashed",
			"no retry/dead-letter queue for unprocessed disputes",
		},
		SystemImpact: "Dispute unresolved, customer chargeback filed, settlement reversed after grace period, merchant loses confidence",
		LinkedRubrics: []string{"RB-MC-003"},
		RecoveryPath:  "Monitoring detects unprocessed disputes; replay from queue, manually route to dispute agent, contact parties",
		PreventionControl: "Use reliable message queue with persistence, implement dispute handler with retry logic, monitor queue depth and latency",
	},

	"FM-MC-004": {
		ID:      "FM-MC-004",
		Name:    "Merchant Settlement Withdraw to Wrong Account",
		Domain:  DomainMerchantCustomer,
		Category: CategoryOperational,
		Description: "Merchant initiates settlement withdrawal but funds transferred to wrong account (e.g., incorrect IFSC, " +
			"account number not validated). Merchant loses funds to third party.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-PS-004: Payment Authorization (beneficiary validation)",
		ExampleScenario: "Merchant submits settlement withdraw with typo in account number; funds sent to random account, non-recoverable",
		RootCausePatterns: []string{
			"no IFSC/account validation before transfer",
			"account name mismatch not checked",
			"no confirmation step for withdraw request",
		},
		SystemImpact: "Merchant funds lost, regulatory complaint, potential liability claim against platform",
		LinkedRubrics: []string{"RB-MC-004"},
		RecoveryPath:  "Detect via merchant complaint; attempt to recover from beneficiary bank, if unsuccessful, file incident, may require loss absorption",
		PreventionControl: "Validate IFSC/account combination via NEFT registry, verify account name matches merchant, require OTP/2FA confirmation, trial transfer",
	},

	"FM-MC-005": {
		ID:      "FM-MC-005",
		Name:    "Customer Refund Never Issued",
		Domain:  DomainMerchantCustomer,
		Category: CategoryOperational,
		Description: "Order cancelled or payment declined but refund not processed (e.g., refund workflow not triggered, " +
			"refund agent down). Customer balance not credited.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-PS-005: Refund Processing (timely refund)",
		ExampleScenario: "Order payment fails; refund initiated but refund agent timeout; settlement shows debit but no credit, customer account stuck",
		RootCausePatterns: []string{
			"refund not triggered on payment failure",
			"refund agent unavailable",
			"no timeout/retry on refund request",
		},
		SystemImpact: "Customer funds trapped, complaint escalation, potential fine for delayed refund (RBI max 7 days)",
		LinkedRubrics: []string{"RB-MC-005", "RB-OR-005"},
		RecoveryPath:  "Monitor refund SLA; detect unrefunded orders >7 days old, issue manual refund, investigate root cause",
		PreventionControl: "Trigger refund automatically on payment failure, implement refund timeout SLA (e.g., 24h), alert on overdue refunds",
	},

	"FM-MC-006": {
		ID:      "FM-MC-006",
		Name:    "Chargeback Not Recorded in Lineage",
		Domain:  DomainMerchantCustomer,
		Category: CategoryCompliance,
		Description: "Customer issues chargeback (card network/bank reversal). Chargeback processed by bank but not " +
			"recorded in lineage or reflected in settlement reconciliation.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AUD-003: Chargeback Tracking (settlement completeness)",
		ExampleScenario: "Customer card chargeback processed by bank; settlement shows merchant received funds but chargeback not in audit trail",
		RootCausePatterns: []string{
			"no integration with card networks for chargeback notification",
			"chargeback notification received but not logged",
			"settlement not adjusted for chargeback",
		},
		SystemImpact: "Settlement appears final but merchant may owe funds due to chargeback; unreconciled discrepancy",
		LinkedRubrics: []string{"RB-LA-010"},
		RecoveryPath:  "Bank or card network notifies platform of chargeback; create lineage entry, reverse settlement, adjust merchant balance",
		PreventionControl: "Subscribe to card network chargeback notifications, log all chargebacks to lineage, adjust settlement in real-time, monitor for net reversals",
	},

	"FM-MC-007": {
		ID:      "FM-MC-007",
		Name:    "Merchant Commission Calculation Error",
		Domain:  DomainMerchantCustomer,
		Category: CategoryOperational,
		Description: "Platform commission/fees calculated incorrectly for merchant settlement (e.g., percentage applied to wrong amount, " +
			"rounding error). Merchant receives more/less than owed.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-FS-001: Financial Stability (settlement accuracy)",
		ExampleScenario: "Merchant settlement of ₹10,000 with 2% commission; commission calculated as ₹200 on ₹10k instead of ₹100 on ₹5k referral",
		RootCausePatterns: []string{
			"commission percentage applied to gross instead of net",
			"rounding to nearest rupee instead of paise",
			"commission formula not validated",
		},
		SystemImpact: "Merchant payment incorrect, platform loses or gains money, audit discrepancy, merchant complaint",
		LinkedRubrics: []string{"RB-MC-006"},
		RecoveryPath:  "Audit detects calculation error; issue adjustment/credit to merchant, correct future calculations",
		PreventionControl: "Validate commission calculation against documented formula, test with known amounts, log commission details in lineage",
	},

	"FM-MC-008": {
		ID:      "FM-MC-008",
		Name:    "Customer Account Lockout Due to Failed Verification",
		Domain:  DomainMerchantCustomer,
		Category: CategoryAvailability,
		Description: "Customer permanently locked out of account due to failed verification (e.g., KYC re-verification, 2FA threshold). " +
			"No recovery path or escalation.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AVL-004: Account Accessibility (account recovery)",
		ExampleScenario: "Customer fails KYC re-verification 3x; account auto-locked with no appeal process, cannot order or access funds",
		RootCausePatterns: []string{
			"no account unlock path after threshold",
			"no HITL escalation for legitimate users",
			"no customer support override mechanism",
		},
		SystemImpact: "Customer unable to use platform, trapped funds, customer churn, regulatory complaint",
		LinkedRubrics: []string{"RB-MC-007"},
		RecoveryPath:  "Customer support escalation; verify customer identity through alternative means, unlock account, investigate false positive",
		PreventionControl: "Implement account lockout with manual unlock, escalate to HITL after 2 failures, provide customer appeal process",
	},

	// === AGENT BEHAVIOR (AB) domain: 6 modes ===
	// Hallucination, consistency, prompt injection, alignment

	"FM-AB-001": {
		ID:      "FM-AB-001",
		Name:    "Agent Hallucination in Settlement Amount",
		Domain:  DomainAgentBehavior,
		Category: CategorySecurity,
		Description: "LLM agent in settlement flow generates/hallucinates incorrect amount or transaction ID that doesn't " +
			"exist in ledger. Amount used for settlement is wrong.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-AI-001: AI Governance (hallucination prevention)",
		ExampleScenario: "Settlement agent queries order amount, LLM generates '₹50,000 paise' (should be '500 paise'); amount used in settlement",
		RootCausePatterns: []string{
			"LLM not grounded in deterministic order lookup",
			"amount not verified against order before settlement",
			"no constraint checking on generated amount",
		},
		SystemImpact: "Settlement amount wrong, merchant overpaid or underpaid, financial loss, regulatory concern",
		LinkedRubrics: []string{"RB-AB-001", "RB-AB-002"},
		RecoveryPath:  "Reconciliation detects amount mismatch; identify hallucinated vs. actual amount, issue adjustment",
		PreventionControl: "Agent must fetch order from database, not generate; validate amount against ledger before settling, use tool-use for amount retrieval",
	},

	"FM-AB-002": {
		ID:      "FM-AB-002",
		Name:    "Agent Prompt Injection (SQL/Command)",
		Domain:  DomainAgentBehavior,
		Category: CategorySecurity,
		Description: "Attacker crafts malicious order or user input that injects SQL/system command into agent prompt. " +
			"Agent executes unintended command (e.g., deletes orders, transfers funds).",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-SEC-001: Security Controls (prompt injection mitigation)",
		ExampleScenario: "Order description contains ''; DROP TABLE orders; --' ; agent parses and executes, deleting order history",
		RootCausePatterns: []string{
			"user input concatenated into agent prompt without sanitization",
			"agent has direct database/CLI access",
			"no prompt validation or constraint enforcement",
		},
		SystemImpact: "Data loss, unauthorized transactions, system compromise, regulatory breach",
		LinkedRubrics: []string{"RB-AB-003", "RB-AB-004"},
		RecoveryPath:  "Detect via audit log of unexpected operations; restore from backup, investigate attacker, secure system",
		PreventionControl: "Never concatenate user input into prompts, use parameterized agent instructions, sandbox agent execution, validate tool inputs",
	},

	"FM-AB-003": {
		ID:      "FM-AB-003",
		Name:    "Agent Consistency Drift (Cross-Turn)",
		Domain:  DomainAgentBehavior,
		Category: CategoryOperational,
		Description: "Multi-turn agent conversation where agent's facts or decisions change between turns (e.g., " +
			"customer risk score evaluated as 'low' in turn 1 but 'high' in turn 2, same customer). Inconsistent settlement decision.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AI-002: AI Reliability (consistent decision-making)",
		ExampleScenario: "Settlement agent evaluates customer risk in turn 1 ('low-risk, approved'), turn 2 same customer ('high-risk, block')",
		RootCausePatterns: []string{
			"context not persisted between turns",
			"LLM temperature too high (non-deterministic)",
			"grounding data changed between turns without notification",
		},
		SystemImpact: "Settlement decisions inconsistent, audit trail shows contradictory assessments, regulatory concern",
		LinkedRubrics: []string{"RB-AB-005"},
		RecoveryPath:  "Audit detects inconsistency; use earlier (more conservative) decision, flag for manual review",
		PreventionControl: "Persist context across turns, use temperature=0 for deterministic responses, log grounding data snapshot per turn",
	},

	"FM-AB-004": {
		ID:      "FM-AB-004",
		Name:    "Agent Tool Misuse (Wrong Agent for Task)",
		Domain:  DomainAgentBehavior,
		Category: CategoryOperational,
		Description: "Agent selects wrong tool/subagent for task (e.g., settlement agent uses fraud-detection agent instead of " +
			"compliance check). Tool returns irrelevant data, decision made on bad info.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-OP-007: Operational Control (agent delegation)",
		ExampleScenario: "Settlement agent tries to verify merchant identity using fraud-detection tool (wrong tool); gets fraud score instead of KYC status",
		RootCausePatterns: []string{
			"agent tool description unclear or overlapping",
			"agent confused about which tool solves which problem",
			"tool registry not organized by domain",
		},
		SystemImpact: "Wrong information used for settlement decision, potential compliance violation, inefficient process",
		LinkedRubrics: []string{"RB-AB-006"},
		RecoveryPath:  "Audit detects wrong tool usage; re-evaluate decision with correct tool, correct if needed",
		PreventionControl: "Define clear tool descriptions with examples, use tool selection validation, test agent tool chains",
	},

	"FM-AB-005": {
		ID:      "FM-AB-005",
		Name:    "Agent Refusal Without Clear Reason",
		Domain:  DomainAgentBehavior,
		Category: CategoryOperational,
		Description: "Agent refuses to process legitimate order (e.g., settlement agent refusal to settle valid batch) " +
			"without clear error explanation. Order stuck.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AVL-005: System Availability (unblocked processing)",
		ExampleScenario: "Settlement agent receives valid batch but LLM interprets batch amount as 'suspicious' and refuses without logging reason",
		RootCausePatterns: []string{
			"LLM refusal due to misaligned safety guardrails",
			"refusal reason not logged",
			"no override mechanism for legitimate but unusual cases",
		},
		SystemImpact: "Legitimate settlement blocked, merchants unpaid, settlement SLA breached",
		LinkedRubrics: []string{"RB-AB-007"},
		RecoveryPath:  "Monitoring detects refusal; add context to agent prompt or override with HITL approval, escalate to model tuning",
		PreventionControl: "Log all agent refusals with reason, tune agent guardrails to avoid false positives, implement HITL override for refusals",
	},

	"FM-AB-006": {
		ID:      "FM-AB-006",
		Name:    "Agent Alignment Drift (Value Misalignment)",
		Domain:  DomainAgentBehavior,
		Category: CategorySecurity,
		Description: "Agent's decision-making drifts from intended business logic (e.g., agent prioritizes speed over compliance, " +
			"approves high-risk transactions). Agent behavior misaligned with RBI guidelines.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-AI-003: AI Alignment (regulatory compliance)",
		ExampleScenario: "Compliance agent increasingly approves high-risk customers to meet settlement SLA targets, ignoring AML rules",
		RootCausePatterns: []string{
			"agent fine-tuned on incorrect reward signal",
			"prompt instructions prioritize speed over compliance",
			"no alignment monitoring or auditing",
		},
		SystemImpact: "Regulatory violations, money laundering, customer sanction evasion, license risk",
		LinkedRubrics: []string{"RB-AB-008"},
		RecoveryPath:  "Compliance audit detects drift; retrain agent on correct objectives, re-evaluate past decisions",
		PreventionControl: "Define explicit agent objectives aligned with RBI guidelines, monitor agent decisions against policy, regular alignment audits",
	},

	// === LINEAGE/AUDIT (LA) domain: 6 modes ===
	// Transaction tracking, immutability, proof chains

	"FM-LA-001": {
		ID:      "FM-LA-001",
		Name:    "Lineage Hash Chain Broken",
		Domain:  DomainLineageAudit,
		Category: CategoryCompliance,
		Description: "Settlement lineage recorded but hash chain integrity broken (e.g., entry modified or deleted, " +
			"hash not re-computed). Audit trail no longer cryptographically verifiable.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-AUD-004: Audit Integrity (immutability)",
		ExampleScenario: "Lineage entry for settlement modified (amount changed); hash not updated; verification returns false",
		RootCausePatterns: []string{
			"lineage entry updated without hash re-computation",
			"hash chain not verified on read",
			"no permission control on lineage modification",
		},
		SystemImpact: "Audit trail invalid, regulatory inspection failure, cannot prove transaction occurred as recorded",
		LinkedRubrics: []string{"RB-LA-001", "RB-LA-002"},
		RecoveryPath:  "Detect via lineage.Verify(ctx) returning broken chain; restore from backup, investigate unauthorized modification",
		PreventionControl: "Enforce immutable lineage writes, verify hash chain on every read, use cryptographic signatures, restrict lineage modification permissions",
	},

	"FM-LA-002": {
		ID:      "FM-LA-002",
		Name:    "Audit Entry Missing Required Fields",
		Domain:  DomainLineageAudit,
		Category: CategoryCompliance,
		Description: "Settlement audit entry created but missing critical fields (e.g., no amount, no timestamp, no error code). " +
			"Later audit cannot reconstruct what happened.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AUD-005: Audit Completeness (full transaction context)",
		ExampleScenario: "AuditEntry for settlement has ID and OrderID but no Input/Output amounts; cannot verify settlement correctness",
		RootCausePatterns: []string{
			"audit logging code skips optional fields",
			"no schema validation on audit entry",
			"error case logging not complete",
		},
		SystemImpact: "Audit trail incomplete, cannot prove settlement correctness, regulatory audit incomplete",
		LinkedRubrics: []string{"RB-LA-003"},
		RecoveryPath:  "Reconstruct missing fields from order/ledger history if possible; otherwise flag for manual review",
		PreventionControl: "Enforce required fields in AuditEntry struct, validate on write, log comprehensive context (amount, parties, status, error)",
	},

	"FM-LA-003": {
		ID:      "FM-LA-003",
		Name:    "Lineage Timestamp Manipulation",
		Domain:  DomainLineageAudit,
		Category: CategoryCompliance,
		Description: "Audit entry timestamp modified (e.g., backdated to hide late settlement or make transaction appear earlier). " +
			"Regulatory timeline audit manipulated.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-AUD-006: Audit Timeline (accurate sequencing)",
		ExampleScenario: "Settlement completed at 17:00 but lineage entry backdated to 09:00 to appear within settlement SLA",
		RootCausePatterns: []string{
			"system clock skew or manual clock adjustment",
			"lineage entry timestamp not server-generated",
			"no audit of timestamp modifications",
		},
		SystemImpact: "Settlement SLA audit invalid, regulatory timeline untrustworthy, compliance audit failure",
		LinkedRubrics: []string{"RB-LA-004"},
		RecoveryPath:  "Detect via cross-check with blockchain/external ledger timestamp; correct timeline, investigate manipulation",
		PreventionControl: "Generate timestamp server-side using atomic clock, make timestamp immutable after creation, audit timestamp changes",
	},

	"FM-LA-004": {
		ID:      "FM-LA-004",
		Name:    "Settlement Order-Ledger-Lineage Mismatch",
		Domain:  DomainLineageAudit,
		Category: CategoryCompliance,
		Description: "Order, ledger, and lineage records show contradictory information about settlement (e.g., order shows fulfilled, " +
			"ledger shows pending, lineage shows cancelled). Impossible to trust any record.",
		Severity:    SeverityCritical,
		RBIConcern:  "RBI-AUD-007: Data Consistency (cross-system reconciliation)",
		ExampleScenario: "Order StatusFulfilled, but lineage shows StepSettlementFailed, ledger shows ledger.Pending; reconciliation fails",
		RootCausePatterns: []string{
			"order status and ledger not updated atomically",
			"lineage entry not created synchronously with ledger",
			"rollback logic not coordinated across systems",
		},
		SystemImpact: "Cannot determine true settlement state, regulatory audit impossible, settlement integrity questioned",
		LinkedRubrics: []string{"RB-LA-005", "RB-LA-006"},
		RecoveryPath:  "Query all three sources (order, ledger, lineage); use lineage as source of truth, correct order/ledger to match",
		PreventionControl: "Use two-phase commit to atomically update order+ledger+lineage, implement reconciliation.IsReconciled() check, log all updates",
	},

	"FM-LA-005": {
		ID:      "FM-LA-005",
		Name:    "Lineage Query Response Tampering",
		Domain:  DomainLineageAudit,
		Category: CategorySecurity,
		Description: "Attacker intercepts lineage query response and modifies data (e.g., changes settlement amount) " +
			"before returning to auditor. Auditor sees false settlement record.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-SEC-002: Data Security (MITM protection)",
		ExampleScenario: "Auditor queries lineage for settlement; attacker MITM intercepts and changes amount from 500 to 5000 paise",
		RootCausePatterns: []string{
			"lineage API response not signed",
			"no TLS/encryption on lineage queries",
			"audit queries over unencrypted channel",
		},
		SystemImpact: "Audit trail forged, regulatory audit untrustworthy, settlement history falsified",
		LinkedRubrics: []string{"RB-LA-007"},
		RecoveryPath:  "Detect via signature verification failure; query backup ledger, compare hashes, identify tampering point",
		PreventionControl: "Sign lineage responses with immutable keys, enforce TLS on all lineage queries, validate signatures on read",
	},

	"FM-LA-006": {
		ID:      "FM-LA-006",
		Name:    "Lineage Record Deletion (Regulatory Erasure)",
		Domain:  DomainLineageAudit,
		Category: CategoryCompliance,
		Description: "Lineage records deleted (legitimately for GDPR or fraudulently) but deletion not tracked. " +
			"Later audit cannot determine what was deleted or why.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-PRIV-001: Data Privacy (deletion audit trail)",
		ExampleScenario: "Customer requests GDPR erasure; settlement lineage deleted but no deletion log entry; auditor cannot see deletion occurred",
		RootCausePatterns: []string{
			"deletion not logged to audit trail",
			"no soft-delete mechanism (logical instead of physical)",
			"deletion reason not recorded",
		},
		SystemImpact: "Cannot audit data deletions, regulatory privacy compliance unverifiable, potential litigation risk",
		LinkedRubrics: []string{"RB-LA-008"},
		RecoveryPath:  "Implement immutable deletion log; query log to see what was deleted and when",
		PreventionControl: "Use soft deletes (logical deletion with timestamp), log all deletions with reason and approver, implement deletion audit trail",
	},

	// === ADDITIONAL MODES (to reach 38 total) ===
	// Distributed across domains based on risk/priority

	"FM-SE-009": {
		ID:      "FM-SE-009",
		Name:    "Settlement Round-Trip Verification Failure",
		Domain:  DomainSettlement,
		Category: CategoryOperational,
		Description: "Settlement amount committed to ledger but round-trip verification (ledger re-query) shows different balance. " +
			"Ledger corruption or concurrency bug.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-FS-003: Financial Stability (ledger consistency)",
		ExampleScenario: "Settlement commits ₹500, ledger confirms, but immediate query shows balance unchanged; double-check fails",
		RootCausePatterns: []string{
			"ledger write not synchronized with read",
			"read-your-own-write not guaranteed",
			"replication lag in distributed ledger",
		},
		SystemImpact: "Ledger state untrustworthy, settlement might not have actually occurred, reconciliation broken",
		LinkedRubrics: []string{"RB-SE-009"},
		RecoveryPath:  "Detect via round-trip verification; query backup ledger, enforce consistency, retry transaction",
		PreventionControl: "Use strongly consistent ledger with ACID guarantees, implement read-your-own-write check, test consistency properties",
	},

	"FM-CO-009": {
		ID:      "FM-CO-009",
		Name:    "Compliance Rule Change Mid-Transaction",
		Domain:  DomainCompliance,
		Category: CategoryCompliance,
		Description: "Compliance rule updated during transaction processing (e.g., velocity limit lowered). " +
			"Transaction started under old rule but completed under new rule. Inconsistent application.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-COMP-001: Compliance Rule Management (rule versioning)",
		ExampleScenario: "Velocity limit lowered from ₹100k to ₹50k mid-settlement; order approved under old rule but rejected under new rule mid-way",
		RootCausePatterns: []string{
			"no rule snapshot at transaction start",
			"rule change not versioned",
			"no enforcement of rule version consistency",
		},
		SystemImpact: "Rule application inconsistent, some transactions allowed under old rules, potential compliance violation",
		LinkedRubrics: []string{"RB-CO-010"},
		RecoveryPath:  "Detect via audit review; re-evaluate order under new rules, escalate if impact",
		PreventionControl: "Snapshot compliance rules at transaction start, use rule version in audit entry, enforce version consistency",
	},

	"FM-OR-009": {
		ID:      "FM-OR-009",
		Name:    "Workflow Deadlock (Circular Dependency)",
		Domain:  DomainOrchestration,
		Category: CategoryAvailability,
		Description: "Workflow enters deadlock where multiple steps wait for each other (e.g., settlement waits for payment confirmation " +
			"but payment status check waits for settlement to start). Workflow hangs permanently.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-AVL-006: Liveness (no deadlock)",
		ExampleScenario: "Settlement step waits for PaymentConfirmed, but payment polling waits for settlement initiation; neither proceeds",
		RootCausePatterns: []string{
			"circular dependency in step prerequisites",
			"no timeout to break deadlock",
			"no deadlock detection algorithm",
		},
		SystemImpact: "Workflow hangs indefinitely, orders not processed, settlement SLA breached",
		LinkedRubrics: []string{"RB-OR-010"},
		RecoveryPath:  "Monitor timeout detects hang; forcefully advance one step, break deadlock, continue workflow",
		PreventionControl: "Design workflow as DAG (no cycles), implement deadlock timeout, use fallback/skip logic for optional dependencies",
	},

	"FM-MC-009": {
		ID:      "FM-MC-009",
		Name:    "Customer KYC Expiry Not Enforced",
		Domain:  DomainMerchantCustomer,
		Category: CategoryCompliance,
		Description: "Customer KYC verification expired (e.g., 1 year old per RBI rules) but not re-verified. " +
			"Customer continues to transact with expired KYC. Settlement proceeds.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-KYC-005: KYC Periodic Re-verification (expiry enforcement)",
		ExampleScenario: "Customer KYC verified 13 months ago; RBI requires re-verification annually; but order settled without KYC renewal check",
		RootCausePatterns: []string{
			"KYC expiry date not checked before order",
			"no trigger for KYC renewal workflow",
			"KYC status not marked expired",
		},
		SystemImpact: "Regulatory violation (stale KYC), customer identity unverified for recent transaction, compliance breach",
		LinkedRubrics: []string{"RB-CO-011"},
		RecoveryPath:  "Detect via KYC expiry audit; flag customer for re-verification, hold settlements until renewed, notify customer",
		PreventionControl: "Check KYC expiry before order/settlement, trigger re-verification workflow at 11-month mark, block orders for expired KYC",
	},

	"FM-AB-007": {
		ID:      "FM-AB-007",
		Name:    "Agent Context Window Overflow",
		Domain:  DomainAgentBehavior,
		Category: CategoryAvailability,
		Description: "Agent prompt + context exceeds model context window limit. Agent response truncated or error returned. " +
			"Decision based on incomplete information.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AI-004: AI System Reliability (graceful degradation)",
		ExampleScenario: "Settlement agent with 500k token history of order details; context window 4k tokens; settlement metadata dropped",
		RootCausePatterns: []string{
			"context history not truncated",
			"no context pruning strategy",
			"large order details in context without summarization",
		},
		SystemImpact: "Decision made on incomplete history, potential duplicate or wrong settlement decision",
		LinkedRubrics: []string{"RB-AB-009"},
		RecoveryPath:  "Detect via model error; re-query with summarized context, or escalate to deterministic path",
		PreventionControl: "Implement context pruning (keep recent N turns), summarize old context, use smaller context window models for high-volume tasks",
	},

	"FM-LA-007": {
		ID:      "FM-LA-007",
		Name:    "Lineage Privacy Data Leakage",
		Domain:  DomainLineageAudit,
		Category: CategorySecurity,
		Description: "Sensitive lineage data (customer name, account number, card details) logged in audit entry without encryption. " +
			"Data accessible to unauthorized parties via audit API.",
		Severity:    SeverityHigh,
		RBIConcern:  "RBI-DATA-001: Data Privacy (sensitive data protection)",
		ExampleScenario: "AuditEntry.Input contains full customer SSN and card details; auditor API accessible by app staff, data exposed",
		RootCausePatterns: []string{
			"lineage entries not encrypted",
			"PII not redacted from audit logs",
			"audit API not access-controlled",
		},
		SystemImpact: "Privacy breach, customer data leak, regulatory fine, liability",
		LinkedRubrics: []string{"RB-LA-009"},
		RecoveryPath:  "Encrypt or redact sensitive fields in lineage, restrict audit access, notify affected customers",
		PreventionControl: "Encrypt lineage at rest, redact PII from audit entries (log only last 4 digits), enforce RBAC on audit API, encrypt in transit",
	},

	"FM-SE-010": {
		ID:      "FM-SE-010",
		Name:    "Settlement Retry Loop Infinite",
		Domain:  DomainSettlement,
		Category: CategoryAvailability,
		Description: "Settlement fails and retry logic loops infinitely without exponential backoff or max retry limit. " +
			"Resource exhaustion and log overflow.",
		Severity:    SeverityMedium,
		RBIConcern:  "RBI-AVL-007: Resource Management (finite retries)",
		ExampleScenario: "Settlement timeout triggers retry; retry also times out; no max retry check; loop continues consuming resources",
		RootCausePatterns: []string{
			"no max retry counter",
			"no exponential backoff",
			"retry triggered for permanent errors (not transient)",
		},
		SystemImpact: "Resource exhaustion, log storage overflow, system slow-down, other orders blocked",
		LinkedRubrics: []string{"RB-OR-011"},
		RecoveryPath:  "Monitor retry loop; kill retry loop, escalate to HITL, fix underlying settlement cause",
		PreventionControl: "Implement max retries (e.g., 3), exponential backoff, classify errors (transient vs. permanent), only retry transient",
	},
}

// FailureModeRegistry interface for accessing failure modes
type FailureModeRegistry interface {
	Get(id string) *FailureMode
	ListByDomain(domain FailureModeDomain) []*FailureMode
	ListBySeverity(severity FailureModeSeverity) []*FailureMode
	ListByCategory(category FailureModeCategory) []*FailureMode
	All() []*FailureMode
}

// DefaultFailureModeRegistry implements FailureModeRegistry
type DefaultFailureModeRegistry struct{}

// Get retrieves a failure mode by ID
func (r *DefaultFailureModeRegistry) Get(id string) *FailureMode {
	return AllFailureModes[id]
}

// ListByDomain returns all failure modes in a domain
func (r *DefaultFailureModeRegistry) ListByDomain(domain FailureModeDomain) []*FailureMode {
	var modes []*FailureMode
	for _, fm := range AllFailureModes {
		if fm.Domain == domain {
			modes = append(modes, fm)
		}
	}
	return modes
}

// ListBySeverity returns all failure modes at a severity level
func (r *DefaultFailureModeRegistry) ListBySeverity(severity FailureModeSeverity) []*FailureMode {
	var modes []*FailureMode
	for _, fm := range AllFailureModes {
		if fm.Severity == severity {
			modes = append(modes, fm)
		}
	}
	return modes
}

// ListByCategory returns all failure modes in a category
func (r *DefaultFailureModeRegistry) ListByCategory(category FailureModeCategory) []*FailureMode {
	var modes []*FailureMode
	for _, fm := range AllFailureModes {
		if fm.Category == category {
			modes = append(modes, fm)
		}
	}
	return modes
}

// All returns all failure modes
func (r *DefaultFailureModeRegistry) All() []*FailureMode {
	var modes []*FailureMode
	for _, fm := range AllFailureModes {
		modes = append(modes, fm)
	}
	return modes
}

// NewFailureModeRegistry creates a new registry
func NewFailureModeRegistry() FailureModeRegistry {
	return &DefaultFailureModeRegistry{}
}
