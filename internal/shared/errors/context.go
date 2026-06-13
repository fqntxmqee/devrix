package errors

import (
	"errors"
	"strings"
)

// Context engine sentinel errors.
var (
	ErrContextExceeded       = errors.New("context token budget exceeded")
	ErrSnapshotCorrupt       = errors.New("context snapshot corrupt")
	ErrLLMUnavailable        = errors.New("llm unavailable")
	ErrFeatureNotImplemented = errors.New("feature not implemented")
)

const (
	CodeContextExceeded      = "CTX_EXCEEDED_4001"
	CodeSnapshotCorrupt      = "CTX_SNAPSHOT_4002"
	CodeLLMUnavailable       = "CTX_LLM_4004"
	CodeMemoryNotImplemented = "CTX_MEMORY_4005"
	CodePermissionDenied     = "CTX_PERMISSION_4006"
	CodeAutocompactFailed    = "CTX_AUTOCOMPACT_4010"
	CodeLongTermDBError      = "CTX_MEMORY_4022"
)

// NewContextExceededError returns a context exceeded error.
func NewContextExceededError() *SentinelError {
	return WithCode(CodeContextExceeded, "context token budget exceeded after compression", ErrContextExceeded)
}

// NewSnapshotCorruptError returns a snapshot corrupt error.
func NewSnapshotCorruptError(err error) *SentinelError {
	return WithCode(CodeSnapshotCorrupt, "context snapshot corrupt or unsupported version", err)
}

// NewLLMUnavailableError returns an LLM unavailable error.
func NewLLMUnavailableError(err error) *SentinelError {
	msg := "llm gateway unavailable"
	detail := FormatLLMError(err)
	if isProviderOverloaded(detail) {
		msg = "MiniMax 服务当前负载较高，请稍后重试（HTTP 529）"
	} else if isLLMTimeout(detail) {
		msg = "LLM 响应超时（模型思考耗时较长），请稍后重试"
	} else if isToolProtocolError(detail) {
		msg = "LLM 工具消息格式被拒绝（tool_call_id 不匹配），已记录详情，请重试或新开会话"
	} else if detail != "" && !strings.Contains(strings.ToLower(detail), "llm gateway unavailable") {
		msg = "LLM 调用失败: " + truncateLLMUserMessage(detail)
	}
	return WithCode(CodeLLMUnavailable, msg, err)
}

func isToolProtocolError(detail string) bool {
	lower := strings.ToLower(detail)
	return strings.Contains(lower, "tool_call_id") ||
		strings.Contains(lower, "tool id") ||
		strings.Contains(lower, "tool_calls") ||
		strings.Contains(lower, "invalid function arguments")
}

func truncateLLMUserMessage(s string) string {
	const max = 240
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func isProviderOverloaded(detail string) bool {
	lower := strings.ToLower(detail)
	return strings.Contains(lower, "529") ||
		strings.Contains(lower, "overloaded") ||
		strings.Contains(detail, "负载较高")
}

func isLLMTimeout(detail string) bool {
	lower := strings.ToLower(detail)
	return strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "llm request timeout")
}

// NewFeatureNotImplementedError returns a feature not implemented error.
func NewFeatureNotImplementedError(feature, version string) *SentinelError {
	return WithCode(CodeMemoryNotImplemented, feature+" not implemented in "+version, ErrFeatureNotImplemented)
}

// NewContextPermissionDeniedError returns a permission denied error for tool calls.
func NewContextPermissionDeniedError(toolName string) *SentinelError {
	return WithCode(CodePermissionDenied, "permission denied for tool: "+toolName, ErrPermissionDenied)
}

// NewAutocompactFailedError returns an autocompact degradation error.
func NewAutocompactFailedError(reason string, err error) *SentinelError {
	return WithCode(CodeAutocompactFailed, "autocompact degraded: "+reason, err)
}

// NewLongTermDBError returns a long-term memory persistence error.
func NewLongTermDBError(err error) *SentinelError {
	return WithCode(CodeLongTermDBError, "long-term memory db error", err)
}
