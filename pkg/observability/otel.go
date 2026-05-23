package observability

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// ExporterKind selects which OTEL exporter pair to install.
type ExporterKind string

const (
	// ExporterStdout writes pretty JSON to the configured Writers. Demo default.
	ExporterStdout ExporterKind = "stdout"
	// ExporterOTLP sends traces+metrics to an OTLP gRPC collector
	// (e.g. otel-collector → Tempo / Prometheus).
	ExporterOTLP ExporterKind = "otlp"
)

// TelemetryConfig controls OpenTelemetry setup.
//
// Defaults: stdout exporter pair. Set Exporter=ExporterOTLP and OTLPEndpoint
// (or env OTEL_EXPORTER_OTLP_ENDPOINT) to ship to a collector instead.
type TelemetryConfig struct {
	ServiceName    string
	ServiceVersion string

	Exporter ExporterKind

	// OTLP options
	OTLPEndpoint string // host:port, no scheme; defaults to env OTEL_EXPORTER_OTLP_ENDPOINT
	OTLPInsecure bool   // skip TLS — recommended only for local dev

	// Stdout options
	TraceWriter  io.Writer // defaults to os.Stdout
	MetricWriter io.Writer // defaults to os.Stdout
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
	if cfg.Exporter == "" {
		cfg.Exporter = ExporterStdout
	}
	if cfg.OTLPEndpoint == "" {
		cfg.OTLPEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	))
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	traceExp, metricReader, err := buildExporters(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Telemetry{TracerProvider: tp, MeterProvider: mp}, nil
}

// buildExporters returns the configured trace exporter and metric reader.
func buildExporters(ctx context.Context, cfg TelemetryConfig) (sdktrace.SpanExporter, sdkmetric.Reader, error) {
	switch cfg.Exporter {
	case ExporterOTLP:
		traceOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint)}
		metricOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint)}
		if cfg.OTLPInsecure {
			traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
			metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
		}
		traceExp, err := otlptracegrpc.New(ctx, traceOpts...)
		if err != nil {
			return nil, nil, fmt.Errorf("otlp trace exporter: %w", err)
		}
		metricExp, err := otlpmetricgrpc.New(ctx, metricOpts...)
		if err != nil {
			return nil, nil, fmt.Errorf("otlp metric exporter: %w", err)
		}
		return traceExp, sdkmetric.NewPeriodicReader(metricExp), nil
	case ExporterStdout, "":
		traceExp, err := stdouttrace.New(stdouttrace.WithWriter(cfg.TraceWriter), stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, nil, fmt.Errorf("stdout trace exporter: %w", err)
		}
		metricExp, err := stdoutmetric.New(stdoutmetric.WithWriter(cfg.MetricWriter))
		if err != nil {
			return nil, nil, fmt.Errorf("stdout metric exporter: %w", err)
		}
		return traceExp, sdkmetric.NewPeriodicReader(metricExp), nil
	default:
		return nil, nil, fmt.Errorf("unknown exporter kind %q", cfg.Exporter)
	}
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
