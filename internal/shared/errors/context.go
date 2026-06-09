package errors

import (
	"errors"
	"fmt"
	"strings"
)

// Context engine sentinel errors.
var (
	ErrContextExceeded      = errors.New("context token budget exceeded")
	ErrSnapshotCorrupt      = errors.New("context snapshot corrupt")
	ErrPEVMaxIterations     = errors.New("pev max iterations exceeded")
	ErrLLMUnavailable       = errors.New("llm unavailable")
	ErrFeatureNotImplemented = errors.New("feature not implemented")
)

const (
	CodeContextExceeded      = "CTX_EXCEEDED_4001"
	CodeSnapshotCorrupt      = "CTX_SNAPSHOT_4002"
	CodePEVMaxIterations     = "CTX_PEV_4003"
	CodeLLMUnavailable       = "CTX_LLM_4004"
	CodeMemoryNotImplemented = "CTX_MEMORY_4005"
	CodePermissionDenied     = "CTX_PERMISSION_4006"
	CodeAutocompactFailed    = "CTX_AUTOCOMPACT_4010"
	CodeVerifyCommandFailed  = "CTX_VERIFY_CMD_4011"
	CodeVerifyCommandReject  = "CTX_VERIFY_CMD_4012"
	CodePlanValidationFailed = "CTX_PLAN_4020"
	CodePlanLLMTimeout       = "CTX_PLAN_4021"
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

// NewPEVMaxIterationsError returns a PEV max iterations error.
func NewPEVMaxIterationsError() *SentinelError {
	return WithCode(CodePEVMaxIterations, "pev max iterations exceeded", ErrPEVMaxIterations)
}

// NewLLMUnavailableError returns an LLM unavailable error.
func NewLLMUnavailableError(err error) *SentinelError {
	msg := "llm gateway unavailable"
	if detail := FormatLLMError(err); isProviderOverloaded(detail) {
		msg = "MiniMax 服务当前负载较高，请稍后重试（HTTP 529）"
	} else if isLLMTimeout(detail) {
		msg = "LLM 响应超时（模型思考耗时较长），请稍后重试"
	}
	return WithCode(CodeLLMUnavailable, msg, err)
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

// NewVerifyCommandFailedError returns a verify command non-zero exit error.
func NewVerifyCommandFailedError(name string, exitCode int) *SentinelError {
	return WithCode(CodeVerifyCommandFailed, fmt.Sprintf("verify command %s failed with exit %d", name, exitCode), ErrPEVMaxIterations)
}

// NewVerifyCommandRejectedError returns a verify command configuration/runtime rejection.
func NewVerifyCommandRejectedError(reason string) *SentinelError {
	return WithCode(CodeVerifyCommandReject, "verify command rejected: "+reason, ErrPEVMaxIterations)
}

// NewPlanValidationFailedError returns a plan DAG/JSON validation error (degraded path).
func NewPlanValidationFailedError(reason string) *SentinelError {
	return WithCode(CodePlanValidationFailed, "plan validation failed: "+reason, ErrFeatureNotImplemented)
}

// NewPlanLLMTimeoutError returns a plan LLM timeout error.
func NewPlanLLMTimeoutError(err error) *SentinelError {
	return WithCode(CodePlanLLMTimeout, "plan llm timeout", err)
}

// NewLongTermDBError returns a long-term memory persistence error.
func NewLongTermDBError(err error) *SentinelError {
	return WithCode(CodeLongTermDBError, "long-term memory db error", err)
}
