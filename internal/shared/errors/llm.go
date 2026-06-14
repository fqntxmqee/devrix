package errors

import (
	"errors"
	"fmt"
	"strings"
)

// LLM layer sentinel errors.
var (
	ErrProviderUnavailable     = errors.New("llm provider unavailable")
	ErrCircuitOpen             = errors.New("llm circuit breaker open")
	ErrLLMTimeout              = errors.New("llm request timeout")
	ErrLLMAuthFailed           = errors.New("llm authentication failed")
	ErrTokenBudgetExceeded     = errors.New("llm token budget exceeded")
	ErrLLMParseError           = errors.New("llm response parse error")
	ErrUnsupportedProvider     = errors.New("unsupported llm provider")
	ErrUnsupportedModel        = errors.New("unsupported llm model")
	// ErrObservabilityRequired — D3-X-A02-F02 FailFastOnObsNil (v1.1 F4).
	// Returned by llmbridge.WireFromConfig when the observability bridge
	// is nil; callers MUST fail-fast instead of silent fallback (R3 P0 #8).
	ErrObservabilityRequired   = errors.New("observability bridge is required for llm gateway wiring")
)

const (
	CodeLLMProviderUnavailable = "LLM_PROVIDER_1001"
	CodeLLMCircuitOpen         = "LLM_CIRCUIT_1002"
	CodeLLMTimeout             = "LLM_TIMEOUT_1003"
	CodeLLMAuthFailed          = "LLM_AUTH_1004"
	CodeLLMTokenBudgetExceeded = "LLM_TOKEN_1005"
	CodeLLMParseError          = "LLM_PARSE_1006"
	CodeLLMUnsupportedProvider = "LLM_UNSUPPORTED_1007"
	CodeLLMUnsupportedModel    = "LLM_UNSUPPORTED_1008"
)

// LLMError is a typed LLM gateway error with a stable code.
type LLMError struct {
	Code    string
	Message string
	Err     error
}

func (e *LLMError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *LLMError) Unwrap() error {
	return e.Err
}

// FormatLLMError unwraps the full error chain for logging and tracing.
func FormatLLMError(err error) string {
	if err == nil {
		return ""
	}
	var parts []string
	seen := make(map[string]struct{})
	for e := err; e != nil; e = errors.Unwrap(e) {
		msg := strings.TrimSpace(e.Error())
		if msg == "" {
			continue
		}
		if _, dup := seen[msg]; dup {
			continue
		}
		seen[msg] = struct{}{}
		parts = append(parts, msg)
	}
	return strings.Join(parts, ": ")
}

// IsRetryable reports whether the error may be retried.
func IsRetryable(err error) bool {
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		switch llmErr.Code {
		case CodeLLMTimeout, CodeLLMProviderUnavailable, CodeLLMParseError:
			return true
		default:
			return false
		}
	}
	var sentinel *SentinelError
	if errors.As(err, &sentinel) {
		switch sentinel.Code {
		case CodeLLMTimeout, CodeLLMProviderUnavailable, CodeLLMParseError:
			return true
		default:
			return false
		}
	}
	return err != nil
}

func newLLMError(code, message string, err error) *LLMError {
	return &LLMError{Code: code, Message: message, Err: err}
}

// NewProviderUnavailableError returns a provider unavailable error.
func NewProviderUnavailableError(err error) *LLMError {
	return newLLMError(CodeLLMProviderUnavailable, "llm provider unavailable", err)
}

// NewCircuitOpenError returns a circuit breaker open error.
func NewCircuitOpenError(provider string) *LLMError {
	return newLLMError(CodeLLMCircuitOpen, fmt.Sprintf("llm circuit open for provider %s", provider), ErrCircuitOpen)
}

// NewLLMTimeoutError returns a timeout error.
func NewLLMTimeoutError(err error) *LLMError {
	return newLLMError(CodeLLMTimeout, "llm request timeout", err)
}

// NewLLMAuthFailedError returns an authentication error.
func NewLLMAuthFailedError(err error) *LLMError {
	return newLLMError(CodeLLMAuthFailed, "llm authentication failed", err)
}

// NewTokenBudgetExceededError returns a token budget exceeded error.
func NewTokenBudgetExceededError(count, budget int) *LLMError {
	return newLLMError(
		CodeLLMTokenBudgetExceeded,
		fmt.Sprintf("token count %d exceeds budget %d", count, budget),
		ErrTokenBudgetExceeded,
	)
}

// NewLLMParseError returns a response parse error.
func NewLLMParseError(err error) *LLMError {
	return newLLMError(CodeLLMParseError, "llm response parse error", err)
}

// NewUnsupportedProviderError returns an unsupported provider error.
func NewUnsupportedProviderError(provider string) *LLMError {
	return newLLMError(
		CodeLLMUnsupportedProvider,
		fmt.Sprintf("unsupported llm provider: %s", provider),
		ErrUnsupportedProvider,
	)
}

// NewUnsupportedModelError returns an unsupported model error.
func NewUnsupportedModelError(model string) *LLMError {
	return newLLMError(
		CodeLLMUnsupportedModel,
		fmt.Sprintf("unsupported llm model: %s", model),
		ErrUnsupportedModel,
	)
}
