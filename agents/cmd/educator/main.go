// Command educator runs the financial-education agent as a standalone HTTP
// service (/handle).
//
// It runs in glossary-only mode: definitions are served directly. RAG-backed
// citations (the FREE-AI Sutras) require an embedder (Ollama) and a seeded
// index; wire educator.New().WithRAG(idx) here once an embedder endpoint is
// configured for this pod. Glossary mode is a valid degraded mode and keeps the
// binary free of an Ollama dependency by default.
package main

import (
	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	educator "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/educator"
)

func main() {
	agentmain.RunLegacyAgent("educator", educator.New())
}
