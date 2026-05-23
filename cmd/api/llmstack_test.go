package main

import (
	"context"
	"strings"
	"testing"
)

type recordLogger struct {
	entries []map[string]any
}

func (r *recordLogger) Info(msg string, args ...any) {
	e := map[string]any{"msg": msg}
	for i := 0; i+1 < len(args); i += 2 {
		k, _ := args[i].(string)
		e[k] = args[i+1]
	}
	r.entries = append(r.entries, e)
}

func TestBuildLLMStack_MockByDefault(t *testing.T) {
	t.Setenv("GENIE_LLM", "")
	rl := &recordLogger{}
	s := buildLLMStack(context.Background(), rl)
	if s.Provider == nil || s.Embedder == nil {
		t.Fatal("expected provider and embedder to be set")
	}
	if s.Provider.Name() != "mock" {
		t.Fatalf("expected mock provider, got %q", s.Provider.Name())
	}
	if s.Probe != nil {
		t.Fatal("mock stack should not install a probe")
	}
}

func TestBuildLLMStack_OllamaWrapped(t *testing.T) {
	t.Setenv("GENIE_LLM", "ollama")
	t.Setenv("GENIE_OLLAMA_URL", "http://unreachable:0")
	rl := &recordLogger{}
	s := buildLLMStack(context.Background(), rl)
	if s.Provider == nil || s.Embedder == nil {
		t.Fatal("expected provider and embedder to be set")
	}
	// Provider name accumulates wrapper suffixes; ensure Ollama is at the base.
	if !strings.Contains(s.Provider.Name(), "ollama") {
		t.Fatalf("expected ollama in name chain, got %q", s.Provider.Name())
	}
	// Circuit breaker, deadline, cache, cost wrappers should all be present.
	for _, want := range []string{"+cost", "+cache", "+budget", "+deadline", "+circuit"} {
		if !strings.Contains(s.Provider.Name(), want) {
			t.Errorf("wrapper %s missing from chain %q", want, s.Provider.Name())
		}
	}
	if s.Probe == nil {
		t.Fatal("ollama stack should install a readiness probe")
	}
}
