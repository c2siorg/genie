package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	pkgagent "github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/agent"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/resilience"
)

// HTTPRegistryAgent is the backend-side adapter that lets cmd/api register a
// REMOTE agent in the in-process registry without importing the agent package.
// It implements pkg/agent.Agent (ID/Name/Capabilities/HandleMessage), but
// HandleMessage proxies to a remote LegacyHTTPServer's POST /handle endpoint.
//
// This is the inverse of agents/adapter.MessageBridgeAdapter:
//   - MessageBridgeAdapter: HandleMessage -> Execute   (per-agent binary side)
//   - HTTPRegistryAgent:    HandleMessage -> HTTP       (backend registry side)
//
// Sub-orchestration is preserved: the remote agent returns []protocol.Message,
// HandleMessage returns them to the orchestrator, which republishes each to the
// bus — so chains like supervisor -> analyzer -> reporter keep working.
type HTTPRegistryAgent struct {
	id           string
	name         string
	capabilities []string // MUST match the original agent's ring capabilities
	baseURL      string
	token        string
	client       *http.Client
	circ         *resilience.Circuit
}

// compile-time assertion that HTTPRegistryAgent satisfies pkg/agent.Agent.
var _ pkgagent.Agent = (*HTTPRegistryAgent)(nil)

// AgentDef describes one remote agent for registration.
type AgentDef struct {
	ID           string
	Name         string
	Capabilities []string
}

// NewHTTPRegistryAgent builds a proxy for a remote agent. baseURL is e.g.
// "http://genie-analyzer:8080"; token is the shared GENIE_AGENT_TOKEN.
func NewHTTPRegistryAgent(def AgentDef, baseURL, token string) *HTTPRegistryAgent {
	return &HTTPRegistryAgent{
		id:           def.ID,
		name:         def.Name,
		capabilities: def.Capabilities,
		baseURL:      baseURL,
		token:        token,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		circ: resilience.New(5, 30*time.Second),
	}
}

func (a *HTTPRegistryAgent) ID() string             { return a.id }
func (a *HTTPRegistryAgent) Name() string           { return a.name }
func (a *HTTPRegistryAgent) Capabilities() []string { return a.capabilities }

// HandleMessage proxies the message to the remote agent's POST /handle endpoint
// through the circuit breaker, and returns the remote agent's response messages.
func (a *HTTPRegistryAgent) HandleMessage(ctx context.Context, msg pkgagent.Message, env pkgagent.Environment) ([]pkgagent.Message, error) {
	var out []pkgagent.Message
	err := a.circ.Do(ctx, func(ctx context.Context) error {
		msgs, e := a.postHandle(ctx, msg)
		if e != nil {
			return e
		}
		out = msgs
		return nil
	})
	if err != nil {
		if err == resilience.ErrOpen {
			if env != nil {
				env.Logf("[proxy:%s] circuit open, rejecting message %s", a.id, msg.ID)
			}
			return nil, fmt.Errorf("agent %s circuit open", a.id)
		}
		return nil, err
	}
	return out, nil
}

// postHandle performs a single POST /handle round-trip.
func (a *HTTPRegistryAgent) postHandle(ctx context.Context, msg pkgagent.Message) ([]pkgagent.Message, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("agent %s marshal: %w", a.id, err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/handle", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("agent %s request: %w", a.id, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("X-Agent-Token", a.token)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent %s http: %w", a.id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent %s status %d: %s", a.id, resp.StatusCode, b)
	}

	var out []pkgagent.Message
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("agent %s decode: %w", a.id, err)
	}
	return out, nil
}
