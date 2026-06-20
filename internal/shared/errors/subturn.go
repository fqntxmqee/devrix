package errors

import (
	"errors"
	"fmt"
)

// Sub-turn (sub-agent recursion) sentinel errors (5011-5019).
var (
	// ErrSubagentDepthExceeded signals that a sub-agent request would
	// exceed the configured MaxSubagentDepth. Callers should retry with
	// mode=brief to reduce context size, or restructure the workflow.
	ErrSubagentDepthExceeded = errors.New("subagent: recursion depth exceeded; use mode=brief to reduce context size")

	// ErrSubagentInvalidMode signals an unknown or empty SubAgentMode value
	// on a SubTurnRequest. Empty is allowed and resolves via Cfg.DefaultMode,
	// but an explicit unknown value is rejected to surface typos early.
	ErrSubagentInvalidMode = errors.New("subagent: invalid mode (must be brief|fork|full)")

	// ErrSubagentStreamError DM-20260620-003 (AC2) — sub-agent stream
	// emitted an "error" event whose code is not in metadata. Wrap with
	// this sentinel so callers can errors.Is(err, ErrSubagentStreamError)
	// for differentiation from depth/mode errors.
	ErrSubagentStreamError = errors.New("subagent: stream emitted error event")

	// ErrSubagentStreamClosed DM-20260620-003 (AC2) — sub-agent stream
	// ended without a complete event. Indicates LLM stream truncation or
	// unexpected channel close.
	ErrSubagentStreamClosed = errors.New("subagent: stream closed without complete event")
)

const (
	CodeSubagentDepthExceeded = "AGT_DEPTH_5011"
	CodeSubagentInvalidMode   = "AGT_DEPTH_5012"
	CodeSubagentStreamError   = "AGT_STREAM_5013"
	CodeSubagentStreamClosed  = "AGT_STREAM_5014"
)

// NewSubagentDepthExceededError returns a depth-exceeded error that includes
// the current depth, configured max, and a hint to use mode=brief.
func NewSubagentDepthExceededError(current, max int) *SentinelError {
	return WithCode(CodeSubagentDepthExceeded,
		fmt.Sprintf("子 Agent 递归深度超限: 当前 %d, 最大 %d (hint: use mode=brief to reduce context size)", current, max),
		ErrSubagentDepthExceeded)
}

// NewSubagentInvalidModeError returns an invalid-mode error.
func NewSubagentInvalidModeError(mode string) *SentinelError {
	return WithCode(CodeSubagentInvalidMode,
		fmt.Sprintf("子 Agent 模式非法: %q (must be brief|fork|full)", mode),
		ErrSubagentInvalidMode)
}

// NewSubagentStreamError wraps a sub-agent stream error event content
// with the AGT_STREAM_5013 sentinel so callers retain error type info.
func NewSubagentStreamError(content string) *SentinelError {
	return WithCode(CodeSubagentStreamError,
		fmt.Sprintf("subagent stream error: %s", content),
		ErrSubagentStreamError)
}

// NewSubagentStreamClosedError returns a typed error for the case where
// the sub-agent stream closed without emitting a complete event.
func NewSubagentStreamClosedError() *SentinelError {
	return WithCode(CodeSubagentStreamClosed,
		"subagent stream closed without complete event",
		ErrSubagentStreamClosed)
}
