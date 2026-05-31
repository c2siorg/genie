// rag.go — Lesson 13: RAG (Retrieval-Augmented Generation) Tools
//
// Wraps pkg/rag as agenttools so the agent can search a knowledge base and
// ingest new documents at runtime — no code changes needed to add content.
//
// Usage:
//
//	index := rag.NewIndex(embedder, rag.NewMemoryStore())
//	reg   := agenttools.RAGTools(index)
//	runner := &agentic.Runner{Registry: reg}
//
// Combine with other tool registries:
//
//	reg := agenttools.FileTools()
//	for _, t := range agenttools.RAGTools(index).Names() { ... }
package agenttools

import (
	"context"
	"fmt"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

// SearchKnowledgeBase returns a tool that performs semantic search over a
// rag.Index. The agent calls this to retrieve relevant context before
// answering questions that require domain knowledge.
func SearchKnowledgeBase(index *rag.Index) Tool {
	return &ToolDef{
		ToolName:        "search_knowledge_base",
		ToolDescription: "Search the internal knowledge base for relevant information. Use this before answering domain-specific questions, looking up policies, regulations, or any content that may have been ingested.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Natural-language search query — be specific for better results",
				},
				"top_k": map[string]any{
					"type":        "integer",
					"description": "Number of results to return (default 5, max 10)",
				},
			},
			"required": []string{"query"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			query, ok := args["query"].(string)
			if !ok || strings.TrimSpace(query) == "" {
				return "error: query is required", nil
			}
			topK := 5
			if n, ok := args["top_k"].(float64); ok && n > 0 {
				topK = int(n)
				if topK > 10 {
					topK = 10
				}
			}

			results, err := index.Search(ctx, query, topK)
			if err != nil {
				return fmt.Sprintf("search error: %v", err), nil
			}
			if len(results) == 0 {
				return fmt.Sprintf("no results found for %q", query), nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Found %d result(s) for %q:\n\n", len(results), query)
			for i, r := range results {
				fmt.Fprintf(&sb, "[%d] %s (score: %.3f)\n", i+1, r.Title, r.Score)
				if r.Source != "" {
					fmt.Fprintf(&sb, "Source: %s\n", r.Source)
				}
				fmt.Fprintf(&sb, "%s\n\n", strings.TrimSpace(r.Text))
			}
			return strings.TrimSpace(sb.String()), nil
		},
	}
}

// IngestDocument returns a tool that adds a new document to the knowledge base
// at runtime. The agent can use this to learn from content provided during a
// session — paste in a policy, a changelog, or a reference article.
func IngestDocument(index *rag.Index) Tool {
	return &ToolDef{
		ToolName:        "ingest_document",
		ToolDescription: "Add a new document to the knowledge base so it can be retrieved later. Use this when the user provides reference material, policies, or documents that should inform future answers.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": map[string]any{
					"type":        "string",
					"description": "Unique identifier for the document (URL, filename, or citation key)",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Human-readable title for the document",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The full text content to ingest",
				},
				"chunk_size": map[string]any{
					"type":        "integer",
					"description": "Maximum characters per chunk (default 800)",
				},
			},
			"required": []string{"source", "title", "content"},
		},
		Fn: func(ctx context.Context, args map[string]any) (string, error) {
			source, _ := args["source"].(string)
			title, _ := args["title"].(string)
			content, _ := args["content"].(string)

			if strings.TrimSpace(source) == "" {
				return "error: source is required", nil
			}
			if strings.TrimSpace(content) == "" {
				return "error: content is required", nil
			}
			if title == "" {
				title = source
			}

			chunkSize := 800
			if n, ok := args["chunk_size"].(float64); ok && n > 0 {
				chunkSize = int(n)
			}

			n, err := index.IngestDocument(ctx, source, title, content, chunkSize)
			if err != nil {
				return fmt.Sprintf("ingest error: %v", err), nil
			}
			return fmt.Sprintf("ingested %d chunk(s) from %q (%s)", n, source, title), nil
		},
	}
}

// RAGTools returns a Registry pre-loaded with search_knowledge_base and
// ingest_document tools, both backed by the given index.
//
// The index can be pre-seeded before the agent starts:
//
//	idx := rag.NewIndex(embedder, rag.NewMemoryStore())
//	idx.IngestDocument(ctx, "policy.pdf", "AI Policy", policyText, 800)
//	reg := agenttools.RAGTools(idx)
func RAGTools(index *rag.Index) *Registry {
	r := NewRegistry()
	r.Register(SearchKnowledgeBase(index))
	r.Register(IngestDocument(index))
	return r
}
