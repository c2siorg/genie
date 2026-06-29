// Package eval implements error classification and analysis for execution traces.
//
// The error classification system analyzes execution traces to identify and categorize
// failure modes against the 38-mode taxonomy (6 domains, spanning settlement, compliance,
// orchestration, merchant/customer, agent behavior, and lineage/audit).
//
// TraceAnalysis struct captures:
//   - TraceID: unique trace identifier
//   - FailureFound: whether a failure was detected
//   - PrimaryFailure: dominant failure mode (FM-DOMAIN-NNN)
//   - SecondaryFailures: contributing failure modes
//   - RubricsFailed: evaluation rubrics that flagged issues
//   - Severity: critical/high/medium/low
//   - RootCauseGulf: gap between symptom and root cause (0-100)
//   - Timestamp: when analysis was performed
//
// Usage:
//
//	ca := NewClassifier()
//	analysis := ca.Analyze(ctx, trace)
//	if analysis.FailureFound {
//	  fmt.Printf("Primary: %s (Severity: %s)\n", analysis.PrimaryFailure, analysis.Severity)
//	  fmt.Printf("Root Cause Gulf: %d%%\n", analysis.RootCauseGulf)
//	}
package eval

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TraceAnalysis represents classified error information from an execution trace.
type TraceAnalysis struct {
	// TraceID unique identifier for the trace
	TraceID string `json:"trace_id"`
	// FailureFound whether a failure was detected in the trace
	FailureFound bool `json:"failure_found"`
	// PrimaryFailure dominant failure mode (FM-DOMAIN-NNN), empty if no failure
	PrimaryFailure string `json:"primary_failure"`
	// SecondaryFailures contributing failure modes
	SecondaryFailures []string `json:"secondary_failures"`
	// RubricsFailed evaluation rubrics that flagged issues
	RubricsFailed []string `json:"rubrics_failed"`
	// Severity critical/high/medium/low
	Severity FailureModeSeverity `json:"severity"`
	// RootCauseGulf gap between symptom and root cause (0-100%, 0=exact root cause identified, 100=symptom only)
	RootCauseGulf int `json:"root_cause_gulf"`
	// Timestamp when analysis was performed
	Timestamp time.Time `json:"timestamp"`
	// Confidence confidence level in classification (0-100%)
	Confidence int `json:"confidence"`
	// Evidence supporting evidence for classification
	Evidence []string `json:"evidence"`
	// LinkedOrderID order ID associated with this trace (if applicable)
	LinkedOrderID string `json:"linked_order_id,omitempty"`
	// IsRegression whether this failure is a regression (new failure in code change)
	IsRegression bool `json:"is_regression"`
}

// ErrorClassifier analyzes execution traces and classifies errors.
type ErrorClassifier struct {
	mu        sync.RWMutex
	registry  FailureModeRegistry
	ruleMap   map[string]*Rubric
	history   []TraceAnalysis
}

// NewClassifier creates a new error classifier.
func NewClassifier() *ErrorClassifier {
	return &ErrorClassifier{
		registry:  NewFailureModeRegistry(),
		ruleMap:   AllRubrics,
		history:   []TraceAnalysis{},
	}
}

// Analyze classifies errors from an execution trace.
// This is the primary analysis function that examines trace data and returns classification.
func (c *ErrorClassifier) Analyze(ctx context.Context, trace *TraceData) *TraceAnalysis {
	c.mu.Lock()
	defer c.mu.Unlock()

	if trace == nil {
		return &TraceAnalysis{
			FailureFound:       false,
			SecondaryFailures:  []string{},
			RubricsFailed:      []string{},
			RootCauseGulf:      0,
			Timestamp:          time.Now(),
			Confidence:         0,
			Evidence:           []string{},
			IsRegression:       false,
		}
	}

	analysis := &TraceAnalysis{
		TraceID:            trace.ID,
		FailureFound:       false,
		SecondaryFailures:  []string{},
		RubricsFailed:      []string{},
		RootCauseGulf:      0,
		Timestamp:          time.Now(),
		Confidence:         0,
		Evidence:           []string{},
		IsRegression:       false,
	}

	// Detect failures from trace signals
	detectedModes := c.detectFailureModes(trace)
	if len(detectedModes) == 0 {
		return analysis
	}

	analysis.FailureFound = true

	// Select primary failure (highest severity)
	primary := c.selectPrimaryFailure(detectedModes)
	if primary != nil {
		analysis.PrimaryFailure = primary.ID
		analysis.Severity = primary.Severity
	}

	// Collect secondary failures
	for _, mode := range detectedModes {
		if mode.ID != analysis.PrimaryFailure {
			analysis.SecondaryFailures = append(analysis.SecondaryFailures, mode.ID)
		}
	}

	// Map failure modes to failed rubrics
	analysis.RubricsFailed = c.mapRubricsFailed(detectedModes)

	// Calculate confidence and root cause gulf
	analysis.Confidence, analysis.RootCauseGulf = c.calculateConfidence(trace, detectedModes)

	// Collect evidence
	analysis.Evidence = c.gatherEvidence(trace, detectedModes)

	// Store in history
	c.history = append(c.history, *analysis)

	return analysis
}

// TraceData represents raw execution trace data for analysis.
type TraceData struct {
	ID             string
	Scenario       string
	StartTime      time.Time
	EndTime        time.Duration
	Error          error
	Signals        map[string]interface{}
	Logs           []string
	MetricSnapshots map[string]float64
}

// detectFailureModes examines trace signals to identify potential failure modes.
func (c *ErrorClassifier) detectFailureModes(trace *TraceData) []*FailureMode {
	var detected []*FailureMode

	patterns := map[string]string{
		// Settlement patterns
		"double_spend":              "FM-SE-001",
		"batch_mismatch":            "FM-SE-002",
		"netting_overflow":          "FM-SE-003",
		"finality_violation":        "FM-SE-004",
		"orphaned_settlement":       "FM-SE-005",
		"currency_conversion_error": "FM-SE-006",
		"settlement_timeout":        "FM-SE-007",
		"reconciliation_data_loss":  "FM-SE-008",
		"round_trip_failure":        "FM-SE-009",
		"retry_loop_infinite":       "FM-SE-010",

		// Compliance patterns
		"aml_velocity_breach":       "FM-CO-001",
		"sanctions_screening_false": "FM-CO-002",
		"kyc_verification_bypass":   "FM-CO-003",
		"consent_missing":           "FM-CO-004",
		"regulatory_reporting_false": "FM-CO-005",
		"risk_scoring_stale":        "FM-CO-006",
		"compliance_audit_missing":  "FM-CO-007",
		"document_verification_fraud": "FM-CO-008",
		"compliance_rule_change":    "FM-CO-009",
		"kyc_expiry_not_enforced":   "FM-MC-009",

		// Orchestration patterns
		"invalid_state_transition":  "FM-OR-001",
		"agent_communication_timeout": "FM-OR-002",
		"workflow_out_of_order":     "FM-OR-003",
		"duplicate_payment_call":    "FM-OR-004",
		"orchestrator_crash":        "FM-OR-005",
		"race_condition_workflow":   "FM-OR-006",
		"missing_error_handler":     "FM-OR-007",
		"workflow_rollback_incomplete": "FM-OR-008",
		"workflow_deadlock":         "FM-OR-009",
		"settlement_retry_infinite": "FM-SE-010",

		// Merchant/Customer patterns
		"merchant_impersonation":    "FM-MC-001",
		"card_reuse_multiple_accounts": "FM-MC-002",
		"dispute_not_routed":        "FM-MC-003",
		"settlement_wrong_account":  "FM-MC-004",
		"refund_never_issued":       "FM-MC-005",
		"chargeback_not_recorded":   "FM-MC-006",
		"commission_calculation_error": "FM-MC-007",
		"account_lockout":           "FM-MC-008",

		// Agent Behavior patterns
		"agent_hallucination":       "FM-AB-001",
		"agent_prompt_injection":    "FM-AB-002",
		"agent_consistency_drift":   "FM-AB-003",
		"agent_tool_misuse":         "FM-AB-004",
		"agent_refusal":             "FM-AB-005",
		"agent_alignment_drift":     "FM-AB-006",
		"context_window_overflow":   "FM-AB-007",

		// Lineage/Audit patterns
		"lineage_hash_broken":       "FM-LA-001",
		"audit_missing_fields":      "FM-LA-002",
		"timestamp_manipulation":    "FM-LA-003",
		"order_ledger_lineage_mismatch": "FM-LA-004",
		"lineage_query_tampering":   "FM-LA-005",
		"lineage_deletion":          "FM-LA-006",
		"lineage_privacy_leakage":   "FM-LA-007",
	}

	// Check trace signals against patterns
	for sigKey, fmID := range patterns {
		if c.hasSignal(trace, sigKey) {
			if fm := c.registry.Get(fmID); fm != nil {
				detected = append(detected, fm)
			}
		}
	}

	return detected
}

// hasSignal checks if trace contains a signal pattern.
func (c *ErrorClassifier) hasSignal(trace *TraceData, pattern string) bool {
	if trace == nil || trace.Signals == nil {
		return false
	}

	// Check for explicit signal key
	if val, ok := trace.Signals[pattern]; ok {
		if b, isBool := val.(bool); isBool {
			return b
		}
		return val != nil
	}

	// Check logs for pattern
	for _, log := range trace.Logs {
		if contains(log, pattern) {
			return true
		}
	}

	return false
}

// selectPrimaryFailure chooses the highest-severity failure mode.
func (c *ErrorClassifier) selectPrimaryFailure(modes []*FailureMode) *FailureMode {
	if len(modes) == 0 {
		return nil
	}

	severityRank := map[FailureModeSeverity]int{
		SeverityCritical: 4,
		SeverityHigh:     3,
		SeverityMedium:   2,
		SeverityLow:      1,
	}

	primary := modes[0]
	for _, mode := range modes[1:] {
		if severityRank[mode.Severity] > severityRank[primary.Severity] {
			primary = mode
		}
	}
	return primary
}

// mapRubricsFailed maps failure modes to corresponding rubrics.
func (c *ErrorClassifier) mapRubricsFailed(modes []*FailureMode) []string {
	rubricSet := make(map[string]bool)

	for _, mode := range modes {
		for _, rubricID := range mode.LinkedRubrics {
			rubricSet[rubricID] = true
		}
	}

	result := make([]string, 0, len(rubricSet))
	for rubricID := range rubricSet {
		result = append(result, rubricID)
	}
	return result
}

// calculateConfidence computes confidence and root cause gulf metrics.
func (c *ErrorClassifier) calculateConfidence(trace *TraceData, modes []*FailureMode) (confidence, rootCauseGulf int) {
	if len(modes) == 0 {
		return 0, 0
	}

	// Base confidence on number of corroborating signals
	signalCount := len(trace.Signals)
	logCount := len(trace.Logs)

	confidence = min(100, 40 + (signalCount*10) + (logCount*5))

	// Root cause gulf: distance from observed symptom to actual root cause
	// 0 = exact root cause identified
	// 100 = only symptom visible, no root cause chain
	// Lower guilt = better diagnosis
	if len(trace.Logs) > 0 {
		// More detailed logs = better gulf
		rootCauseGulf = max(0, 80 - (logCount * 3))
	} else {
		rootCauseGulf = 100 // No logs = symptom only
	}

	return confidence, rootCauseGulf
}

// gatherEvidence collects supporting evidence for classification.
func (c *ErrorClassifier) gatherEvidence(trace *TraceData, modes []*FailureMode) []string {
	var evidence []string

	if trace.Error != nil {
		evidence = append(evidence, fmt.Sprintf("error: %v", trace.Error))
	}

	for k, v := range trace.Signals {
		if v != nil {
			evidence = append(evidence, fmt.Sprintf("signal_%s", k))
		}
	}

	if len(trace.Logs) > 0 {
		evidence = append(evidence, fmt.Sprintf("%d_log_entries", len(trace.Logs)))
	}

	for k, v := range trace.MetricSnapshots {
		if v > 0 {
			evidence = append(evidence, fmt.Sprintf("metric_%s=%.2f", k, v))
		}
	}

	return evidence
}

// History returns all stored analyses.
func (c *ErrorClassifier) History() []TraceAnalysis {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]TraceAnalysis, len(c.history))
	copy(result, c.history)
	return result
}

// Clear resets all stored analyses.
func (c *ErrorClassifier) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = []TraceAnalysis{}
}

// GetFailureStats returns statistics about analyzed failures.
func (c *ErrorClassifier) GetFailureStats() *FailureStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &FailureStats{
		TotalTraces:      len(c.history),
		FailuresDetected: 0,
		BySeverity:       make(map[string]int),
		ByDomain:         make(map[string]int),
		ByFailureMode:    make(map[string]int),
	}

	for _, analysis := range c.history {
		if analysis.FailureFound {
			stats.FailuresDetected++
			stats.BySeverity[string(analysis.Severity)]++

			if analysis.PrimaryFailure != "" {
				if fm := c.registry.Get(analysis.PrimaryFailure); fm != nil {
					stats.ByDomain[string(fm.Domain)]++
					stats.ByFailureMode[analysis.PrimaryFailure]++
				}
			}
		}
	}

	return stats
}

// FailureStats represents aggregate statistics about classified failures.
type FailureStats struct {
	TotalTraces      int            `json:"total_traces"`
	FailuresDetected int            `json:"failures_detected"`
	BySeverity       map[string]int `json:"by_severity"`
	ByDomain         map[string]int `json:"by_domain"`
	ByFailureMode    map[string]int `json:"by_failure_mode"`
}

// Helper functions

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
