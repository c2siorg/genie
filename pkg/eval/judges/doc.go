// Package judges implements LLM-as-Judge evaluators with calibration for Genie.
//
// This package contains 4 specialized judges for evaluating critical system components:
//
// 1. settlement_judge.go — Evaluates settlement correctness, netting, CBDC alignment, reconciliation
// 2. compliance_judge.go — Evaluates KYC justification, AML thresholds, velocity, audit trails
// 3. orchestration_judge.go — Evaluates state machine validity, workflow sequences, multi-agent consistency
// 4. lineage_judge.go — Evaluates hash chain integrity, decision traceability, timestamp monotonicity
//
// Each judge is calibrated to achieve:
//   - True Positive Rate (TPR) ≥ 0.90
//   - True Negative Rate (TNR) ≥ 0.90
//   - 95% confidence intervals via bootstrapping
//
// The judges use LLM-based evaluation with structured prompts to assess:
//   - Rubric adherence (from pkg/eval/rubrics.go)
//   - Binary decisions (pass/fail) based on configured thresholds
//   - Detailed reasoning and evidence for each judgment
//
// Calibration Process:
// Each judge runs calibration via bootstrapping to compute:
//   - TPR (recall): P(positive verdict | ground-truth positive)
//   - TNR (specificity): P(negative verdict | ground-truth negative)
//   - 95% confidence intervals via percentile bootstrap
//   - Optimal threshold selection (maximizing sensitivity + specificity)
//
// Example usage:
//
//	judge := NewSettlementJudge(cfg)
//	calibration, err := judge.Calibrate(ctx, groundTruth)
//	if err != nil {
//		log.Fatal(err)
//	}
//	verdict, evidence, err := judge.Evaluate(ctx, settlement)
//	if err != nil {
//		log.Fatal(err)
//	}
package judges
