package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

// TestNewRateLimiter tests the rate limiter creation
func TestNewRateLimiter(t *testing.T) {
	cfg := &RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        10,
		Enabled:          true,
	}

	limiter := NewRateLimiter(cfg)
	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	if limiter.maxTokens != 10 {
		t.Errorf("maxTokens = %v, want 10", limiter.maxTokens)
	}
}

// TestNewRateLimiter_DefaultConfig tests with nil config
func TestNewRateLimiter_DefaultConfig(t *testing.T) {
	limiter := NewRateLimiter(nil)
	if limiter == nil {
		t.Fatal("NewRateLimiter(nil) returned nil")
	}

	// Check default values
	if limiter.maxTokens != 10 {
		t.Errorf("default maxTokens = %v, want 10", limiter.maxTokens)
	}
}

// TestRateLimiter_Allow tests basic allow functionality
func TestRateLimiter_Allow(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        5,
		Enabled:          true,
	})

	// First request creates bucket with maxTokens (no decrement)
	// Subsequent requests decrement
	// So with burst=5, we get 1 (creation) + 5 (from burst) = 6 allowed
	for i := 0; i < 6; i++ {
		allowed := limiter.Allow("test_adapter")
		if !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 7th request should be denied (burst exhausted)
	allowed := limiter.Allow("test_adapter")
	if allowed {
		t.Error("7th request should be denied")
	}
}

// TestRateLimiter_Allow_EmptyAdapter tests empty adapter ID uses default
func TestRateLimiter_Allow_EmptyAdapter(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        3,
		Enabled:          true,
	})

	// First request creates bucket, then 3 more decrement
	for i := 0; i < 4; i++ {
		allowed := limiter.Allow("")
		if !allowed {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 5th request should be denied
	allowed := limiter.Allow("")
	if allowed {
		t.Error("5th request should be denied")
	}
}

// TestRateLimiter_Allow_DifferentAdapters tests different adapters have separate limits
func TestRateLimiter_Allow_DifferentAdapters(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        2,
		Enabled:          true,
	})

	// Exhaust adapter1's limit (1 create + 2 from burst = 3 allowed)
	limiter.Allow("adapter1")
	limiter.Allow("adapter1")
	limiter.Allow("adapter1")

	// adapter1 should be denied
	if limiter.Allow("adapter1") {
		t.Error("adapter1 should be denied after burst exhausted")
	}

	// adapter2 should still be allowed (fresh bucket)
	if !limiter.Allow("adapter2") {
		t.Error("adapter2 should be allowed (separate limit)")
	}
}

// TestRateLimiter_Reset tests reset functionality
func TestRateLimiter_Reset(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        2,
		Enabled:          true,
	})

	// Exhaust limit
	limiter.Allow("adapter1")
	limiter.Allow("adapter1")

	// Reset adapter1
	limiter.Reset("adapter1")

	// adapter1 should be allowed again
	if !limiter.Allow("adapter1") {
		t.Error("adapter1 should be allowed after reset")
	}
}

// TestRateLimiter_ResetAll tests reset all functionality
func TestRateLimiter_ResetAll(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        2,
		Enabled:          true,
	})

	// Exhaust both adapters
	limiter.Allow("adapter1")
	limiter.Allow("adapter1")
	limiter.Allow("adapter2")
	limiter.Allow("adapter2")

	// Reset all
	limiter.ResetAll()

	// Both should be allowed again
	if !limiter.Allow("adapter1") {
		t.Error("adapter1 should be allowed after ResetAll")
	}
	if !limiter.Allow("adapter2") {
		t.Error("adapter2 should be allowed after ResetAll")
	}
}

// TestRateLimiter_Remaining tests remaining quota
func TestRateLimiter_Remaining(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        5,
		Enabled:          true,
	})

	// First Allow creates bucket without decrementing
	limiter.Allow("adapter1")

	// After first (creative) call, remaining should be max
	remaining := limiter.Remaining("adapter1")
	if remaining != 5 {
		t.Errorf("remaining after creation = %d, want 5", remaining)
	}

	// Use some tokens (these decrement)
	limiter.Allow("adapter1")
	limiter.Allow("adapter1")

	// After 2 more (decrementing) calls, remaining should be 3
	remaining = limiter.Remaining("adapter1")
	if remaining != 3 {
		t.Errorf("remaining after 3 total requests = %d, want 3", remaining)
	}
}

// TestRateLimiter_Remaining_UnknownAdapter tests unknown adapter returns max
func TestRateLimiter_Remaining_UnknownAdapter(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        10,
		Enabled:          true,
	})

	remaining := limiter.Remaining("unknown_adapter")
	if remaining != 10 {
		t.Errorf("remaining for unknown = %d, want 10", remaining)
	}
}

// TestRateLimiter_Middleware_Denied tests middleware denies when limit exceeded
func TestRateLimiter_Middleware_Denied(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        1,
		Enabled:          true,
	})

	cfg := &config.RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        1,
		Enabled:          true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(limiter, cfg)
	decorated := middleware(handler)

	// First request - allowed (creates bucket with maxTokens=1, no decrement)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Adapter-ID", "test_adapter")
	rec := httptest.NewRecorder()
	decorated.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("first request status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Second request - still allowed (decrements from 1 to 0)
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Adapter-ID", "test_adapter")
	rec = httptest.NewRecorder()
	decorated.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("second request status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Third request - denied (bucket exhausted, tokens=0)
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Adapter-ID", "test_adapter")
	rec = httptest.NewRecorder()
	decorated.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("third request status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

// TestRateLimiter_Middleware_Allowed tests middleware allows when within limit
func TestRateLimiter_Middleware_Allowed(t *testing.T) {
	limiter := NewRateLimiter(&RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        10,
		Enabled:          true,
	})

	cfg := &config.RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:        10,
		Enabled:          true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(limiter, cfg)
	decorated := middleware(handler)

	// Request within burst limit
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Adapter-ID", "test_adapter")
	rec := httptest.NewRecorder()
	decorated.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("request status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Check rate limit headers are present
	if rec.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("X-RateLimit-Limit header missing")
	}
}
