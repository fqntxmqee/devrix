// Escape wiring helpers (DM-20260625-003, PR-V5.5)
//
// 关键设计 (doc 38 §21.3.4, design.md §6):
//   - buildEscapeLoopContext: 在 5 节点接线点构造 LoopContext
//   - planKindFromIntent: 将 orchtypes.IntentKind 映射为 escape.PlanKind
//   - processEscapeDecision: 统一处理 6 类 EscapeAction
//
// 失败降级: Evaluate 内部 panic / error 已由 EscapeEngine 处理;
// 这里 processEscapeDecision 仅在 err path 上传递, 不重复 panic recover.
package sessionorchestrator

import (
	"errors"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// buildEscapeLoopContext constructs the escape.LoopContext for wiring points.
//
// Parameters:
//   - sessionID: from ProcessRequest.SessionID
//   - kind: current PlanKind (1b 前: intent.Kind mapped to planKind)
//   - failureCriterion: from verdict.Reason or path error (empty if not applicable)
//
// Note: PlanKindSwitchCount is tracked internally by PlanKindSwitchPolicy
// (V5.2), not via LoopContext fields.
func (o *SessionOrchestrator) buildEscapeLoopContext(sessionID string, kind escape.PlanKind, failureCriterion string) escape.LoopContext {
	return escape.LoopContext{
		SessionID:        sessionID,
		PlanKind:         kind,
		ObservationKind:  0, // 0 = unspecified (not exposed at this level)
		FailureCriterion: failureCriterion,
	}
}

// planKindFromIntent maps orchtypes.IntentKind to escape.PlanKind.
//
// IntentSkip → 0 (no plan)
// IntentCommand → plan.CommitmentPlan
// IntentFast → plan.ExplorationPlan
// IntentOrchestrate → plan.ScenarioPlan
func planKindFromIntent(kind orchtypes.IntentKind) escape.PlanKind {
	switch kind {
	case orchtypes.IntentCommand:
		return plan.CommitmentPlan
	case orchtypes.IntentFast:
		return plan.ExplorationPlan
	case orchtypes.IntentOrchestrate:
		return plan.ScenarioPlan
	default:
		return 0 // IntentSkip / unknown
	}
}

// processEscapeDecision converts an EscapeDecision to (terminate, augmentedErr).
//
// Returns:
//   - terminate=true  → caller should return augmentedErr (or baseErr if augmentedErr is nil)
//   - terminate=false → caller should continue (EscapeContinue)
//   - augmentedErr    → error wrapped with the escape reason, or baseErr if no augmentation needed
//
// 处理 6 类 EscapeAction:
//   - EscapeContinue       → continue, augmentedErr=baseErr
//   - EscalateTo*          → terminate, augmentedErr=join(baseErr, "decision.Reason + unhandled escalation")
//   - EscapePendingHuman   → terminate, augmentedErr=baseErr (Human 异步, session 状态已持久化)
//   - EscapeForceExit      → terminate, augmentedErr=baseErr
//   - EscapeAbortWithAudit → terminate, augmentedErr=baseErr
//
// 设计要点: 透传 augmentedErr 给 caller, 避免静默吞错 (S4-Gate review C-1 修复).
func (o *SessionOrchestrator) processEscapeDecision(decision escape.EscapeDecision, baseErr error) (bool, error) {
	switch decision.Action {
	case escape.EscapeContinue:
		return false, baseErr
	case escape.EscalateToRule, escape.EscalateToHuman:
		// ChainedArbitrator 链式裁决后已收敛; 兜底为 terminate, augment error 透传
		if baseErr != nil {
			return true, errors.Join(baseErr, errors.New(decision.Reason+"_unhandled_escalation"))
		}
		return true, errors.New(decision.Reason + "_unhandled_escalation")
	case escape.EscapePendingHuman:
		// Human 异步路径: session 状态已持久化, 等下次 ProcessMessage 续跑
		return true, baseErr
	case escape.EscapeForceExit, escape.EscapeAbortWithAudit:
		return true, baseErr
	default:
		return true, baseErr
	}
}

// escapeErr wraps an EscapeDecision reason as a Go error.
//
// Used at wiring points to convert a terminate decision into a returnable
// error without leaking the decision shape to upstream callers.
func escapeErr(reason string) error {
	return errors.New("orchestrator: escape: " + reason)
}
