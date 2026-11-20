// Package eval provides evaluation infrastructure for Genie.
// monitoring.go implements production evaluation with confidence intervals via bootstrap sampling.
//
// The ProductionEvalConfig struct controls evaluation behavior: sample rate (1% default),
// judge definitions, and confidence levels. RunProductionEvaluation executes the evaluation
// loop with real-time trace export to Laminar and metrics collection via OpenTelemetry.
package eval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval/hallucination"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
)

// JudgeFunc defines the signature for an evaluation judge.
// It takes context, LLM provider, model name, test input, and system output,
// returning a score [0..1] and optional error.
//
// Example judges:
//   - Settlement amount hallucination detection
//   - Regulatory compliance validation
//   - Customer data privacy checks
type JudgeFunc func(ctx context.Context, p llm.Provider, model, input, output string) (score float64, err error)

// JudgeMetrics captures per-judge performance statistics.
type JudgeMetrics struct {
	Name                string
	TotalScored         int
	PassCount           int
	FailCount           int
	PassRate            float64
	TPR                 float64 // True positive rate
	TNR                 float64 // True negative rate
	ConfidenceIntervals ConfidenceInterval
}

// ConfidenceInterval holds [Lower, Point, Upper] estimates (e.g., [0.85, 0.90, 0.95]).
type ConfidenceInterval struct {
	Lower float64
	Point float64
	Upper float64
}

// ProductionEvalMetrics aggregates all evaluation metrics for a run.
type ProductionEvalMetrics struct {
	RunID                          string
	SampleSize                     int
	SampleRate                     float64
	ConfidenceLevel                float64
	SettlementAmountHallucinatedCI ConfidenceInterval
	JudgeMetrics                   map[string]JudgeMetrics
	DriftDetected                  bool
	DriftReason                    string
	StartedAt                      time.Time
	CompletedAt                    time.Time
}

// ProductionEvalConfig controls production evaluation behavior.
type ProductionEvalConfig struct {
	// SampleRate is the fraction of requests to evaluate (0..1, default: 0.01 = 1%).
	SampleRate float64

	// ConfidenceLevel is the desired CI coverage (default: 0.95 = 95%).
	ConfidenceLevel float64

	// Judges is a map of judge name → judge function.
	// Built-in: "settlement_hallucination" (detects unsupported settlement claims).
	Judges map[string]JudgeFunc

	// LLMProvider and Model configure the LLM backend for judges.
	// Judges use the same provider/model for consistent scoring.
	LLMProvider llm.Provider
	LLMModel    string

	// DriftThreshold: if pass rate falls below this, drift is reported (default: 0.85).
	DriftThreshold float64

	// BootstrapSamples: number of bootstrap resamples for CI (default: 10000).
	BootstrapSamples int

	// Store persists evaluation records (optional; use InMemoryStore for tests).
	Store Store
}

// RunProductionEvaluation executes the evaluation loop over recorded interactions.
//
// The function:
// 1. Loads recorded interactions from the store
// 2. Samples them according to SampleRate
// 3. Runs each judge against sampled interactions
// 4. Computes confidence intervals via bootstrap resampling
// 5. Detects drift (pass rate below threshold)
// 6. Exports traces to Laminar via OpenTelemetry
// 7. Collects and returns aggregated metrics
//
// All judges run in parallel. Bootstrap resampling happens serially per judge.
func RunProductionEvaluation(ctx context.Context, cfg ProductionEvalConfig) (ProductionEvalMetrics, error) {
	tracer := otel.Tracer("github.com/c2siorg/genie/pkg/eval/monitoring")
	meter := otel.Meter("github.com/c2siorg/genie/pkg/eval/monitoring")

	ctx, span := tracer.Start(ctx, "RunProductionEvaluation")
	defer span.End()

	// Validate config.
	if cfg.SampleRate <= 0 || cfg.SampleRate > 1 {
		cfg.SampleRate = 0.01
	}
	if cfg.ConfidenceLevel <= 0 || cfg.ConfidenceLevel >= 1 {
		cfg.ConfidenceLevel = 0.95
	}
	if cfg.DriftThreshold <= 0 || cfg.DriftThreshold > 1 {
		cfg.DriftThreshold = 0.85
	}
	if cfg.BootstrapSamples <= 0 {
		cfg.BootstrapSamples = 10000
	}
	if cfg.Store == nil {
		cfg.Store = NewInMemoryStore()
	}

	// Load all recorded interactions.
	allRecords := cfg.Store.List()
	if len(allRecords) == 0 {
		return ProductionEvalMetrics{}, fmt.Errorf("no records to evaluate")
	}

	span.AddEvent("loaded_records", trace.WithAttributes(
		attribute.Int("record_count", len(allRecords)),
	))

	// Sample interactions.
	sampled := sampleRecords(allRecords, cfg.SampleRate)
	if len(sampled) == 0 {
		return ProductionEvalMetrics{}, fmt.Errorf("sampling resulted in zero records")
	}

	span.AddEvent("sampled_records", trace.WithAttributes(
		attribute.Int("sample_size", len(sampled)),
		attribute.Float64("sample_rate", cfg.SampleRate),
	))

	// Build result container.
	result := ProductionEvalMetrics{
		RunID:           fmt.Sprintf("eval-%d", time.Now().UnixNano()),
		SampleSize:      len(sampled),
		SampleRate:      cfg.SampleRate,
		ConfidenceLevel: cfg.ConfidenceLevel,
		JudgeMetrics:    make(map[string]JudgeMetrics),
		StartedAt:       time.Now(),
	}

	// Run all judges in parallel.
	judgeResults := make(chan struct {
		name    string
		metrics JudgeMetrics
		err     error
	}, len(cfg.Judges))
	judgeWg := &sync.WaitGroup{}

	for judgeName, judgeFunc := range cfg.Judges {
		judgeWg.Add(1)
		go func(name string, judge JudgeFunc) {
			defer judgeWg.Done()

			_, judgeSpan := tracer.Start(ctx, fmt.Sprintf("judge_%s", name))
			defer judgeSpan.End()

			metrics, err := runJudge(ctx, judge, name, sampled, cfg, meter)
			if err != nil {
				judgeSpan.RecordError(err)
				judgeResults <- struct {
					name    string
					metrics JudgeMetrics
					err     error
				}{name: name, err: err}
				return
			}

			judgeSpan.AddEvent("judge_complete", trace.WithAttributes(
				attribute.Float64("pass_rate", metrics.PassRate),
				attribute.Int("total_scored", metrics.TotalScored),
			))

			judgeResults <- struct {
				name    string
				metrics JudgeMetrics
				err     error
			}{name: name, metrics: metrics, err: nil}
		}(judgeName, judgeFunc)
	}

	// Collect results.
	judgeWg.Wait()
	close(judgeResults)

	for jr := range judgeResults {
		if jr.err != nil {
			span.RecordError(jr.err)
			return result, fmt.Errorf("judge %q: %w", jr.name, jr.err)
		}
		result.JudgeMetrics[jr.name] = jr.metrics

		// Detect drift for each judge.
		if jr.metrics.PassRate < cfg.DriftThreshold {
			result.DriftDetected = true
			result.DriftReason = fmt.Sprintf("judge %q pass rate %.2f below threshold %.2f",
				jr.name, jr.metrics.PassRate, cfg.DriftThreshold)
		}
	}

	// Special handling for settlement hallucination judge.
	if settlementMetrics, ok := result.JudgeMetrics["settlement_hallucination"]; ok {
		result.SettlementAmountHallucinatedCI = settlementMetrics.ConfidenceIntervals
	}

	result.CompletedAt = time.Now()

	span.AddEvent("evaluation_complete", trace.WithAttributes(
		attribute.String("run_id", result.RunID),
		attribute.Bool("drift_detected", result.DriftDetected),
	))

	return result, nil
}

// runJudge executes one judge across all sampled records and computes CI via bootstrap.
func runJudge(
	ctx context.Context,
	judge JudgeFunc,
	judgeName string,
	sampled []InteractionRecord,
	cfg ProductionEvalConfig,
	meter metric.Meter,
) (JudgeMetrics, error) {
	tracer := otel.Tracer("github.com/c2siorg/genie/pkg/eval/monitoring")

	ctx, span := tracer.Start(ctx, fmt.Sprintf("runJudge_%s", judgeName))
	defer span.End()

	// Score each sampled record.
	scores := make([]float64, len(sampled))
	passCount := 0

	scoresMu := &sync.Mutex{}
	scoreWg := &sync.WaitGroup{}
	scoreLimiter := make(chan struct{}, 8) // Max 8 concurrent judge calls.

	for i, record := range sampled {
		scoreWg.Add(1)
		go func(idx int, rec InteractionRecord) {
			defer scoreWg.Done()
			scoreLimiter <- struct{}{}        // Acquire slot.
			defer func() { <-scoreLimiter }() // Release slot.

			// Extract input/output from record metadata.
			input := getStringMetadata(rec, "input", "")
			output := getStringMetadata(rec, "output", "")

			score, err := judge(ctx, cfg.LLMProvider, cfg.LLMModel, input, output)
			if err != nil {
				// On judge error, record as fail (0.0).
				score = 0.0
			}

			scoresMu.Lock()
			scores[idx] = score
			if score >= 0.5 {
				passCount++
			}
			scoresMu.Unlock()
		}(i, record)
	}

	scoreWg.Wait()

	passRate := float64(passCount) / float64(len(sampled))

	// Bootstrap confidence interval.
	ci := bootstrapCI(scores, cfg.ConfidenceLevel, cfg.BootstrapSamples)

	// Record metrics.
	metrics := JudgeMetrics{
		Name:                judgeName,
		TotalScored:         len(sampled),
		PassCount:           passCount,
		FailCount:           len(sampled) - passCount,
		PassRate:            passRate,
		ConfidenceIntervals: ci,
	}

	// Export metrics to OpenTelemetry.
	if meter != nil {
		passRateGauge, _ := meter.Float64Gauge(
			fmt.Sprintf("eval.judge.%s.pass_rate", judgeName),
			metric.WithDescription(fmt.Sprintf("Pass rate for judge %s", judgeName)),
		)
		passRateGauge.Record(ctx, passRate)

		ciGauge, _ := meter.Float64Gauge(
			fmt.Sprintf("eval.judge.%s.confidence_interval.lower", judgeName),
		)
		ciGauge.Record(ctx, ci.Lower)

		ciGaugeUpper, _ := meter.Float64Gauge(
			fmt.Sprintf("eval.judge.%s.confidence_interval.upper", judgeName),
		)
		ciGaugeUpper.Record(ctx, ci.Upper)
	}

	span.AddEvent("judge_scored", trace.WithAttributes(
		attribute.Int("pass_count", passCount),
		attribute.Int("fail_count", metrics.FailCount),
		attribute.Float64("pass_rate", passRate),
		attribute.Float64("ci_lower", ci.Lower),
		attribute.Float64("ci_point", ci.Point),
		attribute.Float64("ci_upper", ci.Upper),
	))

	return metrics, nil
}

// bootstrapCI computes a confidence interval via bootstrap resampling.
//
// Algorithm:
// 1. Resample the score array N times with replacement
// 2. Compute the mean of each resample
// 3. Sort the means
// 4. Return the [lower, point, upper] percentiles
func bootstrapCI(scores []float64, confidenceLevel float64, numSamples int) ConfidenceInterval {
	if len(scores) == 0 {
		return ConfidenceInterval{}
	}

	// Point estimate is the empirical mean.
	pointMean := 0.0
	for _, s := range scores {
		pointMean += s
	}
	pointMean /= float64(len(scores))

	// Bootstrap resample.
	bootMeans := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		mean := 0.0
		for j := 0; j < len(scores); j++ {
			// Resample with replacement.
			idx := int(randomFloat64() * float64(len(scores)))
			if idx >= len(scores) {
				idx = len(scores) - 1
			}
			mean += scores[idx]
		}
		mean /= float64(len(scores))
		bootMeans[i] = mean
	}

	// Sort to find percentiles.
	sort.Float64s(bootMeans)

	// Compute lower and upper percentiles.
	alpha := 1.0 - confidenceLevel
	lowerIdx := int((alpha / 2) * float64(numSamples))
	upperIdx := int((1.0 - alpha/2) * float64(numSamples))

	if lowerIdx < 0 {
		lowerIdx = 0
	}
	if upperIdx >= len(bootMeans) {
		upperIdx = len(bootMeans) - 1
	}

	lower := bootMeans[lowerIdx]
	upper := bootMeans[upperIdx]

	return ConfidenceInterval{
		Lower: lower,
		Point: pointMean,
		Upper: upper,
	}
}

// sampleRecords performs stratified sampling: selects approximately
// (sampleRate * len(records)) records uniformly at random.
func sampleRecords(records []InteractionRecord, sampleRate float64) []InteractionRecord {
	if sampleRate >= 1.0 {
		return records
	}

	targetSize := int(math.Ceil(float64(len(records)) * sampleRate))
	if targetSize < 1 {
		targetSize = 1
	}

	// Fisher-Yates shuffle to select uniformly random subset.
	selected := make([]InteractionRecord, len(records))
	copy(selected, records)

	for i := 0; i < targetSize; i++ {
		j := i + int(randomFloat64()*float64(len(records)-i))
		selected[i], selected[j] = selected[j], selected[i]
	}

	return selected[:targetSize]
}

// getStringMetadata extracts a string value from an InteractionRecord's metadata.
func getStringMetadata(record InteractionRecord, key, defaultValue string) string {
	if record.Metadata == nil {
		return defaultValue
	}
	if v, ok := record.Metadata[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

// randomFloat64 returns a pseudo-random float in [0, 1).
// In production, use crypto/rand or math/rand with proper seeding.
// For now, uses a simple time-based approach.
var (
	randMu sync.Mutex
	randSt uint64
)

func randomFloat64() float64 {
	randMu.Lock()
	defer randMu.Unlock()

	// Simple LCG: not cryptographically secure but deterministic and fast.
	if randSt == 0 {
		randSt = uint64(time.Now().UnixNano())
	}
	randSt = randSt*1103515245 + 12345
	return float64((randSt/65536)%32768) / 32768.0
}

// DefaultSettlementHallucinationJudge detects unsupported settlement amount claims.
//
// It uses the hallucination package to grade sentences against transaction context.
// A settlement is considered valid if >= 80% of sentences are supported.
func DefaultSettlementHallucinationJudge(
	ctx context.Context,
	p llm.Provider,
	model string,
	input string,
	output string,
) (float64, error) {
	if p == nil {
		return 0.5, nil // Neutral when no provider.
	}

	report, err := hallucination.Detect(ctx, p, model, input, output)
	if err != nil {
		return 0.0, err
	}

	if report.SupportedFraction >= 0.8 {
		return 1.0, nil
	}
	return 0.0, nil
}
