// Package advisorpipeline exists only to host the cross-agent integration test
// that wires ProfileAnalyzer → FinancialAnalyst → RecommendationGenerator
// together. It has no runtime code; the real wiring (with the glue that reshapes
// each stage's output into the next stage's input) lives in the backend handler.
package advisorpipeline
