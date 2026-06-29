// Escape wiring helpers (DM-20260625-003, PR-V5.5 + PR-V5.6)
//
// 关键设计 (doc 38 §21.3.4, design.md §6):
//   - buildEscapeLoopContext: 在 5 节点接线点构造 LoopContext
//   - planKindFromIntent: 将 orchtypes.IntentKind 映射为 escape.PlanKind
//   - processEscapeDecision: 统一处理 6 类 EscapeAction
//   - applyResumeSession: PR-V5.6 T2 ResumeSession 续跑入口
//
// 失败降级: Evaluate 内部 panic / error 已由 EscapeEngine 处理;
// 这里 processEscapeDecision 仅在 err path 上传递, 不重复 panic recover.
package sessionorchestrator

import (
	"context"
	"errors"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/contracts"
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
// IntentFast | IntentOrchestrate → plan.ScenarioPlan (both flow through RunSessionTurnLoop + MUPS)
func planKindFromIntent(kind orchtypes.IntentKind) escape.PlanKind {
	switch kind {
	case orchtypes.IntentCommand:
		return plan.CommitmentPlan
	case orchtypes.IntentFast, orchtypes.IntentOrchestrate:
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

// applyResumeSession is the T2 entry point (PR-V5.6, D7-S14-A50-T12).
//
// Invoked at ProcessMessage entry, AFTER buildObserveRequest (so resume
// is unaffected by LP-1 prior) and BEFORE classify (so terminal
// decisions short-circuit the 5-node pipeline).
//
// Returns:
//   - (nil, false, nil) — fall through: no engine / no pending /
//     user_continue decision / ResumeSession error / TTL expired.
//     Caller proceeds with normal ProcessMessage flow.
//   - (ch,   true,  nil) — short-circuit: terminal decision found
//     (user_accept or user_cancel). ch emits a single "complete"
//     EngineEvent and then closes. Caller returns ch and skips
//     classify + 5-node pipeline.
//   - (nil,  false, err) — error path (currently unused; reserved
//     for future fail-fast semantic).
//
// 3 层 fail-safe (defensive):
//   1. nil engine → fall through (no escape wired → behave as legacy)
//   2. ResumeSession error → slog.Warn + fall through
//   3. found=false (TTL expired or not set) → fall through
//
// sessionSpan attributes (D5 observability):
//   - escape.resume.attempted=true (always set after the call)
//   - escape.resume.decision_action=<action> (only on found=true)
//   - escape.resume.decision_pending_id=<id> (only on found=true)
//
// 3 类 terminal decision 映射 (设计稿):
//   - A user_continue → fall through (用户希望继续走完整 5 节点)
//   - B user_accept → EscapeForceExit → emit "complete" (audit already recorded
//     at SubmitUserChoice time (V5.4); resume is read-only, 不重复写 audit)
//   - C user_cancel → EscapeAbortWithAudit → emit "complete" (audit already recorded)
func (o *SessionOrchestrator) applyResumeSession(
	_ context.Context, // reserved for future audit/tracing
	req orchtypes.ProcessRequest,
	sessionSpan tracer.Span,
) (<-chan *contracts.EngineEvent, bool, error) {
	// M-1 (DM-20260625-004 review-fixes): guard against empty SessionID.
	// Empty SessionID is a contract violation (ProcessRequest constructor
	// requires non-empty), not a transient error — silently fall through
	// without triggering slog.Warn to avoid log noise / misdiagnosis.
	if req.SessionID == "" {
		if sessionSpan != nil {
			sessionSpan.SetAttributes(tracer.Attribute{
				Key: "escape.resume.attempted", Value: "false",
			})
		}
		return nil, false, nil
	}
	if o.escapeEngine == nil {
		if sessionSpan != nil {
			sessionSpan.SetAttributes(tracer.Attribute{
				Key: "escape.resume.attempted", Value: "false",
			})
		}
		return nil, false, nil
	}

	decision, found, err := o.escapeEngine.ResumeSession(req.SessionID)
	if err != nil {
		// Fail-safe 2: ResumeSession error → log + fall through.
		slog.Warn("orchestrator: escape: resume_session_error",
			"session_id", req.SessionID,
			"err", err,
		)
		if sessionSpan != nil {
			sessionSpan.SetAttributes(
				tracer.Attribute{Key: "escape.resume.attempted", Value: "true"},
				tracer.Attribute{Key: "escape.resume.decision_action", Value: "error_failsafe"},
			)
		}
		return nil, false, nil
	}
	if !found {
		// Fail-safe 3: TTL expired or no pending → fall through.
		if sessionSpan != nil {
			sessionSpan.SetAttributes(tracer.Attribute{
				Key: "escape.resume.attempted", Value: "true",
			})
		}
		return nil, false, nil
	}

	// Always record decision action + pending id on success.
	if sessionSpan != nil {
		sessionSpan.SetAttributes(
			tracer.Attribute{Key: "escape.resume.attempted", Value: "true"},
			tracer.Attribute{Key: "escape.resume.decision_action", Value: decision.Action.String()},
			tracer.Attribute{Key: "escape.resume.decision_pending_id", Value: decision.PendingID},
		)
	}

	// DM-20260629-001 PR-6 t-span-coverage (T38): emit a dedicated
	// D7_Resume_Decision_Path span around the 3 决策路径 so that
	// traces/dashboards can filter by route independently of sessionSpan.
	// The 3 paths are: A user_continue fall through / B user_accept→
	// ForceExit (terminal emit "complete") / C user_cancel→AbortWithAudit
	// (terminal emit "complete"). A is the no-span path (no decision to
	// observe); B/C emit one span each with attributes below.
	endResumeSpan := hardening.EmitResumeDecisionPath(
		context.Background(),
		req.SessionID,
		decision.Reason,
		decision.Action.String(),
		decision.AuditLevel,
		decision.Depth,
	)
	defer endResumeSpan(nil)

	// user_continue → fall through to full 5-node pipeline.
	if decision.Action == escape.EscapeContinue {
		return nil, false, nil
	}

	// Terminal decision (B=user_accept → ForceExit, C=user_cancel → AbortWithAudit):
	// emit single "complete" EngineEvent (audit already written at SubmitUserChoice time) + close channel early.
	out := make(chan *contracts.EngineEvent, 1)
	out <- &contracts.EngineEvent{
		Type:      "complete",
		Content:   resumeContentForDecision(decision),
		SessionID: req.SessionID,
		Metadata: map[string]string{
			"escape.resume":       "true",
			"escape.action":       decision.Action.String(),
			"escape.reason":       decision.Reason,
			"escape.pending_id":   decision.PendingID,
			"exit_reason_source":  "user_resume",
		},
	}
	close(out)
	return out, true, nil
}

// resumeContentForDecision converts an EscapeDecision into a human-readable
// Chinese text message shown in the final "complete" EngineEvent.
//
// 6 类 EscapeAction 全部覆盖:
//   - EscapeContinue:      "（用户选择继续完整流程）"
//   - EscapeForceExit:     "（用户接受当前结果）"
//   - EscapeAbortWithAudit:"（用户取消当前任务）"
//   - EscapePendingHuman:  "（等待用户响应超时）" (兜底)
//   - EscalateToRule/Human:"（需要进一步决策）" (兜底)
//
// 设计: 保持内容简短, 让飞书卡片 / CLI 终端能直接显示。
func resumeContentForDecision(d escape.EscapeDecision) string {
	switch d.Action {
	case escape.EscapeContinue:
		return "（用户选择继续完整流程）"
	case escape.EscapeForceExit:
		return "（用户接受当前结果）"
	case escape.EscapeAbortWithAudit:
		return "（用户取消当前任务）"
	case escape.EscapePendingHuman:
		return "（等待用户响应超时）"
	case escape.EscalateToRule, escape.EscalateToHuman:
		return "（需要进一步决策）"
	default:
		return "（会话终止）"
	}
}
