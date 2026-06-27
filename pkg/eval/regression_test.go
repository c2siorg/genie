package eval

// DECOMMISSIONED (2026-06-25): the golden-dataset regression gate that used to
// live here was evaluation theater. It synthesized ~55 cases as Go literals and
// scored them with evaluateTestCase(), which returned Pass:true UNCONDITIONALLY
// — so TestGoldenDataset_AllCases reported 55/55 = 100% and its "90% gate" could
// never fail. The cases also used ad-hoc keys (customerID, amount, velocityLimit)
// that did not map to any judge input, so even a real evaluator could not have
// scored them.
//
// The honest replacement lives in pkg/eval/golden:
//   - golden.Case / golden.Dataset      — canonical schema, NO pre-baked output
//   - golden.LoadGoldenDir              — loader that rejects pre-baked outputs
//   - golden.EvaluateCase               — typed dispatch to the deterministic
//                                         judges (judges.Mock*Judge rules)
//   - data/*.json                       — real, correctly-keyed cases
//   - TestGoldenGate                    — loads the dataset and scores it for
//                                         real; FAILS if a judge rule regresses
//   - TestEvaluateCase_GateCanFail      — proves the gate is not tautological
//
// See EVAL_EXPANSION_PLAN.md (Phase 0). The old judge-accuracy and
// failure-mode-coverage tests were tautological too (they reused
// evaluateTestCase) and are superseded by golden.TestGoldenGate_CoverageReport
// and the Phase-1 nightly judge-calibration gate.
