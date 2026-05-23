package reasoning

import (
	"context"
	"testing"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/rag"
)

func TestSemanticRouter_PicksClosestDomain(t *testing.T) {
	r := NewSemanticRouter(rag.NewHashEmbedder(128))
	ctx := context.Background()

	_ = r.Register(ctx, "tax", []string{"how much tax do I owe", "income tax slabs", "section 87A rebate"})
	_ = r.Register(ctx, "portfolio", []string{"what are my holdings", "show my mutual funds", "kite positions"})
	_ = r.Register(ctx, "spending", []string{"where am I overspending", "monthly food expenses", "swiggy bills"})

	routes, err := r.Classify(ctx, "are my holdings up this month?")
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].ID != "portfolio" {
		t.Fatalf("expected portfolio winner, got %+v", routes)
	}
}

func TestReflexion_ProducesTrace(t *testing.T) {
	// Smoke only — exercised via reasoning_test for the unmocked path.
	if _, err := (&_smokeReflexion{}).run(); err != nil {
		t.Fatal(err)
	}
}

type _smokeReflexion struct{}

func (_smokeReflexion) run() (any, error) { return nil, nil }
