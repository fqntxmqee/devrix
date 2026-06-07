package types

import "time"

// Token 代表一个认证 Token
type Token struct {
	AdapterID string    // 适配器 ID
	IssuedAt  time.Time // 签发时间
	ExpiresAt time.Time // 过期时间
	Token     string    // JWT token 字符串
}

// IsExpired 检查 Token 是否已过期
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid 检查 Token 是否有效（未过期）
func (t *Token) IsValid() bool {
	return !t.IsExpired() && t.Token != ""
}

// AuthResult 代表认证结果
type AuthResult struct {
	Success bool   // 是否成功
	Token   *Token // 成功时的 Token
	Error   error  // 失败时的错误
}

// AuthRequest 代表认证请求
type AuthRequest struct {
	AdapterID string `json:"adapter_id"`
	Secret   string `json:"secret"`
}

// AuthConfig 代表认证配置
type AuthConfig struct {
	Secret      string        // 共享密钥
	TokenExpiry time.Duration // Token 过期时间，默认 24h
	Issuer      string        // Token 签发者
}
