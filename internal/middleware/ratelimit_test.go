package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter()

	for i := 0; i < 5; i++ {
		if !rl.Allow("key", 5, time.Minute) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow("key", 5, time.Minute) {
		t.Error("6th request should be denied")
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	rl := NewRateLimiter()

	// Use a very short window
	for i := 0; i < 3; i++ {
		rl.Allow("key", 3, 10*time.Millisecond)
	}

	// Should be blocked
	if rl.Allow("key", 3, 10*time.Millisecond) {
		t.Error("should be blocked within window")
	}

	// Wait for window to expire
	time.Sleep(15 * time.Millisecond)

	if !rl.Allow("key", 3, 10*time.Millisecond) {
		t.Error("should be allowed after window expires")
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter()

	rl.Allow("expired", 5, 10*time.Millisecond)
	time.Sleep(15 * time.Millisecond)

	rl.Allow("active", 5, time.Minute)

	rl.Cleanup()

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if _, ok := rl.entries["expired"]; ok {
		t.Error("expired entry should have been cleaned up")
	}
	if _, ok := rl.entries["active"]; !ok {
		t.Error("active entry should still exist")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter()
	keyFunc := func(r *http.Request) string { return "test" }

	handler := RateLimit(rl, keyFunc, 2, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 2 requests should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("3rd request: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}
