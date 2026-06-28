package llmgateway

import (
	"errors"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// APIError represents a typed LLM provider HTTP error with closed-set classification.
//
// DM-20260628-001 (devrix-api-error-classification) — V4 layer:
//   - Status: HTTP status code (preserved for callers that read it directly)
//   - Message: human-readable provider message (preserved for SanitizeForUser)
//   - Code: closed-set APIErrorCode (new in V4; auto-mapped from Status by NewAPIError)
//   - Cause: underlying provider error, accessible via Unwrap()
//
// Backward compat (AC6): existing callers reading Status / Message / Unwrap
// see no behavior change.
type APIError struct {
	Status  int
	Message string
	Code    sharederrors.APIErrorCode
	Cause   error
}

// Error implements the error interface. Message takes priority; if empty,
// falls back to Cause.Error() (consistent with SentinelError.Error()).
func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Code.String()
}

// Unwrap returns Cause so errors.Is / errors.As traverse into the provider error.
func (e *APIError) Unwrap() error {
	return e.Cause
}

// APICode implements sharederrors.APICodeProvider so sharederrors.Code() can
// extract the closed-set code without import cycle (sharederrors ↔ llmgateway).
func (e *APIError) APICode() sharederrors.APIErrorCode {
	return e.Code
}

// NewAPIError creates an APIError with Code auto-mapped from HTTP status.
//
// Code is derived via sharederrors.NewAPIErrorCodeFromStatus(status); callers
// needing provider-specific codes (e.g. Anthropic media_too_large) should set
// the Code field directly after construction.
func NewAPIError(status int, message string) *APIError {
	return &APIError{
		Status:  status,
		Message: message,
		Code:    sharederrors.NewAPIErrorCodeFromStatus(status),
	}
}

// NewAPIErrorWithCause creates an APIError with the given underlying cause.
func NewAPIErrorWithCause(status int, message string, cause error) *APIError {
	apiErr := NewAPIError(status, message)
	apiErr.Cause = cause
	return apiErr
}

// IsAPICode reports whether err is (or wraps) an *APIError with the given code.
func IsAPICode(err error, code sharederrors.APIErrorCode) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == code
	}
	return false
}
