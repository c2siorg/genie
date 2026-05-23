package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllama_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model == "" {
			http.Error(w, "model required", http.StatusBadRequest)
			return
		}
		resp := ollamaResponse{Done: true}
		resp.Message.Role = "assistant"
		resp.Message.Content = "test answer"
		resp.PromptEvalCount = 10
		resp.EvalCount = 5
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "llama3.1")
	r, err := p.Complete(context.Background(), CompletionRequest{
		Model:    "llama3.1",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "test answer" {
		t.Fatalf("text: %q", r.Text)
	}
	if p.Region() != "on-prem" {
		t.Fatalf("expected on-prem provider region, got %s", p.Region())
	}
}
