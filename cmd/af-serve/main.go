// Command af-serve is the framework-native HTTP edge for the agent-framework
// rewrite. It loads the board-approved policy, builds the full 58-agent governed
// registry, and serves /v1/ask + /v1/ai-inventory behind real JWT auth — with NO
// Postgres or message bus.
//
//	GENIE_JWT_SECRET=devsecret GENIE_AI_POLICY=config/ai-policy.example.yaml \
//	    GENIE_HTTP_ADDR=:8081 go run ./cmd/af-serve
//
// Example (mint a token with any HS256 signer using the same GENIE_JWT_SECRET,
// aud "genie-af-serve", iss "genie-af-serve"):
//
//	curl -s localhost:8081/v1/ai-inventory -H "Authorization: Bearer $TOKEN" | jq '. | length'   # 58
//	curl -s localhost:8081/v1/ask -H "Authorization: Bearer $TOKEN" -d '{"agent":"currency_converter",
//	     "input":"{\"amount_minor\":10000,\"from\":\"USD\",\"to\":\"INR\"}","classification":"public"}'
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/afg/catalog"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/policy"
)

func main() {
	policyPath := os.Getenv("GENIE_AI_POLICY")
	if policyPath == "" {
		policyPath = "config/ai-policy.example.yaml"
	}
	p, err := policy.Load(policyPath)
	if err != nil {
		log.Fatalf("load policy: %v", err)
	}
	gate := p.BuildComposite(nil)
	reg := catalog.Registry(gate)

	secret := mustEnv("GENIE_JWT_SECRET")
	issuer := auth.NewIssuer([]byte(secret), "genie-af-serve", []string{"genie-af-serve"}, 60*time.Minute)

	addr := os.Getenv("GENIE_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	log.Printf("af-serve: board policy %s, %d governed agents, listening on %s",
		p.Version, len(reg.Inventory()), addr)
	if err := http.ListenAndServe(addr, afg.NewHandler(reg, issuer)); err != nil {
		log.Fatal(err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env: %s", key)
	}
	return v
}
