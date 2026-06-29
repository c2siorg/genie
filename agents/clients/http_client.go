// Package clients implements client adapters for calling agents.
// Allows backend to call agents: locally (memory), remotely (HTTP), or via gRPC.
package clients

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/agents/core"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/resilience"
)

// HTTPClient calls agents via HTTP.
// Implements core.Agent interface, so it's compatible with pipeline orchestration.
type HTTPClient struct {
	baseURL string
	client  *http.Client
	name    string
	token   string              // X-Agent-Token sent on every request
	circ    *resilience.Circuit // trips after repeated failures, fails fast while open
}

// ClientOption configures an HTTPClient.
type ClientOption func(*HTTPClient)

// WithClientToken sets the shared-secret token sent as X-Agent-Token.
func WithClientToken(token string) ClientOption {
	return func(c *HTTPClient) { c.token = token }
}

// WithTimeout overrides the per-request timeout (default 30s).
func WithTimeout(d time.Duration) ClientOption {
	return func(c *HTTPClient) {
		if d > 0 {
			c.client.Timeout = d
		}
	}
}

// WithCircuit overrides the circuit-breaker threshold and cooldown.
func WithCircuit(threshold int, cooldown time.Duration) ClientOption {
	return func(c *HTTPClient) { c.circ = resilience.New(threshold, cooldown) }
}

// NewHTTPClient creates a new HTTP client for remote agent calls.
// baseURL: e.g., "http://profile-analyzer:8080".
//
// Defaults: 30s timeout, a pooled transport, and a circuit breaker that opens
// after 5 consecutive failures and cools down for 30s.
func NewHTTPClient(baseURL, agentName string, opts ...ClientOption) *HTTPClient {
	c := &HTTPClient{
		baseURL: baseURL,
		name:    agentName,
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
	for _, o := range opts {
		o(c)
	}
	return c
}

// setHeaders applies the common headers (content type + auth token) to a request.
func (c *HTTPClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Agent-Token", c.token)
	}
}

// Name returns the agent name.
func (c *HTTPClient) Name() string {
	return c.name
}

// Version gets version from remote agent.
func (c *HTTPClient) Version() string {
	info, err := c.getInfo(context.Background())
	if err != nil {
		return "unknown"
	}
	if v, ok := info["version"].(string); ok {
		return v
	}
	return "unknown"
}

// Health checks remote agent health.
func (c *HTTPClient) Health(ctx context.Context) error {
	resp, err := c.client.Get(fmt.Sprintf("%s/health", c.baseURL))
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check returned %d: %s", resp.StatusCode, body)
	}

	return nil
}

// Execute calls the remote agent through the circuit breaker. While the breaker
// is open it fails fast with resilience.ErrOpen instead of hammering a degraded
// agent; a successful call closes it again.
func (c *HTTPClient) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	// Build request
	req := core.AgentInput{
		Payload: input,
	}

	// Extract execution context if available
	if execCtx := core.FromContext(ctx); execCtx.UserID != "" {
		req.UserID = execCtx.UserID
		req.TraceID = execCtx.TraceID
		req.Metadata = execCtx.Metadata
	}

	// Marshal
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var result interface{}
	cerr := c.circ.Do(ctx, func(ctx context.Context) error {
		res, e := c.doExecute(ctx, body)
		if e != nil {
			return e
		}
		result = res
		return nil
	})
	if cerr != nil {
		if cerr == resilience.ErrOpen {
			return nil, core.NewExecutionError(c.name, "circuit_open", "agent circuit breaker open", cerr)
		}
		return nil, cerr
	}
	return result, nil
}

// doExecute performs a single HTTP round-trip to /execute (no breaker logic).
func (c *HTTPClient) doExecute(ctx context.Context, body []byte) (interface{}, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/execute", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(httpReq)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// A non-2xx status (e.g. 401 from auth, 5xx) is a failure the breaker counts.
	if httpResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(httpResp.Body)
		return nil, core.NewExecutionError(c.name, "http", fmt.Sprintf("status %d: %s", httpResp.StatusCode, b), nil)
	}

	var agentOutput core.AgentOutput
	if err := json.NewDecoder(httpResp.Body).Decode(&agentOutput); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if agentOutput.Status != "success" {
		return nil, core.NewExecutionError(c.name, "http", agentOutput.Error, nil)
	}
	return agentOutput.Result, nil
}

// ExecuteStream calls the remote agent and streams results.
// Returns a channel of AgentOutput events.
func (c *HTTPClient) ExecuteStream(ctx context.Context, input interface{}) (<-chan core.AgentOutput, error) {
	// Build request
	req := core.AgentInput{
		Payload: input,
	}

	if execCtx := core.FromContext(ctx); execCtx.UserID != "" {
		req.UserID = execCtx.UserID
		req.TraceID = execCtx.TraceID
		req.Metadata = execCtx.Metadata
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// POST to /execute-stream
	httpReq, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/execute-stream", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(httpReq)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		httpResp.Body.Close()
		return nil, fmt.Errorf("stream request returned %d", httpResp.StatusCode)
	}

	// Parse SSE stream
	out := make(chan core.AgentOutput, 10)
	go func() {
		defer close(out)
		defer httpResp.Body.Close()

		// Simple SSE parser
		scanner := newSSEScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if bytes.HasPrefix(line, []byte("data: ")) {
				data := line[6:]
				var event core.AgentOutput
				if err := json.Unmarshal(data, &event); err == nil {
					out <- event
				}
			}
		}
	}()

	return out, nil
}

// getInfo fetches agent info.
func (c *HTTPClient) getInfo(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.client.Get(fmt.Sprintf("%s/info", c.baseURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&info)
	return info, nil
}

// sseScanner parses Server-Sent Events line by line. The server emits each
// event as "data: <json>\n\n" (a data line followed by a blank line), so a
// line-oriented bufio.Scanner is sufficient: data lines are matched by the
// "data: " prefix check in ExecuteStream, and blank separator lines are
// skipped. Previously Scan() always returned false, which silently discarded
// every streamed event.
type sseScanner struct {
	s *bufio.Scanner
}

func newSSEScanner(r io.Reader) *sseScanner {
	return &sseScanner{s: bufio.NewScanner(r)}
}

func (s *sseScanner) Scan() bool    { return s.s.Scan() }
func (s *sseScanner) Bytes() []byte { return s.s.Bytes() }
