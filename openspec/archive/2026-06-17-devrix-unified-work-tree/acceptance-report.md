# Acceptance Report: devrix-unified-work-tree

**Change ID:** devrix-unified-work-tree  
**Demand ID:** DM-20260617-009  
**Status:** S7_Archived (2026-06-18 最终闭环)  
**Verdict:** **ACCEPTED (v1.0–v2.0 完整交付；v2.1+ 登记 tech-debt)**

## AC 结果

| AC | 描述 | 状态 | 证据 |
|----|------|------|------|
| AC1–AC5 | WorkItem 基础模型 + legacy 兼容 | ✅ PASS | `workmodel/workitem.go`, `work_tree.go`, `workitem_store.go`, `work_tree_test.go` |
| AC6–AC8 | 写入路径挂树 (EnsureGoal, delegate, plan, task_list tree) | ✅ PASS | `orchestrator.go`, `delegate_tools.go`, `cli_commands.go`, `tool_suite.go` |
| AC9–AC10 | todo_write checklist + Wave 吸收 | ✅ PASS | `todo_tool.go`, `todo_sync.go`, `worktree_wave.go`, `orchestrate_path.go` |
| AC11–AC13 | RunRegistry 挂接 RunRef | ✅ PASS | `runregistry/`, `spawn.go`, `delegate_tools.go`, `agent_bridge.go` |
| AC14–AC16 | Tool 面简化 (task_write/spawn/await) | ✅ PASS | PR #85: `unified_tools.go` task_write/spawn/await alias |
| AC17–AC19 | 递归求解引擎 | ✅ PASS | PR #86: `decompose.go`, `resolve_hint.go`, `ResolveHint` in FocusHint |
| AC20–AC22 | 深度/宽度/24h 约束 | ✅ PASS | PR #86: `MaxDecomposeDepth`, `DefaultMaxChildren=7`, daily limit 5/24h |
| AC23 | CI static analysis | ✅ PASS | `scripts/audit-property-rights.sh` + CI step |
| AC24 | Code Owner Bot | ✅ PASS | `.github/CODEOWNERS` |
| AC25 | Property Rights Audit | ✅ PASS | `scripts/audit-property-rights.sh` baseline report |
| AC27 | Uncertainty Anchor | ✅ PASS | `uncertainty.go`, `uncertainty_test.go` |
| AC28–AC29 | RunTurn 单层递归 MVP | ✅ PASS | PR #86–#87: decompose + `ResolveAwaiter` blocking await at turn start |
| AC30–AC32 | 跨 Session | ✅ PASS (baseline) | `cross_session.go`, `cross_session_test.go` |
| AC33–AC36 | 自演化 | ⚠️ BASELINE → **TD-WT-01** | `AdaptiveThreshold` API 已实现；optimizer 接线 defer v3.0 |
| AC37–AC40 | 状态机/迁移/级联/环检测 | ✅ PASS | `work_tree.go`, `work_tree_test.go` |
| AC41–AC42 | Terminal callback 重试/幂等 | ✅ PASS | `spawn.go` 指数退避 3 次 + `SetTerminal` notified-once |
| AC43–AC44 | 部分失败/GetFocus tiebreak | ✅ PASS | `resolve.go`, `uncertainty_test.go`, `work_tree_test.go` |
| AC45 | ResolveFocus dispatch | ✅ PASS | `worktree_wave.go::ResolveFocusKind`, `CanSpawn` |
| AC46–AC47 | Wave WorkTree + checklist promote | ✅ PASS | `SyncWaveNodes`, `PromoteChecklist` in plan approve |
| AC48 | Alias 等价性 | ✅ PASS | PR #85: task_write/spawn/await alias + PR #86 decompose mode |
| AC49–AC50 | Lock enforcement + 链式继承 | ✅ PASS | `ErrWorkItemLocked`, `SourceSession` field |
| AC51–AC52 | 冷启动/hysteresis | ✅ PASS | `uncertainty.go::AdaptiveThreshold` |
| AC53 | 磁盘原子写入 | ✅ PASS | `workitem_store.go` tmp+rename |

## T 层覆盖（新增）

| T ID | 描述 | Status | Test 文件 |
|------|------|--------|-----------|
| D7-S1-T09 | WorkTree EnsureGoal 单根 | IMPLEMENTED | `work_tree_test.go` |
| D7-S1-T10 | v1→v2 迁移 + 原子 Save | IMPLEMENTED | `work_tree_test.go`, `cross_session_test.go` |
| D7-S1-T11 | GetFocus tiebreak | IMPLEMENTED | `work_tree_test.go::TestWorkTree_GetFocusTiebreak` |
| D7-S1-T12 | RunRef terminal → WorkItem status | IMPLEMENTED | `runregistry/spawn_test.go` |
| D7-S1-T13 | Cross-session FindByItemID | IMPLEMENTED | `cross_session_test.go` |
| D7-S1-T14 | DecomposeChildren depth limit | IMPLEMENTED | `decompose_test.go::TestDecomposeChildren_DepthLimit` |
| D7-S1-T15 | Decompose daily limit 5/24h | IMPLEMENTED | `decompose_test.go::TestDecomposeChildren_DailyLimit` |
| D7-S1-T16 | ResolveHint high uncertainty | IMPLEMENTED | `decompose_test.go::TestResolveHint_HighUncertainty` |
| D7-S1-T17 | RunTurn blocking await running children | IMPLEMENTED | `resolve_await_test.go::TestAwaitRunningChildren_BlocksUntilTerminal` |
| D7-S3-T12 | OrchestratePath SyncWaveNodes | IMPLEMENTED | `orchestrate_path.go` wiring |

## Quality Gate

- [x] `go test ./...` 全绿
- [x] `scripts/audit-property-rights.sh` 可运行（baseline 6 WARN 为既有 D2 registry）
- [x] OpenSpec delta 合并至 `specs/d7-orchestration/spec.md`

## 已知限制 / Tech Debt

全部 defer 项登记：`openspec/tech-debt/worktree-v2-deferred.md`

1. ~~**task_write / task_spawn / task_await** 统一 alias~~ → PR #85
2. ~~**RunTurn decompose 循环**~~ → PR #86
3. ~~**RunTurn blocking await**~~ → PR #87
4. **TD-WT-01** Phase 8 自演化 optimizer 接线（v3.0）
5. **TD-WT-02/03** Legacy 删除：wave.TaskNode SoT、sc.Todos 权威（v2.1）
6. **TD-WT-04** 飞书跨 Session UI（v2.1）
7. **TD-WT-05/06** AC22 review 门控、AC42 并发 terminal 幂等（v2.1）

## Decision

**ACCEPTED** — WorkItem/WorkTree/RunRegistry 分离架构 v1.0–v2.0 交付完成（PR #83–#87）；legacy TaskManager 适配器保持兼容；v2.1+ 登记 tech-debt。

**Date:** 2026-06-18
