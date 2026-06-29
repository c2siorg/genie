package eval

import (
	"time"
)

// RubricDimension represents a measurable aspect of system behavior
type RubricDimension string

const (
	DimensionDetectability   RubricDimension = "detectability"   // can the failure be detected?
	DimensionSeverity        RubricDimension = "severity"        // impact of failure
	DimensionRecovery        RubricDimension = "recovery"        // can system recover?
	DimensionPrevention      RubricDimension = "prevention"      // can failure be prevented?
	DimensionAuditability    RubricDimension = "auditability"    // can failure be audited?
	DimensionIDempotency     RubricDimension = "idempotency"     // is operation idempotent?
	DimensionAtomicity       RubricDimension = "atomicity"       // is operation all-or-nothing?
	DimensionConsistency     RubricDimension = "consistency"     // are multiple sources consistent?
	DimensionDataIntegrity   RubricDimension = "data_integrity"  // is data uncorrupted?
	DimensionComplianceAlign RubricDimension = "compliance_align" // aligned with RBI rules?
)

// RubricLevel represents evaluation score levels
type RubricLevel string

const (
	LevelFailing      RubricLevel = "failing"       // 0-20%
	LevelPoor         RubricLevel = "poor"          // 21-40%
	LevelFair         RubricLevel = "fair"          // 41-60%
	LevelGood         RubricLevel = "good"          // 61-80%
	LevelExcellent    RubricLevel = "excellent"     // 81-100%
)

// RubricScore represents a single evaluation result
type RubricScore struct {
	Dimension   RubricDimension `json:"dimension"`
	Level       RubricLevel     `json:"level"`
	Score       int             `json:"score"`      // 0-100
	Evidence    string          `json:"evidence"`   // observed evidence
	Timestamp   time.Time       `json:"timestamp"`
}

// Rubric defines evaluation criteria for a failure mode or system property
type Rubric struct {
	// ID unique identifier (RB-DOMAIN-NNN)
	ID string `json:"id"`
	// Name human-readable rubric title
	Name string `json:"name"`
	// Description what this rubric measures
	Description string `json:"description"`
	// Domain which system area this rubric covers
	Domain FailureModeDomain `json:"domain"`
	// PrimaryDimension main aspect being evaluated
	PrimaryDimension RubricDimension `json:"primary_dimension"`
	// Criteria specific evaluation criteria (text description)
	Criteria []string `json:"criteria"`
	// ScoringGuideline how to score different performance levels
	ScoringGuideline string `json:"scoring_guideline"`
	// LevelDescriptions descriptions of each level (failing/poor/fair/good/excellent)
	LevelDescriptions map[RubricLevel]string `json:"level_descriptions"`
	// Evidence what kind of evidence demonstrates this rubric (test results, logs, queries)
	Evidence []string `json:"evidence"`
	// RelatedFailureModes which failure modes this rubric evaluates
	RelatedFailureModes []string `json:"related_failure_modes"`
	// TestProcedure how to test this rubric
	TestProcedure string `json:"test_procedure"`
	// AcceptanceCriteria what score level indicates passing
	AcceptanceCriteria RubricLevel `json:"acceptance_criteria"`
}

// AllRubrics registry of 18 evaluation rubrics
var AllRubrics = map[string]*Rubric{
	// === SETTLEMENT (SE) Rubrics: 8 ===

	"RB-SE-001": {
		ID:   "RB-SE-001",
		Name: "Double-Spend Prevention (Ledger Idempotency)",
		Description: "Evaluates whether CBDC ledger enforces transaction idempotency to prevent " +
			"same token from being spent twice. Tests concurrent payment confirmations.",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionIDempotency,
		Criteria: []string{
			"Concurrent payment requests for same order result in single ledger entry",
			"Duplicate payment with same OrderID is detected and rejected",
			"Idempotency key validation prevents double-spend",
			"Ledger commit is atomic and all-or-nothing",
		},
		ScoringGuideline: "Failing: concurrent payments both succeed. Poor: one succeeds after delay. Fair: duplicate detected but not prevented. Good: duplicate prevented. Excellent: prevented + alert + audit trail.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Concurrent payments result in double-spend (both succeed)",
			LevelPoor:      "Double-spend detected only after commit",
			LevelFair:      "Duplicate rejected with error but race window exists",
			LevelGood:      "Idempotency enforced; duplicates rejected immediately",
			LevelExcellent: "Idempotency enforced with alert + detailed audit trail",
		},
		Evidence: []string{
			"reconciliation.IsReconciled() result",
			"ledger balance history for customer",
			"audit trail showing payment attempts",
			"concurrent test execution logs",
		},
		RelatedFailureModes: []string{"FM-SE-001", "FM-OR-004"},
		TestProcedure: "Execute two concurrent payment requests for same order; verify ledger shows single entry; check customer balance",
		AcceptanceCriteria: LevelGood,
	},

	"RB-SE-002": {
		ID:   "RB-SE-002",
		Name: "Ledger Transaction Consistency",
		Description: "Evaluates whether ledger maintains ACID properties (atomicity, consistency, " +
			"isolation, durability) for settlement transactions.",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionConsistency,
		Criteria: []string{
			"Settlement amount matches order amount exactly",
			"All-or-nothing semantics: settlement either fully commits or fully rolls back",
			"No partial commits visible to auditors",
			"Read-your-own-write consistency after commit",
		},
		ScoringGuideline: "Failing: inconsistent reads, partial commits visible. Poor: eventual consistency delays. Fair: mostly consistent. Good: ACID compliant. Excellent: strong consistency + replication.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Inconsistent reads; settlement amount wrong or partial",
			LevelPoor:      "Eventual consistency but >1s latency to consistency",
			LevelFair:      "Consistent within 500ms",
			LevelGood:      "Strong consistency; all reads see committed value",
			LevelExcellent: "Strong consistency with replicated confirmation from backup ledger",
		},
		Evidence: []string{
			"ledger.CommitTransaction() success/failure code",
			"round-trip verification (write then immediate read)",
			"settlement.SettlementResult.Success flag",
			"ledger backup replica consistency check",
		},
		RelatedFailureModes: []string{"FM-SE-002", "FM-SE-004", "FM-SE-009"},
		TestProcedure: "Commit settlement; immediately query ledger; verify amount matches; check backup ledger",
		AcceptanceCriteria: LevelGood,
	},

	"RB-SE-003": {
		ID:   "RB-SE-003",
		Name: "Settlement Batch Composition Correctness",
		Description: "Evaluates whether settlement batch contains exactly the intended orders " +
			"(correct merchants, correct amounts, no duplicates).",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"Batch sum equals sum of individual orders",
			"Batch contains no duplicate orders",
			"Batch merchant grouping deterministic (sorted consistently)",
			"Batch does not include rolled-back or cancelled orders",
		},
		ScoringGuideline: "Failing: batch sum wrong or includes wrong merchants. Poor: minor discrepancies. Fair: correct composition but not auditable. Good: correct and auditable. Excellent: correct + immutable proof.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Batch sum mismatches order sums significantly (>1%)",
			LevelPoor:      "Minor discrepancy (<1%) or includes one rolled-back order",
			LevelFair:      "Correct composition but ordering or grouping not deterministic",
			LevelGood:      "Correct composition, deterministic grouping, no duplicates",
			LevelExcellent: "Correct + merkle tree proof + immutable commitment",
		},
		Evidence: []string{
			"ConsolidateSettlementBatch() output",
			"sum(batch orders) vs. batch.TotalPaise",
			"batch order list (no duplicates)",
			"merchant grouping sequence",
		},
		RelatedFailureModes: []string{"FM-SE-002"},
		TestProcedure: "Create 5 orders for 3 merchants; consolidate batch; verify sum and composition",
		AcceptanceCriteria: LevelGood,
	},

	"RB-SE-004": {
		ID:   "RB-SE-004",
		Name: "Netting Calculation Correctness",
		Description: "Evaluates whether netting calculations (bilateral/multilateral settlement) " +
			"produce correct net positions without overflow or precision loss.",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"Netting sum does not overflow or underflow integer bounds",
			"Rounding applied consistently (banker's rounding to paise)",
			"Negative and positive net positions correct",
			"Formula matches documented netting rule",
		},
		ScoringGuideline: "Failing: overflow or rounding error >0.5% of settlement. Poor: minor rounding error. Fair: correct but not formally verified. Good: correct and tested. Excellent: correct + formal proof.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Overflow/underflow or rounding error >1 paise",
			LevelPoor:      "Minor rounding error, accumulates over time",
			LevelFair:      "Correct for most cases but edge cases untested",
			LevelGood:      "Correct for all tested cases with comprehensive test coverage",
			LevelExcellent: "Correct + formal proof of netting algorithm",
		},
		Evidence: []string{
			"ApplyNettingRules() output",
			"netting sum vs. manual calculation",
			"integer bounds check (no overflow)",
			"rounding validation (paise precision)",
		},
		RelatedFailureModes: []string{"FM-SE-003"},
		TestProcedure: "Apply netting to 3 merchants with large amounts; verify sum and check for overflow; test edge cases",
		AcceptanceCriteria: LevelGood,
	},

	"RB-SE-005": {
		ID:   "RB-SE-005",
		Name: "Settlement Finality and Irreversibility",
		Description: "Evaluates whether settled transactions are final and irreversible except for " +
			"authorized reversal (chargeback, refund). Orders move to fulfilled atomically with ledger.",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionAtomicity,
		Criteria: []string{
			"Order status Fulfilled only after ledger commit confirms",
			"Settlement cannot be reversed without explicit authorization",
			"Time-locked settlement (no reversal after grace period)",
			"Lineage records finality event with timestamp",
		},
		ScoringGuideline: "Failing: order fulfilled before ledger confirms. Poor: finality delayed >30s. Fair: eventual finality. Good: immediate finality. Excellent: time-locked irreversibility.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Order fulfilled before ledger confirmation",
			LevelPoor:      "Finality delayed >30s after ledger commit",
			LevelFair:      "Eventual finality but ordering dependent",
			LevelGood:      "Finality immediate upon ledger commit",
			LevelExcellent: "Finality immediate + time-locked + cryptographic proof",
		},
		Evidence: []string{
			"order.PaidAt timestamp vs. ledger.CommitTransaction timestamp",
			"order.Status == StatusFulfilled check",
			"lineage entry with finality marker",
			"settlement reversal authorization audit",
		},
		RelatedFailureModes: []string{"FM-SE-004"},
		TestProcedure: "Settle order and immediately query status; verify order fulfilled only after ledger confirms",
		AcceptanceCriteria: LevelGood,
	},

	"RB-SE-006": {
		ID:   "RB-SE-006",
		Name: "Reconciliation Data Integrity",
		Description: "Evaluates whether settlement reconciliation (VerifyOrderSettlement, " +
			"IsReconciled) detects and reports mismatches between order, ledger, and lineage.",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"Reconciliation detects amount mismatch",
			"Reconciliation detects status mismatch (order fulfilled but ledger shows pending)",
			"Reconciliation detects missing settlement record",
			"Reconciliation returns false on any mismatch",
		},
		ScoringGuideline: "Failing: reconciliation passes despite known mismatch. Poor: detects some mismatches. Fair: detects most mismatches. Good: detects all mismatches. Excellent: detects + auto-corrects.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Reconciliation returns true despite order-ledger mismatch",
			LevelPoor:      "Detects amount mismatch but not status mismatch",
			LevelFair:      "Detects most mismatches; edge cases missed",
			LevelGood:      "Detects all mismatches and returns false",
			LevelExcellent: "Detects + returns detailed mismatch report + auto-correction proposal",
		},
		Evidence: []string{
			"reconciliation.IsReconciled() result",
			"reconciliation.VerifyOrderSettlement() output",
			"order status vs. ledger status",
			"reconciliation.ReconciliationCheck.BrokenAt field",
		},
		RelatedFailureModes: []string{"FM-SE-002", "FM-SE-008"},
		TestProcedure: "Create settlement mismatch (e.g., order fulfilled but ledger pending); run reconciliation; verify false result",
		AcceptanceCriteria: LevelGood,
	},

	"RB-SE-007": {
		ID:   "RB-SE-007",
		Name: "Currency Conversion Accuracy (FX)",
		Description: "Evaluates whether FX conversion (e.g., EUR to INR) applies current rates, " +
			"correct rounding, and records rate used for audit.",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"FX rate used is current (max 5 minutes old)",
			"Conversion formula correct and documented",
			"Rounding applied to paise (₹0.01) not rupee",
			"Conversion logged in audit with rate, timestamp, formula used",
		},
		ScoringGuideline: "Failing: stale FX rate or rounding error >1%. Poor: stale rate >60min. Fair: rate <15min old. Good: rate <5min old. Excellent: real-time rate + audit trail.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "FX rate >60min old or rounding error >1 paise",
			LevelPoor:      "FX rate 15-60min old",
			LevelFair:      "FX rate 5-15min old",
			LevelGood:      "FX rate <5min old with audit trail",
			LevelExcellent: "Real-time FX rate with cryptographic signature on audit entry",
		},
		Evidence: []string{
			"FX rate timestamp vs. conversion timestamp",
			"conversion formula in audit entry",
			"rounding method (banker's vs. truncation)",
			"converted amount vs. manual calculation",
		},
		RelatedFailureModes: []string{"FM-SE-006"},
		TestProcedure: "Convert EUR to INR; verify rate current; recalculate manually; compare results",
		AcceptanceCriteria: LevelGood,
	},

	"RB-SE-008": {
		ID:   "RB-SE-008",
		Name: "Settlement Timeout and Retry Resilience",
		Description: "Evaluates whether settlement handles timeouts gracefully with exponential backoff, " +
			"max retry limits, and proper error escalation.",
		Domain:           DomainSettlement,
		PrimaryDimension: DimensionRecovery,
		Criteria: []string{
			"Timeout SLA enforced (e.g., 5 minutes max)",
			"Retry uses exponential backoff (not constant)",
			"Max retry limit prevents infinite loops (e.g., max 3)",
			"Permanent errors not retried",
			"Escalation to HITL after max retries",
		},
		ScoringGuideline: "Failing: no timeout or infinite retry loop. Poor: timeout >30min or no backoff. Fair: timeout <10min. Good: timeout 5min + backoff. Excellent: timeout + backoff + HITL escalation.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "No timeout or infinite retry loop observed",
			LevelPoor:      "Timeout >30min or retry loop >5 attempts",
			LevelFair:      "Timeout 5-30min with basic retry",
			LevelGood:      "Timeout 5min + exponential backoff + max 3 retries",
			LevelExcellent: "Timeout 5min + exponential backoff + max 3 retries + HITL escalation + fallback to async queue",
		},
		Evidence: []string{
			"retry attempt count and timestamps",
			"backoff interval progression",
			"max retry limit enforcement",
			"HITL escalation trigger",
			"error classification (transient vs. permanent)",
		},
		RelatedFailureModes: []string{"FM-SE-007", "FM-SE-010"},
		TestProcedure: "Simulate ledger timeout; observe retry behavior; verify exponential backoff and max retry limit",
		AcceptanceCriteria: LevelGood,
	},

	// === COMPLIANCE (CO) Rubrics: 8 ===

	"RB-CO-001": {
		ID:   "RB-CO-001",
		Name: "Velocity Monitoring (AML Transaction Limits)",
		Description: "Evaluates whether AML velocity checks detect rapid transaction patterns and " +
			"enforce configured thresholds (e.g., max ₹50k in 1 hour).",
		Domain:           DomainCompliance,
		PrimaryDimension: DimensionDetectability,
		Criteria: []string{
			"Velocity window correctly calculated (e.g., last 1 hour)",
			"Real-time transaction sum checked against threshold",
			"Cache invalidated on each transaction (not stale)",
			"Concurrent transactions included in velocity calculation",
			"Alert fired when threshold approached (e.g., 80%)",
		},
		ScoringGuideline: "Failing: velocity check bypassed or cached. Poor: misses some transactions. Fair: detects most violations. Good: detects all in-window transactions. Excellent: detects + alert + detailed evidence.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Velocity check bypassed or false negative",
			LevelPoor:      "Misses concurrent transactions; cache outdated",
			LevelFair:      "Detects most violations; minor false positives",
			LevelGood:      "Detects all in-window transactions; accurate alerts",
			LevelExcellent: "Real-time detection + alert + HITL escalation with evidence",
		},
		Evidence: []string{
			"transaction history for customer in window",
			"velocity cache invalidation timestamps",
			"concurrent transaction handling",
			"alert fired when threshold reached",
		},
		RelatedFailureModes: []string{"FM-CO-001"},
		TestProcedure: "Submit 5 rapid orders >threshold; verify velocity check blocks 5th; check alert generation",
		AcceptanceCriteria: LevelGood,
	},

	"RB-CO-002": {
		ID:   "RB-CO-002",
		Name: "Sanctions Screening (FATF/OFAC Blacklist Matching)",
		Description: "Evaluates whether sanctions screening detects customer/merchant on blacklist " +
			"(FATF, OFAC, RBI) with fuzzy name matching and list currency.",
		Domain:           DomainCompliance,
		PrimaryDimension: DimensionDetectability,
		Criteria: []string{
			"Sanctions lists updated daily (max 24h stale)",
			"Name matching threshold appropriate (e.g., >0.95)",
			"Multiple FATF list sources checked",
			"Whitelist exceptions logged and auditable",
			"Screening result cached with TTL <4h",
		},
		ScoringGuideline: "Failing: stale list or high false negative. Poor: threshold >0.90 or list >24h old. Fair: threshold 0.85-0.90. Good: threshold >0.95 and daily update. Excellent: real-time update + evidence log.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "List >7 days old or threshold <0.80 (high false negatives)",
			LevelPoor:      "List 1-7 days old or threshold 0.80-0.90",
			LevelFair:      "List <24h old; threshold 0.90-0.95",
			LevelGood:      "List <24h old; threshold >0.95; multiple sources",
			LevelExcellent: "Real-time list update + threshold >0.95 + cryptographic audit trail",
		},
		Evidence: []string{
			"sanctions list update timestamp",
			"name matching score and threshold",
			"FATF list sources used",
			"whitelist exceptions logged",
			"screening result cache TTL",
		},
		RelatedFailureModes: []string{"FM-CO-002"},
		TestProcedure: "Screen known sanctioned customer; verify detection; check list age and matching threshold",
		AcceptanceCriteria: LevelGood,
	},

	"RB-CO-003": {
		ID:   "RB-CO-003",
		Name: "KYC Verification Enforcement",
		Description: "Evaluates whether KYC verification is enforced before account can transact " +
			"(order creation, settlement). Unverified accounts blocked.",
		Domain:           DomainCompliance,
		PrimaryDimension: DimensionDetectability,
		Criteria: []string{
			"Order creation blocked if merchant/customer unverified",
			"Settlement blocked if KYC pending",
			"KYC status checked at multiple stages (onboarding, order, settlement)",
			"Temporary account flag enforced consistently",
			"KYC expiry enforced (re-verification required)",
		},
		ScoringGuideline: "Failing: order placed with unverified account. Poor: KYC check missing from one stage. Fair: KYC checked but expiry not enforced. Good: KYC enforced at all stages. Excellent: enforced + auto-renewal.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Order/settlement allowed with unverified KYC",
			LevelPoor:      "KYC checked at order but not at settlement",
			LevelFair:      "KYC enforced at order/settlement but expiry not checked",
			LevelGood:      "KYC verified at order/settlement; expiry enforced",
			LevelExcellent: "KYC enforced + auto-renewal triggered at 11-month mark",
		},
		Evidence: []string{
			"merchant.KYCStatus before order creation",
			"customer.VerificationPending check",
			"order creation handler KYC validation",
			"settlement handler KYC validation",
			"KYC expiry check and renewal flow",
		},
		RelatedFailureModes: []string{"FM-CO-003"},
		TestProcedure: "Create order with unverified merchant; verify rejection; verify after KYC completion acceptance",
		AcceptanceCriteria: LevelGood,
	},

	"RB-CO-004": {
		ID:   "RB-CO-004",
		Name: "Consent Management (A2A/Data Sharing)",
		Description: "Evaluates whether explicit consent is obtained and recorded before accessing " +
			"customer data (e.g., account aggregator, payment history).",
		Domain:           DomainCompliance,
		PrimaryDimension: DimensionDetectability,
		Criteria: []string{
			"Consent obtained before data access (not retroactive)",
			"Consent scope documented (which data fields)",
			"Consent expiry enforced (re-consent required)",
			"All data accesses logged against consent registry",
			"Audit trail shows consent -> data access link",
		},
		ScoringGuideline: "Failing: data accessed without consent. Poor: consent obtained retroactively. Fair: consent enforced but not fully scoped. Good: consent verified before each access. Excellent: consent + immutable proof.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Data accessed without consent evidence",
			LevelPoor:      "Consent obtained after data access",
			LevelFair:      "Consent enforced but scope or expiry not checked",
			LevelGood:      "Consent verified before data access; scope and expiry checked",
			LevelExcellent: "Consent verified + immutable consent proof + cryptographic signature",
		},
		Evidence: []string{
			"consent.Verify() result before data access",
			"consent scope (fields listed)",
			"consent expiry timestamp",
			"audit log of data access with consent ID",
		},
		RelatedFailureModes: []string{"FM-CO-004"},
		TestProcedure: "Attempt to access customer data without consent; verify rejection; grant consent and retry; verify approval",
		AcceptanceCriteria: LevelGood,
	},

	"RB-CO-005": {
		ID:   "RB-CO-005",
		Name: "Regulatory Reporting Accuracy",
		Description: "Evaluates whether settlement data reported to regulators (RBI, FATF) is " +
			"accurate (correct amounts, no duplicates, excludes reversals).",
		Domain:           DomainCompliance,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"Reported settlement amount matches ledger",
			"Cancelled/reversed orders excluded from report",
			"Report data validated against settlement ledger before filing",
			"Report generation time window auditable",
			"No duplicate entries in report",
		},
		ScoringGuideline: "Failing: reported amount mismatches ledger. Poor: minor discrepancies or includes reversals. Fair: mostly accurate. Good: accurate with audit trail. Excellent: accurate + immutable proof.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Reported amount mismatches ledger >1%",
			LevelPoor:      "Minor discrepancy <1% or includes one reversal",
			LevelFair:      "Mostly accurate; time window not fully auditable",
			LevelGood:      "Accurate with full audit trail and validation",
			LevelExcellent: "Accurate + signed by independent auditor + cryptographic commitment",
		},
		Evidence: []string{
			"regulatory report data",
			"settlement ledger totals for report period",
			"reversed/cancelled orders excluded",
			"report generation timestamp",
			"validation log comparing report to ledger",
		},
		RelatedFailureModes: []string{"FM-CO-005"},
		TestProcedure: "Generate regulatory report; verify total against ledger; check for excluded reversals",
		AcceptanceCriteria: LevelGood,
	},

	"RB-CO-006": {
		ID:   "RB-CO-006",
		Name: "Customer Risk Scoring Accuracy",
		Description: "Evaluates whether customer risk scores (for AML/KYC decisions) are current, " +
			"accurate, and trigger timely escalation for high-risk profiles.",
		Domain:           DomainCompliance,
		PrimaryDimension: DimensionDetectability,
		Criteria: []string{
			"Risk score cache TTL <4h (max 4 hours old)",
			"PEP/sanctions designation triggers immediate score recalculation",
			"Risk score decision properly documented in audit",
			"High-risk threshold and escalation rules clear",
			"Score recalculation on any customer data update",
		},
		ScoringGuideline: "Failing: cache TTL >24h or PEP not triggering update. Poor: TTL 4-24h. Fair: TTL <4h but not recalc on data change. Good: TTL <4h + recalc on change. Excellent: real-time + alert.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Risk score cache TTL >24h or stale after PEP update",
			LevelPoor:      "TTL 4-24h or recalculation delayed",
			LevelFair:      "TTL <4h but not triggered on data updates",
			LevelGood:      "TTL <4h and recalc on any customer data change",
			LevelExcellent: "Real-time score with automatic PEP/sanctions alert",
		},
		Evidence: []string{
			"risk score timestamp vs. current time",
			"PEP list update timestamp",
			"score recalculation trigger events",
			"risk score decision audit entry",
			"high-risk escalation audit",
		},
		RelatedFailureModes: []string{"FM-CO-006"},
		TestProcedure: "Score customer; mark as PEP; verify score recalculates immediately; check high-risk escalation",
		AcceptanceCriteria: LevelGood,
	},

	"RB-CO-007": {
		ID:   "RB-CO-007",
		Name: "Compliance Decision Audit Trail Completeness",
		Description: "Evaluates whether compliance decisions (AML, KYC, sanctions) are fully logged " +
			"with decision reason, evidence, and review ability.",
		Domain:           DomainCompliance,
		PrimaryDimension: DimensionAuditability,
		Criteria: []string{
			"Compliance check type logged (AML velocity, sanctions, etc.)",
			"Decision reason documented (approved/blocked/escalated)",
			"Evidence logged (customer score, matching results, thresholds)",
			"Compliance officer review audit included",
			"Full context retrievable for regulatory inquiry",
		},
		ScoringGuideline: "Failing: no decision log. Poor: decision logged but not evidence. Fair: decision + partial evidence. Good: decision + evidence + reasoning. Excellent: good + immutable proof + cryptographic signature.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "No compliance decision logged",
			LevelPoor:      "Decision logged but without evidence or reasoning",
			LevelFair:      "Decision + partial evidence (e.g., score but not threshold)",
			LevelGood:      "Decision + full evidence + reasoning documented",
			LevelExcellent: "Decision + evidence + immutable audit trail + signed by officer",
		},
		Evidence: []string{
			"audit entry for compliance check",
			"check type (velocity, sanctions, KYC, etc.)",
			"decision (approve/block/escalate)",
			"evidence fields (scores, thresholds, matching results)",
			"officer review timestamp and signature",
		},
		RelatedFailureModes: []string{"FM-CO-007"},
		TestProcedure: "Run compliance check; query audit entry; verify all fields present and complete",
		AcceptanceCriteria: LevelGood,
	},

	// === ORCHESTRATION (OR) Rubrics: 4 ===

	"RB-OR-001": {
		ID:   "RB-OR-001",
		Name: "Workflow State Machine Validity",
		Description: "Evaluates whether order workflow only executes valid state transitions " +
			"(no impossible sequences like Cancelled -> Fulfilled).",
		Domain:           DomainOrchestration,
		PrimaryDimension: DimensionConsistency,
		Criteria: []string{
			"Transition rules enforce valid state graph",
			"Invalid transitions rejected with error",
			"All transitions logged with current state and next state",
			"State machine cannot skip required steps (e.g., payment before settlement)",
			"Lineage shows valid sequence only",
		},
		ScoringGuideline: "Failing: invalid transition allowed. Poor: detected but not prevented. Fair: prevented but logging incomplete. Good: prevented + logged. Excellent: prevented + validated + immutable.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Invalid transition allowed (e.g., Cancelled -> Fulfilled)",
			LevelPoor:      "Invalid transition detected but not prevented",
			LevelFair:      "Invalid transition prevented but logging incomplete",
			LevelGood:      "Invalid transition prevented + full audit log",
			LevelExcellent: "Prevented + immutable state proof + cryptographic signature",
		},
		Evidence: []string{
			"order.Status before/after transition",
			"lineage entry for state transition",
			"state machine rule validation",
			"rejection of invalid transition attempt",
		},
		RelatedFailureModes: []string{"FM-OR-001"},
		TestProcedure: "Attempt invalid transition (e.g., Cancelled -> Fulfilled); verify rejection; verify audit log",
		AcceptanceCriteria: LevelGood,
	},

	"RB-OR-002": {
		ID:   "RB-OR-002",
		Name: "Agent Communication Timeout Handling",
		Description: "Evaluates whether calls to payment/settlement agents have explicit timeouts " +
			"and failures are properly handled.",
		Domain:           DomainOrchestration,
		PrimaryDimension: DimensionRecovery,
		Criteria: []string{
			"Payment agent call has explicit timeout (e.g., 30s)",
			"Settlement agent call has explicit timeout",
			"Timeout error logged with context",
			"Workflow does not hang indefinitely",
			"Fallback/retry logic triggered on timeout",
		},
		ScoringGuideline: "Failing: no timeout or workflow hangs. Poor: timeout >30s. Fair: timeout <30s but no retry. Good: timeout + retry. Excellent: timeout + retry + HITL.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "No timeout; workflow hangs indefinitely",
			LevelPoor:      "Timeout >30s or timeout error not logged",
			LevelFair:      "Timeout 30s with basic logging",
			LevelGood:      "Timeout 30s + retry with exponential backoff",
			LevelExcellent: "Timeout 30s + retry + HITL escalation + async fallback",
		},
		Evidence: []string{
			"agent call timeout configuration",
			"timeout error log with timestamp",
			"workflow continuation after timeout",
			"retry attempt count",
		},
		RelatedFailureModes: []string{"FM-OR-002", "FM-SE-007"},
		TestProcedure: "Simulate agent timeout; verify workflow timeout within 30s; verify retry triggered",
		AcceptanceCriteria: LevelGood,
	},

	"RB-OR-003": {
		ID:   "RB-OR-003",
		Name: "Workflow Step Ordering and Dependencies",
		Description: "Evaluates whether workflow steps execute in correct order (payment before settlement, " +
			"settlement before fulfillment) with proper dependency checking.",
		Domain:           DomainOrchestration,
		PrimaryDimension: DimensionConsistency,
		Criteria: []string{
			"Step prerequisites checked before execution (e.g., payment confirmed before settlement)",
			"Steps cannot execute concurrently unless explicitly allowed",
			"Lineage shows steps in execution order",
			"Out-of-order execution rejected with error",
			"Retry restarts from failed step, not from beginning",
		},
		ScoringGuideline: "Failing: steps execute out of order. Poor: step ordering not validated. Fair: ordered but concurrent execution possible. Good: ordered + dependencies enforced. Excellent: ordered + dependencies + immutable proof.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Steps execute out of order (e.g., settlement before payment)",
			LevelPoor:      "Step ordering assumed but not validated",
			LevelFair:      "Ordering enforced but concurrent execution possible",
			LevelGood:      "Ordering enforced; dependencies validated; no concurrency",
			LevelExcellent: "Ordered + dependencies + immutable commitment + formal verification",
		},
		Evidence: []string{
			"workflow step sequence in lineage",
			"dependency check before step execution",
			"prerequisite validation error if missing",
			"step execution timestamps",
		},
		RelatedFailureModes: []string{"FM-OR-003"},
		TestProcedure: "Attempt settlement before payment; verify rejection; verify forced order after payment confirmed",
		AcceptanceCriteria: LevelGood,
	},

	"RB-OR-004": {
		ID:   "RB-OR-004",
		Name: "Workflow Idempotency and Retry Safety",
		Description: "Evaluates whether rerunning same workflow step (e.g., retry payment) " +
			"is safe (idempotent) or detects duplicate attempts.",
		Domain:           DomainOrchestration,
		PrimaryDimension: DimensionIDempotency,
		Criteria: []string{
			"Retry of failed payment check uses idempotency key",
			"Duplicate payment request rejected",
			"Workflow can be safely replayed without side effects",
			"Idempotency key validated at agent boundary",
			"Duplicate detection logged with original request ID",
		},
		ScoringGuideline: "Failing: duplicate payment allowed. Poor: detected only after commit. Fair: duplicate detected but logging incomplete. Good: duplicate prevented. Excellent: prevented + immutable proof.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Retry results in duplicate payment (charge twice)",
			LevelPoor:      "Duplicate detected after ledger commit",
			LevelFair:      "Duplicate detected before commit but logging incomplete",
			LevelGood:      "Duplicate prevented immediately with alert",
			LevelExcellent: "Duplicate prevented + immutable dedup proof + full audit trail",
		},
		Evidence: []string{
			"idempotency key in payment request",
			"duplicate request rejection",
			"payment request/response logged once",
			"deduplication cache hit log",
		},
		RelatedFailureModes: []string{"FM-OR-004"},
		TestProcedure: "Trigger payment retry; verify idempotency key used; attempt duplicate submission; verify rejection",
		AcceptanceCriteria: LevelGood,
	},

	// === LINEAGE/AUDIT (LA) Rubrics: 4 ===

	"RB-LA-001": {
		ID:   "RB-LA-001",
		Name: "Audit Trail Immutability and Hash Chain",
		Description: "Evaluates whether lineage entries are immutable and linked by hash chain " +
			"(each entry commits hash of previous entry).",
		Domain:           DomainLineageAudit,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"Lineage entries stored immutably (no update/delete post-creation)",
			"Hash chain computed: H(n) = SHA256(H(n-1) || data(n))",
			"Hash chain verified on read with lineage.Verify(ctx)",
			"Any modification detected as broken chain",
			"Audit entries cryptographically signed",
		},
		ScoringGuideline: "Failing: lineage modified after creation. Poor: hash chain not validated on read. Fair: hash chain validated. Good: hash chain + signed. Excellent: hash chain + blockchain.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Lineage entry modified after creation",
			LevelPoor:      "Hash chain computed but not validated on read",
			LevelFair:      "Hash chain validated; no cryptographic signature",
			LevelGood:      "Hash chain + cryptographic signature on entries",
			LevelExcellent: "Hash chain + signature + blockchain-backed proof",
		},
		Evidence: []string{
			"lineage.Verify(ctx) result",
			"hash chain computation and verification",
			"audit entry before/after attempt",
			"cryptographic signature on entries",
		},
		RelatedFailureModes: []string{"FM-LA-001"},
		TestProcedure: "Create lineage entry; verify hash chain; attempt modification; verify chain broken",
		AcceptanceCriteria: LevelGood,
	},

	"RB-LA-002": {
		ID:   "RB-LA-002",
		Name: "Audit Entry Completeness and Schema Validation",
		Description: "Evaluates whether audit entries contain all required fields (amount, timestamp, " +
			"status, error code) and match documented schema.",
		Domain:           DomainLineageAudit,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"Required fields present: OrderID, Step, Action, Timestamp, Input, Output",
			"Amount field required for settlement entries",
			"Error field populated if action failed",
			"Schema validation enforced on write",
			"Null/empty fields detected and logged",
		},
		ScoringGuideline: "Failing: missing required field. Poor: field present but empty. Fair: mostly complete with minor gaps. Good: complete with validation. Excellent: complete + validated + signed.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Required field missing (e.g., no amount in settlement entry)",
			LevelPoor:      "Required field present but empty/null",
			LevelFair:      "Mostly complete; occasional fields empty",
			LevelGood:      "All required fields present + schema validation enforced",
			LevelExcellent: "Complete + schema validation + typed fields + cryptographic commitment",
		},
		Evidence: []string{
			"audit entry structure (OrderID, Step, Action, etc.)",
			"amount field present and non-zero",
			"error field populated on failure",
			"schema validation log",
		},
		RelatedFailureModes: []string{"FM-LA-002"},
		TestProcedure: "Create audit entry with missing amount; verify rejection; create complete entry; verify acceptance",
		AcceptanceCriteria: LevelGood,
	},

	"RB-LA-003": {
		ID:   "RB-LA-003",
		Name: "Settlement Lineage Order-Ledger-Audit Consistency",
		Description: "Evaluates whether order status, ledger state, and lineage records are consistent " +
			"(all three show same settlement outcome).",
		Domain:           DomainLineageAudit,
		PrimaryDimension: DimensionConsistency,
		Criteria: []string{
			"Order status matches ledger settlement state",
			"Lineage entry exists for every settlement event",
			"Amount in order, ledger, and lineage identical",
			"reconciliation.IsReconciled() detects any mismatch",
			"Timestamp ordering consistent across systems",
		},
		ScoringGuideline: "Failing: order fulfilled but ledger pending. Poor: minor timestamp difference. Fair: mostly consistent. Good: fully consistent. Excellent: consistent + immutable commitment.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Order fulfilled but ledger shows pending",
			LevelPoor:      "Mostly consistent; timestamp differences >1s",
			LevelFair:      "Consistent within 500ms; minor discrepancies",
			LevelGood:      "Fully consistent; reconciliation passes",
			LevelExcellent: "Consistent + immutable proof + cryptographic commitment",
		},
		Evidence: []string{
			"order.Status vs. ledger.TransactionState",
			"order.Amount vs. lineage.Input.Amount vs. ledger.Amount",
			"reconciliation.IsReconciled() result",
			"timestamp alignment across systems",
		},
		RelatedFailureModes: []string{"FM-LA-004"},
		TestProcedure: "Settle order; verify status consistent across order/ledger/lineage; run reconciliation",
		AcceptanceCriteria: LevelGood,
	},

	"RB-LA-004": {
		ID:   "RB-LA-004",
		Name: "Lineage Timestamp Accuracy and Monotonicity",
		Description: "Evaluates whether lineage entry timestamps are accurate (server-generated, " +
			"not client-supplied) and monotonically increasing.",
		Domain:           DomainLineageAudit,
		PrimaryDimension: DimensionDataIntegrity,
		Criteria: []string{
			"Timestamp generated server-side (not client-supplied)",
			"Timestamp immutable after entry creation",
			"Timestamps monotonically increasing within order",
			"No future timestamps (clock skew detection)",
			"Timestamp audit trail for modifications",
		},
		ScoringGuideline: "Failing: client-supplied timestamp or backdated. Poor: timestamp client-modifiable. Fair: server-generated but not monotonic. Good: monotonic + immutable. Excellent: monotonic + immutable + blockchain.",
		LevelDescriptions: map[RubricLevel]string{
			LevelFailing:   "Timestamp client-supplied or manually backdated",
			LevelPoor:      "Server-generated but client can modify",
			LevelFair:      "Server-generated, immutable, but not fully monotonic",
			LevelGood:      "Server-generated, immutable, monotonically increasing",
			LevelExcellent: "Monotonic + immutable + blockchain-backed timestamping service",
		},
		Evidence: []string{
			"timestamp generation location (server/client)",
			"timestamp immutability enforcement",
			"timestamp sequence for order steps",
			"no future timestamp detected",
		},
		RelatedFailureModes: []string{"FM-LA-003"},
		TestProcedure: "Create lineage entries; verify timestamps monotonic; attempt to backdate; verify rejection",
		AcceptanceCriteria: LevelGood,
	},
}

// RubricRegistry interface for accessing rubrics
type RubricRegistry interface {
	Get(id string) *Rubric
	ListByDomain(domain FailureModeDomain) []*Rubric
	All() []*Rubric
}

// DefaultRubricRegistry implements RubricRegistry
type DefaultRubricRegistry struct{}

// Get retrieves a rubric by ID
func (r *DefaultRubricRegistry) Get(id string) *Rubric {
	return AllRubrics[id]
}

// ListByDomain returns all rubrics in a domain
func (r *DefaultRubricRegistry) ListByDomain(domain FailureModeDomain) []*Rubric {
	var rubrics []*Rubric
	for _, rubric := range AllRubrics {
		if rubric.Domain == domain {
			rubrics = append(rubrics, rubric)
		}
	}
	return rubrics
}

// All returns all rubrics
func (r *DefaultRubricRegistry) All() []*Rubric {
	var rubrics []*Rubric
	for _, rubric := range AllRubrics {
		rubrics = append(rubrics, rubric)
	}
	return rubrics
}

// NewRubricRegistry creates a new registry
func NewRubricRegistry() RubricRegistry {
	return &DefaultRubricRegistry{}
}
