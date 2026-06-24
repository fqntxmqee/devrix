// EscapeAction and EscapeDecision — escape 子包核心类型 (DM-20260625-003).
//
// PR-V5.1 (本 PR) 只声明类型 + Continue/ForceExit 2 类常量,
// 完整 6 类在 PR-V5.3 (ChainedArbitrator) 中落地。
package escape

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EscapeAction is a typed enum classifying the outcome of an escape
// evaluation. 6 classes total:
//
//   - EscapeContinue:      继续回路（未到上限）
//   - EscalateToRule:      升级到规则强制（ChainedArbitrator 内部链式中间态）
//   - EscalateToHuman:     升级到人工接管（ChainedArbitrator 内部链式中间态）
//   - EscapeForceExit:     强制退出（带 ExitReason）
//   - EscapeAbortWithAudit: 强制终止 + 完整审计（最严重）
//   - EscapePendingHuman:  中间态：等待用户响应（dev 扩展，携 PendingID）
//
// 对外 API（EscapeEngine.Evaluate 返回值）实际只暴露 4 类核心
// (Continue / PendingHuman / ForceExit / AbortWithAudit)，
// EscalateToRule / EscalateToHuman 由 ChainedArbitrator 内部消化。
type EscapeAction uint8

const (
	// EscapeContinue — depth < MaxDepth or all 3 depth-limit classes
	// return Continue. Caller should loop (re-plan / re-execute).
	EscapeContinue EscapeAction = iota

	// EscalateToRule — ChainedArbitrator 内部中间态。LLM 选 Exit →
	// Rule 还没裁决。
	EscalateToRule

	// EscalateToHuman — ChainedArbitrator 内部中间态。Rule 选 Human →
	// Human 还没裁决。
	EscalateToHuman

	// EscapeForceExit — 强制退出。兜底决策（loop depth / circuit
	// breaker open / LLM timeout / Human 10s timeout）。
	EscapeForceExit

	// EscapeAbortWithAudit — 强制终止 + 完整审计。最严重决策
	// （不可恢复失败 / 用户选择 C 取消）。
	EscapeAbortWithAudit

	// EscapePendingHuman — 中间态。Human 异步路径：session 状态
	// 已持久化，等待 SubmitUserChoice。ProcessMessage 同步返回 nil
	// （不阻塞飞书卡片）。
	EscapePendingHuman
)

// String returns the human-readable form of EscapeAction.
func (a EscapeAction) String() string {
	switch a {
	case EscapeContinue:
		return "continue"
	case EscalateToRule:
		return "escalate_to_rule"
	case EscalateToHuman:
		return "escalate_to_human"
	case EscapeForceExit:
		return "force_exit"
	case EscapeAbortWithAudit:
		return "abort_with_audit"
	case EscapePendingHuman:
		return "pending_human"
	default:
		return fmt.Sprintf("EscapeAction(%d)", uint8(a))
	}
}

// MarshalJSON serializes EscapeAction as its string form for wire output.
func (a EscapeAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON accepts the string form.
func (a *EscapeAction) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "continue":
		*a = EscapeContinue
	case "escalate_to_rule":
		*a = EscalateToRule
	case "escalate_to_human":
		*a = EscalateToHuman
	case "force_exit":
		*a = EscapeForceExit
	case "abort_with_audit":
		*a = EscapeAbortWithAudit
	case "pending_human":
		*a = EscapePendingHuman
	default:
		*a = EscapeContinue
		return fmt.Errorf("escape: unknown EscapeAction %q", s)
	}
	return nil
}

// EscapeDecision is the result of an escape evaluation. 9 fields total:
//
//   5 核心 (escape 决策必须填):
//     Action     EscapeAction
//     Reason     string             // 决策原因（人类可读）
//     AuditLevel int                // 0=无审计, 1=记录, 2=完整审计
//     Depth      int                // 当前回路深度（仅 Continue/ForceExit 有意义）
//     PendingID  string             // 仅 EscapePendingHuman 时填充
//
//   4 审计/续跑 (audit log + T2 ResumeSession):
//     ExitReason        ExitReason  // 14 类 Phase 4 映射
//     SessionID         string      // audit 持久化 key
//     CreatedAt         time.Time   // 审计时间戳
//     SourceDecisionIDs []string    // 上游决策链（audit 追溯）
type EscapeDecision struct {
	Action     EscapeAction
	Reason     string
	AuditLevel int
	Depth      int
	PendingID  string

	ExitReason        ExitReason
	SessionID         string
	CreatedAt         time.Time
	SourceDecisionIDs []string
}

// NewPendingID returns a UUID v4 string for EscapePendingHuman decisions.
// V5.3 (HumanArbitrator) will use this to register pending decisions.
func NewPendingID() string {
	return uuid.NewString()
}