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
)

const (
	CodeSubagentDepthExceeded = "AGT_DEPTH_5011"
	CodeSubagentInvalidMode   = "AGT_DEPTH_5012"
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
