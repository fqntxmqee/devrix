package errors

import "errors"

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
	return WithCode(CodeLLMUnavailable, "llm gateway unavailable", err)
}

// NewFeatureNotImplementedError returns a feature not implemented error.
func NewFeatureNotImplementedError(feature, version string) *SentinelError {
	return WithCode(CodeMemoryNotImplemented, feature+" not implemented in "+version, ErrFeatureNotImplemented)
}

// NewContextPermissionDeniedError returns a permission denied error for tool calls.
func NewContextPermissionDeniedError(toolName string) *SentinelError {
	return WithCode(CodePermissionDenied, "permission denied for tool: "+toolName, ErrPermissionDenied)
}
