package hardening

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/errors"
)

// IsContextLengthError reports prompt-too-long failures (TD-QL-01, D7 path).
func IsContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	if code := errors.ErrorCode(err); code == errors.CodeContextExceeded {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "prompt too long") ||
		strings.Contains(lower, "maximum context length") ||
		strings.Contains(lower, " 413")
}

// IsOverloadOr5xx reports overload / 5xx / rate-limit errors suitable for
// gateway-level fallback retry (TD-QL-03 — handled in llmgateway.Stream).
func IsOverloadOr5xx(err error) bool {
	if err == nil || IsContextLengthError(err) {
		return false
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "500") || strings.Contains(lower, "502") ||
		strings.Contains(lower, "503") || strings.Contains(lower, "504") {
		return true
	}
	if strings.Contains(lower, "overloaded") || strings.Contains(lower, " 529") {
		return true
	}
	if strings.Contains(lower, " 429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return true
	}
	if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return true
	}
	return false
}

// MaxOutputTokensRecoveryMessage is injected when the provider stops with
// finish_reason=length so the model can continue without orphan UI chunks.
const MaxOutputTokensRecoveryMessage = "Your previous response was truncated due to output token limits. Continue from where you left off without repeating content already shown."

// NeedsMaxOutputTokenRecovery reports finish_reason=length truncation (TD-QL-02).
func NeedsMaxOutputTokenRecovery(finishReason string) bool {
	return finishReason == "length"
}