package config

import (
	"os"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// DefaultAuthSecretPlaceholder is the dev-only JWT signing key.
// Production deployments MUST override via DEVRIX_AUTH_SECRET or auth.secret in config.yaml.
const DefaultAuthSecretPlaceholder = "devrix-secret-change-me"

// IsDefaultAuthSecret reports whether secret is empty or still the dev placeholder.
func IsDefaultAuthSecret(secret string) bool {
	return secret == "" || secret == DefaultAuthSecretPlaceholder
}

// DefaultAuthConfig 返回默认认证配置
func DefaultAuthConfig() *types.AuthConfig {
	return &types.AuthConfig{
		Secret:      getEnvOrDefault("DEVRIX_AUTH_SECRET", DefaultAuthSecretPlaceholder),
		TokenExpiry: 24 * time.Hour,
		Issuer:      "devrix",
	}
}

// AuthConfigLoader 加载认证配置
type AuthConfigLoader struct {
	config *types.AuthConfig
}

// NewAuthConfigLoader 创建新的认证配置加载器
func NewAuthConfigLoader() *AuthConfigLoader {
	return &AuthConfigLoader{
		config: DefaultAuthConfig(),
	}
}

// Load 加载配置，应用环境变量覆盖
func (c *AuthConfigLoader) Load() (*types.AuthConfig, error) {
	if secret := os.Getenv("DEVRIX_AUTH_SECRET"); secret != "" {
		c.config.Secret = secret
	}

	if expiryStr := os.Getenv("DEVRIX_AUTH_TOKEN_EXPIRY"); expiryStr != "" {
		if d, err := time.ParseDuration(expiryStr); err == nil {
			c.config.TokenExpiry = d
		}
	}

	if issuer := os.Getenv("DEVRIX_AUTH_ISSUER"); issuer != "" {
		c.config.Issuer = issuer
	}

	return c.config, nil
}

// Get 返回加载的配置
func (c *AuthConfigLoader) Get() *types.AuthConfig {
	return c.config
}

// getEnvOrDefault 获取环境变量或默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
