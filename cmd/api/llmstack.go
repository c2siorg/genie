package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/llm"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

// LLMStack bundles the LLM provider and embedder selected by env config.
// Built once at startup and shared across handlers / agents.
type LLMStack struct {
	Provider llm.Provider
	Embedder rag.Embedder
	Model    string

	// Health probe — non-nil when an external runtime needs liveness checks
	// (e.g. Ollama). nil for the Mock path.
	Probe func(ctx context.Context) error
}

// buildLLMStack picks the LLM stack from env. Defaults to Mock + HashEmbedder
// so demo deployments work with zero configuration.
//
// Env:
//
//	GENIE_LLM              "mock" (default) | "ollama"
//	GENIE_OLLAMA_URL       Ollama HTTP root, default http://localhost:11434
//	GENIE_OLLAMA_CHAT      chat model id, default "llama3.2:1b"
//	GENIE_OLLAMA_EMBED     embedding model id, default "nomic-embed-text"
//	GENIE_LLM_BUDGET       daily token cap per principal, default 1000000
//	GENIE_LLM_CACHE_TTL    cache TTL in seconds, default 600
//	GENIE_LLM_TIMEOUT      per-call timeout seconds, default 30
//	GENIE_LLM_CIRCUIT      consecutive-error threshold, default 5
func buildLLMStack(ctx context.Context, logger interface {
	Info(msg string, args ...any)
}) LLMStack {
	kind := strings.ToLower(os.Getenv("GENIE_LLM"))
	if kind == "" {
		kind = "mock"
	}

	switch kind {
	case "ollama":
		return ollamaStack(ctx, logger)
	default:
		return mockStack(logger)
	}
}

func mockStack(logger interface{ Info(msg string, args ...any) }) LLMStack {
	logger.Info("llm.stack", "kind", "mock")
	m := llm.NewMock()
	// Seed a couple of deterministic responses so smoke tests get sensible
	// output even without a real model.
	m.Responses = []llm.CompletionResponse{
		{Text: "SCORE: 9\nREASONING: aligned with sutras", Usage: llm.Usage{PromptTokens: 50, CompletionTokens: 10}},
	}
	return LLMStack{
		Provider: m,
		Embedder: rag.NewHashEmbedder(256),
		Model:    "mock-model",
	}
}

func ollamaStack(ctx context.Context, logger interface{ Info(msg string, args ...any) }) LLMStack {
	url := envDefault("GENIE_OLLAMA_URL", "http://localhost:11434")
	chatModel := envDefault("GENIE_OLLAMA_CHAT", "llama3.2:1b")
	embedModel := envDefault("GENIE_OLLAMA_EMBED", "nomic-embed-text")

	logger.Info("llm.stack",
		"kind", "ollama",
		"url", url,
		"chat_model", chatModel,
		"embed_model", embedModel,
	)

	// Provider stack (inside → outside): Ollama → Cost → Cache → Budget → Deadline → Circuit.
	base := llm.NewOllamaProvider(url, chatModel)
	withCost := llm.NewCostObserver(base, 0, 0) // local model, costs are non-monetary
	cached := llm.NewCachedProvider(withCost, time.Duration(envInt("GENIE_LLM_CACHE_TTL", 600))*time.Second)
	budgeted := llm.NewBudgeted(cached, llm.NewInMemoryBudget(), envInt("GENIE_LLM_BUDGET", 1_000_000))
	deadlined := llm.NewDeadline(budgeted, time.Duration(envInt("GENIE_LLM_TIMEOUT", 30))*time.Second)
	stack := llm.NewCircuit(deadlined, envInt("GENIE_LLM_CIRCUIT", 5), 30*time.Second)

	emb := rag.NewOllamaEmbedder(url, embedModel)

	return LLMStack{
		Provider: stack,
		Embedder: emb,
		Model:    chatModel,
		Probe:    ollamaProbe(url),
	}
}

// ollamaProbe returns a readiness check that GETs /api/tags. Used by
// Health.Readiness so /readyz only goes 200 once Ollama is reachable.
func ollamaProbe(url string) func(ctx context.Context) error {
	client := &http.Client{Timeout: 2 * time.Second}
	endpoint := strings.TrimRight(url, "/") + "/api/tags"
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		return nil
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
