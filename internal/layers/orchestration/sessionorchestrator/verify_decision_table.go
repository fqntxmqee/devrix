package sessionorchestrator

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// M4 (mups-verify-table-driven, DM-20260705-005) — Verify decision-table kernel.
//
// 12 trigger 命名函数 (5 artifact + 3 workItem overlay + 3 rollup + 1 rollup guard)
// + 2 包级决策表 var (artifactDecisionTable + rollupDecisionTable)
// + applyDecisionTable 顺序遍历。
//
// 3 verify 函数体替换为"建 ctx + applyDecisionTable"+ overlay detector 链。
// 0 行为变化承诺：13 现有测试 + 7 byte-equivalent 测试覆盖。

// verifyContext 不可变结构体 (包内未导出)，detector 只读。
type verifyContext struct {
	art      *wavescheduler.Artifact
	item     *workmodel.WorkItem // 可选
	pl       *plan.Plan          // 可选
	contract workmodel.DeliverableContract // 可选
	stats    workmodel.ChildOutcomeStats   // 可选
	id       string                        // art.TaskID 兜底 "artifact_unknown"
}

// VerdictTemplate 触发命中时构造 verdict 的模板。
// ReasonFunc 优先于 Reason（用于注入动态文案如 "execute failed: %s"）。
type VerdictTemplate struct {
	Kind                types.VerdictKind
	Confidence          float64
	Reason              string
	ReasonFunc          func(art *wavescheduler.Artifact, ctx *verifyContext) string
	IndeterminateReason string
}

// VerdictTrigger 单个 trigger 单元。
type VerdictTrigger struct {
	Name     string
	Fire     func(art *wavescheduler.Artifact, ctx *verifyContext) bool
	Template VerdictTemplate
}

// VerifyDecisionTable 有序 trigger 列表。
type VerifyDecisionTable []VerdictTrigger

// applyDecisionTable 顺序遍历 table；第一个 Fire==true 的 trigger 应用 Template 返回。
// 都不 fire → 返回 defaultVerdict(ctx) (Pass 0.9)。SourceID 由 ctx.id 注入。
func applyDecisionTable(table VerifyDecisionTable, art *wavescheduler.Artifact, ctx *verifyContext) workmodel.Verdict {
	for _, trigger := range table {
		if trigger.Fire(art, ctx) {
			v := buildVerdictFromTemplate(trigger.Template, art, ctx)
			if ctx.id != "" {
				v = v.WithSourceID(ctx.id)
			}
			return v
		}
	}
	return defaultVerdict(ctx)
}

// buildVerdictFromTemplate 从 template + art + ctx 构造 Verdict。
// ReasonFunc 优先于 Reason。
func buildVerdictFromTemplate(t VerdictTemplate, art *wavescheduler.Artifact, ctx *verifyContext) workmodel.Verdict {
	reason := t.Reason
	if t.ReasonFunc != nil {
		reason = t.ReasonFunc(art, ctx)
	}
	v := workmodel.Verdict{
		Kind:       t.Kind,
		Confidence: t.Confidence,
		Reason:     reason,
	}
	if t.IndeterminateReason != "" {
		v = v.WithIndeterminateReason(t.IndeterminateReason)
	}
	return v
}

// defaultVerdict happy path fallback (Pass 0.9 + art.Summary)。
func defaultVerdict(ctx *verifyContext) workmodel.Verdict {
	summary := ""
	if ctx.art != nil {
		summary = ctx.art.Summary
	}
	v := workmodel.Verdict{
		Kind:       types.VerdictPass,
		Confidence: 0.9,
		Reason:     summary,
	}
	if ctx.id != "" {
		v = v.WithSourceID(ctx.id)
	}
	return v
}

// ---- 5 artifact detector ----

// detectNilArtifact trigger 1: art == nil → Indeterminate("env_limited")
func detectNilArtifact(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	return art == nil
}

// detectMaxItersPartial trigger 2: Error/ExitCode + max_iters + tool_calls > 0 → Partial(0.55)
func detectMaxItersPartial(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if art == nil {
		return false
	}
	if art.Error == "" && art.ExitCode == 0 {
		return false
	}
	reason, _ := art.Metadata["stop_reason"].(string)
	if reason != "max_iters" {
		return false
	}
	calls, _ := art.Metadata["tool_calls"].(int)
	return calls > 0
}

// detectExecuteFail trigger 3: Error/ExitCode（非 max_iters+tool_calls>0）→ Fail(0.9)
func detectExecuteFail(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if art == nil {
		return false
	}
	if art.Error == "" && art.ExitCode == 0 {
		return false
	}
	// 已被 detectMaxItersPartial 接管的 case 不在此处理
	return !detectMaxItersPartial(art, ctx)
}

// detectSideEffectRolledBack trigger 4: SideEffectStatus == RolledBack → Fail(0.85)
func detectSideEffectRolledBack(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if art == nil {
		return false
	}
	return art.SideEffectStatus == types.SideEffectRolledBack
}

// detectSideEffectUncertain trigger 5: SideEffectStatus == Unknown/Inflight → Partial(0.6)
func detectSideEffectUncertain(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if art == nil {
		return false
	}
	return art.SideEffectStatus == types.SideEffectUnknown ||
		art.SideEffectStatus == types.SideEffectInflight
}

// ---- 3 workItem overlay detector ----

// detectUserGate overlay 6: user-gate phrase/regex → Partial(0.85)
func detectUserGate(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if art == nil {
		return false
	}
	return artifactAwaitingUserGate(art)
}

// detectScopeOnlyDeliverable overlay 7: ExplorationPlan + open questions + 无 file:line citation → Partial(0.8)
func detectScopeOnlyDeliverable(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if art == nil || ctx.item == nil || ctx.pl == nil {
		return false
	}
	if !workmodel.CanDecompose(ctx.item.Kind) {
		return false
	}
	if ctx.pl.Kind != plan.ExplorationPlan {
		return false
	}
	return isScopeOnlyDeliverable(art, ctx.item)
}

// detectDeliverableIncomplete overlay 8: ContractApplicable + StatusIncomplete + 前序 verdict 是 Pass → Partial(0.65)
// 组合判定：detector 只检查 Contract + Incomplete 条件；"前序 verdict 是 Pass" 由 verifyArtifactForWorkItemWithContract 函数体外层 if 控制。
func detectDeliverableIncomplete(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if !ctx.contract.ContractApplicable() {
		return false
	}
	deliverable := VerifyDeliverableContract(ctx.contract, art)
	return deliverable.Status == workmodel.DeliverableStatusIncomplete
}

// ---- 3 rollup detector ----

// detectRollupAllFailed trigger 9: stats.Total > 0 && Failed == Total → Fail(0.95)
func detectRollupAllFailed(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	return ctx.stats.Total > 0 && ctx.stats.Failed == ctx.stats.Total
}

// detectRollupMixedFailedRunning trigger 10: Total > 0 && Failed > 0 && Running > 0 → Partial(0.8)
func detectRollupMixedFailedRunning(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	return ctx.stats.Total > 0 && ctx.stats.Failed > 0 && ctx.stats.Running > 0
}

// detectRollupContractSatisfied trigger 11: RollupDeliverableContract.Status == Complete → Pass(0.9)
// 不 fire 时 default 走 rollupDefault = Fail(0.85, "rollup deliverable contract not satisfied")
// 见 rollupDecisionTable 表尾 default entry。
func detectRollupContractSatisfied(art *wavescheduler.Artifact, ctx *verifyContext) bool {
	if art == nil {
		return false
	}
	summary := strings.TrimSpace(art.Summary)
	if summary == "" {
		return false // 由 default entry 兜底
	}
	contract := workmodel.RollupDeliverableContract()
	got := workmodel.VerifyDeliverableContract(contract, summary, "")
	return got.Status == workmodel.DeliverableStatusComplete
}

// ---- 2 包级决策表 ----

// artifactDecisionTable 5 trigger；verifyArtifact 走。
// trigger 顺序：nil → max_iters+tool → execute_fail → side_effect_rolledback → side_effect_uncertain → default Pass
var artifactDecisionTable = VerifyDecisionTable{
	{
		Name: "nil-artifact",
		Fire: detectNilArtifact,
		Template: VerdictTemplate{
			Kind:                types.VerdictIndeterminate,
			Confidence:          0,
			Reason:              "missing artifact",
			IndeterminateReason: "env_limited",
		},
	},
	{
		Name: "max-iters-partial",
		Fire: detectMaxItersPartial,
		Template: VerdictTemplate{
			Kind:       types.VerdictPartial,
			Confidence: 0.55,
			Reason:     "iteration cap with partial progress",
		},
	},
	{
		Name: "execute-fail",
		Fire: detectExecuteFail,
		Template: VerdictTemplate{
			Kind:       types.VerdictFail,
			Confidence: 0.9,
			ReasonFunc: func(art *wavescheduler.Artifact, ctx *verifyContext) string {
				return fmt.Sprintf("execute failed: %s", art.Error)
			},
		},
	},
	{
		Name: "side-effect-rolledback",
		Fire: detectSideEffectRolledBack,
		Template: VerdictTemplate{
			Kind:       types.VerdictFail,
			Confidence: 0.85,
			Reason:     "side effect rolled back",
		},
	},
	{
		Name: "side-effect-uncertain",
		Fire: detectSideEffectUncertain,
		Template: VerdictTemplate{
			Kind:       types.VerdictPartial,
			Confidence: 0.6,
			Reason:     "side effect uncertain",
		},
	},
}

// rollupDecisionTable 3 trigger；verifyRollupArtifact 走。
// trigger 顺序：all_failed → mixed_running → contract_satisfied → default Fail(0.85)
//
// 注意：rollup 的 art == nil / Error / ExitCode ≠ 0 由 verifyRollupArtifact 函数体顶部 guard
// 提前 return verifyArtifact(art)（不走表，与现状 1:1 保留）。
var rollupDecisionTable = VerifyDecisionTable{
	{
		Name: "all-failed",
		Fire: detectRollupAllFailed,
		Template: VerdictTemplate{
			Kind:       types.VerdictFail,
			Confidence: 0.95,
			ReasonFunc: func(art *wavescheduler.Artifact, ctx *verifyContext) string {
				return fmt.Sprintf("all %d rollup children failed; refusing Pass", ctx.stats.Failed)
			},
		},
	},
	{
		Name: "mixed-failed-running",
		Fire: detectRollupMixedFailedRunning,
		Template: VerdictTemplate{
			Kind:       types.VerdictPartial,
			Confidence: 0.8,
			ReasonFunc: func(art *wavescheduler.Artifact, ctx *verifyContext) string {
				return fmt.Sprintf("rollup synthesized with %d failed + %d running children", ctx.stats.Failed, ctx.stats.Running)
			},
		},
	},
	{
		Name: "contract-satisfied",
		Fire: detectRollupContractSatisfied,
		Template: VerdictTemplate{
			Kind:       types.VerdictPass,
			Confidence: 0.9,
			ReasonFunc: func(art *wavescheduler.Artifact, ctx *verifyContext) string {
				return strings.TrimSpace(art.Summary)
			},
		},
	},
	// default: empty summary / contract not satisfied → Fail(0.85)
	// 用 detectRollupContractSatisfied 的 false 路径 + 兜底 default verdict
	// 但 applyDecisionTable 的 default 走 defaultVerdict (Pass 0.9)，与 rollup 期望的 Fail(0.85) 不符。
	// 解决方案：rollupDecisionTable 不在 applyDecisionTable 走 default；用显式 catch-all trigger。
	{
		Name: "rollup-default-fail",
		Fire: func(art *wavescheduler.Artifact, ctx *verifyContext) bool {
			return true // 永不失败前置 trigger 走到这里
		},
		Template: VerdictTemplate{
			Kind:       types.VerdictFail,
			Confidence: 0.85,
			ReasonFunc: func(art *wavescheduler.Artifact, ctx *verifyContext) string {
				summary := ""
				if art != nil {
					summary = strings.TrimSpace(art.Summary)
				}
				if summary == "" {
					return "rollup summary empty"
				}
				contract := workmodel.RollupDeliverableContract()
				got := workmodel.VerifyDeliverableContract(contract, summary, "")
				if got.Reason != "" {
					return got.Reason
				}
				return "rollup deliverable contract not satisfied"
			},
		},
	},
}
