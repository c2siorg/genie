// Package agentmain provides RunAgent, the shared entry-point harness every
// standalone agent binary uses. A per-agent main is then just:
//
//	func main() { agentmain.RunAgent("profile-analyzer", profile_analyzer.NewAgent()) }
//
// The harness owns the operational concerns so individual agents stay pure:
//   - reads PORT (default 8080) and GENIE_AGENT_TOKEN from the environment
//   - serves the hardened transports.HTTPServer (token auth, body limits)
//   - handles SIGINT/SIGTERM and drains in-flight requests with a 30s budget
//
// This 30s graceful-shutdown budget pairs with the Kubernetes
// terminationGracePeriodSeconds (set to 35 in the deployment manifests) so the
// pod finishes in-flight work before SIGKILL — important because the distroless
// runtime image has no shell for a preStop hook.
package agentmain

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/transports"
	pkgagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
)

const shutdownBudget = 30 * time.Second

// RunAgent serves an agents/core.Agent (the Execute contract, used by the advisor
// agents) over POST /execute. It blocks until SIGINT/SIGTERM, then drains.
func RunAgent(name string, agent core.Agent) {
	token := tokenOrWarn(name)
	serve(name, transports.NewHTTPServer(agent, transports.WithToken(token)))
}

// RunLegacyAgent serves a pkg/agent.Agent (the HandleMessage contract, used by
// the ~31 registered legacy agents) over POST /handle. It is the counterpart to
// RunAgent for message-driven agents; the backend reaches it via
// pkg/registry.HTTPRegistryAgent.
func RunLegacyAgent(name string, agent pkgagent.Agent) {
	token := tokenOrWarn(name)
	serve(name, transports.NewLegacyHTTPServer(agent, transports.WithToken(token)))
}

// tokenOrWarn reads GENIE_AGENT_TOKEN and warns loudly if it is empty (auth off).
func tokenOrWarn(name string) string {
	token := os.Getenv("GENIE_AGENT_TOKEN")
	if token == "" {
		log.Printf("[%s] WARNING: GENIE_AGENT_TOKEN is empty — endpoint is UNAUTHENTICATED (dev mode)", name)
	}
	return token
}

// serve runs handler on PORT (default 8080) until SIGINT/SIGTERM, then shuts down
// gracefully within shutdownBudget. It never returns under normal operation; it
// logs and exits non-zero on a fatal startup error.
func serve(name string, handler http.Handler) {
	port := getenv("PORT", "8080")
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // slowloris guard
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("[%s] listening on :%s", name, port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		log.Fatalf("[%s] server error: %v", name, err)
	case <-ctx.Done():
		log.Printf("[%s] shutdown signal received; draining (budget %s)", name, shutdownBudget)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownBudget)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[%s] graceful shutdown failed: %v", name, err)
	} else {
		log.Printf("[%s] stopped cleanly", name)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
