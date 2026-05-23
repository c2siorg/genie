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

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/aa_fetcher"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/analyzer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/anomaly"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/auditor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/currency"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/educator"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/enricher"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/fallback"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/forecaster"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/ingestor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/loan"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/macro"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/normalizer"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/portfolio_advisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/rates"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/recommender"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/reporter"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/supervisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/tax_estimator"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/voice"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/comm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/eval"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/observability"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/orchestration"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/registry"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/busio"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/compliance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/constitution"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/incidents"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/mcp"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/policy"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/aibom"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/sovereignty"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/synth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/handlers"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
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

	consents := compliance.NewInMemoryLedger()
	auditLog := compliance.NewInMemoryAuditLog()
	_ = auditLog // reserved for incident-correlated audit entries.

	// Annexure V — board-approved AI policy lives in YAML, not Go code.
	policyPath := os.Getenv("GENIE_AI_POLICY")
	if policyPath == "" {
		policyPath = "config/ai-policy.example.yaml"
	}
	aiPolicy, err := policy.Load(policyPath)
	if err != nil {
		return err
	}
	composite := aiPolicy.BuildComposite(consents)
	homeRegion := sovereignty.Region(aiPolicy.Sovereignty.HomeRegion)

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
	register(macro.New())
	register(rates.New())
	register(loan.New())
	// educator + auditor are registered below in their RAG / LLM-judge form
	// so they pick up the citations and constitutional critique paths.

	mcpTokenRepo := postgres.NewMCPTokenRepo(db)
	register(portfolio_advisor.New(mcpTokenRepo, enc))

	// Fallback agents (Rec 21 — BCP for AI Systems). Each one emits a
	// "human review required" notice if its primary fails.
	register(fallback.NewFor("portfolio_advisor"))
	register(fallback.NewFor("recommender"))

	// India stack — AA fetcher (consented data sharing), voice ASR/TTS stub,
	// tax estimator.
	aaFI := aa_fetcher.NewInMemoryFIClient()
	register(aa_fetcher.New(aaFI, consents))
	register(voice.New(voice.EchoProvider{}))
	register(tax_estimator.New())

	// LLM + embedder stack — Mock by default, Ollama when GENIE_LLM=ollama.
	llmStack := buildLLMStack(ctx, logger)

	// RAG knowledge — seed the FREE-AI report's Sutras so the educator can
	// cite them at runtime. Production loads richer corpora.
	ragIndex := rag.NewIndex(llmStack.Embedder, rag.NewMemoryStore())
	seedFreeAISutras(ctx, ragIndex)
	register(educator.New().WithRAG(ragIndex))

	// Constitution + LLM-as-judge — only enable if the constitution YAML loads.
	if cst, err := constitution.Load("config/constitution.yaml"); err == nil {
		register(auditor.New(evalStore).WithJudge(llmStack.Provider, cst, llmStack.Model))
	}

	incidentStore := postgres.NewIncidentStore(db)

	orch := orchestration.NewOrchestrator(reg, bus, composite, env)
	orch.SetFallback("portfolio_advisor", "portfolio_advisor_fallback")
	orch.SetFallback("recommender", "recommender_fallback")
	orch.WithHooks(orchestration.Hooks{
		OnPolicyDeny: func(ctx context.Context, msg agent.Message, reason string) {
			_, _ = incidentStore.Create(ctx, incidents.Incident{
				UseCase:     msg.Type,
				Description: "policy denied message: " + reason,
				FailureMode: incidents.FailurePolicyDenied,
				Severity:    incidents.SeverityLow,
				Metadata:    map[string]any{"msg_id": msg.ID, "from": msg.From, "to": msg.To},
			})
		},
		OnAgentError: func(ctx context.Context, agentID string, msg agent.Message, err error) {
			_, _ = incidentStore.Create(ctx, incidents.Incident{
				UseCase:     agentID,
				Description: "agent error: " + err.Error(),
				FailureMode: incidents.FailureAgentError,
				Severity:    incidents.SeverityModerate,
				Metadata:    map[string]any{"msg_id": msg.ID, "type": msg.Type},
			})
		},
	})
	orch.Start(ctx)

	bus.Subscribe("", auditor.NewHandler(evalStore))

	corr := busio.NewCorrelator(bus, "user")
	// Replies from agents invoked via MCP BusTool are addressed back to "mcp".
	mcpCorr := busio.NewCorrelator(bus, "mcp")
	// EventTap broadcasts every message tagged with a trace_id to SSE consumers.
	eventTap := busio.NewEventTap(bus)

	// HTTP wiring
	userRepo := postgres.NewUserRepo(db)
	acctRepo := postgres.NewAccountRepo(db)
	docRepo := postgres.NewDocumentRepo(db)

	mcpServer := mcp.NewServer(
		mcp.BusTool("explain_finance", "Explain a finance concept",
			map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
			bus, mcpCorr, "financial_educator", "explain_finance"),
		mcp.BusTool("macro_context", "Return a one-line macro outlook for a region",
			map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
			bus, mcpCorr, "macro_research", "macro_context"),
		mcp.BusTool("rate_outlook", "Return a central-bank rate outlook",
			map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
			bus, mcpCorr, "rate_watcher", "rate_outlook"),
	)

	fallbacks := map[string]string{
		"portfolio_advisor": "portfolio_advisor_fallback",
		"recommender":       "recommender_fallback",
	}

	deps := web.Deps{
		Issuer:    issuer,
		Logger:    logger,
		Users:     &handlers.Users{Repo: userRepo, Issuer: issuer},
		Accounts:  &handlers.Accounts{Repo: acctRepo},
		Documents: &handlers.Documents{Repo: docRepo, Encryptor: enc},
		Ask: &handlers.Ask{
			Bus:                bus,
			Correlator:         corr,
			Documents:          docRepo,
			Encryptor:          enc,
			Timeout:            8 * time.Second,
			AIDisclosureBanner: aiPolicy.Consumer.AIDisclosureBanner,
		},
		AskStream: &handlers.AskStream{
			Bus:                bus,
			Tap:                eventTap,
			Documents:          docRepo,
			Encryptor:          enc,
			Timeout:            10 * time.Second,
			AIDisclosureBanner: aiPolicy.Consumer.AIDisclosureBanner,
		},
		ChatWS: &handlers.ChatWS{
			Bus:                bus,
			Tap:                eventTap,
			Documents:          docRepo,
			Encryptor:          enc,
			Timeout:            12 * time.Second,
			AIDisclosureBanner: aiPolicy.Consumer.AIDisclosureBanner,
		},
		Health: &handlers.Health{Ready: func() error {
			if err := db.Pool.Ping(ctx); err != nil {
				return err
			}
			if llmStack.Probe != nil {
				probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				if err := llmStack.Probe(probeCtx); err != nil {
					return err
				}
			}
			return nil
		}},
		MCPTokens: &handlers.MCPTokens{Repo: mcpTokenRepo, Encryptor: enc},
		MCPServer: mcpServer,
		Incidents: &handlers.Incidents{Store: incidentStore},
		Inventory: &handlers.Inventory{Reg: reg, Fallbacks: fallbacks},
		Disclosures: &handlers.Disclosures{
			Reg:                  reg,
			PolicyVersion:        aiPolicy.Version,
			PolicyApprovedOn:     aiPolicy.BoardApproved,
			Principles:           aiPolicy.Principles,
			HomeRegion:           string(homeRegion),
			IncidentReportingURL: "/v1/incidents",
		},
		RateLimit: mid.NewRateLimit(60, 1.0), // 60-req burst, 1/sec refill
		AIBOM:     &handlers.AIBOM{Reg: reg, Builder: aibom.NewBuilder()},
		Feedback:  &handlers.Feedback{Store: synth.NewInMemoryFeedbackStore()},
	}

	// Retention purge — runs every 6h (Rec 15).
	db.StartRetentionJob(ctx, 6*time.Hour, logger.Info)

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

// seedFreeAISutras populates the RAG index with the 7 Sutras so the educator
// can cite the FREE-AI report at runtime. Real deployments load richer
// corpora (full RBI rulebook, SEBI circulars, internal SOPs).
func seedFreeAISutras(ctx context.Context, idx *rag.Index) {
	docs := []struct{ src, title, body string }{
		{"free-ai#sutra-1", "Sutra 1 — Trust is the Foundation", "Trust is non-negotiable and should remain uncompromised. AI must reinforce, not erode, public trust in the financial system."},
		{"free-ai#sutra-2", "Sutra 2 — People First", "AI should augment human decision-making but defer to human judgment and citizen interest. Disclose when the user is interacting with AI."},
		{"free-ai#sutra-3", "Sutra 3 — Innovation over Restraint", "Responsible innovation aligned with societal values should be prioritised over cautionary restraint when all else is equal."},
		{"free-ai#sutra-4", "Sutra 4 — Fairness and Equity", "AI outcomes must be fair and non-discriminatory. AI should be used to advance financial inclusion, not accentuate exclusion."},
		{"free-ai#sutra-5", "Sutra 5 — Accountability", "The entity deploying AI is accountable for outcomes; accountability cannot be delegated to the model."},
		{"free-ai#sutra-6", "Sutra 6 — Understandable by Design", "Explainability is a core design feature, not an afterthought. Outputs must carry plain-language rationales."},
		{"free-ai#sutra-7", "Sutra 7 — Safety, Resilience, and Sustainability", "AI systems must be secure, resilient to attacks, and energy efficient."},
	}
	for _, d := range docs {
		_, _ = idx.IngestDocument(ctx, d.src, d.title, d.body, 600)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("missing required env", "key", key)
		os.Exit(2)
	}
	return v
}
