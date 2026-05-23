package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// CachedProvider wraps any Provider with an exact-match response cache.
//
// Keys are SHA-256 of the canonical-JSON of the messages + model + temperature.
// Identical retries within TTL hit the cache; misses fall through to Inner.
//
// Production should add semantic caching (embedding-based lookup) and a
// shared backend (Redis); the interface is the same.
type CachedProvider struct {
	Inner  Provider
	TTL    time.Duration

	mu    sync.Mutex
	store map[string]cachedEntry
}

type cachedEntry struct {
	expiresAt time.Time
	resp      CompletionResponse
}

// NewCachedProvider builds a cache. TTL defaults to 10 min.
func NewCachedProvider(inner Provider, ttl time.Duration) *CachedProvider {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &CachedProvider{Inner: inner, TTL: ttl, store: map[string]cachedEntry{}}
}

func (c *CachedProvider) Name() string   { return c.Inner.Name() + "+cache" }
func (c *CachedProvider) Region() string { return c.Inner.Region() }

func (c *CachedProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	key := cacheKey(req)
	c.mu.Lock()
	if e, ok := c.store[key]; ok && time.Now().Before(e.expiresAt) {
		c.mu.Unlock()
		return e.resp, nil
	}
	c.mu.Unlock()

	resp, err := c.Inner.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	c.mu.Lock()
	c.store[key] = cachedEntry{expiresAt: time.Now().Add(c.TTL), resp: resp}
	c.mu.Unlock()
	return resp, nil
}

func cacheKey(req CompletionRequest) string {
	body, _ := json.Marshal(struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Temperature float64   `json:"temperature"`
	}{req.Model, req.Messages, req.Temperature})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
