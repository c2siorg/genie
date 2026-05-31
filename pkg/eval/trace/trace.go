// Package trace wires OpenTelemetry OTLP HTTP tracing to a Laminar instance.
//
// Usage:
//
//	tp, err := trace.NewLaminarProvider(ctx, "genie-eval")
//	defer tp.Shutdown(ctx)
//	runner.TP = tp   // singleturn.Runner or multiturn.Runner
//
// Environment variables (all optional):
//
//	LMNR_PROJECT_API_KEY  project API key from Settings → API Keys in the Laminar UI
//	LMNR_BASE_URL         Laminar HTTP base URL  (default: http://localhost:8000)
//	GENIE_OTEL_SERVICE    service name attribute (default: "genie-eval")
package trace

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// NewLaminarProvider creates a TracerProvider that exports spans to a local
// Laminar instance via OTLP/HTTP.
//
// Spans appear in the Laminar UI under Traces once the provider flushes
// (on Shutdown or after the batcher timeout).
func NewLaminarProvider(ctx context.Context, serviceName string) (*sdktrace.TracerProvider, error) {
	baseURL := os.Getenv("LMNR_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Strip scheme — otlptracehttp.WithEndpoint wants "host:port".
	endpoint := strings.TrimPrefix(baseURL, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithURLPath("/v1/traces"),
		otlptracehttp.WithInsecure(), // local dev; override per service
	}

	if key := os.Getenv("LMNR_PROJECT_API_KEY"); key != "" {
		opts = append(opts, otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + key,
		}))
	}

	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("laminar: OTLP exporter: %w", err)
	}

	svcName := serviceName
	if n := os.Getenv("GENIE_OTEL_SERVICE"); n != "" {
		svcName = n
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(svcName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("laminar: resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	return tp, nil
}
