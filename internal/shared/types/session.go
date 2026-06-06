package types

import "time"

// SessionState represents the current state of a session
type SessionState string

const (
	SessionStateIdle              SessionState = "idle"
	SessionStateThinking          SessionState = "thinking"
	SessionStateStreaming         SessionState = "streaming"
	SessionStateToolExecuting     SessionState = "tool_executing"
	SessionStateWaitingPermission SessionState = "waiting_permission"
	SessionStateCompleted         SessionState = "completed"
	SessionStateFailed            SessionState = "failed"
)

// Session represents a conversation session (Aggregate Root)
type Session struct {
	// 标识
	SessionID string // 内部会话 ID（sess_时间戳_随机）
	RequestID string // 请求关联 ID
	AdapterID string // 所属 Adapter

	// 参与者
	UserID   string // 用户 ID（CLI 无用户概念）
	UserName string // 用户名

	// 环境
	WorkDir string // 工作目录
	Model   string // 指定模型

	// 状态
	State         SessionState // idle | thinking | streaming | tool_executing | waiting_permission | completed | failed
	CurrentAgentID string      // 当前 Agent ID

	// 生命周期
	CreatedAt    time.Time // 创建时间
	UpdatedAt    time.Time // 更新时间
	LastMessageAt time.Time // 最后消息时间

	// 上下文快照（可选持久化）
	ContextSnapshot []byte
}

// SetState transitions the session to a new state
func (s *Session) SetState(state SessionState) {
	s.State = state
	s.UpdatedAt = time.Now()
}

// IsIdle returns true if session has been idle longer than the given duration
func (s *Session) IsIdle(timeout time.Duration) bool {
	return time.Since(s.LastMessageAt) > timeout
}

// NewSession creates a new session with default values
func NewSession(sessionID, adapterID, workDir string) *Session {
	now := time.Now()
	return &Session{
		SessionID:      sessionID,
		AdapterID:      adapterID,
		WorkDir:        workDir,
		State:          SessionStateIdle,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastMessageAt:  now,
	}
}
