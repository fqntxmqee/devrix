# Tech Debt: Unified Work Tree v2.1+ Deferred Items

**TD ID:** TD-WT-DEF
**Status:** PARTIAL CLOSED (TD-WT-02/03 closed DM-20260619-005; TD-WT-01/05/06 **WorkItem Pipeline path CLOSED** v0.6.0; legacy RunTurn path gaps remain)
**Severity:** Low–Medium（功能已可用；遗留为演进与清理债）
**Created:** 2026-06-18
**Owner:** —（待指派）
**Linked Change:** `devrix-unified-work-tree` (DM-20260617-009)
**Related:** PR #83–#87（v1.0–v2.0 核心交付）；`openspec/archive/2026-06-17-devrix-unified-work-tree/version-roadmap.md`

---

## 1. 债务描述

DM-20260617-009 于 2026-06-18 完成 v1.0–v2.0 核心交付（WorkItem/WorkTree/RunRegistry 分离、统一工具 alias、RunTurn resolve/decompose/blocking await）。以下项按 `version-roadmap.md` 有意 defer 至后续里程碑，登记为技术债务以便跟踪。

## 2. 债务清单

| TD 子项 | 来源 AC / Phase | 描述 | 目标版本 | 优先级 |
|---------|----------------|------|----------|--------|
| **TD-WT-01** | AC33–AC36 / Phase 8 | **自演化 optimizer 接线** — `AdaptiveThreshold` 已接入 WorkItem Pipeline `DefaultTreeEvalContext` + `ResolveHint`（per-user via `ProcessRequest.UserID`）；RunTurn 路径与 Session 历史采集仍 OPEN | v3.0 | P2 |
| **TD-WT-02** | v2.0 T5.5 | **删除 wave.TaskNode 持久化 SoT** — Wave 仍持有独立 TaskGraph 投影；WorkTree 已是语义 SoT，TaskNode 应退化为 dispatch 投影 | v2.1 | P1 |
| **TD-WT-03** | v2.0 Deprecated | **sc.Todos 权威降级** — `todo_write` 已写 WorkTree checklist ephemeral 节点；SessionContext `sc.Todos` 仍可作为读投影，不应再作为写入 SoT | v2.1 | P1 |
| **TD-WT-04** | Phase 7 / v2.1 | **飞书跨 Session UI** — `cross_session.go` baseline 只读查询已有；卡片展示「上次 N 个 task 完成/未完成，是否继续」未实现 | v2.1 | P2 |
| **TD-WT-05** | AC22 | **Decompose 超限人工 review 门控** — WorkItem Pipeline：`SpawnEscalateHuman` → verify 子项 + `/task review approve` + `human_review` 事件；legacy `decompose.go` Err-only 路径仍 OPEN | v2.1 | P2 |
| **TD-WT-06** | AC42 | **并发 terminal parent re-resolve 幂等** — `ReevaluateParentAfterChild` per-parent mutex + 单测（WorkItem Pipeline path CLOSED） | v2.1 | P2 |

## 3. 现状证据

| 子项 | 已实现 baseline | 缺口 |
|------|----------------|------|
| TD-WT-01 | `uncertainty.go::AdaptiveThreshold` + WorkItem Pipeline 接线 | RunTurn 路径 + Session 历史采集、optimizer 循环 |
| TD-WT-02 | `SyncWaveNodes` + `WaveNodesFromSubtree` 投影 | TaskNode 不再独立 SoT；磁盘持久化路径仍待 audit（PARTIAL） |
| TD-WT-03 | `TodoWriteBackend` → WorkTree checklist；`sc.Todos` Deprecated | 需确认无写入路径将 `sc.Todos` 作权威 SoT |
| TD-WT-04 | `cross_session_test.go` FindByItemID | D1 Feishu card 无跨 session 摘要 UI |
| TD-WT-05 | WorkItem Pipeline：`spawn_apply` review 子项 + CLI `/task review approve` | legacy decompose daily limit Err-only |
| TD-WT-06 | `resolve.go` parent mutex + `resolve_concurrency_test.go` | — |

## 4. 解除条件

### TD-WT-01（自演化）
- [x] `AdaptiveThreshold` 接入 WorkItem Pipeline decompose 阈值（`DefaultTreeEvalContext` + UserID）
- [ ] Session 级 decompose/spawn 结果 metrics 持久化
- [ ] `ResolveHint` / RunTurn legacy 路径 fully wired
- [ ] 10+ Session 数据验证 hysteresis 不震荡

### TD-WT-02 / TD-WT-03（Legacy 清理）
- [x] Wave 调度从 WorkTree 投影 TaskNode（`orchestrate_path` + `WaveNodesFromSubtree`）
- [x] `sc.Todos` 标记 Deprecated
- [ ] `scripts/audit-property-rights.sh` 无 WorkTree 产权 WARN（全量 audit 待后续 change）

### TD-WT-04（跨 Session UI）
- [ ] D1 Feishu adapter 读取 `QueryWorkPlan(historical_session_id)`
- [ ] 卡片模板 + E2E 验收

### TD-WT-05 / TD-WT-06（约束硬化）
- [x] Daily limit 超限 → review WorkItem + `/task review approve` + `human_review` 事件（WorkItem Pipeline）
- [x] Parent re-eval mutex 单测覆盖并发 terminal

## 5. 关联 PR（已完成部分）

| PR | 内容 |
|----|------|
| #83 | WorkItem/WorkTree/RunRegistry 核心 |
| #84 | 归档冲突标记 hotfix |
| #85 | task_write/spawn/await 统一 alias + FocusHint |
| #86 | decompose + ResolveHint + depth/daily limits |
| #87 | RunTurn blocking await (`ResolveAwaiter`) |

## 6. 建议后续 Change ID

- `devrix-worktree-legacy-cleanup` — TD-WT-02, TD-WT-03
- `devrix-worktree-cross-session-ui` — TD-WT-04
- `devrix-worktree-self-evolving` — TD-WT-01（依赖数据基线）
