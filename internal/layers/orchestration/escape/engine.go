// EscapeEngine (DM-20260625-003, PR-V5.4 + DM-20260629-008 PR-B additive)
//
// 关键设计 (doc 38 §21.1, design.md §5.3.2):
//   - 整合 3 类深度限制: LoopDepthTracker + LoopBudget + CircuitBreaker
//   - 任一非 Continue → ChainedArbitrator.Arbitrate
//   - 失败降级: Evaluate error → slog.Warn + EscapeContinue (不阻塞主链路)
//   - ResumeSession 委托给 HumanArbitrator (T2 续跑入口)
//
// Evaluate 完整流程 (V5.4 简化版):
//   1. 收集决策: tracker + circuitBreakers (parallel)
//   2. 决策合并: 任一非空 → ChainedArbitrator.Arbitrate
//   3. 全部空 → EscapeContinue (正常回路)
//   4. auditLog.Record 终态 (无论是否仲裁)
//
// PR-B additive (DM-20260629-008, devrix-d7-taskcontract-unification-pr-b):
//   - pessimisticGuard 字段: PessimisticCommitGuard 可选注入 (nil = no-op)
//   - NotifyPessimistic(report): guard 评估 + 决策, 把 Pessimistic Commit
//     行为 attach 到 report (FallbackUsed + MVPArtifact + 必要 Blockage).
//     仅在 guard != nil 且 Enabled=true 时生效, 默认 0 行为变更.
package escape

import (
	"context"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// DepthChecker is the interface EscapeEngine uses to consult the
// loop-depth tracker. The default implementation is *LoopDepthTracker;
// tests may inject a panicking mock to verify error-recovery paths.
type DepthChecker interface {
	ShouldContinue(ctx LoopContext) EscapeDecision
}

// EscapeEngine is the unified entry point for all escape decisions.
//
// Wiring:
//   - tracker: LoopDepthTracker v2 (V5.1) — implements DepthChecker
//   - chain:   ChainedArbitrator LLM→Rule→Human (V5.3)
//   - cbSet:   CircuitBreakerSet 5 层 (V5.4)
//   - audit:   EscapeAuditLog (终态记录)
//   - resume:  HumanArbitrator.ResumeSession 代理入口 (T2 续跑)
//   - pessimisticGuard: PessimisticCommitGuard (PR-B, optional — nil = no-op)
type EscapeEngine struct {
	tracker          DepthChecker
	chain            *ChainedArbitrator
	cbSet            *CircuitBreakerSet
	audit            *EscapeAuditLog
	resume           *HumanArbitrator
	pessimisticGuard interfaces.PessimisticCommitGuard
}

// NewEscapeEngine constructs the engine with all components.
func NewEscapeEngine(
	tracker DepthChecker,
	chain *ChainedArbitrator,
	cbSet *CircuitBreakerSet,
	audit *EscapeAuditLog,
	resume *HumanArbitrator,
) *EscapeEngine {
	return &EscapeEngine{
		tracker: tracker,
		chain:   chain,
		cbSet:   cbSet,
		audit:   audit,
		resume:  resume,
	}
}

// Evaluate is the main escape decision entry point.
//
// Flow (design.md §5.3.2):
//   1. LoopDepthTracker.ShouldContinue → tracker decision
//   2. CircuitBreakerSet.EvaluateAll    → CB decision
//   3. Collect non-Continue signals:
//        - 0 → return Continue (正常回路)
//        - 1 → return the signal directly (硬信号优先级, 跳过 LLM)
//        - 2+ → ChainedArbitrator.Arbitrate (多源冲突, 需要 LLM 仲裁)
//   4. audit.Record 终态
//
// 失败降级: 任何内部 panic/error → slog.Warn + EscapeContinue.
func (e *EscapeEngine) Evaluate(ctx context.Context, loopCtx LoopContext) (decision EscapeDecision) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("escape_engine_panic",
				"panic", r,
				"session_id", loopCtx.SessionID,
			)
			decision = EscapeDecision{
				Action:     EscapeContinue,
				Reason:     "escape_engine_panic_recovered",
				AuditLevel: 1,
				SessionID:  loopCtx.SessionID,
				CreatedAt:  nowFunc(),
			}
		}
	}()

	// 1. LoopDepthTracker
	trackerDecision := e.tracker.ShouldContinue(loopCtx)

	// 2. CircuitBreakerSet
	cbDecision := e.cbSet.EvaluateAll(ctx, loopCtx)

	// 3. 收集非 Continue 的上游信号
	upstream := []EscapeDecision{}
	if trackerDecision.Action != EscapeContinue {
		upstream = append(upstream, trackerDecision)
	}
	if cbDecision.Action != EscapeContinue {
		upstream = append(upstream, cbDecision)
	}

	// 4. 0 信号 → 正常回路 Continue
	if len(upstream) == 0 {
		return EscapeDecision{
			Action:     EscapeContinue,
			Reason:     "all_depth_limits_passed",
			AuditLevel: 0,
			Depth:      trackerDecision.Depth,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}

	// 5. 1 信号 (单源硬信号) → 直接返回 (跳过 LLM 仲裁, 避免 LLM 覆盖硬信号)
	if len(upstream) == 1 {
		final := upstream[0]
		// Ensure Required fields are populated (defensive)
		if final.SessionID == "" {
			final.SessionID = loopCtx.SessionID
		}
		if final.CreatedAt.IsZero() {
			final.CreatedAt = nowFunc()
		}
		if e.audit != nil && final.AuditLevel > 0 {
			e.audit.Record(loopCtx, upstream, final)
		}
		return final
	}

	// 6. 2+ 信号 (多源冲突) → ChainedArbitrator 仲裁
	final := e.chain.Arbitrate(ctx, loopCtx, upstream)

	// Chain was triggered → always audit (multi-signal coordination trail).
	if e.audit != nil {
		e.audit.Record(loopCtx, upstream, final)
	}

	return final
}

// ResumeSession is the T2 entry point — delegates to HumanArbitrator.
// Returns (decision, found, error).
// found=false → 走完整 5 节点流程 (无 pending decision).
func (e *EscapeEngine) ResumeSession(sessionID string) (EscapeDecision, bool, error) {
	if e.resume == nil {
		return EscapeDecision{}, false, nil
	}
	return e.resume.ResumeSession(sessionID)
}

// AuditLog returns the audit log (for tests / observability).
func (e *EscapeEngine) AuditLog() *EscapeAuditLog {
	return e.audit
}

// LoopDepthTracker returns the underlying tracker (for tests).
// Returns nil if the engine was constructed with a non-*LoopDepthTracker DepthChecker.
func (e *EscapeEngine) LoopDepthTracker() *LoopDepthTracker {
	if t, ok := e.tracker.(*LoopDepthTracker); ok {
		return t
	}
	return nil
}

// CircuitBreakerSet returns the CB set (for tests / metrics push).
func (e *EscapeEngine) CircuitBreakerSet() *CircuitBreakerSet {
	return e.cbSet
}

// SetPessimisticGuard wires the PessimisticCommitGuard into the engine.
// PR-B additive: passing nil reverts to the no-op behavior. The bootstrap
// (internal/bootstrap/wire_coordinator.go) consults
// D7_PESSIMISTIC_COMMIT_ENABLED at startup and calls this once. The setter
// is idempotent — later calls overwrite earlier ones.
func (e *EscapeEngine) SetPessimisticGuard(g interfaces.PessimisticCommitGuard) {
	e.pessimisticGuard = g
}

// PessimisticGuard returns the currently-wired guard (for tests / metrics
// push). Returns nil when no guard has been wired (default 6.0.x path).
func (e *EscapeEngine) PessimisticGuard() interfaces.PessimisticCommitGuard {
	return e.pessimisticGuard
}

// NotifyPessimistic is the PR-B entry point called by Channel.Execute
// exits (and any consumer that has a TaskReport in hand). The method is
// safe to call with a nil guard (no-op) and a nil report (no-op). When
// the guard decides to fall back, the report is updated in place:
//
//   - report.FallbackUsed = true
//   - report.Result.Kind forced to ResultKindIndeterminate (Pessimistic path)
//     or kept as Pass (RuleBased path overrides) or set to ResultKindFailed
//     (Abort path)
//   - report.MVPArtifact populated via BuildMVPArtifact
//
// Returns the (possibly mutated) report. The original receiver is not
// guaranteed to be the same pointer (PessimisticCommitGuard implementations
// are allowed to return a new report via TaskReport.WithMVPArtifact), so
// callers must use the returned pointer.
//
// Feature Flag interaction: when the guard is disabled (the default), this
// is a pure pass-through that returns the input report unchanged.
func (e *EscapeEngine) NotifyPessimistic(
	spec *interfaces.TaskSpec,
	report *interfaces.TaskReport,
) (*interfaces.TaskReport, error) {
	if e == nil || e.pessimisticGuard == nil {
		return report, nil
	}
	if report == nil {
		return report, nil
	}

	budget := interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic)
	if spec != nil {
		budget = spec.ConvergenceBudget
	}

	ok, blockedReason, err := e.pessimisticGuard.Evaluate(spec, report, budget)
	if err != nil {
		slog.Warn("pessimistic_guard_evaluate_error",
			"trace_id", report.TraceID,
			"err", err.Error(),
		)
		return report, nil // fail-open: do not break the pipeline
	}
	if ok {
		return report, nil
	}

	policy, ruleName := e.pessimisticGuard.ResolveFallback(report)
	_ = ruleName // currently only used for telemetry; PR-C exposes via env
	_ = policy

	mvp := e.pessimisticGuard.BuildMVPArtifact(report, blockedReason)
	updated := report.
		WithFallbackUsed(true).
		WithMVPArtifact(&mvp)

	// Force Result.Kind based on the policy.
	switch policy {
	case interfaces.FallbackPessimistic:
		updated = updated.WithResult(interfaces.Result{
			Kind:       interfaces.ResultKindIndeterminate,
			Confidence: report.Result.Confidence,
			Message:    "pessimistic commit: " + blockedReason,
			At:         report.Result.At,
		})
	case interfaces.FallbackRuleBased:
		// PR-B keeps the existing Result.Kind (caller's verdict is honored
		// because the rule-based path is the "best candidate wins" semantic).
		// No-op here; PR-C will overwrite with the chosen candidate's verdict.
	case interfaces.FallbackAbort:
		updated = updated.WithResult(interfaces.Result{
			Kind:       interfaces.ResultKindFailed,
			Confidence: 0.0,
			Message:    "fallback abort: " + blockedReason,
			At:         report.Result.At,
		})
	}

	slog.Info("pessimistic_commit_emit",
		"trace_id", report.TraceID,
		"reason", blockedReason,
		"policy", policy.String(),
		"fallback_used", true,
	)
	return updated, nil
}