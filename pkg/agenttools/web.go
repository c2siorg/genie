// web.go — Lesson 07: Web Search Tool
//
// WebSearch returns a tool that queries a real search provider when an API key
// is present, or falls back to DuckDuckGo Instant Answers for key-free use.
//
// Provider selection (first key found wins):
//
//	TAVILY_API_KEY   → Tavily Search API  (best for AI agents, structured output)
//	EXA_API_KEY      → Exa neural search  (semantic, full-text)
//	BRAVE_API_KEY    → Brave Search API   (index-based, good coverage)
//	(none)           → DuckDuckGo Instant Answer (free, limited — summaries only)
//
// Usage:
//
//	reg := agenttools.NewRegistry()
//	reg.Register(agenttools.WebSearch())
package agenttools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// WebSearch returns a web-search Tool that auto-selects the best available
// provider based on which API keys are set in the environment.
func WebSearch() Tool {
	return &ToolDef{
		ToolName:        "web_search",
		ToolDescription: "Search the web for current information: recent news, documentation, package versions, prices, events, or anything not in training data. Prefer specific queries over broad ones.",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query — be specific for better results",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default 5, max 10)",
				},
			},
			"required": []string{"query"},
		},
		Fn: webSearchFn,
	}
}

// WebSearchStub is kept for backward-compatibility. Prefer WebSearch().
func WebSearchStub() Tool { return WebSearch() }

func webSearchFn(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "error: query argument is required and must be a non-empty string", nil
	}
	query = strings.TrimSpace(query)

	maxResults := 5
	if n, ok := args["max_results"].(float64); ok && n > 0 {
		maxResults = int(n)
		if maxResults > 10 {
			maxResults = 10
		}
	}

	httpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	switch {
	case os.Getenv("TAVILY_API_KEY") != "":
		return tavilySearch(httpCtx, query, maxResults)
	case os.Getenv("EXA_API_KEY") != "":
		return exaSearch(httpCtx, query, maxResults)
	case os.Getenv("BRAVE_API_KEY") != "":
		return braveSearch(httpCtx, query, maxResults)
	default:
		return ddgSearch(httpCtx, query)
	}
}

// ─── Tavily ────────────────────────────────────────────────────────────────

func tavilySearch(ctx context.Context, query string, maxResults int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"api_key":              os.Getenv("TAVILY_API_KEY"),
		"query":                query,
		"max_results":          maxResults,
		"include_answer":       true,
		"include_raw_content":  false,
		"search_depth":         "basic",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf("tavily: request error: %v", err), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf("tavily: %v", err), nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Sprintf("tavily: read error: %v", err), nil
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Sprintf("tavily: http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))), nil
	}

	var out struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Sprintf("tavily: decode error: %v", err), nil
	}

	var sb strings.Builder
	if out.Answer != "" {
		fmt.Fprintf(&sb, "Summary: %s\n\n", out.Answer)
	}
	for i, r := range out.Results {
		fmt.Fprintf(&sb, "[%d] %s\n%s\n%s\n\n", i+1, r.Title, r.URL, truncate(r.Content, 300))
	}
	if sb.Len() == 0 {
		return fmt.Sprintf("no results found for %q", query), nil
	}
	return strings.TrimSpace(sb.String()), nil
}

// ─── Exa ───────────────────────────────────────────────────────────────────

func exaSearch(ctx context.Context, query string, maxResults int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"query":           query,
		"numResults":      maxResults,
		"useAutoprompt":   true,
		"contents":        map[string]any{"text": map[string]any{"maxCharacters": 500}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.exa.ai/search", bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf("exa: request error: %v", err), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", os.Getenv("EXA_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf("exa: %v", err), nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Sprintf("exa: read error: %v", err), nil
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Sprintf("exa: http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))), nil
	}

	var out struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Text    string `json:"text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Sprintf("exa: decode error: %v", err), nil
	}

	var sb strings.Builder
	for i, r := range out.Results {
		fmt.Fprintf(&sb, "[%d] %s\n%s\n%s\n\n", i+1, r.Title, r.URL, truncate(r.Text, 300))
	}
	if sb.Len() == 0 {
		return fmt.Sprintf("no results found for %q", query), nil
	}
	return strings.TrimSpace(sb.String()), nil
}

// ─── Brave ─────────────────────────────────────────────────────────────────

func braveSearch(ctx context.Context, query string, maxResults int) (string, error) {
	endpoint := fmt.Sprintf(
		"https://api.search.brave.com/res/v1/web/search?q=%s&count=%d&text_decorations=false",
		url.QueryEscape(query), maxResults,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Sprintf("brave: request error: %v", err), nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("X-Subscription-Token", os.Getenv("BRAVE_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf("brave: %v", err), nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Sprintf("brave: read error: %v", err), nil
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Sprintf("brave: http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))), nil
	}

	var out struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Sprintf("brave: decode error: %v", err), nil
	}

	var sb strings.Builder
	for i, r := range out.Web.Results {
		fmt.Fprintf(&sb, "[%d] %s\n%s\n%s\n\n", i+1, r.Title, r.URL, truncate(r.Description, 300))
	}
	if sb.Len() == 0 {
		return fmt.Sprintf("no results found for %q", query), nil
	}
	return strings.TrimSpace(sb.String()), nil
}

// ─── DuckDuckGo (free fallback) ────────────────────────────────────────────

func ddgSearch(ctx context.Context, query string) (string, error) {
	endpoint := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) +
		"&format=json&no_html=1&skip_disambig=1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Sprintf("ddg: request error: %v", err), nil
	}
	req.Header.Set("User-Agent", "genie-agent/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintf("web search error: %v", err), nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return fmt.Sprintf("ddg: read error: %v", err), nil
	}

	var out struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Answer       string `json:"Answer"`
		RelatedTopics []struct {
			Text string `json:"Text"`
			URL  string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Sprintf("ddg: decode error: %v — set TAVILY_API_KEY, EXA_API_KEY, or BRAVE_API_KEY for better results", err), nil
	}

	var sb strings.Builder
	if out.Answer != "" {
		fmt.Fprintf(&sb, "Answer: %s\n\n", out.Answer)
	}
	if out.AbstractText != "" {
		fmt.Fprintf(&sb, "%s\nSource: %s\n\n", out.AbstractText, out.AbstractURL)
	}
	for i, r := range out.RelatedTopics {
		if i >= 3 {
			break
		}
		if r.Text != "" {
			fmt.Fprintf(&sb, "- %s\n  %s\n", truncate(r.Text, 200), r.URL)
		}
	}

	if sb.Len() == 0 {
		return fmt.Sprintf(
			"no instant answer for %q — set TAVILY_API_KEY, EXA_API_KEY, or BRAVE_API_KEY for full web search results",
			query,
		), nil
	}
	return strings.TrimSpace(sb.String()), nil
}

// ─── helpers ───────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
