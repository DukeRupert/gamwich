package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RealIP extracts the client's real IP address, preferring Cloudflare's
// CF-Connecting-IP header, then X-Forwarded-For, and falling back to RemoteAddr.
func RealIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the chain is the original client
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type entry struct {
	count    int
	windowAt time.Time
}

// RateLimiter provides in-memory rate limiting.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*entry
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*entry),
	}
}

// Allow returns true if the key has not exceeded limit in the given window.
func (rl *RateLimiter) Allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	e, ok := rl.entries[key]
	if !ok || now.After(e.windowAt) {
		rl.entries[key] = &entry{count: 1, windowAt: now.Add(window)}
		return true
	}
	e.count++
	return e.count <= limit
}

// Cleanup removes expired entries.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, e := range rl.entries {
		if now.After(e.windowAt) {
			delete(rl.entries, key)
		}
	}
}

// RateLimit returns middleware that rate-limits requests by a key function.
func RateLimit(limiter *RateLimiter, keyFunc func(*http.Request) string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if !limiter.Allow(key, limit, window) {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
