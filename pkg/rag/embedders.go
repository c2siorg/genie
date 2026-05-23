package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// HashEmbedder is a deterministic, ML-free embedder. It maps tokens to a
// fixed-size bag-of-words vector via FNV hashing. Useful for tests and as a
// baseline before plugging in a real embedder.
//
// Not semantically meaningful — two paraphrases will have very different
// vectors. Use OllamaEmbedder for production-quality embeddings.
type HashEmbedder struct {
	Dimension int
}

// NewHashEmbedder builds a HashEmbedder. Default dim 256.
func NewHashEmbedder(dim int) *HashEmbedder {
	if dim <= 0 {
		dim = 256
	}
	return &HashEmbedder{Dimension: dim}
}

func (h *HashEmbedder) Dim() int { return h.Dimension }

func (h *HashEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, h.Dimension)
	if text == "" {
		return vec, nil
	}
	for _, tok := range tokenize(text) {
		hh := fnv.New32a()
		_, _ = hh.Write([]byte(tok))
		vec[hh.Sum32()%uint32(h.Dimension)] += 1
	}
	// L2-normalise so cosine ≈ dot.
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	if sum > 0 {
		s := float32(1.0 / math.Sqrt(float64(sum)))
		for i := range vec {
			vec[i] *= s
		}
	}
	return vec, nil
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var out []string
	var cur strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
		} else if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// OllamaEmbedder calls Ollama's /api/embeddings endpoint. Use for on-prem
// embeddings — sits behind the same Embedder interface so callers don't change.
type OllamaEmbedder struct {
	URL    string       // default http://localhost:11434
	Model  string       // e.g. "nomic-embed-text"
	Client *http.Client // optional; default 30s timeout
	cachedDim int
}

// NewOllamaEmbedder builds an embedder.
func NewOllamaEmbedder(url, model string) *OllamaEmbedder {
	if url == "" {
		url = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{URL: url, Model: model, Client: &http.Client{Timeout: 30 * time.Second}}
}

// Dim returns the embedding dimension. First Embed() call discovers it.
func (o *OllamaEmbedder) Dim() int { return o.cachedDim }

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{"model": o.Model, "prompt": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(o.URL, "/")+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama embed http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	o.cachedDim = len(out.Embedding)
	return out.Embedding, nil
}
