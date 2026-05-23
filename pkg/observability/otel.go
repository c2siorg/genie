package observability

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TelemetryConfig controls OpenTelemetry setup.
//
// Defaults target local development: stdout exporters and a small batch timeout.
// In production you would wire OTLP exporters by setting Writer to nil and
// providing alternative SpanProcessor/Reader via WithSpanProcessor / WithReader.
type TelemetryConfig struct {
	ServiceName    string
	ServiceVersion string

	// TraceWriter receives stdout trace JSON. Defaults to os.Stdout.
	// Set to io.Discard to silence traces while keeping the SDK active.
	TraceWriter io.Writer

	// MetricWriter receives stdout metric JSON. Defaults to os.Stdout.
	MetricWriter io.Writer
}

// Telemetry holds the OTEL providers so callers can shut them down.
type Telemetry struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
}

// Shutdown flushes pending spans/metrics and releases provider resources.
// Always call this from main via defer.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	var first error
	if t.TracerProvider != nil {
		if err := t.TracerProvider.Shutdown(ctx); err != nil {
			first = err
		}
	}
	if t.MeterProvider != nil {
		if err := t.MeterProvider.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// SetupTelemetry installs global TracerProvider, MeterProvider, and a
// composite text-map propagator (W3C TraceContext + Baggage).
//
// The global propagator is what we use to inject/extract trace context
// into protocol.Message.Metadata across the in-memory bus.
func SetupTelemetry(ctx context.Context, cfg TelemetryConfig) (*Telemetry, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "genie"
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "dev"
	}
	if cfg.TraceWriter == nil {
		cfg.TraceWriter = os.Stdout
	}
	if cfg.MetricWriter == nil {
		cfg.MetricWriter = os.Stdout
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	))
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	traceExp, err := stdouttrace.New(
		stdouttrace.WithWriter(cfg.TraceWriter),
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, fmt.Errorf("otel trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := stdoutmetric.New(stdoutmetric.WithWriter(cfg.MetricWriter))
	if err != nil {
		return nil, fmt.Errorf("otel metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Telemetry{TracerProvider: tp, MeterProvider: mp}, nil
}

// MetadataCarrier adapts a protocol.Message.Metadata map for OTEL propagation.
//
// Propagators speak map[string]string while Message.Metadata is map[string]any.
// We round-trip via fmt.Sprintf, which is enough for the W3C TraceContext header
// values (already strings).
type MetadataCarrier map[string]any

func (c MetadataCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func (c MetadataCarrier) Set(key, value string) {
	c[key] = value
}

func (c MetadataCarrier) Keys() []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	return out
}

// InjectTraceContext writes the current span context into msg.Metadata.
// It mutates the map in place; pass a non-nil Metadata.
func InjectTraceContext(ctx context.Context, metadata map[string]any) {
	if metadata == nil {
		return
	}
	otel.GetTextMapPropagator().Inject(ctx, MetadataCarrier(metadata))
}

// ExtractTraceContext reads trace context from msg.Metadata and returns a
// derived context that carries the remote span context. Use the returned
// context to start child spans tied to the publisher's trace.
func ExtractTraceContext(ctx context.Context, metadata map[string]any) context.Context {
	if metadata == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, MetadataCarrier(metadata))
}
