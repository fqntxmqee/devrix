package adapters

import (
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// errorCopyByCode returns the IM-facing copy for each closed-set APIErrorCode.
// DM-20260628-001 (FR-16): 5 distinct codes each have dedicated copy; the
// Unknown bucket falls back to the caller-supplied fallback (existing
// unified copy from Event.Content) to preserve backward compatibility.
//
// Locale: zh-CN (matches devrix's primary user base). Add i18n later.
func errorCopyByCode(code sharederrors.APIErrorCode, fallback string) string {
	switch code {
	case sharederrors.APICodeRateLimit:
		return "⚠️ 模型繁忙，请稍候重试"
	case sharederrors.APICodeAuthenticationFailed:
		return "🔑 API key 失效，请检查 ~/.devrix/config.yaml"
	case sharederrors.APICodePromptTooLong:
		return "📦 会话过长，已尝试压缩"
	case sharederrors.APICodeMediaSize, sharederrors.APICodeImageSize:
		return "📎 文件/图片过大，请缩小后重试"
	case sharederrors.APICodeServerError:
		return "🔧 服务暂时不可用，请稍候重试"
	default:
		return fallback
	}
}

// errorCodeFromMetadata extracts the closed-set code from Event.Metadata["error_code"].
// Returns APICodeUnknown if the field is missing or unparseable.
func errorCodeFromMetadata(metadata map[string]string) sharederrors.APIErrorCode {
	if metadata == nil {
		return sharederrors.APICodeUnknown
	}
	return sharederrors.ParseAPIErrorCode(metadata["error_code"])
}
