package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

func TestSemanticMemory_PerUserIsolation(t *testing.T) {
	m := NewSemanticMemory(rag.NewHashEmbedder(64))
	ctx := context.Background()
	_ = m.Add(ctx, "u1", "goal-1", "save for emergency fund", nil)
	_ = m.Add(ctx, "u2", "goal-1", "buy a car", nil)

	hits, _ := m.Search(ctx, "u1", "emergency fund", 5)
	if len(hits) == 0 || !strings.Contains(hits[0].Text, "emergency") {
		t.Fatalf("u1 search missed own memory: %+v", hits)
	}
	// u2 should see nothing about emergency funds.
	hits, _ = m.Search(ctx, "u2", "emergency fund", 5)
	for _, h := range hits {
		if strings.Contains(h.Text, "emergency") {
			t.Fatal("user isolation breached")
		}
	}
}

type fakeSummariser struct{ s string }

func (f fakeSummariser) Summarise(_ context.Context, eps []Episode) (string, error) {
	return f.s, nil
}

func TestEpisodicMemory_ConsolidatesAtThreshold(t *testing.T) {
	mem := NewEpisodicMemory(2, fakeSummariser{s: "rolled up"})
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = mem.Append(ctx, "s1", "user", "msg")
	}
	summary, recent := mem.Snapshot("s1")
	if !strings.Contains(summary, "rolled up") {
		t.Fatalf("expected consolidation, summary=%q", summary)
	}
	if len(recent) == 4 {
		t.Fatalf("expected old half rolled up, still got %d episodes", len(recent))
	}
}
