package observability

import (
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// PlatformMetrics groups OTEL instruments used across orchestrator/bus/governance.
//
// Acquired lazily via Metrics() so packages can call it without worrying about
// initialization order. Returns no-op instruments before SetupTelemetry runs.
type PlatformMetrics struct {
	MessagesPublished metric.Int64Counter
	MessagesHandled   metric.Int64Counter
	PolicyDenials     metric.Int64Counter
	AgentErrors       metric.Int64Counter
	HandleDuration    metric.Float64Histogram
}

var (
	metricsOnce sync.Once
	metricsInst *PlatformMetrics
	metricsErr  error
)

// Metrics returns the process-wide PlatformMetrics, building it on first call.
// Errors are stored but most call sites ignore them since OTEL falls back to
// no-op instruments and the system keeps working.
func Metrics() *PlatformMetrics {
	metricsOnce.Do(func() {
		meter := otel.Meter("github.com/c2siorg/genie")
		m := &PlatformMetrics{}
		var err error
		if m.MessagesPublished, err = meter.Int64Counter(
			"genie.bus.messages_published",
			metric.WithDescription("Total messages published on the bus."),
		); err != nil {
			metricsErr = err
		}
		if m.MessagesHandled, err = meter.Int64Counter(
			"genie.agent.messages_handled",
			metric.WithDescription("Total messages handled by agents."),
		); err != nil {
			metricsErr = err
		}
		if m.PolicyDenials, err = meter.Int64Counter(
			"genie.governance.denials",
			metric.WithDescription("Total messages denied by governance policies."),
		); err != nil {
			metricsErr = err
		}
		if m.AgentErrors, err = meter.Int64Counter(
			"genie.agent.errors",
			metric.WithDescription("Total errors returned by agent HandleMessage."),
		); err != nil {
			metricsErr = err
		}
		if m.HandleDuration, err = meter.Float64Histogram(
			"genie.agent.handle_duration_ms",
			metric.WithDescription("Latency of agent.HandleMessage in milliseconds."),
			metric.WithUnit("ms"),
		); err != nil {
			metricsErr = err
		}
		metricsInst = m
	})
	return metricsInst
}

// MetricsError returns the first error encountered while building instruments.
func MetricsError() error { return metricsErr }
