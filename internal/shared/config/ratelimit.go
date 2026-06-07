package config

import (
	"os"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerMinute int  // 每分钟请求数，默认 100
	BurstSize        int  // 突发容量，默认 10
	Enabled          bool // 是否启用，默认 true
}

// DefaultRateLimitConfig 返回默认限流配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         10,
		Enabled:          true,
	}
}

// RateLimitConfigLoader 加载限流配置
type RateLimitConfigLoader struct {
	config *RateLimitConfig
}

// NewRateLimitConfigLoader 创建新的限流配置加载器
func NewRateLimitConfigLoader() *RateLimitConfigLoader {
	return &RateLimitConfigLoader{
		config: DefaultRateLimitConfig(),
	}
}

// Load 加载配置，应用环境变量覆盖
func (c *RateLimitConfigLoader) Load() (*RateLimitConfig, error) {
	if rpm := os.Getenv("DEVRIX_RATE_LIMIT_RPM"); rpm != "" {
		if val, err := parseInt(rpm); err == nil && val > 0 {
			c.config.RequestsPerMinute = val
		}
	}

	if burst := os.Getenv("DEVRIX_RATE_LIMIT_BURST"); burst != "" {
		if val, err := parseInt(burst); err == nil && val > 0 {
			c.config.BurstSize = val
		}
	}

	if enabled := os.Getenv("DEVRIX_RATE_LIMIT_ENABLED"); enabled != "" {
		c.config.Enabled = enabled == "true" || enabled == "1"
	}

	return c.config, nil
}

// Get 返回加载的配置
func (c *RateLimitConfigLoader) Get() *RateLimitConfig {
	return c.config
}

// parseInt 解析整数
func parseInt(s string) (int, error) {
	var val int
	_, err := parseIntFromString(s, &val)
	return val, err
}

func parseIntFromString(s string, val *int) (int, error) {
	result := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseError{s}
		}
		result = result*10 + int(c-'0')
	}
	*val = result
	return result, nil
}

type parseError struct {
	s string
}

func (e *parseError) Error() string {
	return "invalid integer: " + e.s
}
