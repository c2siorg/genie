// Command af-hello is the Phase 1 vertical-slice demo for the agent-framework
// rewrite. It stands up the governed, framework-native currency agent on the REAL
// board-approved policy and shows the governance gate allowing a benign request and
// denying a secret-classified one, then constructs the on-prem Ollama agent through
// the same single door. Run from the repo root:
//
//	go run ./cmd/af-hello
//	OLLAMA_LIVE=1 go run ./cmd/af-hello   # also invokes a local Ollama
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/policy"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

func main() {
	policyPath := os.Getenv("GENIE_AI_POLICY")
	if policyPath == "" {
		policyPath = "config/ai-policy.example.yaml"
	}
	p, err := policy.Load(policyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load policy:", err)
		os.Exit(1)
	}
	gate := p.BuildComposite(nil)
	fmt.Printf("board policy: version=%s owner=%q home_region=%s\n\n", p.Version, p.Owner, p.Sovereignty.HomeRegion)

	ctx := context.Background()
	cur := afg.NewCurrencyAgent(gate)
	const req = `{"amount_minor":5000,"from":"USD","to":"INR"}`

	// (1) ALLOW — benign, public-classified conversion passes the full composite.
	allow := protocol.Message{
		From: "user", Role: protocol.RoleUser, Type: "convert_currency", Content: req,
		Metadata: map[string]any{
			protocol.MetaKeyUserRoles:      "user",
			protocol.MetaKeyClassification: string(protocol.ClassPublic),
			protocol.MetaKeyUserID:         "u1",
		},
	}
	if resp, err := cur.RunText(afg.WithGovMessage(ctx, allow), req).Collect(); err != nil {
		fmt.Println("[allow] UNEXPECTED error:", err)
	} else {
		fmt.Println("[allow] 5000 USD -> INR :", afg.ResponseText(resp))
	}

	// (2) DENY — same request tagged 'secret' exceeds the recipient's internal
	// ceiling; the gate short-circuits before the currency logic runs.
	deny := protocol.Message{
		From: "user", Role: protocol.RoleUser, Type: "convert_currency", Content: req,
		Metadata: map[string]any{protocol.MetaKeyClassification: string(protocol.ClassSecret)},
	}
	if resp, err := cur.RunText(afg.WithGovMessage(ctx, deny), req).Collect(); err != nil {
		fmt.Println("[deny ] secret-classified       :", err)
	} else {
		fmt.Println("[deny ] UNEXPECTED allow:", afg.ResponseText(resp))
	}

	// (3) OLLAMA — governed LLM agent on the on-prem OpenAI-compatible endpoint.
	oll := afg.NewGovernedOllama(gate, "fx_explainer", "You explain FX conversions in one sentence.", "llama3.1")
	if os.Getenv("OLLAMA_LIVE") == "1" {
		const q = "In one sentence: why is 5000 USD about 415000 INR at rate 83?"
		msg := protocol.Message{
			From: "user", Role: protocol.RoleUser, Type: "convert_currency", Content: q,
			Metadata: map[string]any{protocol.MetaKeyClassification: string(protocol.ClassPublic)},
		}
		if resp, err := oll.RunText(afg.WithGovMessage(ctx, msg), q).Collect(); err != nil {
			fmt.Println("[ollama] error:", err)
		} else {
			fmt.Println("[ollama]", afg.ResponseText(resp))
		}
	} else {
		fmt.Printf("[ollama] governed agent constructed OK (%p) — set OLLAMA_LIVE=1 to invoke\n", oll)
	}
}
