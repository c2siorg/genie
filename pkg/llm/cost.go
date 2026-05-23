package llm

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// CostObserver wraps any Provider and emits OTel metrics + span attributes
// for tokens spent and latency per call. Cents-per-1k-tokens prices live in
// CostsPer1k (set them at construction).
type CostObserver struct {
	Inner    Provider
	CostsPer1k struct {
		Prompt     float64
		Completion float64
	}
}

// NewCostObserver wraps a provider with cost telemetry.
func NewCostObserver(inner Provider, promptPer1k, completionPer1k float64) *CostObserver {
	o := &CostObserver{Inner: inner}
	o.CostsPer1k.Prompt = promptPer1k
	o.CostsPer1k.Completion = completionPer1k
	return o
}

func (o *CostObserver) Name() string   { return o.Inner.Name() + "+cost" }
func (o *CostObserver) Region() string { return o.Inner.Region() }

// metric handles are lazily initialised — Genie's observability package
// usually owns the meter, but the wrapper is self-contained so the panel
// works whether or not the user pre-registered.
var (
	costMeter           = otel.Meter("github.com/c2siorg/genie/pkg/llm")
	tokenCounter, _     = costMeter.Int64Counter("genie.llm.tokens", metric.WithDescription("Total tokens consumed."))
	costCounter, _      = costMeter.Float64Counter("genie.llm.cost_micros", metric.WithDescription("Approximate cost in microcurrency."))
	latencyHistogram, _ = costMeter.Float64Histogram("genie.llm.latency_ms", metric.WithDescription("LLM call latency."), metric.WithUnit("ms"))
)

func (o *CostObserver) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	tracer := otel.Tracer("github.com/c2siorg/genie/pkg/llm")
	ctx, span := tracer.Start(ctx, "llm.complete",
		trace.WithAttributes(
			attribute.String("genie.llm.provider", o.Inner.Name()),
			attribute.String("genie.llm.model", req.Model),
		),
	)
	defer span.End()

	start := time.Now()
	resp, err := o.Inner.Complete(ctx, req)
	elapsedMs := float64(time.Since(start).Milliseconds())

	commonAttrs := metric.WithAttributes(
		attribute.String("provider", o.Inner.Name()),
		attribute.String("model", req.Model),
	)
	latencyHistogram.Record(ctx, elapsedMs, commonAttrs)

	if err != nil {
		span.RecordError(err)
		return resp, err
	}
	prompt := resp.Usage.PromptTokens
	completion := resp.Usage.CompletionTokens
	tokenCounter.Add(ctx, int64(prompt+completion), commonAttrs)

	cost := (float64(prompt)/1000.0)*o.CostsPer1k.Prompt + (float64(completion)/1000.0)*o.CostsPer1k.Completion
	costCounter.Add(ctx, cost*1e6, commonAttrs) // microcurrency

	span.SetAttributes(
		// Genie-native attributes — keep for existing dashboards.
		attribute.Int("genie.llm.tokens.prompt", prompt),
		attribute.Int("genie.llm.tokens.completion", completion),
		attribute.Float64("genie.llm.cost", cost),
		// OpenInference semantic conventions — recognised by Arize Phoenix,
		// Langfuse, and other LLM-aware observability tools.
		attribute.String("openinference.span.kind", "LLM"),
		attribute.String("llm.provider", o.Inner.Name()),
		attribute.String("llm.model_name", req.Model),
		attribute.Int("llm.token_count.prompt", prompt),
		attribute.Int("llm.token_count.completion", completion),
		attribute.Int("llm.token_count.total", prompt+completion),
	)
	return resp, nil
}
