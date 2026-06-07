package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// IAuthService 定义认证服务接口
type IAuthService interface {
	// Register 注册适配器并颁发 Token
	Register(adapterID, secret string) (*types.AuthResult, error)
	// Validate 验证 Token 是否有效
	Validate(token string) (string, error)
	// Refresh 刷新 Token
	Refresh(token string) (*types.AuthResult, error)
}

// AuthService 实现 IAuthService
type AuthService struct {
	config   *types.AuthConfig
	tokens   map[string]*types.Token // token string -> Token
	adapters map[string]string      // adapterID -> secret hash
	mu       sync.RWMutex
}

// NewAuthService 创建新的 AuthService
func NewAuthService(config *types.AuthConfig) *AuthService {
	if config.TokenExpiry == 0 {
		config.TokenExpiry = 24 * time.Hour
	}
	if config.Issuer == "" {
		config.Issuer = "devrix"
	}

	return &AuthService{
		config:   config,
		tokens:   make(map[string]*types.Token),
		adapters: make(map[string]string),
	}
}

// Register 注册适配器并颁发 Token
func (s *AuthService) Register(adapterID, secret string) (*types.AuthResult, error) {
	if adapterID == "" {
		return &types.AuthResult{Success: false, Error: fmt.Errorf("adapter_id is required")}, nil
	}

	if secret == "" {
		return &types.AuthResult{Success: false, Error: fmt.Errorf("secret is required")}, nil
	}

	// 验证 shared secret
	expectedSecret := s.config.Secret
	if expectedSecret == "" {
		return &types.AuthResult{Success: false, Error: fmt.Errorf("auth service not configured")}, nil
	}

	if secret != expectedSecret {
		return &types.AuthResult{Success: false, Error: fmt.Errorf("invalid secret")}, nil
	}

	// 生成 Token
	now := time.Now()
	tokenStr := s.generateToken(adapterID, now)

	token := &types.Token{
		AdapterID: adapterID,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.config.TokenExpiry),
		Token:     tokenStr,
	}

	// 存储 Token
	s.mu.Lock()
	s.tokens[tokenStr] = token
	s.mu.Unlock()

	return &types.AuthResult{Success: true, Token: token}, nil
}

// Validate 验证 Token 是否有效
func (s *AuthService) Validate(tokenStr string) (string, error) {
	if tokenStr == "" {
		return "", fmt.Errorf("token is required")
	}

	// 解析 token
	adapterID, issuedAt, expiresAt, err := s.parseToken(tokenStr)
	if err != nil {
		return "", fmt.Errorf("invalid token format: %w", err)
	}

	// 检查是否过期
	if time.Now().After(expiresAt) {
		return "", fmt.Errorf("token expired")
	}

	// 验证签名
	expectedSig := s.signToken(adapterID, issuedAt)
	if !strings.HasSuffix(tokenStr, "."+expectedSig) {
		return "", fmt.Errorf("invalid token signature")
	}

	// 检查 token 是否在有效期内
	s.mu.RLock()
	token, exists := s.tokens[tokenStr]
	s.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("token not found")
	}

	if time.Now().After(token.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}

	return adapterID, nil
}

// Refresh 刷新 Token
func (s *AuthService) Refresh(tokenStr string) (*types.AuthResult, error) {
	adapterID, err := s.Validate(tokenStr)
	if err != nil {
		return &types.AuthResult{Success: false, Error: err}, nil
	}

	// 作废旧 token
	s.mu.Lock()
	delete(s.tokens, tokenStr)
	s.mu.Unlock()

	// 颁发新 token
	return s.Register(adapterID, s.config.Secret)
}

// generateToken 生成 Token
func (s *AuthService) generateToken(adapterID string, issuedAt time.Time) string {
	// Token 格式: base64(issuer|adapterID|issuedAt)|signature
	issuedAtUnix := issuedAt.Unix()
	payload := fmt.Sprintf("%s|%s|%d", s.config.Issuer, adapterID, issuedAtUnix)
	signature := s.signToken(adapterID, issuedAt)

	tokenStr := base64.StdEncoding.EncodeToString([]byte(payload))
	return fmt.Sprintf("%s.%s", tokenStr, signature)
}

// parseToken 解析 Token
func (s *AuthService) parseToken(tokenStr string) (adapterID string, issuedAt time.Time, expiresAt time.Time, err error) {
	parts := strings.SplitN(tokenStr, ".", 2)
	if len(parts) != 2 {
		err = fmt.Errorf("invalid token format")
		return
	}

	payloadBytes, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		err = fmt.Errorf("invalid token encoding")
		return
	}

	payload := string(payloadBytes)
	payloadParts := strings.SplitN(payload, "|", 3)
	if len(payloadParts) != 3 {
		err = fmt.Errorf("invalid token payload")
		return
	}

	issuer := payloadParts[0]
	adapterID = payloadParts[1]
	var issuedAtUnix int64

	_, err = fmt.Sscanf(payloadParts[2], "%d", &issuedAtUnix)
	if err != nil {
		err = fmt.Errorf("invalid issued_at")
		return
	}

	if issuer != s.config.Issuer {
		err = fmt.Errorf("invalid issuer")
		return
	}

	issuedAt = time.Unix(issuedAtUnix, 0)
	expiresAt = issuedAt.Add(s.config.TokenExpiry)

	return
}

// signToken 生成签名
func (s *AuthService) signToken(adapterID string, issuedAt time.Time) string {
	data := fmt.Sprintf("%s|%s|%d", s.config.Issuer, adapterID, issuedAt.Unix())
	mac := hmac.New(sha256.New, []byte(s.config.Secret))
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// generateToken 序列化 (实现 types.Token 接口需要)
func (s *AuthService) SerializeToken(t *types.Token) (string, error) {
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// DeserializeToken 反序列化
func (s *AuthService) DeserializeToken(data string) (*types.Token, error) {
	bytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}

	var t types.Token
	if err := json.Unmarshal(bytes, &t); err != nil {
		return nil, err
	}

	return &t, nil
}
