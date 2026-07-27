package afg

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/governance"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"

	"github.com/microsoft/agent-framework-go/agent"
	"github.com/microsoft/agent-framework-go/message"
	"github.com/microsoft/agent-framework-go/provider/openaiprovider"
	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// retDefaultTopK is used when a caller doesn't specify how many chunks to retrieve.
const retDefaultTopK = 5

// Retriever is the minimal grounding lookup a RetrievalMiddleware needs. It
// returns plain chunk texts (no scores/metadata) so callers can swap in any
// retrieval backend — pkg/rag today, a hosted vector DB tomorrow — without
// this package caring about the shape.
type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]string, error)
}

// RetrievalMiddleware grounds an agent run in retrieved context. It MUST be
// placed AFTER GovMiddleware in an agent's Middlewares slice (gate outermost,
// retrieval just inside it): the framework runs ContextProviders — and any
// middleware — only via the chain built at Run time, and middleware ordering
// is call order, so a mis-ordered slice would let retrieval fire on a message
// the gate was about to deny. See NewGovernedAdvisory, which wires this
// correctly, for the sanctioned construction path.
//
// On a retrieval error this fails closed: it returns an error stream and does
// NOT call next. An advisory answer produced without its grounding would be a
// silent, unlabeled failure — worse than refusing to answer.
type RetrievalMiddleware struct {
	R    Retriever
	TopK int
}

// Run implements agent.Middleware.
func (m RetrievalMiddleware) Run(next agent.RunFunc, ctx context.Context, msgs []*message.Message, opts ...agent.Option) iter.Seq2[*agent.ResponseUpdate, error] {
	topK := m.TopK
	if topK <= 0 {
		topK = retDefaultTopK
	}
	query := messagesText(msgs)
	chunks, err := m.R.Retrieve(ctx, query, topK)
	if err != nil {
		return errStream(fmt.Errorf("retrieval failed: %w", err))
	}
	if len(chunks) == 0 {
		return next(ctx, msgs, opts...)
	}
	ctxMsg := &message.Message{
		Role:     message.RoleSystem,
		Contents: message.Contents{&message.TextContent{Text: ragContextBlock(chunks)}},
	}
	augmented := make([]*message.Message, 0, len(msgs)+1)
	augmented = append(augmented, ctxMsg)
	augmented = append(augmented, msgs...)
	return next(ctx, augmented, opts...)
}

// ragContextBlock joins retrieved chunks under a header the model can key off
// of when distinguishing grounding material from the conversation proper.
func ragContextBlock(chunks []string) string {
	return "Context:\n" + strings.Join(chunks, "\n---\n")
}

// IndexRetriever adapts a *rag.Index (pkg/rag) to the Retriever interface used
// by RetrievalMiddleware.
type IndexRetriever struct {
	Idx *rag.Index
}

// Retrieve implements Retriever.
func (r IndexRetriever) Retrieve(ctx context.Context, query string, topK int) ([]string, error) {
	scored, err := r.Idx.Search(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(scored))
	for _, sc := range scored {
		out = append(out, sc.Text)
	}
	return out, nil
}

// NewGovernedAdvisory builds an LLM-backed advisory agent on the on-prem
// Ollama OpenAI-compatible endpoint (mirroring NewGovernedOllama's client
// setup) with RAG grounding injected INSIDE the governance gate: Middlewares
// is [GovMiddleware, RetrievalMiddleware] in that order, so the gate runs
// first (outermost, call order) and only allowed requests ever reach
// retrieval or the model. This is NOT one of the two constructors named in
// the package doc (NewGovernedDeterministic / NewGovernedOllama) but it
// follows the same single-door discipline: it lives in pkg/afg, it puts
// GovMiddleware outermost, and singledoor_test.go's exemption for this
// package covers it.
func NewGovernedAdvisory(gate governance.Policy, name, instructions, model string, r Retriever, topK int) *agent.Agent {
	client := openai.NewClient(
		option.WithBaseURL(ollamaBaseURL()),
		option.WithAPIKey(ollamaAPIKey()),
	)
	return openaiprovider.NewChatCompletionsAgent(client, openaiprovider.AgentConfig{
		Config: agent.Config{
			Name: name,
			Middlewares: []agent.Middleware{
				GovMiddleware{Gate: gate},
				RetrievalMiddleware{R: r, TopK: topK},
			},
			DisableFuncAutoCall: true,
		},
		Instructions: instructions,
		Model:        model,
	})
}

// NewOfflineRetriever builds a deterministic, offline Retriever (no network,
// no external embedding service) from a fixed set of documents. It exists for
// tests and offline demos of NewGovernedAdvisory — the hash embedder and
// in-memory store make retrieval reproducible without Ollama or any vector DB
// running.
func NewOfflineRetriever(docs []string) Retriever {
	idx := rag.NewIndex(rag.NewHashEmbedder(0), rag.NewMemoryStore())
	ctx := context.Background()
	for i, doc := range docs {
		_, _ = idx.IngestDocument(ctx, fmt.Sprintf("offline-doc-%d", i), "", doc, 0)
	}
	return IndexRetriever{Idx: idx}
}
