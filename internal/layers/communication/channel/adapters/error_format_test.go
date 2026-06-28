package adapters

import (
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// T: D1-S3-A08-T01 — IM adapter differentiated copy (FR-16, AC5).
//
// DM-20260628-001 (devrix-api-error-classification) regression tests.

func TestErrorCopyByCode(t *testing.T) {
	const fallback = "网络异常请重试"

	cases := []struct {
		code sharederrors.APIErrorCode
		want string
	}{
		{sharederrors.APICodeRateLimit, "⚠️ 模型繁忙，请稍候重试"},
		{sharederrors.APICodeAuthenticationFailed, "🔑 API key 失效，请检查 ~/.devrix/config.yaml"},
		{sharederrors.APICodePromptTooLong, "📦 会话过长，已尝试压缩"},
		{sharederrors.APICodeMediaSize, "📎 文件/图片过大，请缩小后重试"},
		{sharederrors.APICodeImageSize, "📎 文件/图片过大，请缩小后重试"},
		{sharederrors.APICodeServerError, "🔧 服务暂时不可用，请稍候重试"},
		{sharederrors.APICodeUnknown, fallback}, // 兜底
	}
	for _, tc := range cases {
		t.Run(tc.code.String(), func(t *testing.T) {
			got := errorCopyByCode(tc.code, fallback)
			if got != tc.want {
				t.Errorf("errorCopyByCode(%v) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

func TestErrorCodeFromMetadata(t *testing.T) {
	cases := []struct {
		name     string
		metadata map[string]string
		want     sharederrors.APIErrorCode
	}{
		{"nil metadata", nil, sharederrors.APICodeUnknown},
		{"missing key", map[string]string{"other": "value"}, sharederrors.APICodeUnknown},
		{"empty value", map[string]string{"error_code": ""}, sharederrors.APICodeUnknown},
		{"rate_limit", map[string]string{"error_code": "rate_limit"}, sharederrors.APICodeRateLimit},
		{"authentication_failed", map[string]string{"error_code": "authentication_failed"}, sharederrors.APICodeAuthenticationFailed},
		{"prompt_too_long", map[string]string{"error_code": "prompt_too_long"}, sharederrors.APICodePromptTooLong},
		{"server_error", map[string]string{"error_code": "server_error"}, sharederrors.APICodeServerError},
		{"case insensitive", map[string]string{"error_code": "RATE_LIMIT"}, sharederrors.APICodeRateLimit},
		{"unknown value", map[string]string{"error_code": "garbage"}, sharederrors.APICodeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := errorCodeFromMetadata(tc.metadata)
			if got != tc.want {
				t.Errorf("errorCodeFromMetadata(%v) = %v, want %v", tc.metadata, got, tc.want)
			}
		})
	}
}
