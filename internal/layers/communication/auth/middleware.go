package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey 是 context 中的键类型
type contextKey string

const (
	// AdapterIDKey 是 context 中存储 adapter ID 的键
	AdapterIDKey contextKey = "adapter_id"
)

// Middleware 创建认证中间件
func Middleware(authService IAuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 获取 Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Unauthorized: missing Authorization header", http.StatusUnauthorized)
				return
			}

			// 解析 Bearer token
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Unauthorized: invalid Authorization format", http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// 验证 token
			adapterID, err := authService.Validate(token)
			if err != nil {
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// 将 adapterID 放入 context
			ctx := context.WithValue(r.Context(), AdapterIDKey, adapterID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAdapterID 从 context 中获取 adapter ID
func GetAdapterID(ctx context.Context) string {
	if adapterID, ok := ctx.Value(AdapterIDKey).(string); ok {
		return adapterID
	}
	return ""
}

// OptionalMiddleware 创建可选认证中间件（不验证也允许通过）
func OptionalMiddleware(authService IAuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 没有认证信息，继续处理
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]
			adapterID, err := authService.Validate(token)
			if err != nil {
				// 认证失败，继续处理（不阻断）
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), AdapterIDKey, adapterID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
