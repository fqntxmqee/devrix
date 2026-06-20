package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors (可使用 errors.Is/errors.As 检查)
// 会话错误 (1000-1999)
var (
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionExpired      = errors.New("session expired")
	ErrSessionCreateFailed = errors.New("failed to create session")
	ErrSessionStore       = errors.New("session store error")
)

// 消息错误 (2000-2999)
var (
	ErrMessageEmpty         = errors.New("message is empty")
	ErrMessageTooLong       = errors.New("message too long")
	ErrMessageInvalidFormat = errors.New("invalid message format")
)

// 权限错误 (3000-3999)
var (
	ErrPermissionDenied  = errors.New("permission denied")
	ErrPermissionTimeout = errors.New("permission timeout")
	ErrPermissionInvalid = errors.New("invalid permission response")
)

// 网关错误 (4000-4999)
var (
	ErrGatewayRoute = errors.New("gateway route error")
	ErrGatewayAdapt = errors.New("gateway adapter error")
)

// SentinelError wraps an error with code and message.
//
// DM-20260620-003 (PR-B tier 2 C2): unified with the legacy LLMError shape
// so all devrix errors use the same type. Error() now falls back to the
// inner Err (and finally Code) when Message is empty, matching the more
// permissive LLMError semantics callers relied on.
type SentinelError struct {
	Code    string
	Message string
	Err     error
}

func (e *SentinelError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *SentinelError) Unwrap() error {
	return e.Err
}

// WithCode creates a new SentinelError with the given code
func WithCode(code, message string, err error) *SentinelError {
	return &SentinelError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ErrorCode extracts the error code from an error
func ErrorCode(err error) string {
	var se *SentinelError
	if errors.As(err, &se) {
		return se.Code
	}
	return ""
}

// NewSessionNotFoundError creates a session not found error with code
func NewSessionNotFoundError(sessionID string) *SentinelError {
	return WithCode(
		"COMM_SESSION_NOT_FOUND_1001",
		fmt.Sprintf("session not found: %s", sessionID),
		ErrSessionNotFound,
	)
}

// NewSessionExpiredError creates a session expired error with code
func NewSessionExpiredError(sessionID string) *SentinelError {
	return WithCode(
		"COMM_SESSION_EXPIRED_1002",
		fmt.Sprintf("session expired: %s", sessionID),
		ErrSessionExpired,
	)
}

// NewMessageEmptyError creates a message empty error with code
func NewMessageEmptyError() *SentinelError {
	return WithCode(
		"COMM_MESSAGE_EMPTY_2001",
		"message is empty",
		ErrMessageEmpty,
	)
}

// NewPermissionDeniedError creates a permission denied error with code
func NewPermissionDeniedError(toolName string) *SentinelError {
	return WithCode(
		"COMM_PERMISSION_DENIED_3001",
		fmt.Sprintf("permission denied for tool: %s", toolName),
		ErrPermissionDenied,
	)
}

// NewPermissionTimeoutError creates a permission timeout error with code
func NewPermissionTimeoutError(requestID string) *SentinelError {
	return WithCode(
		"COMM_PERMISSION_TIMEOUT_3002",
		fmt.Sprintf("permission timeout: %s", requestID),
		ErrPermissionTimeout,
	)
}
