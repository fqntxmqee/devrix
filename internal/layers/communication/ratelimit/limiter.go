package ratelimit

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
)

// RateLimiter 基于 Token Bucket 算法的限流器
type RateLimiter struct {
	mu          sync.Mutex
	tokens      map[string]*bucket // per-adapter tokens
	maxTokens   float64
	rate        float64 // tokens per second
	lastUpdate  time.Time
}

// bucket 代表单个 adapter 的 token bucket
type bucket struct {
	tokens      float64
	lastUpdate  time.Time
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerMinute int   // 每分钟请求数，默认 100
	BurstSize         int   // 突发容量，默认 10
	Enabled           bool  // 是否启用，默认 true
}

// DefaultRateLimitConfig 返回默认配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         10,
		Enabled:           true,
	}
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(cfg *RateLimitConfig) *RateLimiter {
	if cfg == nil {
		cfg = DefaultRateLimitConfig()
	}

	// rate = requests per minute / 60 seconds
	rate := float64(cfg.RequestsPerMinute) / 60.0
	maxTokens := float64(cfg.BurstSize)

	return &RateLimiter{
		tokens:     make(map[string]*bucket),
		maxTokens:  maxTokens,
		rate:       rate,
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许请求
func (l *RateLimiter) Allow(adapterID string) bool {
	if adapterID == "" {
		adapterID = "default"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, exists := l.tokens[adapterID]

	if !exists {
		// 创建新的 bucket
		b = &bucket{
			tokens:     l.maxTokens,
			lastUpdate: now,
		}
		l.tokens[adapterID] = b
		return true
	}

	// 补充 tokens
	elapsed := now.Sub(b.lastUpdate).Seconds()
	b.tokens = min(l.maxTokens, b.tokens+elapsed*l.rate)
	b.lastUpdate = now

	// 检查是否有足够的 token
	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

// Reset 重置 adapter 的限流状态
func (l *RateLimiter) Reset(adapterID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.tokens, adapterID)
}

// ResetAll 重置所有限流状态
func (l *RateLimiter) ResetAll() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.tokens = make(map[string]*bucket)
}

// Remaining 返回 adapter 剩余的请求配额
func (l *RateLimiter) Remaining(adapterID string) int {
	if adapterID == "" {
		adapterID = "default"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists := l.tokens[adapterID]
	if !exists {
		return int(l.maxTokens)
	}

	now := time.Now()
	elapsed := now.Sub(b.lastUpdate).Seconds()
	tokens := min(l.maxTokens, b.tokens+elapsed*l.rate)

	return int(tokens)
}

// Middleware 创建限流中间件
func Middleware(limiter *RateLimiter, cfg *config.RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 获取 adapter ID（从 context 或 header）
			adapterID := r.Header.Get("X-Adapter-ID")
			if adapterID == "" {
				adapterID = "default"
			}

			// 检查是否允许请求
			if !limiter.Allow(adapterID) {
				// 获取重置时间
				retryAfter := 60 // 1 minute
				remaining := limiter.Remaining(adapterID)
				limit := 100
				if cfg != nil && cfg.RequestsPerMinute > 0 {
					// Use float64 to avoid integer division issues
					// retryAfter = (60 / rpm) * burst = seconds per token * burst
					retryAfter = int(math.Ceil(float64(60*cfg.BurstSize) / float64(cfg.RequestsPerMinute)))
					if retryAfter < 1 {
						retryAfter = 1
					}
					limit = cfg.RequestsPerMinute
				}

				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
				w.Header().Set("X-RateLimit-Reset", strconv.Itoa(retryAfter))

				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// 添加 rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(100))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(limiter.Remaining(adapterID)))

			next.ServeHTTP(w, r)
		})
	}
}
