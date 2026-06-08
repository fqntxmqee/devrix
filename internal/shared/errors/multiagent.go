package errors

import (
	"errors"
	"fmt"
)

// Multi-agent layer sentinel errors (5000-5999).
var (
	ErrAgentInvalidTransition = errors.New("agent invalid state transition")
	ErrAgentMaxChildren       = errors.New("agent max children exceeded")
	ErrAgentAlreadyTerminated = errors.New("agent already terminated")
	ErrAgentJoinNotCompleted  = errors.New("agent join before child completed")
	ErrAgentTimeout           = errors.New("agent execution timeout")
	ErrAgentInvalidConfig     = errors.New("agent invalid config")
	ErrAgentPermissionTimeout = errors.New("agent permission timeout")
	ErrAgentPermissionDenied  = errors.New("agent permission denied")
	ErrAgentContextCancelled  = errors.New("agent context cancelled")
	ErrAgentMaxTotal          = errors.New("agent max total exceeded")
)

const (
	CodeAgentInvalidTransition = "AGT_LIFECYCLE_5001"
	CodeAgentMaxChildren       = "AGT_FACTORY_5002"
	CodeAgentAlreadyTerminated = "AGT_LIFECYCLE_5003"
	CodeAgentJoinNotCompleted  = "AGT_FORK_5004"
	CodeAgentTimeout           = "AGT_LIFECYCLE_5005"
	CodeAgentInvalidConfig     = "AGT_FACTORY_5006"
	CodeAgentPermissionTimeout = "AGT_PERMISSION_5007"
	CodeAgentPermissionDenied  = "AGT_PERMISSION_5008"
	CodeAgentContextCancelled  = "AGT_CONTEXT_5009"
	CodeAgentMaxTotal          = "AGT_FACTORY_5010"
)

// NewAgentInvalidTransitionError returns an invalid state transition error.
func NewAgentInvalidTransitionError(from, to string) *SentinelError {
	return WithCode(CodeAgentInvalidTransition,
		fmt.Sprintf("非法状态转换: %s → %s", from, to), ErrAgentInvalidTransition)
}

// NewAgentMaxChildrenError returns a max children exceeded error.
func NewAgentMaxChildrenError(current, max int) *SentinelError {
	return WithCode(CodeAgentMaxChildren,
		fmt.Sprintf("子 Agent 数超限: 当前 %d, 最大 %d", current, max), ErrAgentMaxChildren)
}

// NewAgentAlreadyTerminatedError returns an already terminated error.
func NewAgentAlreadyTerminatedError(agentID string) *SentinelError {
	return WithCode(CodeAgentAlreadyTerminated,
		fmt.Sprintf("Agent %s 已终止，无法继续操作", agentID), ErrAgentAlreadyTerminated)
}

// NewAgentJoinNotCompletedError returns a join-before-complete error.
func NewAgentJoinNotCompletedError(childID string) *SentinelError {
	return WithCode(CodeAgentJoinNotCompleted,
		fmt.Sprintf("子 Agent %s 尚未完成，请先 Wait", childID), ErrAgentJoinNotCompleted)
}

// NewAgentTimeoutError returns an agent timeout error.
func NewAgentTimeoutError(agentID string, timeout string) *SentinelError {
	return WithCode(CodeAgentTimeout,
		fmt.Sprintf("Agent %s 执行超时 (timeout=%s)", agentID, timeout), ErrAgentTimeout)
}

// NewAgentInvalidConfigError returns an invalid config error.
func NewAgentInvalidConfigError(reason string) *SentinelError {
	return WithCode(CodeAgentInvalidConfig,
		fmt.Sprintf("AgentConfig 校验失败: %s", reason), ErrAgentInvalidConfig)
}

// NewAgentPermissionTimeoutError returns a permission timeout error.
func NewAgentPermissionTimeoutError(toolName string) *SentinelError {
	return WithCode(CodeAgentPermissionTimeout,
		fmt.Sprintf("工具 %s 权限确认超时（60s）", toolName), ErrAgentPermissionTimeout)
}

// NewAgentPermissionDeniedError returns a permission denied error.
func NewAgentPermissionDeniedError(toolName string) *SentinelError {
	return WithCode(CodeAgentPermissionDenied,
		fmt.Sprintf("工具 %s 权限被用户拒绝", toolName), ErrAgentPermissionDenied)
}

// NewAgentContextCancelledError returns a context cancelled error.
func NewAgentContextCancelledError(agentID string) *SentinelError {
	return WithCode(CodeAgentContextCancelled,
		fmt.Sprintf("Agent %s 上下文已取消", agentID), ErrAgentContextCancelled)
}

// NewAgentMaxTotalError returns a max total agents exceeded error.
func NewAgentMaxTotalError(current, max int) *SentinelError {
	return WithCode(CodeAgentMaxTotal,
		fmt.Sprintf("Session Agent 总数超限: 当前 %d, 最大 %d", current, max), ErrAgentMaxTotal)
}
