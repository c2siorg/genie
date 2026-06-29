// Command portfolio_advisor runs the portfolio advisor agent as a standalone
// HTTP service (/handle).
//
// Unlike the other agents this one has real infrastructure dependencies: a
// Postgres connection (to read the user's encrypted MCP/Kite token) and a KMS
// encryptor. It connects with a SMALL pool (MaxConns: 2) per the decoupling
// plan's connection-budget math; deploy behind PgBouncer at scale.
//
// Required env: GENIE_DB_DSN (postgres://...). The KEK id matches cmd/api.
package main

import (
	"context"
	"log"
	"os"

	agentmain "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/cmd/template"
	portfolio_advisor "github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/portfolio_advisor"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/storage/postgres"
)

func main() {
	dsn := os.Getenv("GENIE_DB_DSN")
	if dsn == "" {
		log.Fatal("portfolio_advisor: GENIE_DB_DSN is required")
	}
	db, err := postgres.Open(context.Background(), postgres.Config{DSN: dsn, MaxConns: 2})
	if err != nil {
		log.Fatalf("portfolio_advisor: open database: %v", err)
	}
	enc := crypto.New(crypto.NewEnvKeyResolver("local-env-v1"))
	repo := postgres.NewMCPTokenRepo(db)

	agentmain.RunLegacyAgent("portfolio_advisor", portfolio_advisor.New(repo, enc))
}
