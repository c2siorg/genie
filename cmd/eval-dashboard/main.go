// cmd/eval-dashboard provides a real-time evaluation metrics dashboard.
//
// The dashboard displays:
// - Settlement amount hallucination pass rate with [95% CI]
// - Judge accuracy (TPR/TNR per judge)
// - Sample size and drift detection status
// - Real-time metric collection via OpenTelemetry
// - Trace export to Laminar (requires LMNR_PROJECT_API_KEY)
//
// Usage:
//
//	eval-dashboard [flags]
//
// Flags:
//
//	-listen      HTTP listen address (default: :8080)
//	-metricsPort Prometheus /metrics listen address (default: :9464)
//	-interval    polling interval (default: 30s)
//	-laminar     export traces to Laminar (requires LMNR_PROJECT_API_KEY)
//	-sample-rate evaluation sample rate (default: 0.01 = 1%)
//	-confidence  confidence level for intervals (default: 0.95 = 95%)
//	-drift       drift threshold (default: 0.85)
//
// Environment:
//
//	LMNR_PROJECT_API_KEY    API key for Laminar trace export
//	OTEL_EXPORTER_OTLP_ENDPOINT  OTLP gRPC collector endpoint
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address for dashboard")
	metricsPort := flag.String("metricsPort", ":9464", "Prometheus /metrics listen address")
	interval := flag.Duration("interval", 30*time.Second, "evaluation polling interval")
	withLaminar := flag.Bool("laminar", false, "export traces to Laminar (requires LMNR_PROJECT_API_KEY)")
	sampleRate := flag.Float64("sample-rate", 0.01, "evaluation sample rate (0..1)")
	confidence := flag.Float64("confidence", 0.95, "confidence level for intervals")
	driftThreshold := flag.Float64("drift", 0.85, "drift detection threshold")
	flag.Parse()

	ctx := context.Background()

	// Setup OpenTelemetry.
	tel, err := setupTelemetry(ctx, *withLaminar)
	if err != nil {
		log.Fatalf("telemetry setup failed: %v", err)
	}
	defer tel.Shutdown(context.Background())

	// Dashboard state.
	state := &DashboardState{
		mu: &sync.RWMutex{},
	}

	// Start evaluation loop.
	go evaluationLoop(ctx, state, eval.ProductionEvalConfig{
		SampleRate:       *sampleRate,
		ConfidenceLevel:  *confidence,
		DriftThreshold:   *driftThreshold,
		BootstrapSamples: 1000,
		Judges: map[string]eval.JudgeFunc{
			"settlement_hallucination": eval.DefaultSettlementHallucinationJudge,
		},
		Store: eval.NewInMemoryStore(),
	}, *interval)

	// HTTP handlers.
	http.HandleFunc("/", handleDashboard(state))
	http.HandleFunc("/api/metrics", handleAPIMetrics(state))
	http.Handle("/metrics", tel.MetricsHandler)

	// Start HTTP server.
	go func() {
		log.Printf("dashboard listening on http://localhost%s", *listen)
		if err := http.ListenAndServe(*listen, nil); err != nil {
			log.Printf("dashboard error: %v", err)
		}
	}()

	// Start metrics server.
	go func() {
		log.Printf("prometheus /metrics on http://localhost%s/metrics", *metricsPort)
		if err := http.ListenAndServe(*metricsPort, http.DefaultServeMux); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()

	// Block forever.
	select {}
}

// DashboardState holds the latest evaluation metrics.
type DashboardState struct {
	mu      *sync.RWMutex
	Metrics eval.ProductionEvalMetrics
	Error   string
	LastRun time.Time
}

// evaluationLoop runs evaluations on a schedule and updates dashboard state.
func evaluationLoop(
	ctx context.Context,
	state *DashboardState,
	cfg eval.ProductionEvalConfig,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		metrics, err := eval.RunProductionEvaluation(ctx, cfg)

		state.mu.Lock()
		state.LastRun = time.Now()
		if err != nil {
			state.Error = fmt.Sprintf("evaluation failed: %v", err)
			state.mu.Unlock()
			continue
		}

		state.Error = ""
		state.Metrics = metrics
		state.mu.Unlock()

		log.Printf("evaluation complete: sample_size=%d, drift=%v",
			metrics.SampleSize, metrics.DriftDetected)
	}
}

// handleDashboard serves the HTML dashboard.
func handleDashboard(state *DashboardState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state.mu.RLock()
		defer state.mu.RUnlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// Render HTML dashboard.
		dashboardHTML := `
<!DOCTYPE html>
<html>
<head>
	<title>Genie Evaluation Dashboard</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
			background: #f5f5f5;
			margin: 0;
			padding: 20px;
		}
		.container {
			max-width: 1200px;
			margin: 0 auto;
		}
		h1 {
			color: #333;
			margin-top: 0;
		}
		.grid {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
			gap: 20px;
			margin-bottom: 30px;
		}
		.card {
			background: white;
			border-radius: 8px;
			padding: 20px;
			box-shadow: 0 2px 4px rgba(0,0,0,0.1);
		}
		.metric {
			display: flex;
			justify-content: space-between;
			align-items: center;
			padding: 10px 0;
			border-bottom: 1px solid #eee;
		}
		.metric:last-child {
			border-bottom: none;
		}
		.label {
			font-weight: 500;
			color: #666;
		}
		.value {
			font-size: 1.2em;
			font-weight: 600;
			color: #333;
		}
		.pass-rate {
			color: #4CAF50;
		}
		.pass-rate.low {
			color: #ff9800;
		}
		.pass-rate.critical {
			color: #f44336;
		}
		.confidence-interval {
			font-size: 0.9em;
			color: #999;
			margin-top: 5px;
		}
		.drift-badge {
			display: inline-block;
			padding: 4px 12px;
			border-radius: 16px;
			font-size: 0.85em;
			font-weight: 600;
		}
		.drift-badge.detected {
			background: #ffebee;
			color: #c62828;
		}
		.drift-badge.normal {
			background: #e8f5e9;
			color: #2e7d32;
		}
		.judge-card {
			margin-top: 15px;
		}
		.judge-name {
			font-weight: 600;
			color: #333;
			margin-bottom: 10px;
		}
		.error {
			background: #ffebee;
			color: #c62828;
			padding: 15px;
			border-radius: 4px;
			margin-bottom: 20px;
		}
		.timestamp {
			color: #999;
			font-size: 0.9em;
			margin-top: 10px;
		}
		table {
			width: 100%;
			border-collapse: collapse;
		}
		th, td {
			text-align: left;
			padding: 10px;
			border-bottom: 1px solid #eee;
		}
		th {
			background: #f9f9f9;
			font-weight: 600;
			color: #333;
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>Genie Evaluation Dashboard</h1>

		{{ if .Error }}
			<div class="error">{{ .Error }}</div>
		{{ end }}

		{{ if .Metrics.RunID }}
			<div class="grid">
				<!-- Settlement Hallucination Summary -->
				<div class="card">
					<h3>Settlement Hallucination Detection</h3>
					{{ with .Metrics.SettlementAmountHallucinatedCI }}
						{{ $passRate := .Point }}
						{{ $class := "pass-rate" }}
						{{ if lt $passRate 0.85 }}{{ $class = "pass-rate low" }}{{ end }}
						{{ if lt $passRate 0.70 }}{{ $class = "pass-rate critical" }}{{ end }}
						<div class="metric">
							<span class="label">Pass Rate</span>
							<span class="value {{ $class }}">{{ printf "%.1f%%" (mul $passRate 100) }}</span>
						</div>
						<div class="confidence-interval">
							[{{ printf "%.1f%%" (mul .Lower 100) }}, {{ printf "%.1f%%" (mul .Upper 100) }}] at 95% CI
						</div>
					{{ end }}
				</div>

				<!-- Sample Size & Configuration -->
				<div class="card">
					<h3>Evaluation Config</h3>
					<div class="metric">
						<span class="label">Sample Size</span>
						<span class="value">{{ .Metrics.SampleSize }}</span>
					</div>
					<div class="metric">
						<span class="label">Sample Rate</span>
						<span class="value">{{ printf "%.2f%%" (mul .Metrics.SampleRate 100) }}</span>
					</div>
					<div class="metric">
						<span class="label">Confidence Level</span>
						<span class="value">{{ printf "%.0f%%" (mul .Metrics.ConfidenceLevel 100) }}</span>
					</div>
				</div>

				<!-- Drift Detection -->
				<div class="card">
					<h3>Drift Detection</h3>
					{{ if .Metrics.DriftDetected }}
						<span class="drift-badge detected">⚠ Drift Detected</span>
						<div style="margin-top: 10px; font-size: 0.9em; color: #c62828;">
							{{ .Metrics.DriftReason }}
						</div>
					{{ else }}
						<span class="drift-badge normal">✓ Normal</span>
					{{ end }}
					<div class="timestamp">Last run: {{ .LastRun.Format "15:04:05" }}</div>
				</div>
			</div>

			<!-- Judge Breakdown -->
			{{ if .Metrics.JudgeMetrics }}
				<div class="card">
					<h3>Judge Performance</h3>
					<table>
						<thead>
							<tr>
								<th>Judge</th>
								<th>Pass Rate</th>
								<th>Total Scored</th>
								<th>Passes</th>
								<th>Failures</th>
							</tr>
						</thead>
						<tbody>
							{{ range $name, $jm := .Metrics.JudgeMetrics }}
								<tr>
									<td><strong>{{ $name }}</strong></td>
									<td>{{ printf "%.1f%%" (mul $jm.PassRate 100) }}</td>
									<td>{{ $jm.TotalScored }}</td>
									<td>{{ $jm.PassCount }}</td>
									<td>{{ $jm.FailCount }}</td>
								</tr>
							{{ end }}
						</tbody>
					</table>
				</div>
			{{ end }}

			<!-- Run Details -->
			<div class="card">
				<h3>Run Details</h3>
				<div class="metric">
					<span class="label">Run ID</span>
					<span class="value" style="font-family: monospace; font-size: 0.95em;">{{ .Metrics.RunID }}</span>
				</div>
				<div class="metric">
					<span class="label">Started</span>
					<span class="value">{{ .Metrics.StartedAt.Format "2006-01-02 15:04:05" }}</span>
				</div>
				<div class="metric">
					<span class="label">Completed</span>
					<span class="value">{{ .Metrics.CompletedAt.Format "2006-01-02 15:04:05" }}</span>
				</div>
				{{ if not .Metrics.StartedAt.IsZero }}
					<div class="metric">
						<span class="label">Duration</span>
						<span class="value">{{ .Metrics.CompletedAt.Sub .Metrics.StartedAt | printf "%v" }}</span>
					</div>
				{{ end }}
			</div>
		{{ else }}
			<div class="card">
				<p>Waiting for first evaluation run...</p>
				<p style="color: #999; font-size: 0.9em;">Metrics will appear once evaluations complete.</p>
			</div>
		{{ end }}

		<div style="margin-top: 40px; text-align: center; color: #999; font-size: 0.9em;">
			<p>Genie Production Evaluation System</p>
			<p>Auto-refreshing every 30 seconds</p>
		</div>
	</div>

	<script>
		// Auto-refresh every 30 seconds
		setTimeout(function() {
			location.reload();
		}, 30000);
	</script>
</body>
</html>
`

		t := template.Must(template.New("dashboard").Parse(dashboardHTML))
		if err := t.Execute(w, state); err != nil {
			http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
		}
	}
}

// handleAPIMetrics serves metrics as JSON.
func handleAPIMetrics(state *DashboardState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state.mu.RLock()
		defer state.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")

		data := map[string]any{
			"metrics":   state.Metrics,
			"error":     state.Error,
			"last_run":  state.LastRun,
			"timestamp": time.Now(),
		}

		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("json encode error: %v", err)
		}
	}
}

// setupTelemetry initializes OpenTelemetry with optional Laminar export.
func setupTelemetry(ctx context.Context, withLaminar bool) (*observability.Telemetry, error) {
	cfg := observability.TelemetryConfig{
		ServiceName:    "genie-eval-dashboard",
		ServiceVersion: "1.0.0",
		Exporter:       observability.ExporterOTLP,
		OTLPInsecure:   true, // Local dev; set to false in production.
	}

	// If Laminar is enabled, configure OTLP endpoint.
	if withLaminar {
		apiKey := os.Getenv("LMNR_PROJECT_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("LMNR_PROJECT_API_KEY not set")
		}

		// Laminar OTLP endpoint.
		cfg.OTLPEndpoint = "api.laminar.run:443"

		// Configure trace exporter with Laminar headers.
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint("api.laminar.run:443"),
			otlptracegrpc.WithHeaders(map[string]string{
				"Authorization": fmt.Sprintf("Bearer %s", apiKey),
			}),
		}

		exporter, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("laminar trace exporter: %w", err)
		}

		res, _ := resource.Merge(
			resource.Default(),
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceName("genie-eval-dashboard"),
			),
		)

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)

		log.Printf("traces will be exported to Laminar")
	}

	tel, err := observability.SetupTelemetry(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry setup: %w", err)
	}

	log.Printf("telemetry initialized (exporter=%s)", cfg.Exporter)
	return tel, nil
}
