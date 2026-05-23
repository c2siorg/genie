package mid

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimit is a per-key token bucket. Keys are typically the authenticated
// user id; if no claims are present, the remote address is used as the key.
//
// In-memory only — fine for a single instance. Behind a load balancer you'd
// swap this for Redis or move it to the gateway.
type RateLimit struct {
	Capacity   int           // bucket size
	RefillRate float64       // tokens per second

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// NewRateLimit constructs a limiter (capacity tokens, refilled at rate/sec).
func NewRateLimit(capacity int, refillPerSecond float64) *RateLimit {
	return &RateLimit{
		Capacity:   capacity,
		RefillRate: refillPerSecond,
		buckets:    map[string]*bucket{},
	}
}

// Allow returns whether the request was permitted and the remaining tokens.
func (l *RateLimit) Allow(key string) (bool, int) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.Capacity), lastSeen: now}
		l.buckets[key] = b
	} else {
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens += elapsed * l.RefillRate
		if b.tokens > float64(l.Capacity) {
			b.tokens = float64(l.Capacity)
		}
		b.lastSeen = now
	}
	if b.tokens < 1 {
		return false, int(b.tokens)
	}
	b.tokens--
	return true, int(b.tokens)
}

// Middleware returns the http.Handler middleware. Adds X-RateLimit-Remaining.
func (l *RateLimit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if c, ok := ClaimsFrom(r.Context()); ok {
			key = c.Subject
		}
		ok, remaining := l.Allow(key)
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !ok {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
