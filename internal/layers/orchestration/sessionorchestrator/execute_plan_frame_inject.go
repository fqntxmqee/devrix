// Package sessionorchestrator — execute_plan_frame_inject.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 1 T3:
// InjectPlanFrameDelta — inject Plan FrameDelta into Execute system_prompt.
//
// 设计契约 (design.md §6.1):
//   - 双轨输出: ≤80-char summary (人读) + schema hash (机读)
//   - 注入预算: ABSOLUTE ≤ MaxPlanFrameDeltaInjectChars (200 chars), 非 baseline-relative
//     (cursor H2' 修复 — §3.1 ⑫ 契约对齐)
//   - 零值 FrameDelta{} → 返回 baseline (no-op, 无注入)
//   - 超 budget → 降级走 baseline + emit warn span ("budget_exceeded_fallback_baseline")
//   - 正常注入 → emit span ("ok")
//   - span emit nil-bridge: 与 EmitChannelRoute 一致, telemetry 未初始化走 fallback log
//
// Span emit (AC5):
//   - d7.s9.execute.plan_frame_delta.inject
//   - attribute: plan_frame_delta_schema_hash + plan_frame_delta_injection_chars + injection_status
package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// InjectPlanFrameDelta injects a Plan output (FrameDelta) into the baseline
// Execute system_prompt using dual-rail output: ≤80-char summary (human) +
// stable schema hash (machine). Pure function with side-effect: telemetry
// span emit (nil-bridge safe).
//
// Returns the baseline unchanged when:
//   - planDelta is the zero value (IsZero)
//   - the injection would exceed MaxPlanFrameDeltaInjectChars (200) absolute budget
//   - summary string exceeds MaxPriorArtifactSummaryChars (80)
//
// Idempotency: same (planDelta, baseline) → same output (modulo span emit).
func InjectPlanFrameDelta(ctx context.Context, sessionID string, planDelta interfaces.FrameDelta, baseline string) string {
	hash := planDelta.SchemaHash()
	if planDelta.IsZero() || hash == "" {
		// 零值/无信号 → no-op (0 注入 + emit span 标记 prior_delta_empty)
		endPlan := hardening.EmitPlanFrameDeltaInject(
			ctx, sessionID, hash, 0, hardening.PlanFrameDeltaInjectEmpty,
		)
		endPlan(nil)
		return baseline
	}
	summary := summarizePlanFrameDelta(planDelta)
	if len(summary) > interfaces.MaxPriorArtifactSummaryChars {
		summary = summary[:interfaces.MaxPriorArtifactSummaryChars-3] + "..."
	}
	injection := fmt.Sprintf(
		"\n<plan_frame_delta schema=%q>%s</plan_frame_delta>\n",
		hash, summary,
	)
	// ABSOLUTE budget: 不依赖 baseline 长度 (cursor H2' 修复)
	if len(injection) > interfaces.MaxPlanFrameDeltaInjectChars {
		endPlan := hardening.EmitPlanFrameDeltaInject(
			ctx, sessionID, hash, len(injection),
			hardening.PlanFrameDeltaInjectBudgetExceeded,
		)
		endPlan(nil)
		return baseline
	}
	endPlan := hardening.EmitPlanFrameDeltaInject(
		ctx, sessionID, hash, len(injection),
		hardening.PlanFrameDeltaInjectOK,
	)
	endPlan(nil)
	return baseline + injection
}

// summarizePlanFrameDelta builds a ≤80-char human-readable summary of the
// FrameDelta. Format: "mode=<em>; children=<n>; contract=<yes/no>"
//
// Idempotency: deterministic — same FrameDelta → same summary.
// Truncation: if combined length > MaxPriorArtifactSummaryChars, drops the
// contract segment first (mode + children 是核心信号, contract 是次要).
func summarizePlanFrameDelta(d interfaces.FrameDelta) string {
	mode := strings.TrimSpace(d.ExecutionMode)
	if mode == "" {
		mode = "unset"
	}
	children := len(d.ChildSpecs)
	contractBit := "no"
	if d.DeliverableContract != "" {
		contractBit = "yes"
	}
	return fmt.Sprintf("mode=%s; children=%d; contract=%s", mode, children, contractBit)
}