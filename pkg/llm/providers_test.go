package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropic_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			http.Error(w, "missing api key", http.StatusUnauthorized)
			return
		}
		var got anthropicRequest
		_ = json.NewDecoder(r.Body).Decode(&got)
		// System should be split out of messages.
		if !strings.Contains(got.System, "you are helpful") {
			t.Errorf("system not extracted: %+v", got)
		}
		resp := anthropicResponse{Model: got.Model}
		resp.Content = []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{{Type: "text", Text: "ok"}}
		resp.Usage.InputTokens = 5
		resp.Usage.OutputTokens = 2
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAnthropic("test-key", "claude-x")
	p.BaseURL = srv.URL
	r, err := p.Complete(context.Background(), CompletionRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "you are helpful"},
			{Role: RoleUser, Content: "hello"},
		},
		Residency: Residency{AllowCrossBorder: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "ok" || r.Usage.PromptTokens != 5 {
		t.Fatalf("unexpected resp: %+v", r)
	}
}

func TestOpenAI_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		resp := openaiResponse{Model: "gpt-x"}
		resp.Choices = []struct {
			Message      openaiMessage `json:"message"`
			FinishReason string        `json:"finish_reason"`
		}{{Message: openaiMessage{Role: "assistant", Content: "yo"}}}
		resp.Usage.PromptTokens = 4
		resp.Usage.CompletionTokens = 2
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	p := NewOpenAI("test-key", "gpt-x")
	p.BaseURL = srv.URL
	r, err := p.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Residency: Residency{AllowCrossBorder: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "yo" {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestGemini_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		resp := geminiResponse{ModelVersion: "gemini-x"}
		resp.Candidates = []struct {
			Content      geminiContent `json:"content"`
			FinishReason string        `json:"finishReason"`
		}{{Content: geminiContent{Parts: []geminiPart{{Text: "namaste"}}}}}
		resp.UsageMetadata.PromptTokenCount = 3
		resp.UsageMetadata.CandidatesTokenCount = 1
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()
	p := NewGemini("test-key", "gemini-x")
	p.BaseURL = srv.URL
	r, err := p.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Residency: Residency{AllowCrossBorder: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "namaste" {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestProviders_RefuseCrossBorder(t *testing.T) {
	for _, p := range []Provider{
		NewAnthropic("k", ""),
		NewOpenAI("k", ""),
		NewGemini("k", ""),
	} {
		_, err := p.Complete(context.Background(), CompletionRequest{
			Residency: Residency{Region: "in", AllowCrossBorder: false},
		})
		if err == nil {
			t.Fatalf("%s should refuse cross-border", p.Name())
		}
	}
}
