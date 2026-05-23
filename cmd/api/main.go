// Command api is Genie's service-edge binary. It exposes the multi-agent
// pipeline as a JSON HTTP API gated by JWT + RBAC, persists users/accounts/
// encrypted documents in Postgres, and ships traces/metrics over OTLP.
//
// Configuration is via environment variables (see README "Run via Docker"):
//
//	GENIE_HTTP_ADDR              default ":8080"
//	GENIE_JWT_SECRET             required, hex/base64 string
//	GENIE_KEK_BASE64             required, base64-encoded 32-byte key
//	GENIE_DB_DSN                 required, e.g. postgres://genie:genie@db:5432/genie?sslmode=disable
//	OTEL_EXPORTER_OTLP_ENDPOINT  optional, defaults to stdout exporter
//	GENIE_OTEL_INSECURE          "true" to skip TLS on the OTLP gRPC connection
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/analyzer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/anomaly"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/auditor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/currency"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/educator"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/enricher"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/forecaster"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/ingestor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/loan"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/macro"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/normalizer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/rates"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/recommender"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/reporter"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supervisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/orchestration"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/busio"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/handlers"
)

func main() {
	if err := run(); err != nil {
		// Fall through to slog so the error lands in structured logs.
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := observability.NewSlogLogger(slog.LevelInfo)

	// Telemetry — OTLP if endpoint set, else stdout.
	telCfg := observability.TelemetryConfig{
		ServiceName:    "genie-api",
		ServiceVersion: "0.1.0",
		Exporter:       observability.ExporterStdout,
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		telCfg.Exporter = observability.ExporterOTLP
		telCfg.OTLPInsecure = os.Getenv("GENIE_OTEL_INSECURE") == "true"
	}
	tel, err := observability.SetupTelemetry(ctx, telCfg)
	if err != nil {
		return err
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tel.Shutdown(sctx)
	}()

	// Database
	dsn := mustEnv("GENIE_DB_DSN")
	db, err := postgres.Open(ctx, postgres.Config{DSN: dsn, MaxConns: 10})
	if err != nil {
		return err
	}
	defer db.Close()

	// Crypto envelope
	if os.Getenv("GENIE_KEK_BASE64") == "" {
		return errors.New("GENIE_KEK_BASE64 is required")
	}
	enc := crypto.New(crypto.NewEnvKeyResolver("local-env-v1"))

	// JWT
	secret := []byte(mustEnv("GENIE_JWT_SECRET"))
	issuer := auth.NewIssuer(secret, "genie-api", []string{"genie-api"}, 60*time.Minute)

	// Bus + agents
	env := &orchestration.SimpleEnvironment{Logger: logger, Clock: observability.SystemClock{}}
	reg := registry.NewInMemory()
	bus := comm.NewInMemoryBus()
	evalStore := eval.NewInMemoryStore() // swap for postgres eval repo when added

	policy := governance.NewComposite(
		governance.MaxContentLengthPolicy{Max: 256 * 1024},
		governance.RequiredMetadataPolicy{
			AppliesTo: []string{supervisor.TypeQuestion},
			Required:  []string{protocol.MetaKeyUserID, "trace_id"},
		},
		governance.RBACPolicy{
			RequiredRolesByType: map[string][]string{
				supervisor.TypeQuestion: {"user", "advisor", "admin"},
			},
			AdminBypass: true,
		},
		governance.ClassificationPolicy{
			DefaultCeiling: protocol.ClassPII,
		},
		governance.PromptInjectionPolicy{},
	)

	register := func(a agent.Agent) {
		if err := reg.Register(ctx, a); err != nil {
			logger.Error("register", "error", err)
		}
	}
	register(ingestor.New())
	register(normalizer.New())
	register(enricher.New())
	register(analyzer.New())
	register(forecaster.New())
	register(anomaly.New())
	register(recommender.New())
	register(reporter.New())
	register(supervisor.New())
	register(currency.New())
	register(educator.New())
	register(macro.New())
	register(rates.New())
	register(loan.New())
	register(auditor.New(evalStore))

	orch := orchestration.NewOrchestrator(reg, bus, policy, env)
	orch.Start(ctx)

	bus.Subscribe("", auditor.NewHandler(evalStore))

	corr := busio.NewCorrelator(bus, "user")

	// HTTP wiring
	userRepo := postgres.NewUserRepo(db)
	acctRepo := postgres.NewAccountRepo(db)
	docRepo := postgres.NewDocumentRepo(db)

	deps := web.Deps{
		Issuer:    issuer,
		Logger:    logger,
		Users:     &handlers.Users{Repo: userRepo, Issuer: issuer},
		Accounts:  &handlers.Accounts{Repo: acctRepo},
		Documents: &handlers.Documents{Repo: docRepo, Encryptor: enc},
		Ask:       &handlers.Ask{Bus: bus, Correlator: corr, Documents: docRepo, Encryptor: enc, Timeout: 8 * time.Second},
		Health:    &handlers.Health{Ready: func() error { return db.Pool.Ping(ctx) }},
	}

	addr := os.Getenv("GENIE_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           web.NewRouter(deps),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown initiated")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("missing required env", "key", key)
		os.Exit(2)
	}
	return v
}
