// Package decisionplanning — Auto-Mode Classifier (P2 interface stub, D7-S10-A50-T22).
//
// AutoModeClassifier 是 P2 stub interface — 当前 0 行实现, 触发升 P1 实施的条件:
//
//	主触发: verify_contract.deny_rate 7 天滑动 > 5%
//	次触发: devrix 真实 incident 涉及 auto-mode 误判 (任意 1 次)
//	任何触发 → 开 devrix-d2-tool-input-aware-concurrency-and-classifier-pr-d-followup Change 实施
//
// 类型命名约定 (D7 design decision D7):
//
//	ClassifierResult / AutoModeDecision / Source{Anthropic,External,RuleFallback}
//	devrix Naming Policy: 语义化名, 不带 Yolo* / LLM*
//
// 当前调用 ClassifyToolUse 必须 panic("P2 interface, not implemented; ..."), 而非返回 error —
// panic 信息在升 P1 后用于审计追溯当前哪些 path 走 auto-mode 决策.
//
// DSAFT: D7-S10-A50-T22 (DM-20260702-009 PR-D+E, 阶段 5 接口 stub).
package decisionplanning

import (
	"context"
	"fmt"
)

// AutoModeDecision enumerates the verdict produced by an AutoModeClassifier.
//
// P2 stub: only the type is defined. Real decisions (allow / ask / deny)
// are populated by a future LLM-driven implementation that materializes
// after the trigger metric (`verify_contract.deny_rate > 5%` over a
// 7-day rolling window) fires.
//
// Naming: devrix Naming Policy — semantic, no LLM/Yolo prefix.
type AutoModeDecision string

const (
	// DecisionAllow: the classifier verdict is "proceed without confirmation".
	// P2 stub: not yet producible.
	DecisionAllow AutoModeDecision = "allow"
	// DecisionAsk: the classifier verdict is "surface to the user for confirmation".
	// P2 stub: not yet producible.
	DecisionAsk AutoModeDecision = "ask"
	// DecisionDeny: the classifier verdict is "drop the call before dispatch".
	// P2 stub: not yet producible.
	DecisionDeny AutoModeDecision = "deny"
)

// Source labels the classifier path that produced a ClassifierResult.
// Useful for dashboards distinguishing anthropic / external / rule-fallback
// in P1 staging environments.
type Source string

const (
	// SourceAnthropic: classifier driven by Anthropic's side-channel API.
	SourceAnthropic Source = "anthropic"
	// SourceExternal: classifier driven by an external classifier
	// (vendored model, in-house trained, etc.).
	SourceExternal Source = "external"
	// SourceRuleFallback: classifier fell back to a rule-only path because
	// the LLM-driven path was unavailable (timeout / error / not yet wired).
	SourceRuleFallback Source = "rule-fallback"
)

// ClassifierResult is the verdict + provenance returned by an AutoModeClassifier.
//
// Decision / Reason / Source 三件套 — 不可变值对象 (D7 P5 设计原则: 不可变 + With*).
type ClassifierResult struct {
	// Decision is the allow / ask / deny verdict. P2 stub: never populated.
	Decision AutoModeDecision
	// Reason is a human-readable explanation (audit log + dashboard label).
	// P2 stub: only set to "P2 stub, not implemented" when ClassifyToolUse
	// panics and the recovery path returns a sentinel.
	Reason string
	// Source identifies the classifier that produced the verdict (Anthropic
	// / External / RuleFallback). P2 stub: only set to SourceRuleFallback
	// when the recovery path returns the sentinel.
	Source Source
}

// AutoModeClassifier is the P2 stub interface for the auto-mode security
// classifier. P2 = interface only, 0 lines of implementation; real work
// happens in the follow-up change once the trigger metric fires.
//
// The contract is intentionally minimal:
//
//  1. ClassifyToolUse consumes the projector-friendly transcript (built
//     by ToCompactBlock from the LLM's full transcript) and returns a
//     ClassifierResult.
//  2. Implementations must be safe for the FastPath hot path (sub-ms
//     rule-only fallbacks; LLM-driven paths ctx.WithTimeout(5s) hard cap).
//  3. Implementations must NEVER silently fall back to allow. The rule
//     fallback path is explicit (Source = SourceRuleFallback).
//
// DSAFT: D7-S10-A50-T22 (P2 stub, 阶段 5).
type AutoModeClassifier interface {
	ClassifyToolUse(ctx context.Context, transcript []TranscriptBlock) (ClassifierResult, error)
}

// ErrAutoModeClassifierPanic is the sentinel that callers can use to
// distinguish "stub not yet wired" from a real classifier error.
//
// Returns from the recover() at the call site (turn_adapter.go's
// TODO integration point) when ClassifyToolUse panics. The recover
// path should log the panic, emit a metric, and treat the call as
// DecisionAllow (current devrix default behavior) — NOT propagate
// the panic up the orchestrator stack.
//
// DSAFT: D7-S10-A50-T22.
var ErrAutoModeClassifierPanic = fmt.Errorf("auto-mode classifier P2 stub: ClassifyToolUse not implemented")

// panicStubClassifier is the placeholder implementation that always
// panics with ErrAutoModeClassifierPanic. It exists so callers can
// type-assert `var c AutoModeClassifier = panicStubClassifier{}` at
// compile time, but the runtime behavior is "panic on call" — which is
// what the P2 spec requires.
//
// Use:
//
//	var c AutoModeClassifier = panicStubClassifier{}
//	_ = c // OK at compile time
//	c.ClassifyToolUse(ctx, nil) // runtime panic + recover at call site
//
// DSAFT: D7-S10-A50-T22.
type panicStubClassifier struct{}

// ClassifyToolUse always panics with ErrAutoModeClassifierPanic. This is the
// P2 stub behavior — the interface contract requires "never silently fall
// back to allow", and panicking forces the call site to write a recover()
// path (which lives at the ChannelRouter TODO integration point).
//
// DSAFT: D7-S10-A50-T22.
func (panicStubClassifier) ClassifyToolUse(_ context.Context, _ []TranscriptBlock) (ClassifierResult, error) {
	panic(ErrAutoModeClassifierPanic.Error())
}
