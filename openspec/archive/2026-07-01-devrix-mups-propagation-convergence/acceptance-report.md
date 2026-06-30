---
demand-id: DM-20260701-001
change-id: devrix-mups-propagation-convergence
status: ACCEPTED
verified-at: 2026-07-01
prs:
  - 358  # P0
  - 359  # P1+P2
---

# Acceptance Report: MUPS+WorkTree 传播闭环修复

## Summary

P0 + P1 + P2 implementation complete. Resolves the 12 RH-MUPS findings from
the 2026-07-01 D7 MUPS+WorkTree design logic review (covering the up/down
propagation chain, uncertainty divergence/convergence, and LLM-visible
acceptance criteria). 18/18 T points IMPLEMENTED, 2 PRs squash-merged to
master, all D7 orchestration packages pass `go test -race -count=1`.

The change touches three layers of the propagation stack:

1. **Convergence correctness (P0, 12 T points)** — `ReconcileUncertainty` as
   the single write point for `item.Uncertainty` (replaces naked max ratchet);
   `RollupRetries` ceiling with `SpawnEscalateHuman` upgrade path; Failed
   children enumerated in the rollup directive (not laundered into success);
   readable acceptance criteria in execute prompt; `PriorVerifyReason`
   fed back into the next round's execute prompt.
2. **Divergence controllability + de-hardcode (P1, 6 T points)** —
   `DivergenceBudget` snapshot of (children, daily, iters) injected into the
   Plan prompt; structured `StrategicPlanReject` for over-budget proposals;
   `NarrowestSchema` enforces "LLM can narrow, never widen" on deliverable
   schemas; `rollupPlanningDenylist` hoisted from Go literal to
   `i18n.RollupPlanningDenylist()`.
3. **Divergence effectiveness (P2, 3 T points)** — `DefaultChildDownlink` no
   longer blindly inherits parent scope; `ValidateChildScopes` enforces
   subset + non-empty + coverage; `ChildUncertaintyBubble` surfaces
   high-uncertainty children as a dedicated rollup directive section.

## P0 Acceptance (12 T points, RH-MUPS-01/02/03/04/10)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P0-1 `ReconcileUncertainty` 纯函数 + 表驱动单测 | PASS | `workmodel/uncertainty_reconcile.go` (NEW) + `uncertainty_reconcile_test.go` (NEW) |
| T-P0-2 `item_pipeline.go` 移除裸 max ratchet | PASS | `sessionorchestrator/item_pipeline.go` 调用 `workmodel.ReconcileUncertainty` |
| T-P0-3 `reevaluateParentAfterChild` 统一走 `ReconcileUncertainty` | PASS | `sessionorchestrator/reevaluate_parent.go` 统一 reconcile 入口 |
| T-P0-4 `WorkItemPipelineRound` 增 `RollupRetries`；`TreeEvalContext` 透传 | PASS | `workmodel/pipeline_round.go` + `sessionorchestrator/reevaluate_context.go` |
| T-P0-5 `SpawnPolicyEvaluator` rollup 分支加 `MaxRollupRetries → SpawnEscalateHuman` | PASS | `sessionorchestrator/spawn_policy_evaluator.go` |
| T-P0-6 `session_turn_loop` break 前检查未收敛 rollup parent，emit 显式结局 | PASS | `sessionorchestrator/turn_loop.go` (turn_break_fail_safe) |
| T-P0-7 rollup 故障注入测试：verify 恒 fail → 达上限 escalate，不超 loop | PASS | `sessionorchestrator/rollup_retry_injection_test.go` (NEW) |
| T-P0-8 `rollup_gate`/`rollup_verify` 读 `ChildOutcomeStats`，`Failed==Total` 禁 Completed | PASS | `sessionorchestrator/rollup_gate.go` + `rollup_verify.go` |
| T-P0-9 `buildRollupDirective` 增"失败子集"区段（不洗白） | PASS | `sessionorchestrator/rollup_directive.go` `FailedSubset:` section |
| T-P0-10 `AppendDeliverableExecuteHint` 注入 i18n 可读验收要点 | PASS | `workmodel/deliverable_execute_hint.go` + `contextengine/i18n/format_hints_deliverable.go` (NEW) |
| T-P0-11 `WorkItemExecContext` 增 `PriorVerifyReason`；inline 重试回灌 verdict.Reason | PASS | `sessionorchestrator/workitem_exec_context.go` + `item_pipeline.go` |
| T-P0-12 execute prompt 快照测试：含验收要点 + 回灌 reason | PASS | `sessionorchestrator/workitem_exec_context_test.go` (NEW, 4 snapshot tests) |

## P1 Acceptance (6 T points, RH-MUPS-07/08/11/12)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P1-1 发散上限常量集中到 `workmodel` 单一来源 | PASS | `workmodel/divergence_budget.go` (NEW) — `DefaultMaxChildrenDiv=7`, `DefaultMaxDecomposePerDay=5`, `DefaultMaxReactIters=5` |
| T-P1-2 `buildStrategicPlanUserPrompt` 注入 depth/max_depth/remaining_children/remaining_daily/parent_scope_in | PASS | `sessionorchestrator/strategic_plan_proposer.go` |
| T-P1-3 LLM 超额提案 → 结构化 reject（含 max_allowed） | PASS | `sessionorchestrator/strategic_plan_proposer.go` `StrategicPlanReject` 类型 + `applyBudgetCap` |
| T-P1-4 Plan prompt 快照测试含全部预算字段 | PASS | `sessionorchestrator/strategic_plan_proposer_test.go` 6 new tests |
| T-P1-5 schema 选择改 `NarrowestSchema(inferred, strategic)`，禁放宽 | PASS | `workmodel/deliverable.go` + 9-case test matrix `deliverable_test.go` (NEW) |
| T-P1-6 `rollupPlanningDenylist` 迁 i18n/`format_hints` | PASS | `contextengine/i18n/format_hints_planning.go` (NEW) + `sessionorchestrator/rollup_verify.go` 改用 `i18n.RollupPlanningDenylist()` |

## P2 Acceptance (3 T points, RH-MUPS-05/06/09)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P2-1 `DefaultChildDownlink` 移除"无脑继承父全量 scope" | PASS | `workmodel/child_downlink.go` — empty spec scope + bounded parent = empty child scope |
| T-P2-2 `ValidateChildScopes(parent, children)` 真子集 + 覆盖校验 | PASS | `workmodel/scope_validate.go` (NEW) + `scope_validate_test.go` (NEW) — 5-case matrix |
| T-P2-3 高不确定性子 bubble 上浮为 `ObsUncertainty` 衍生信号 | PASS | `workmodel/context_bubble_apply.go` `ChildUncertaintyBubble` + `ChildUncertaintyBubbleStatement` + `rollup_directive.go` `UncertainChildren:` section + `rollup_uncertain_test.go` (NEW) |

## Test Execution

```text
go test -race -count=1 ./internal/layers/orchestration/sessionorchestrator/...  → PASS (2.583s)
go test -race -count=1 ./internal/layers/orchestration/workmodel/...           → PASS (1.897s)
go test -race -count=1 ./internal/layers/orchestration/workmodel/notify/...     → PASS (2.575s)
go test -race -count=1 ./internal/layers/contextengine/i18n/...                 → PASS (2.071s)
```

## CI Verification

| Check | Result | Duration |
|-------|--------|----------|
| layer-lint (D1 boundary + D7 main-path) | PASS | 11s |
| unit tests (full repo) | PASS | 3m19s |
| PR #358 squash merge | OK | 1d |
| PR #359 squash merge | OK | <1h |

## Findings Closure

All 12 RH-MUPS findings closed:

| Finding | Severity | Status | T points |
|---------|----------|--------|----------|
| RH-MUPS-01 | Critical | CLOSED | T-P0-1/2 |
| RH-MUPS-02 | Critical | CLOSED | T-P0-1/3 |
| RH-MUPS-03 | Warning+ | CLOSED | T-P0-4/5/6/7 |
| RH-MUPS-04 | Warning | CLOSED | T-P0-8/9 |
| RH-MUPS-05 | Info | CLOSED | T-P2-1/2 |
| RH-MUPS-06 | Info | CLOSED | T-P2-3 |
| RH-MUPS-07 | Critical | CLOSED | T-P1-1/2/3/4 |
| RH-MUPS-08 | Warning | CLOSED | T-P1-1/2 |
| RH-MUPS-09 | Warning | CLOSED | T-P2-2 |
| RH-MUPS-10 | Critical | CLOSED | T-P0-10/11/12 |
| RH-MUPS-11 | Warning | CLOSED | T-P1-5 |
| RH-MUPS-12 | Info | CLOSED | T-P1-6 |

## DoD Checklist

- [x] P0 全部 12 任务完成；L5-MUPS-01/02/03/04/10 测试通过
- [x] `go test -race ./internal/layers/orchestration/...` 全绿
- [x] 无新增 D7→D3 直依赖；denylist 迁出 Go 源码（来自 i18n/format_hints）
- [x] 发散上限常量（children/daily/iters）来自 `workmodel` 单一来源
- [x] T 点登记 `openspec/specs/d7-orchestration/t-registry.md` (IMPLEMENTED 230→248)
- [x] 域架构文档 CHANGELOG.md 追加 1 行
- [x] S5 验收 verdict: ACCEPTED
- [x] S6-交付：PR #358 + PR #359 squash merged
- [x] S6-归档：本 acceptance-report.md + archive/ 目录就位
